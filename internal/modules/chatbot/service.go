package chatbot

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Jd-aristizabal/backend/internal/shared"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Request struct {
	Message        string     `json:"message"`
	ConversationID *uuid.UUID `json:"conversation_id"`
}
type Response struct {
	Message        string    `json:"message"`
	ConversationID uuid.UUID `json:"conversation_id"`
}
type Conversation struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
type Service struct {
	db        *pgxpool.Pool
	provider  AIProvider
	knowledge string
	timeout   time.Duration
	tokens    int
}

func New(db *pgxpool.Pool, p AIProvider, knowledge string, timeout time.Duration, tokens int) *Service {
	return &Service{db, p, knowledge, timeout, tokens}
}
func (s *Service) Create(ctx context.Context, client uuid.UUID) (Conversation, error) {
	var c Conversation
	err := s.db.QueryRow(ctx, "INSERT INTO chat_conversations(id,client_id) VALUES($1,$2) RETURNING id,created_at,updated_at", uuid.New(), client).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}
func (s *Service) Chat(ctx context.Context, client uuid.UUID, r Request) (Response, error) {
	message, err := ValidateMessage(r.Message)
	if err != nil {
		return Response{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Response{}, err
	}
	defer tx.Rollback(context.Background())
	id := uuid.New()
	if r.ConversationID != nil {
		if *r.ConversationID == uuid.Nil {
			return Response{}, shared.Invalid("conversation_id inválido")
		}
		id = *r.ConversationID
		var owned uuid.UUID
		err = tx.QueryRow(ctx, "SELECT id FROM chat_conversations WHERE id=$1 AND client_id=$2 FOR UPDATE NOWAIT", id, client).Scan(&owned)
		if err != nil {
			var code interface{ SQLState() string }
			if errors.As(err, &code) && code.SQLState() == "55P03" {
				return Response{}, shared.Conflict
			}
			return Response{}, shared.DatabaseError(err)
		}
	} else {
		if _, err = tx.Exec(ctx, "INSERT INTO chat_conversations(id,client_id) VALUES($1,$2)", id, client); err != nil {
			return Response{}, err
		}
	}
	rows, err := tx.Query(ctx, "SELECT role,left(content,1000) FROM (SELECT role,content,sequence FROM chat_messages WHERE conversation_id=$1 ORDER BY sequence DESC LIMIT 12) recent ORDER BY sequence", id)
	if err != nil {
		return Response{}, err
	}
	history := []Message{}
	for rows.Next() {
		var m Message
		if err = rows.Scan(&m.Role, &m.Content); err != nil {
			rows.Close()
			return Response{}, err
		}
		history = append(history, m)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return Response{}, err
	}
	answer, refused := RefuseInjection(message)
	if !refused {
		result, err := s.provider.Chat(ctx, ChatRequest{Messages: BuildContext(s.knowledge, history, message), MaxTokens: s.tokens})
		if err != nil {
			return Response{}, shared.Unavailable
		}
		answer = result.Message
	}
	answer = strings.TrimSpace(answer)
	if answer == "" || utf8.RuneCountInString(answer) > 8000 {
		return Response{}, shared.Unavailable
	}
	if _, err = tx.Exec(ctx, "INSERT INTO chat_messages(id,conversation_id,role,content) VALUES($1,$2,'user',$3),($4,$2,'assistant',$5)", uuid.New(), id, message, uuid.New(), answer); err != nil {
		return Response{}, err
	}
	if _, err = tx.Exec(ctx, "UPDATE chat_conversations SET updated_at=now() WHERE id=$1", id); err != nil {
		return Response{}, err
	}
	// Retención acotada por conversación; solo se envían los últimos doce mensajes.
	if _, err = tx.Exec(ctx, "DELETE FROM chat_messages WHERE conversation_id=$1 AND sequence NOT IN (SELECT sequence FROM chat_messages WHERE conversation_id=$1 ORDER BY sequence DESC LIMIT 100)", id); err != nil {
		return Response{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Response{}, err
	}
	return Response{answer, id}, nil
}
func (s *Service) History(ctx context.Context, client, id uuid.UUID, limit, offset int) ([]Message, error) {
	var owner uuid.UUID
	err := s.db.QueryRow(ctx, "SELECT id FROM chat_conversations WHERE id=$1 AND client_id=$2", id, client).Scan(&owner)
	if err != nil {
		return nil, shared.DatabaseError(err)
	}
	rows, err := s.db.Query(ctx, "SELECT role,content FROM chat_messages WHERE conversation_id=$1 ORDER BY sequence LIMIT $2 OFFSET $3", id, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		if err = rows.Scan(&m.Role, &m.Content); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (s *Service) Delete(ctx context.Context, client, id uuid.UUID) error {
	tag, err := s.db.Exec(ctx, "DELETE FROM chat_conversations WHERE id=$1 AND client_id=$2", id, client)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return shared.NotFound
	}
	return nil
}
