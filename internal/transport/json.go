package transport

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"

	"github.com/Jd-aristizabal/backend/internal/shared"
	"github.com/google/uuid"
)

func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if status != 204 {
		_ = json.NewEncoder(w).Encode(data)
	}
}
func Data(w http.ResponseWriter, status int, data any) { JSON(w, status, map[string]any{"data": data}) }
func Error(w http.ResponseWriter, err error) {
	e := &shared.Error{Status: 500, Code: "INTERNAL_ERROR", Message: "No se pudo completar la solicitud"}
	var known *shared.Error
	if errors.As(err, &known) {
		e = known
	}
	JSON(w, e.Status, map[string]any{"error": map[string]string{"code": e.Code, "message": e.Message}})
}
func Decode(w http.ResponseWriter, r *http.Request, v any) error {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		return &shared.Error{Status: 400, Code: "INVALID_JSON", Message: "Usa Content-Type application/json"}
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err = dec.Decode(v); err != nil {
		return &shared.Error{Status: 400, Code: "INVALID_JSON", Message: "JSON inválido o demasiado grande"}
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return &shared.Error{Status: 400, Code: "INVALID_JSON", Message: "Se espera un único objeto JSON"}
	}
	return nil
}
func UUID(raw string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil || len(raw) != 36 || id.String() != raw {
		return uuid.Nil, shared.Invalid("UUID inválido; usa formato canónico en minúsculas")
	}
	return id, nil
}
func Pagination(r *http.Request, defaultLimit, maxLimit int) (int, int, error) {
	limit, offset := defaultLimit, 0
	var err error
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > maxLimit {
			return 0, 0, shared.Invalid("limit fuera de rango")
		}
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil || offset < 0 || offset > 10000 {
			return 0, 0, shared.Invalid("offset fuera de rango")
		}
	}
	return limit, offset, nil
}
