package populous2

import (
	legacy "go-populous2/internal/legacy"
	"testing"
)

func townFixture(t *testing.T, population int) *legacy.World {
	t.Helper()
	rules, err := DecodeTownRules(testBundle(t).Executable, testBundle(t).Landscapes[0])
	if err != nil {
		t.Fatal(err)
	}
	w := &legacy.World{OlympianTowns: rules, Rules: legacy.DefaultTerrainRules(), Peeps: []legacy.Peep{{Player: 0, AtPos: 32 + 32*64, Population: population, Flags: legacy.InTown}}}
	for y := 31; y <= 33; y++ {
		for x := 31; x <= 33; x++ {
			w.MapBlk[x+y*64] = legacy.FlatBlock
		}
	}
	w.MapWho[32+32*64] = 1
	return w
}

func TestNativeTownSupportThresholds(t *testing.T) {
	w := townFixture(t, 900)
	pos := w.Peeps[0].AtPos
	if got := w.OlympianTownStage(0, pos); got != 9 {
		t.Fatalf("nine support cells: stage %d", got)
	}
	for _, d := range w.OlympianTowns.Footprint[:25] {
		w.MapBlk[pos+d[0]+64*d[1]] = legacy.FlatBlock
	}
	if got := w.OlympianTownStage(0, pos); got != 17 {
		t.Fatalf("25 cells without outer clearance: stage %d", got)
	}
	for _, d := range w.OlympianTowns.Footprint[25:] {
		w.MapBlk[pos+d[0]+64*d[1]] = legacy.FlatBlock
	}
	if got := w.OlympianTownStage(0, pos); got != 18 {
		t.Fatalf("full native footprint: stage %d", got)
	}
	w.MapBlk[pos] = legacy.WaterBlock
	if got := w.OlympianTownStage(0, pos); got != 0 {
		t.Fatalf("water center supported a town: %d", got)
	}
	if w.OlympianTowns.ManaAdd[18] != 100 || w.OlympianTowns.PopulationLimit[18] != 4000 {
		t.Fatal("nineteenth stage collapsed")
	}
}

func TestNativeTownGrowthAndEmigration(t *testing.T) {
	w := townFixture(t, 900)
	for range 7 {
		w.TickWithComputer([2]bool{})
	}
	if w.Peeps[0].Population != 900 {
		t.Fatal("town worked before its eighth tick")
	}
	w.TickWithComputer([2]bool{})
	if w.Peeps[0].Population != 905 || w.Peeps[0].TownStage != 9 || w.Peeps[0].TownWork != 0 {
		t.Fatalf("native growth: %+v", w.Peeps[0])
	}
	// Four passive mana increments plus the ten-unit stage-nine work award.
	if w.Magnets[0].Mana != 14 {
		t.Fatalf("native town mana %d", w.Magnets[0].Mana)
	}
	w = townFixture(t, 1000)
	w.Peeps[0].TownWork = 7
	w.TickWithComputer([2]bool{})
	if len(w.Peeps) != 2 || w.Peeps[0].Population != 665 || w.Peeps[1].Population > 335 {
		t.Fatalf("native one-third emigration failed: %+v", w.Peeps)
	}
	full := townFixture(t, 1000)
	for len(full.Peeps) < legacy.MaxFollowers {
		i := full.AllocateHeroClone(0)
		full.Peeps[i].Flags = legacy.OnMove
		full.Peeps[i].Frame = 1
	}
	full.Peeps[0].TownWork = 7
	full.TickWithComputer([2]bool{})
	if full.Peeps[0].Population != 1000 {
		t.Fatal("full pool changed town population")
	}
}

func TestNativeTownWorkSurvivesSave(t *testing.T) {
	w, err := NewWorld(testBundle(t), 0, false)
	if err != nil {
		t.Fatal(err)
	}
	w.Core.Peeps[0].TownWork = 5
	w.Core.Peeps[0].TownStage = 14
	restored, err := Restore(testBundle(t), w.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	if restored.Core.OlympianTowns == nil || restored.Core.Peeps[0].TownWork != 5 || restored.Core.StateHash() != w.Core.StateHash() {
		t.Fatal("save lost native town configuration or work state")
	}
}
