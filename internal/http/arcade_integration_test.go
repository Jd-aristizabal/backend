package httpapi

import (
	"context"
	"github.com/Jd-aristizabal/backend/migrations"
	"github.com/google/uuid"
	"testing"
)

func TestArcadeMigrationRollback(t *testing.T) {
	_, db, _ := setup(t)
	down, _ := migrations.Files.ReadFile("000002_arcade_scores.down.sql")
	if _, err := db.Exec(context.Background(), string(down)); err != nil {
		t.Fatal(err)
	}
	up, _ := migrations.Files.ReadFile("000002_arcade_scores.up.sql")
	if _, err := db.Exec(context.Background(), string(up)); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(context.Background(), "SELECT count(*) FROM arcade_scores").Scan(&count); err != nil {
		t.Fatal(err)
	}
}

func TestNamedArcadePersistenceAndRetries(t *testing.T) {
	h, _, _ := setup(t)
	for _, game := range []string{"pixel-sprint", "orbit-match", "pulse-orbit"} {
		t.Run(game, func(t *testing.T) {
			client, id := uuid.NewString(), uuid.NewString()
			duration, score := 30, 100
			if game == "orbit-match" {
				duration, score = 60, 175
			}
			if game == "pulse-orbit" {
				duration, score = 40, 200
			}
			level := 1
			if game == "orbit-match" {
				level = 2
			}
			body := map[string]any{"score": score, "max_combo": 1, "level": level, "duration_seconds": duration, "player_name": "  Johan   A  ", "submission_id": id}
			base := "/api/v1/games/" + game
			for i := 0; i < 2; i++ {
				row := request(t, h, "POST", base+"/scores", client, body, 201)["data"].(map[string]any)
				if row["id"] != id || row["player_name"] != "Johan A" {
					t.Fatal(row)
				}
			}
			me := request(t, h, "GET", base+"/me", client, nil, 200)["data"].(map[string]any)
			if me["games_played"] != float64(1) || me["best_score"] != float64(score) {
				t.Fatal(me)
			}
			empty := request(t, h, "GET", base+"/me", uuid.NewString(), nil, 200)["data"].(map[string]any)
			if empty["games_played"] != float64(0) {
				t.Fatal("Historial ajeno expuesto")
			}
			rows := request(t, h, "GET", base+"/leaderboard", "", nil, 200)["data"].([]any)
			if len(rows) != 1 || rows[0].(map[string]any)["player_name"] != "Johan A" {
				t.Fatal(rows)
			}
			body["player_name"] = "Otro"
			request(t, h, "POST", base+"/scores", client, body, 409)
			request(t, h, "POST", base+"/scores", uuid.NewString(), body, 409)
		})
	}
	request(t, h, "GET", "/api/v1/games/unknown/leaderboard", "", nil, 404)
}

func TestLeaderboardNamesPreviousAnonymousRecord(t *testing.T) {
	h, _, _ := setup(t)
	client := uuid.NewString()
	base := "/api/v1/games/pixel-sprint"
	body := map[string]any{"score": 200, "max_combo": 2, "level": 1, "duration_seconds": 30}
	request(t, h, "POST", base+"/scores", client, body, 201)
	body["score"], body["max_combo"], body["player_name"] = 100, 1, "Johan"
	request(t, h, "POST", base+"/scores", client, body, 201)
	rows := request(t, h, "GET", base+"/leaderboard", "", nil, 200)["data"].([]any)
	if len(rows) != 1 {
		t.Fatal(rows)
	}
	row := rows[0].(map[string]any)
	if row["score"] != float64(200) || row["player_name"] != "Johan" {
		t.Fatal(row)
	}
}
