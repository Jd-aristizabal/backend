package games

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Jd-aristizabal/backend/internal/shared"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SubmitScore struct {
	Score           int        `json:"score"`
	MaxCombo        int        `json:"max_combo"`
	Level           int        `json:"level"`
	DurationSeconds int        `json:"duration_seconds"`
	PlayerName      *string    `json:"player_name"`
	SubmissionID    *uuid.UUID `json:"submission_id,omitempty"`
}

func (r *SubmitScore) UnmarshalJSON(b []byte) error {
	var raw struct {
		Score           *int       `json:"score"`
		MaxCombo        *int       `json:"max_combo"`
		Level           *int       `json:"level"`
		DurationSeconds *int       `json:"duration_seconds"`
		PlayerName      *string    `json:"player_name"`
		SubmissionID    *uuid.UUID `json:"submission_id"`
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return err
	}
	if raw.Score == nil || raw.MaxCombo == nil || raw.Level == nil || raw.DurationSeconds == nil {
		return shared.Invalid("score, max_combo, level y duration_seconds son obligatorios")
	}
	*r = SubmitScore{Score: *raw.Score, MaxCombo: *raw.MaxCombo, Level: *raw.Level, DurationSeconds: *raw.DurationSeconds, PlayerName: raw.PlayerName, SubmissionID: raw.SubmissionID}
	return nil
}

type Score struct {
	ID              uuid.UUID `json:"id"`
	Score           int       `json:"score"`
	MaxCombo        int       `json:"max_combo"`
	Level           int       `json:"level"`
	DurationSeconds int       `json:"duration_seconds"`
	PlayerName      *string   `json:"player_name"`
	CreatedAt       time.Time `json:"created_at"`
}
type Personal struct {
	BestScore   int     `json:"best_score"`
	GamesPlayed int     `json:"games_played"`
	Scores      []Score `json:"scores"`
}
type Service struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Service { return &Service{db} }
func (r *SubmitScore) Validate() error {
	if r.DurationSeconds < 29 || r.DurationSeconds > 31 {
		return shared.Invalid("Pixel Sprint dura 30 segundos")
	}
	if r.Score < 0 || r.Score > 150000 || r.Score%100 != 0 {
		return shared.Invalid("score fuera de rango o incompatible con el juego")
	}
	if r.MaxCombo < 0 || r.MaxCombo > 300 || r.Level < 1 || r.Level > 8 {
		return shared.Invalid("combo o nivel fuera de rango")
	}
	if r.Score == 0 {
		if r.MaxCombo != 0 || r.Level != 1 {
			return shared.Invalid("Una partida sin aciertos debe tener combo 0 y nivel 1")
		}
	} else {
		if r.MaxCombo < 1 || r.MaxCombo*100 > r.Score {
			return shared.Invalid("combo incompatible con score")
		}
		// Buscar un número de aciertos compatible con el nivel, la racha y las reglas públicas.
		possible := false
		streakMinimum := 0
		for hit := 0; hit < r.MaxCombo; hit++ {
			streakMinimum += 100 * min(5, 1+hit/5)
		}
		for hits := 1; hits <= 300; hits++ {
			if min(8, 1+hits/7) != r.Level || hits < r.MaxCombo {
				continue
			}
			if r.Score >= streakMinimum+(hits-r.MaxCombo)*100 && r.Score <= hits*100*min(5, 1+(r.MaxCombo-1)/5) {
				possible = true
				break
			}
		}
		if !possible {
			return shared.Invalid("score, combo y nivel incompatibles")
		}
	}
	return r.validateName()
}
func (r *SubmitScore) validateName() error {
	if r.SubmissionID != nil && *r.SubmissionID == uuid.Nil {
		return shared.Invalid("submission_id inválido")
	}
	if r.PlayerName != nil {
		v := strings.TrimSpace(*r.PlayerName)
		if utf8.RuneCountInString(v) > 40 {
			return shared.Invalid("player_name demasiado largo")
		}
		for _, c := range v {
			if unicode.IsControl(c) {
				return shared.Invalid("player_name inválido")
			}
		}
		if v == "" {
			r.PlayerName = nil
		} else {
			v = strings.Join(strings.Fields(v), " ")
			if utf8.RuneCountInString(v) < 2 {
				return shared.Invalid("player_name debe tener al menos 2 caracteres")
			}
			r.PlayerName = &v
		}
	}
	return nil
}

const columns = "id,score,max_combo,level,duration_seconds,player_name,created_at"

func scan(row pgx.Row) (Score, error) {
	var s Score
	err := row.Scan(&s.ID, &s.Score, &s.MaxCombo, &s.Level, &s.DurationSeconds, &s.PlayerName, &s.CreatedAt)
	return s, err
}
func (s *Service) Submit(ctx context.Context, client uuid.UUID, r SubmitScore) (Score, error) {
	return s.SubmitGame(ctx, client, "pixel-sprint", r)
}
func readScores(rows pgx.Rows) ([]Score, error) {
	defer rows.Close()
	scores := []Score{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		scores = append(scores, v)
	}
	return scores, rows.Err()
}
func (s *Service) Leaderboard(ctx context.Context, limit int) ([]Score, error) {
	return s.LeaderboardGame(ctx, "pixel-sprint", limit)
}
func (s *Service) Me(ctx context.Context, client uuid.UUID, limit, offset int) (Personal, error) {
	return s.MeGame(ctx, client, "pixel-sprint", limit, offset)
}
