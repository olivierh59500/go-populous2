package populous2

import (
	"bytes"
	"encoding/json"
	"testing"

	legacy "go-populous2/internal/legacy"
)

func encodeSnapshot(t *testing.T, w *World) []byte {
	t.Helper()
	data, err := json.Marshal(w.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRejectedCastsDoNotMutateTheWorld(t *testing.T) {
	w, err := NewWorld(testBundle(t), 0, false)
	if err != nil {
		t.Fatal(err)
	}
	before := encodeSnapshot(t, w)
	for _, test := range []struct {
		player int
		id     SpellID
		target Target
	}{{0, Volcano, Target{X: 20, Y: 20}}, {0, PapalMagnet, Target{X: -1, Y: 20}}, {2, Perseus, Target{}}, {0, SpellID(35), Target{}}, {0, Perseus, Target{}}} {
		if w.Cast(test.player, test.id, test.target) {
			t.Fatalf("invalid cast accepted: %+v", test)
		}
		if !bytes.Equal(before, encodeSnapshot(t, w)) {
			t.Fatal("rejected cast changed simulation state")
		}
	}
}

func TestNativeManaHeroAndGlobalPowers(t *testing.T) {
	b := testBundle(t)
	for _, id := range []SpellID{PapalMagnet, Perseus, Heracles, Adonis, Odysseus, Achilles, Helen, Earthquake, Swamp, Volcano, Armageddon} {
		t.Run(spellName(b, id), func(t *testing.T) {
			w, err := NewWorld(b, 0, true)
			if err != nil {
				t.Fatal(err)
			}
			w.Core.Magnets[0].Mana = 1000000
			carrier := w.Core.Magnets[0].Carried - 1
			target := Target{X: w.Core.Peeps[carrier].AtPos % 64, Y: w.Core.Peeps[carrier].AtPos / 64}
			cost := w.ManaCost(0, id)
			if !w.Cast(0, id, target) {
				t.Fatal("available funded cast rejected")
			}
			if got := w.Core.Magnets[0].Mana; got != 1000000-cost {
				t.Fatalf("mana %d, want %d", got, 1000000-cost)
			}
			if id.IsHero() && (!w.Heroes[carrier].Active || w.Core.Peeps[carrier].Status != legacy.KnightStatus || w.Core.Magnets[0].Carried != 0) {
				t.Fatal("leader was not converted")
			}
			if id == Armageddon && (w.Core.War || w.NativeRaiseEnabled != 1) {
				t.Fatal("native global hero conversion/terrain permission missing")
			}
		})
	}
}

func spellName(b *Bundle, id SpellID) string { spell, _ := SpellByID(b.Spells, id); return spell.Name }

func TestSnapshotContinuesEffectsAndRandomness(t *testing.T) {
	b := testBundle(t)
	w, err := NewWorld(b, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	w.Demo = true
	w.Core.Magnets[0].Mana = 100000
	p := w.Core.Peeps[0].AtPos
	if !w.Cast(0, FireColumn, Target{X: p % 64, Y: p / 64}) {
		t.Fatal("fire column cast rejected")
	}
	for range 15 {
		w.Tick()
	}
	data := encodeSnapshot(t, w)
	restored, err := ReadSave(b, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	for range 120 {
		w.Tick()
		restored.Tick()
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
		t.Fatal("save/load changed continuation")
	}
	copy := w.Snapshot()
	if len(copy.Core.Peeps) > 0 {
		copy.Core.Peeps[0].Population = 12345
		if w.Core.Peeps[0].Population == 12345 {
			t.Fatal("snapshot aliases people")
		}
	}
}

func TestSaveValidationAndSimulationDeterminism(t *testing.T) {
	b := testBundle(t)
	w, err := NewWorld(b, 10, false)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := w.Snapshot()
	snapshot.Version = 42
	if _, err := Restore(b, snapshot); err == nil {
		t.Fatal("unknown save version accepted")
	}
	snapshot = w.Snapshot()
	snapshot.Core.MapWho[0] = 255
	if _, err := Restore(b, snapshot); err == nil {
		t.Fatal("invalid occupant accepted")
	}
	a, _ := NewWorld(b, 7, true)
	c, _ := NewWorld(b, 7, true)
	a.Demo, c.Demo = true, true
	for range 800 {
		a.Tick()
		c.Tick()
	}
	if !bytes.Equal(encodeSnapshot(t, a), encodeSnapshot(t, c)) {
		t.Fatal("same seed diverged")
	}
}
