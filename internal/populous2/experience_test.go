package populous2

import (
	"bytes"
	"testing"
)

func TestNativeExperienceManaThresholds(t *testing.T) {
	b := testBundle(t)
	if b.ManaRules.Divisors != [8]uint8{0, 0, 10, 9, 8, 7, 6, 5} {
		t.Fatal("native divisor table changed")
	}
	spell, _ := SpellByID(b.Spells, FireColumn)
	for _, test := range []struct {
		experience uint8
		cost       int
	}{
		{0, 5625}, {31, 5625}, {32, 5625}, {63, 5625},
		{64, 5063}, {95, 5063}, {96, 5000}, {128, 4922},
		{160, 4822}, {192, 4688}, {224, 4500}, {255, 4500},
	} {
		var experience [6]uint8
		experience[Fire] = test.experience
		if got := b.ManaRules.Cost(FireColumn, spell.Cost, experience); got != test.cost {
			t.Fatalf("experience %d: cost %d, want %d", test.experience, got, test.cost)
		}
		if got := b.ManaRules.Cost(Baptism, 6250, experience); got != 6250 {
			t.Fatalf("fire experience changed a water power: %d", got)
		}
	}
	if b.ManaRules.Cost(SpellID(35), 65535, [6]uint8{255, 255, 255, 255, 255, 255}) != 65535 {
		t.Fatal("reserved slot discounted")
	}
}

func TestExperienceControlsCastsAndSurvivesSave(t *testing.T) {
	b := testBundle(t)
	w, err := NewWorld(b, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	w.Experience[0][Fire] = 224
	w.Core.Magnets[0].Mana = 18000
	p := w.Core.Peeps[0].AtPos
	if !w.Cast(0, FireColumn, Target{X: p % 64, Y: p / 64}) {
		t.Fatal("discounted, funded cast rejected")
	}
	if w.Core.Magnets[0].Mana != 0 {
		t.Fatal("cast did not pay discounted cost")
	}
	restored, err := ReadSave(b, bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	if restored.Experience != w.Experience || restored.ManaCost(0, FireColumn) != 18000 {
		t.Fatal("save lost deity experience")
	}
	if _, err := DecodeManaRules(nil); err == nil {
		t.Fatal("missing executable accepted")
	}
}
