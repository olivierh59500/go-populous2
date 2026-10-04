package populous2

import (
	legacy "go-populous2/internal/legacy"
	"testing"
)

func TestPlagueTargetsOneCellAndSpreadsThroughContact(t *testing.T) {
	w, err := NewWorld(testBundle(t), 0, true)
	if err != nil {
		t.Fatal(err)
	}
	w.Core.Peeps = []legacy.Peep{
		{Player: 1, Population: 100, AtPos: 2000, Flags: legacy.OnMove},
		{Player: 1, Population: 100, AtPos: 2001, Flags: legacy.OnMove},
		{Player: 0, Population: 100, AtPos: 2000, Flags: legacy.OnMove},
	}
	w.Core.Magnets[0].Mana = 1000000
	if !w.Cast(0, Plague, Target{X: 2000 % 64, Y: 2000 / 64}) {
		t.Fatal("enemy plague cast rejected")
	}
	if !w.Core.Peeps[0].Plague || w.Core.Peeps[1].Plague || w.Core.Peeps[2].Plague {
		t.Fatal("plague cast used an area or friendly target")
	}
	w.spreadPlague()
	if !w.Core.Peeps[2].Plague || w.Core.Peeps[1].Plague {
		t.Fatal("plague did not follow direct contact")
	}
	before := w.Core.Peeps[0].Population
	for range 300 {
		w.tickEffects()
	}
	if !w.Core.Peeps[0].Plague || w.Core.Peeps[0].Population != before {
		t.Fatal("old expiring plague marker or invented damage retained")
	}
	if !w.Cast(0, Armageddon, Target{}) {
		t.Fatal("armageddon cast rejected")
	}
	if w.Core.Peeps[0].Population != 0 || w.Core.Peeps[2].Population != 0 || w.Core.Peeps[1].Population == 0 {
		t.Fatal("armageddon did not remove only infected groups")
	}
}
