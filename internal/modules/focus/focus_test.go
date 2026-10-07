package focus

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestTaskValidation(t *testing.T) {
	for _, tt := range []struct {
		title, priority string
		valid           bool
	}{{"Organizar ideas", "alta", true}, {" café ", "media", true}, {"", "alta", false}, {strings.Repeat("ñ", 121), "baja", false}, {"hola\nsecreto", "baja", false}, {"Tarea", "high", false}} {
		r := CreateTask{tt.title, tt.priority}
		if (r.Validate() == nil) != tt.valid {
			t.Errorf("validación inesperada: %q %q", tt.title, tt.priority)
		}
	}
	r := CreateTask{"  idea  ", "baja"}
	_ = r.Validate()
	if r.Title != "idea" {
		t.Fatal("título sin normalizar")
	}
	if (&PatchTask{}).Validate() == nil {
		t.Fatal("patch vacío aceptado")
	}
	done := false
	if err := (&PatchTask{Completed: &done}).Validate(); err != nil {
		t.Fatal(err)
	}
}
func TestSessionValidation(t *testing.T) {
	for _, duration := range []int{-1, 0, 59, 7201} {
		if (CreateSession{DurationSeconds: duration}).Validate() == nil {
			t.Fatal("duración inválida aceptada")
		}
	}
	for _, duration := range []int{60, 900, 1500, 3000, 7200} {
		if err := (CreateSession{DurationSeconds: duration}).Validate(); err != nil {
			t.Fatal(err)
		}
	}
	id := uuid.Nil
	if (CreateSession{TaskID: &id, DurationSeconds: 60}).Validate() == nil {
		t.Fatal("tarea nula aceptada")
	}
}
