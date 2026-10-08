package portfolio

import (
	"strings"
	"testing"
)

func TestKnowledge(t *testing.T) {
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if s.Profile.Name != "Johan Aristizabal" || len(s.Projects) != 5 || len(s.Skills) != 8 {
		t.Fatal("conocimiento incompleto")
	}
	if s.Contact.GitHub != nil {
		t.Fatal("GitHub inventado")
	}
	if !strings.Contains(s.Context(), s.Contact.Email) || !strings.Contains(s.Context(), "Focus Flow") {
		t.Fatal("contexto no usa la fuente central")
	}
	for _, name := range []string{"Orbit Match", "Pulse Orbit", "Color Studio"} {
		if !strings.Contains(s.Context(), name) {
			t.Fatalf("Proyecto ausente del contexto: %s", name)
		}
	}
}
