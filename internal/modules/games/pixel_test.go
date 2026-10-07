package games

import "testing"

func TestScoreValidation(t *testing.T) {
	for _, tt := range []struct {
		name                          string
		score, combo, level, duration int
		valid                         bool
	}{
		{"sin aciertos", 0, 0, 1, 30, true}, {"un acierto", 100, 1, 1, 30, true}, {"racha siete", 900, 7, 2, 30, true}, {"racha cincuenta", 20000, 50, 8, 30, true},
		{"negativo", -100, 1, 1, 30, false}, {"absurdo", 150100, 300, 8, 30, false}, {"paso inválido", 101, 1, 1, 30, false}, {"combo imposible", 100, 300, 8, 30, false}, {"duración imposible", 100, 1, 1, 0, false}, {"combo vacío", 100, 0, 1, 30, false}, {"nivel imposible", 100, 1, 8, 30, false}, {"racha incompatible", 800, 7, 2, 30, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := SubmitScore{Score: tt.score, MaxCombo: tt.combo, Level: tt.level, DurationSeconds: tt.duration}
			if (r.Validate() == nil) != tt.valid {
				t.Fatalf("validación inesperada: %+v", r)
			}
		})
	}
	name := "  Johan  "
	r := SubmitScore{Score: 100, MaxCombo: 1, Level: 1, DurationSeconds: 30, PlayerName: &name}
	if err := r.Validate(); err != nil || *r.PlayerName != "Johan" {
		t.Fatal("nombre sin normalizar")
	}
}
func TestValidGameTrajectories(t *testing.T) {
	// Verificar cientos de partidas generadas con las reglas reales, incluyendo fallos.
	for seed := 0; seed < 150; seed++ {
		score, combo, best, hits := 0, 0, 0, 0
		for n := 0; n < 120; n++ {
			if (n+seed)%13 == 0 {
				combo = 0
				continue
			}
			score += 100 * min(5, 1+combo/5)
			combo++
			hits++
			best = max(best, combo)
		}
		r := SubmitScore{Score: score, MaxCombo: best, Level: min(8, 1+hits/7), DurationSeconds: 30}
		if err := r.Validate(); err != nil {
			t.Fatalf("partida real rechazada: %+v", r)
		}
	}
}
