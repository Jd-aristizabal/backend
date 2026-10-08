package games

import "testing"

func TestArcadeValidation(t *testing.T) {
	name := "Johan"
	for _, tc := range []struct {
		game                          string
		score, combo, level, duration int
		valid                         bool
	}{
		{"orbit-match", 0, 0, 1, 60, true}, {"orbit-match", 175, 1, 2, 60, true},
		{"orbit-match", 2015, 6, 7, 1, true}, {"orbit-match", 1425, 6, 7, 60, true},
		{"orbit-match", 175, 1, 2, 40, false}, {"orbit-match", 100000, 6, 7, 1, false},
		{"orbit-match", 175, 2, 2, 60, false}, {"orbit-match", 0, 0, 1, 0, false},
		{"pulse-orbit", 0, 0, 1, 40, true}, {"pulse-orbit", 200, 1, 1, 40, true},
		{"pulse-orbit", 800, 4, 2, 40, true}, {"pulse-orbit", 200, 0, 1, 40, false},
		{"pulse-orbit", 200, 1, 8, 40, false}, {"pulse-orbit", 201, 1, 1, 40, false},
		{"pulse-orbit", 800, 4, 2, 60, false}, {"unknown", 0, 0, 1, 40, false},
	} {
		r := SubmitScore{Score: tc.score, MaxCombo: tc.combo, Level: tc.level, DurationSeconds: tc.duration, PlayerName: &name}
		if err := r.ValidateGame(tc.game); (err == nil) != tc.valid {
			t.Errorf("%+v: %v", tc, err)
		}
	}
	r := SubmitScore{DurationSeconds: 40, Level: 1}
	if r.ValidateGame("pulse-orbit") == nil {
		t.Fatal("Nombre obligatorio")
	}
}

func TestPulseAcceptsActualScoring(t *testing.T) {
	name := "Jugador"
	for hits := 1; hits <= 300; hits++ {
		score := 0
		for i := 0; i < hits; i++ {
			score += 200 * min(4, 1+i/4)
		}
		r := SubmitScore{Score: score, MaxCombo: hits, Level: min(8, 1+hits/4), DurationSeconds: 40, PlayerName: &name}
		if err := r.ValidateGame("pulse-orbit"); err != nil {
			t.Fatalf("%d aciertos: %v", hits, err)
		}
	}
}
