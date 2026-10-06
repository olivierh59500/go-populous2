package populous2

import (
	"encoding/json"
	"os"
	"testing"
)

func TestNativeHostCadenceBudgetAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/native_host_cadence_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		PALCPUClock uint64
		Cases       []struct {
			Kind    string
			Name    string
			Samples []struct{ CPU, DMA uint64 }
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil || corpus.PALCPUClock != 7093790 || len(corpus.Cases) != 35 {
		t.Fatal("original pacing measurements are incomplete", err)
	}
	help, gameplay := 0, 0
	for _, row := range corpus.Cases {
		var total uint64
		for _, sample := range row.Samples {
			total += sample.CPU + sample.DMA
		}
		if len(row.Samples) == 0 {
			t.Fatal("empty source pacing measurement", row.Name)
		}
		mean := float64(total) / float64(len(row.Samples)) / float64(corpus.PALCPUClock)
		switch row.Kind {
		case "help":
			help++
			if len(row.Samples) != 32 || mean < 0.085 || mean > 0.11 {
				t.Fatalf("source help timing changed for %s: %d samples / %.4fs", row.Name, len(row.Samples), mean)
			}
		case "campaign":
			gameplay++
			if len(row.Samples) != 6 || mean < 0.055 || mean > 0.085 {
				t.Fatalf("source gameplay timing changed for %s: %d samples / %.4fs", row.Name, len(row.Samples), mean)
			}
		default:
			t.Fatal("unexpected source timing path", row.Kind)
		}
	}
	if help != 29 || gameplay != 6 {
		t.Fatal("source timing path coverage changed", help, gameplay)
	}
}

func TestNativeHostCadencePacesPassesWithoutReducingPALInput(t *testing.T) {
	var gameplay, help NativeHostCadence
	gamePasses, helpPasses := 0, 0
	// The host still supplies all 100 VBlanks. Only admission to new drawing
	// and simulation passes changes; retained source wait counters stay live.
	for blank := uint64(0); blank < 100; blank++ {
		if gameplay.Ready(blank, NativeHostGameplayPeriod) {
			gamePasses++
		}
		if help.Ready(blank, NativeHostHelpPeriod) {
			helpPasses++
		}
	}
	if gamePasses != 25 || helpPasses != 20 {
		t.Fatalf("100 PAL VBlanks admitted %d game / %d help passes", gamePasses, helpPasses)
	}
}

func TestNativeHostCadenceResumesWithoutCatchUpOrClockStall(t *testing.T) {
	var cadence NativeHostCadence
	if !cadence.Ready(5, 4) || cadence.Ready(5, 4) || cadence.Ready(8, 4) || !cadence.Ready(9, 4) {
		t.Fatal("gameplay period was not measured from its admitted pass")
	}
	if !cadence.Ready(500, 4) || cadence.Ready(500, 4) || cadence.Ready(503, 4) || !cadence.Ready(504, 4) {
		t.Fatal("return from a modal attempted a burst of overdue passes")
	}
	if !cadence.Ready(505, 5) || cadence.Ready(509, 5) || !cadence.Ready(510, 5) {
		t.Fatal("changed host period retained an obsolete deadline")
	}
	if !cadence.Ready(0, 5) || cadence.Ready(4, 5) || !cadence.Ready(5, 5) {
		t.Fatal("restarted host clock stalled the schedule")
	}
	if !cadence.Ready(6, 0) || cadence.Ready(6, 0) || !cadence.Ready(7, 0) {
		t.Fatal("zero period must retain one pass per distinct VBlank")
	}
}
