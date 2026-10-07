package focus

import (
	"context"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Jd-aristizabal/backend/internal/shared"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Task struct {
	ID          uuid.UUID  `json:"id"`
	Title       string     `json:"title"`
	Priority    string     `json:"priority"`
	Completed   bool       `json:"completed"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at"`
}
type CreateTask struct {
	Title    string `json:"title"`
	Priority string `json:"priority"`
}
type PatchTask struct {
	Title     *string `json:"title"`
	Priority  *string `json:"priority"`
	Completed *bool   `json:"completed"`
}
type Session struct {
	ID              uuid.UUID  `json:"id"`
	TaskID          *uuid.UUID `json:"task_id"`
	TaskTitle       string     `json:"task_title"`
	DurationSeconds int        `json:"duration_seconds"`
	Status          string     `json:"status"`
	StartedAt       time.Time  `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at"`
	CreatedAt       time.Time  `json:"created_at"`
}
type CreateSession struct {
	TaskID          *uuid.UUID `json:"task_id"`
	DurationSeconds int        `json:"duration_seconds"`
}
type Day struct {
	Date              string  `json:"date"`
	TasksCompleted    int     `json:"tasks_completed"`
	FocusedMinutes    float64 `json:"focused_minutes"`
	SessionsCompleted int     `json:"sessions_completed"`
}
type Stats struct {
	TasksCompleted    int     `json:"tasks_completed"`
	TasksTotal        int     `json:"tasks_total"`
	FocusedMinutes    float64 `json:"focused_minutes"`
	SessionsCompleted int     `json:"sessions_completed"`
	TimeZone          string  `json:"time_zone"`
	Days              []Day   `json:"last_7_days"`
}
type Service struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Service { return &Service{db} }
func validTitle(s string) bool {
	if utf8.RuneCountInString(s) < 1 || utf8.RuneCountInString(s) > 120 {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func validPriority(s string) bool { return s == "alta" || s == "media" || s == "baja" }
func (r *CreateTask) Validate() error {
	r.Title = strings.TrimSpace(r.Title)
	if !validTitle(r.Title) {
		return shared.Invalid("title debe contener entre 1 y 120 caracteres")
	}
	if !validPriority(r.Priority) {
		return shared.Invalid("priority debe ser alta, media o baja")
	}
	return nil
}
func (r *PatchTask) Validate() error {
	if r.Title == nil && r.Priority == nil && r.Completed == nil {
		return shared.Invalid("Incluye al menos un campo")
	}
	if r.Title != nil {
		v := strings.TrimSpace(*r.Title)
		r.Title = &v
		if !validTitle(v) {
			return shared.Invalid("title inválido")
		}
	}
	if r.Priority != nil && !validPriority(*r.Priority) {
		return shared.Invalid("priority inválida")
	}
	return nil
}
func (r CreateSession) Validate() error {
	if r.DurationSeconds < 60 || r.DurationSeconds > 7200 {
		return shared.Invalid("duration_seconds debe estar entre 60 y 7200")
	}
	if r.TaskID != nil && *r.TaskID == uuid.Nil {
		return shared.Invalid("task_id inválido")
	}
	return nil
}

const taskColumns = "id,title,priority,completed,created_at,updated_at,completed_at"

func scanTask(row pgx.Row) (Task, error) {
	var t Task
	err := row.Scan(&t.ID, &t.Title, &t.Priority, &t.Completed, &t.CreatedAt, &t.UpdatedAt, &t.CompletedAt)
	return t, shared.DatabaseError(err)
}
func (s *Service) CreateTask(ctx context.Context, client uuid.UUID, r CreateTask) (Task, error) {
	if err := r.Validate(); err != nil {
		return Task{}, err
	}
	return scanTask(s.db.QueryRow(ctx, "INSERT INTO focus_tasks(id,client_id,title,priority) VALUES($1,$2,$3,$4) RETURNING "+taskColumns, uuid.New(), client, r.Title, r.Priority))
}
func (s *Service) Tasks(ctx context.Context, client uuid.UUID, limit, offset int) ([]Task, error) {
	rows, err := s.db.Query(ctx, "SELECT "+taskColumns+" FROM focus_tasks WHERE client_id=$1 ORDER BY created_at DESC,id LIMIT $2 OFFSET $3", client, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Task{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}
func (s *Service) UpdateTask(ctx context.Context, client, id uuid.UUID, r PatchTask) (Task, error) {
	if err := r.Validate(); err != nil {
		return Task{}, err
	}
	return scanTask(s.db.QueryRow(ctx, `UPDATE focus_tasks SET title=coalesce($3,title),priority=coalesce($4,priority),
 completed_at=CASE WHEN $5::boolean IS NULL THEN completed_at WHEN $5 THEN coalesce(completed_at,now()) ELSE NULL END,
 completed=coalesce($5,completed),updated_at=now() WHERE id=$1 AND client_id=$2 RETURNING `+taskColumns, id, client, r.Title, r.Priority, r.Completed))
}
func (s *Service) DeleteTask(ctx context.Context, client, id uuid.UUID) error {
	tag, err := s.db.Exec(ctx, "DELETE FROM focus_tasks WHERE id=$1 AND client_id=$2", id, client)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return shared.NotFound
	}
	return nil
}

const sessionColumns = "id,task_id,task_title,duration_seconds,status,started_at,finished_at,created_at"

func scanSession(row pgx.Row) (Session, error) {
	var s Session
	err := row.Scan(&s.ID, &s.TaskID, &s.TaskTitle, &s.DurationSeconds, &s.Status, &s.StartedAt, &s.FinishedAt, &s.CreatedAt)
	return s, shared.DatabaseError(err)
}
func (s *Service) StartSession(ctx context.Context, client uuid.UUID, r CreateSession) (Session, error) {
	if err := r.Validate(); err != nil {
		return Session{}, err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback(ctx)
	title := "Concentración libre"
	if r.TaskID != nil {
		err = tx.QueryRow(ctx, "SELECT title FROM focus_tasks WHERE id=$1 AND client_id=$2 FOR KEY SHARE", r.TaskID, client).Scan(&title)
		if err != nil {
			return Session{}, shared.DatabaseError(err)
		}
	}
	session, err := scanSession(tx.QueryRow(ctx, "INSERT INTO focus_sessions(id,client_id,task_id,task_title,duration_seconds) VALUES($1,$2,$3,$4,$5) RETURNING "+sessionColumns, uuid.New(), client, r.TaskID, title, r.DurationSeconds))
	if err != nil {
		return session, err
	}
	return session, tx.Commit(ctx)
}
func (s *Service) Sessions(ctx context.Context, client uuid.UUID, limit, offset int) ([]Session, error) {
	rows, err := s.db.Query(ctx, "SELECT "+sessionColumns+" FROM focus_sessions WHERE client_id=$1 ORDER BY started_at DESC,id LIMIT $2 OFFSET $3", client, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Session{}
	for rows.Next() {
		v, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
func (s *Service) FinishSession(ctx context.Context, client, id uuid.UUID, status string) (Session, error) {
	if status != "completed" && status != "cancelled" {
		return Session{}, shared.Invalid("status inválido")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback(ctx)
	session, err := scanSession(tx.QueryRow(ctx, "SELECT "+sessionColumns+" FROM focus_sessions WHERE id=$1 AND client_id=$2 FOR UPDATE", id, client))
	if err != nil {
		return session, err
	}
	if session.Status == status {
		return session, tx.Commit(ctx)
	}
	if session.Status != "active" {
		return session, shared.Conflict
	}
	if status == "completed" {
		var elapsed bool
		err = tx.QueryRow(ctx, "SELECT now() >= started_at + duration_seconds * interval '1 second' FROM focus_sessions WHERE id=$1", id).Scan(&elapsed)
		if err != nil {
			return session, err
		}
		if !elapsed {
			return session, &shared.Error{Status: 409, Code: "SESSION_NOT_FINISHED", Message: "El tiempo de enfoque todavía no ha terminado"}
		}
	}
	session, err = scanSession(tx.QueryRow(ctx, "UPDATE focus_sessions SET status=$2,finished_at=now() WHERE id=$1 RETURNING "+sessionColumns, id, status))
	if err != nil {
		return session, err
	}
	return session, tx.Commit(ctx)
}
func (s *Service) Stats(ctx context.Context, client uuid.UUID, zone string) (Stats, error) {
	if zone == "" {
		zone = "America/Bogota"
	}
	if len(zone) > 64 {
		return Stats{}, shared.Invalid("time_zone inválida")
	}
	if _, err := time.LoadLocation(zone); err != nil {
		return Stats{}, shared.Invalid("time_zone inválida")
	}
	st := Stats{TimeZone: zone, Days: []Day{}}
	err := s.db.QueryRow(ctx, "SELECT count(*) FILTER (WHERE completed),count(*) FROM focus_tasks WHERE client_id=$1", client).Scan(&st.TasksCompleted, &st.TasksTotal)
	if err != nil {
		return st, err
	}
	err = s.db.QueryRow(ctx, "SELECT count(*),coalesce(sum(duration_seconds),0)::float8/60 FROM focus_sessions WHERE client_id=$1 AND status='completed'", client).Scan(&st.SessionsCompleted, &st.FocusedMinutes)
	if err != nil {
		return st, err
	}
	rows, err := s.db.Query(ctx, `WITH days AS (SELECT generate_series(0,6) AS n),
 dates AS (SELECT (now() AT TIME ZONE $2)::date - n AS day FROM days),
 tasks AS (SELECT (completed_at AT TIME ZONE $2)::date AS day,count(*) AS total FROM focus_tasks WHERE client_id=$1 AND completed AND completed_at >= ((now() AT TIME ZONE $2)::date - 6)::timestamp AT TIME ZONE $2 GROUP BY 1),
 sessions AS (SELECT (finished_at AT TIME ZONE $2)::date AS day,count(*) AS total,sum(duration_seconds)::float8/60 AS minutes FROM focus_sessions WHERE client_id=$1 AND status='completed' AND finished_at >= ((now() AT TIME ZONE $2)::date - 6)::timestamp AT TIME ZONE $2 GROUP BY 1)
 SELECT to_char(d.day,'YYYY-MM-DD'),coalesce(t.total,0),coalesce(s.minutes,0),coalesce(s.total,0) FROM dates d LEFT JOIN tasks t ON d.day=t.day LEFT JOIN sessions s ON d.day=s.day ORDER BY d.day`, client, zone)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	for rows.Next() {
		var d Day
		if err = rows.Scan(&d.Date, &d.TasksCompleted, &d.FocusedMinutes, &d.SessionsCompleted); err != nil {
			return st, err
		}
		st.Days = append(st.Days, d)
	}
	return st, rows.Err()
}
