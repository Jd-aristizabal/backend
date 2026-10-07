package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Jd-aristizabal/backend/internal/config"
	mw "github.com/Jd-aristizabal/backend/internal/middleware"
	"github.com/Jd-aristizabal/backend/internal/modules/chatbot"
	"github.com/Jd-aristizabal/backend/migrations"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type testProvider struct {
	mu       sync.Mutex
	messages []chatbot.Message
	fail     bool
}

func (p *testProvider) Chat(ctx context.Context, r chatbot.ChatRequest) (chatbot.ChatResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.messages = r.Messages
	if p.fail {
		return chatbot.ChatResponse{}, context.DeadlineExceeded
	}
	return chatbot.ChatResponse{Message: "Johan usa SvelteKit y TypeScript."}, nil
}
func setup(t *testing.T) (http.Handler, *pgxpool.Pool, *testProvider) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL no configurada")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal("PostgreSQL de pruebas no disponible")
	}
	schema := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal("URL de pruebas inválida")
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.MaxConns = 10
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close(); _, _ = admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE"); _ = admin.Close(ctx) })
	up, err := migrations.Files.ReadFile("000001_initial.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, string(up)); err != nil {
		t.Fatal(err)
	}
	p := &testProvider{}
	c := config.Config{Origins: []string{"https://jd-aristizabal.vercel.app"}, AITimeout: time.Second, MaxTokens: 400, ChatDailyLimit: 1000}
	handler, _, err := New(c, db, slog.New(slog.NewJSONHandler(io.Discard, nil)), p)
	if err != nil {
		t.Fatal(err)
	}
	return handler, db, p
}
func request(t *testing.T, h http.Handler, method, path, client string, body any, status int) map[string]any {
	t.Helper()
	var b []byte
	if body != nil {
		var err error
		b, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	r.RemoteAddr = "127.0.0.1:1234"
	if client != "" {
		r.Header.Set("X-Client-ID", client)
	}
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != status {
		t.Fatalf("%s %s: esperado %d, recibido %d: %s", method, path, status, w.Code, w.Body.String())
	}
	if status == 204 {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func TestFocusPersistence(t *testing.T) {
	h, db, _ := setup(t)
	client, other := uuid.NewString(), uuid.NewString()
	task := request(t, h, "POST", "/api/v1/focus/tasks", client, map[string]any{"title": "  Diseñar una idea  ", "priority": "alta"}, 201)["data"].(map[string]any)
	id := task["id"].(string)
	if task["title"] != "Diseñar una idea" {
		t.Fatal("normalización fallida")
	}
	request(t, h, "PATCH", "/api/v1/focus/tasks/"+id, other, map[string]any{"completed": true}, 404)
	done := request(t, h, "PATCH", "/api/v1/focus/tasks/"+id, client, map[string]any{"completed": true}, 200)["data"].(map[string]any)
	if done["completed_at"] == nil {
		t.Fatal("fecha completada ausente")
	}
	request(t, h, "POST", "/api/v1/focus/sessions", other, map[string]any{"task_id": id, "duration_seconds": 60}, 404)
	session := request(t, h, "POST", "/api/v1/focus/sessions", client, map[string]any{"task_id": id, "duration_seconds": 60}, 201)["data"].(map[string]any)
	sid := session["id"].(string)
	request(t, h, "POST", "/api/v1/focus/sessions", client, map[string]any{"duration_seconds": 60}, 409)
	request(t, h, "PATCH", "/api/v1/focus/sessions/"+sid+"/complete", client, nil, 409)
	if _, err := db.Exec(context.Background(), "UPDATE focus_sessions SET started_at=now()-interval '61 seconds' WHERE id=$1", sid); err != nil {
		t.Fatal(err)
	}
	request(t, h, "PATCH", "/api/v1/focus/sessions/"+sid+"/complete", client, nil, 200)
	request(t, h, "PATCH", "/api/v1/focus/sessions/"+sid+"/complete", client, nil, 200)
	request(t, h, "PATCH", "/api/v1/focus/sessions/"+sid+"/cancel", client, nil, 409)
	stats := request(t, h, "GET", "/api/v1/focus/stats", client, nil, 200)["data"].(map[string]any)
	if stats["tasks_completed"] != float64(1) || stats["sessions_completed"] != float64(1) || stats["focused_minutes"] != float64(1) || len(stats["last_7_days"].([]any)) != 7 {
		t.Fatal(stats)
	}
	request(t, h, "DELETE", "/api/v1/focus/tasks/"+id, other, nil, 404)
	request(t, h, "DELETE", "/api/v1/focus/tasks/"+id, client, nil, 204)
	history := request(t, h, "GET", "/api/v1/focus/sessions", client, nil, 200)["data"].([]any)[0].(map[string]any)
	if history["task_id"] != nil || history["task_title"] != "Diseñar una idea" {
		t.Fatal("historial perdido al borrar tarea")
	}
	next := request(t, h, "POST", "/api/v1/focus/sessions", client, map[string]any{"duration_seconds": 60}, 201)["data"].(map[string]any)
	request(t, h, "PATCH", "/api/v1/focus/sessions/"+next["id"].(string)+"/cancel", client, nil, 200)
	request(t, h, "GET", "/api/v1/focus/tasks?limit=0", client, nil, 422)
	request(t, h, "GET", "/api/v1/focus/tasks", "", nil, 422)
}
func TestLeaderboard(t *testing.T) {
	h, _, _ := setup(t)
	client, other := uuid.NewString(), uuid.NewString()
	body := map[string]any{"score": 100, "max_combo": 1, "level": 1, "duration_seconds": 30, "player_name": "Johan"}
	request(t, h, "POST", "/api/v1/games/pixel-sprint/scores", client, body, 201)
	body["score"] = 900
	body["max_combo"] = 7
	body["level"] = 2
	request(t, h, "POST", "/api/v1/games/pixel-sprint/scores", client, body, 201)
	body["score"] = 200
	body["max_combo"] = 2
	body["level"] = 1
	request(t, h, "POST", "/api/v1/games/pixel-sprint/scores", other, body, 201)
	leaders := request(t, h, "GET", "/api/v1/games/pixel-sprint/leaderboard", "", nil, 200)["data"].([]any)
	if len(leaders) != 2 || leaders[0].(map[string]any)["score"] != float64(900) {
		t.Fatal(leaders)
	}
	if _, exposed := leaders[0].(map[string]any)["client_id"]; exposed {
		t.Fatal("client_id publicado")
	}
	me := request(t, h, "GET", "/api/v1/games/pixel-sprint/me", client, nil, 200)["data"].(map[string]any)
	if me["best_score"] != float64(900) || me["games_played"] != float64(2) {
		t.Fatal(me)
	}
	request(t, h, "POST", "/api/v1/games/pixel-sprint/scores", client, map[string]any{"duration_seconds": 30, "level": 1}, 400)
	body["score"] = -100
	request(t, h, "POST", "/api/v1/games/pixel-sprint/scores", client, body, 422)
}
func TestChatPersistence(t *testing.T) {
	h, db, p := setup(t)
	client, other := uuid.NewString(), uuid.NewString()
	reply := request(t, h, "POST", "/api/v1/chat", client, map[string]any{"message": "¿Qué tecnologías usa Johan?"}, 200)["data"].(map[string]any)
	id := reply["conversation_id"].(string)
	request(t, h, "POST", "/api/v1/chat", other, map[string]any{"message": "Hola", "conversation_id": id}, 404)
	request(t, h, "POST", "/api/v1/chat", client, map[string]any{"message": "¿Y su contacto?", "conversation_id": id}, 200)
	p.mu.Lock()
	if len(p.messages) != 4 || p.messages[0].Role != "system" || !strings.Contains(p.messages[0].Content, "jdaristizabal8725@gmail.com") {
		t.Fatal("contexto perdido")
	}
	p.fail = true
	p.mu.Unlock()
	request(t, h, "POST", "/api/v1/chat", client, map[string]any{"message": "otra pregunta", "conversation_id": id}, 503)
	history := request(t, h, "GET", "/api/v1/chat/conversations/"+id+"/messages", client, nil, 200)["data"].([]any)
	if len(history) != 4 {
		t.Fatal("mensajes fallidos persistidos")
	}
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(context.Background(), "SELECT id FROM chat_conversations WHERE id=$1 FOR UPDATE", id); err != nil {
		t.Fatal(err)
	}
	request(t, h, "POST", "/api/v1/chat", client, map[string]any{"message": "pregunta concurrente", "conversation_id": id}, 409)
	_ = tx.Rollback(context.Background())
	request(t, h, "DELETE", "/api/v1/chat/conversations/"+id, other, nil, 404)
	request(t, h, "DELETE", "/api/v1/chat/conversations/"+id, client, nil, 204)
	var messages int
	_ = db.QueryRow(context.Background(), "SELECT count(*) FROM chat_messages WHERE conversation_id=$1", id).Scan(&messages)
	if messages != 0 {
		t.Fatal("historial sin borrar")
	}
}
func TestReadinessAndMigrationRollback(t *testing.T) {
	h, db, _ := setup(t)
	request(t, h, "GET", "/health", "", nil, 200)
	request(t, h, "GET", "/ready", "", nil, 200)
	request(t, h, "GET", "/api/v1/portfolio/profile", "", nil, 200)
	down, _ := migrations.Files.ReadFile("000001_initial.down.sql")
	if _, err := db.Exec(context.Background(), string(down)); err != nil {
		t.Fatal(err)
	}
	up, _ := migrations.Files.ReadFile("000001_initial.up.sql")
	if _, err := db.Exec(context.Background(), string(up)); err != nil {
		t.Fatal(err)
	}
	db.Close()
	request(t, h, "GET", "/ready", "", nil, 503)
	request(t, h, "GET", "/health", "", nil, 200)
}
func TestScoreRateLimit(t *testing.T) {
	h, _, _ := setup(t)
	client := uuid.NewString()
	body := map[string]any{"score": 0, "max_combo": 0, "level": 1, "duration_seconds": 30}
	for i := 0; i < 6; i++ {
		request(t, h, "POST", "/api/v1/games/pixel-sprint/scores", client, body, 201)
	}
	request(t, h, "POST", "/api/v1/games/pixel-sprint/scores", client, body, 429)
}

func TestDistributedLimiter(t *testing.T) {
	_, db, _ := setup(t)
	first, second := mw.Limiter{DB: db}, mw.Limiter{DB: db}
	var wg sync.WaitGroup
	var mu sync.Mutex
	accepted := 0
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			limiter := &first
			if i%2 == 1 {
				limiter = &second
			}
			ok, retry, err := limiter.Allow(context.Background(), []mw.Budget{{Key: "shared-budget", Limit: 2, Seconds: 86400}})
			if err != nil {
				t.Error(err)
				return
			}
			if !ok && retry < 1 {
				t.Error("Retry-After inválido")
			}
			if ok {
				mu.Lock()
				accepted++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if accepted != 2 {
		t.Fatalf("presupuesto compartido aceptó %d solicitudes", accepted)
	}
}
