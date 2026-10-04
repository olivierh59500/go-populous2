package fixedstep

import "testing"

func TestPALCadenceFromSixtyHzInput(t *testing.T) {
	s := New(50, 60)
	total := 0
	for i := 1; i <= 3600; i++ {
		steps := s.Advance()
		if steps < 0 || steps > 1 {
			t.Fatal("PAL scheduling burst")
		}
		total += steps
		if i%60 == 0 && total != i/60*50 {
			t.Fatalf("input update %d: %d simulation steps", i, total)
		}
	}
}
