package games

import (
	"context"
	"errors"

	"github.com/Jd-aristizabal/backend/internal/shared"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Solo los identificadores de esta lista determinan nombres y filtros SQL.
func source(game string) (string, string, error) {
	switch game {
	case "pixel-sprint":
		return "pixel_sprint_scores", "TRUE", nil
	case "orbit-match", "pulse-orbit":
		return "arcade_scores", "game_id='" + game + "'", nil
	default:
		return "", "", shared.NotFound
	}
}

func (r *SubmitScore) ValidateGame(game string) error {
	if _, _, err := source(game); err != nil {
		return err
	}
	if game == "pixel-sprint" {
		return r.Validate()
	}
	if r.Score < 0 || r.MaxCombo < 0 {
		return shared.Invalid("score o combo negativo")
	}
	if err := r.validateName(); err != nil {
		return err
	}
	if r.PlayerName == nil {
		return shared.Invalid("player_name es obligatorio")
	}
	if game == "orbit-match" {
		pairs := r.Level - 1
		if pairs < 0 || pairs > 6 || r.MaxCombo > pairs || r.DurationSeconds < 1 || r.DurationSeconds > 60 {
			return shared.Invalid("Partida de Orbit Match fuera de rango")
		}
		if pairs < 6 && r.DurationSeconds != 60 {
			return shared.Invalid("Una partida incompleta de Orbit Match dura 60 segundos")
		}
		score := r.Score
		if pairs == 6 {
			score -= (60 - r.DurationSeconds) * 10
		}
		if !memoryScore(pairs, r.MaxCombo, score) {
			return shared.Invalid("Puntuación, parejas y racha incompatibles")
		}
		return nil
	}
	if r.DurationSeconds < 39 || r.DurationSeconds > 41 || r.Score > 240000 || r.Score%100 != 0 || r.MaxCombo > 300 || r.Level < 1 || r.Level > 8 {
		return shared.Invalid("Partida de Pulse Orbit fuera de rango")
	}
	if r.Score == 0 {
		if r.MaxCombo != 0 || r.Level != 1 {
			return shared.Invalid("Partida sin aciertos incompatible")
		}
		return nil
	}
	if r.MaxCombo < 1 {
		return shared.Invalid("Racha incompatible con score")
	}
	minimum := 0
	for i := 0; i < r.MaxCombo; i++ {
		minimum += 100 * min(4, 1+i/4)
	}
	for hits := r.MaxCombo; hits <= 300; hits++ {
		if min(8, 1+hits/4) == r.Level && r.Score >= minimum+(hits-r.MaxCombo)*100 && r.Score <= hits*200*min(4, 1+(r.MaxCombo-1)/4) {
			return nil
		}
	}
	return shared.Invalid("Puntuación, nivel y racha incompatibles")
}

// Enumerar las particiones de hasta seis parejas para validar las rachas.
func memoryScore(pairs, maxCombo, score int) bool {
	var visit func(int, int, int) bool
	visit = func(left, longest, total int) bool {
		if left == 0 {
			return longest == maxCombo && total == score
		}
		for streak := 1; streak <= left; streak++ {
			if visit(left-streak, max(longest, streak), total+150*streak+25*streak*(streak+1)/2) {
				return true
			}
		}
		return false
	}
	return visit(pairs, 0, 0)
}

func (s *Service) SubmitGame(ctx context.Context, client uuid.UUID, game string, r SubmitScore) (Score, error) {
	if err := r.ValidateGame(game); err != nil {
		return Score{}, err
	}
	table, filter, _ := source(game)
	id := uuid.New()
	if r.SubmissionID != nil {
		id = *r.SubmissionID
	}
	names, values := "id,client_id,score,max_combo,level,duration_seconds,player_name", "$1,$2,$3,$4,$5,$6,$7"
	args := []any{id, client, r.Score, r.MaxCombo, r.Level, r.DurationSeconds, r.PlayerName}
	if game != "pixel-sprint" {
		names += ",game_id"
		values += ",$8"
		args = append(args, game)
	}
	saved, err := scan(s.db.QueryRow(ctx, "INSERT INTO "+table+"("+names+") VALUES("+values+") ON CONFLICT(id) DO NOTHING RETURNING "+columns, args...))
	if !errors.Is(err, pgx.ErrNoRows) {
		return saved, err
	}
	previous, err := scan(s.db.QueryRow(ctx, "SELECT "+columns+" FROM "+table+" WHERE id=$1 AND client_id=$2 AND "+filter, id, client))
	if errors.Is(err, pgx.ErrNoRows) {
		return Score{}, shared.Conflict
	}
	if err != nil {
		return Score{}, err
	}
	sameName := previous.PlayerName == nil && r.PlayerName == nil || previous.PlayerName != nil && r.PlayerName != nil && *previous.PlayerName == *r.PlayerName
	if previous.Score != r.Score || previous.MaxCombo != r.MaxCombo || previous.Level != r.Level || previous.DurationSeconds != r.DurationSeconds || !sameName {
		return Score{}, shared.Conflict
	}
	return previous, nil
}

func (s *Service) LeaderboardGame(ctx context.Context, game string, limit int) ([]Score, error) {
	table, filter, err := source(game)
	if err != nil {
		return nil, err
	}
	// La última identidad publicada también identifica récords anteriores sin apodo.
	query := `SELECT b.id,b.score,b.max_combo,b.level,b.duration_seconds,coalesce(n.player_name,b.player_name),b.created_at
 FROM (SELECT DISTINCT ON(client_id) * FROM ` + table + ` WHERE ` + filter + ` ORDER BY client_id,score DESC,created_at DESC,id) b
 LEFT JOIN (SELECT DISTINCT ON(client_id) client_id,player_name FROM ` + table + ` WHERE ` + filter + ` AND player_name IS NOT NULL ORDER BY client_id,created_at DESC,id DESC) n USING(client_id)
 ORDER BY b.score DESC,b.created_at,b.id LIMIT $1`
	rows, err := s.db.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	return readScores(rows)
}

func (s *Service) MeGame(ctx context.Context, client uuid.UUID, game string, limit, offset int) (Personal, error) {
	p := Personal{Scores: []Score{}}
	table, filter, err := source(game)
	if err != nil {
		return p, err
	}
	if err = s.db.QueryRow(ctx, "SELECT coalesce(max(score),0),count(*) FROM "+table+" WHERE client_id=$1 AND "+filter, client).Scan(&p.BestScore, &p.GamesPlayed); err != nil {
		return p, err
	}
	rows, err := s.db.Query(ctx, "SELECT "+columns+" FROM "+table+" WHERE client_id=$1 AND "+filter+" ORDER BY created_at DESC,id LIMIT $2 OFFSET $3", client, limit, offset)
	if err != nil {
		return p, err
	}
	p.Scores, err = readScores(rows)
	return p, err
}
