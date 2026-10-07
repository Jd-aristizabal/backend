package shared

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string     { return e.Code }
func Invalid(message string) error { return &Error{422, "VALIDATION_ERROR", message} }

var NotFound = &Error{404, "NOT_FOUND", "Recurso no encontrado"}
var Conflict = &Error{409, "CONFLICT", "El recurso no permite esta operación"}
var Unavailable = &Error{503, "SERVICE_UNAVAILABLE", "Servicio temporalmente no disponible"}

func DatabaseError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return NotFound
	}
	var p *pgconn.PgError
	if errors.As(err, &p) && p.Code == "23505" {
		return Conflict
	}
	return err
}
