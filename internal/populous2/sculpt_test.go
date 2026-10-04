package populous2

import (
	legacy "go-populous2/internal/legacy"
	"testing"
)

func TestNativeSculptChargesPropagatedChanges(t *testing.T) {
	w, err := NewWorld(testBundle(t), 0, true)
	if err != nil {
		t.Fatal(err)
	}
	w.Core.Peeps = []legacy.Peep{{Player: 0, AtPos: 32 + 32*64, Population: 100, Flags: legacy.OnMove}}
	w.Core.Magnets[0].Carried = 1
	w.Core.MapWho = [4096]uint16{}
	w.Core.MapWho[32+32*64] = 1
	w.Core.Magnets[0].Mana = 101
	w.Core.Alt = [65 * 65]int{}
	w.Core.MapAlt = [4096]byte{}
	if !w.Sculpt(0, 32, 32, true) {
		t.Fatal("funded local raise rejected")
	}
	if w.Core.Magnets[0].Mana != 81 {
		t.Fatalf("single native terrain price %d", w.Core.Magnets[0].Mana)
	}
	before := w.Core.Alt
	w.Core.Magnets[0].Mana = 10000
	if !w.Sculpt(0, 32, 32, true) {
		t.Fatal("propagated raise rejected")
	}
	changed := 0
	for i, h := range w.Core.Alt {
		changed += abs(h - before[i])
	}
	if changed <= 1 || w.Core.Magnets[0].Mana != 10000-20*changed {
		t.Fatalf("propagated terrain debit: changes %d mana %d", changed, w.Core.Magnets[0].Mana)
	}
	if got := w.Experience[1]; got != w.Level.OpponentExperience {
		t.Fatal("native opponent experience missing")
	}
}

func TestSculptClearsGroundOverridesOnAllTouchedNeighbors(t *testing.T) {
	w := flatGroundWorld(t)
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 100, AtPos: 32 + 32*64, Flags: legacy.OnMove}}
	w.Core.MapWho[32+32*64] = 1
	for y := 31; y <= 32; y++ {
		for x := 31; x <= 32; x++ {
			w.Marks[x+y*64] = Mark{Spell: Flowers, Player: 0, Life: 1, Persistent: true, NativeTile: 245}
		}
	}
	w.Marks[10+10*64] = Mark{Spell: Flowers, Player: 0, Life: 1, Persistent: true, NativeTile: 245}
	if !w.Sculpt(0, 32, 32, true) {
		t.Fatal("valid sculpt rejected")
	}
	for y := 31; y <= 32; y++ {
		for x := 31; x <= 32; x++ {
			if w.Marks[x+y*64].NativeTile != 0 {
				t.Fatalf("stale ground graphic at %d,%d", x, y)
			}
		}
	}
	if w.Marks[10+10*64].NativeTile != 245 {
		t.Fatal("unrelated ground effect cleared")
	}
}
