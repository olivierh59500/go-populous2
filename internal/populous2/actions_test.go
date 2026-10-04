package populous2

import "testing"

func TestNativeCommandPowerBindings(t *testing.T) {
	b := testBundle(t)
	commands := make(map[uint8]NativeAction)
	for _, action := range b.Actions {
		commands[action.Command] = action
	}
	for _, tc := range []struct {
		command uint8
		spell   SpellID
		handler int
	}{
		{36, Perseus, 0x1783c}, {58, Adonis, 0x17856}, {60, Heracles, 0x17870},
		{66, Odysseus, 0x1788a}, {68, Achilles, 0x178a4}, {70, Helen, 0x178be},
		{56, Tsunami, 0x17a7c}, {6, FireColumn, 0x17638},
	} {
		action := commands[tc.command]
		if action.Spell != tc.spell || action.Handler != tc.handler {
			t.Fatalf("native command %d: %+v", tc.command, action)
		}
	}
	if Helen != 33 || Tsunami != 34 {
		t.Fatal("water hero and tsunami IDs inverted")
	}
	if cues := b.CastSoundCues(FireColumn, 0); len(cues) != 1 || cues[0] != 115 {
		t.Fatal("native fire column cue missing")
	}
	if b.Audio.Cues[108].Companion != 16 {
		t.Fatal("native whirlwind secondary sound missing")
	}
}

func TestOlderWaterPowerSaveMigration(t *testing.T) {
	b := testBundle(t)
	w, err := NewWorld(b, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	w.Core.Magnets[0].Mana = 1000000
	i := w.Core.Magnets[0].Carried - 1
	if !w.Cast(0, Helen, Target{}) {
		t.Fatal("hero cast rejected")
	}
	s := w.Snapshot()
	s.Version = 2
	s.Heroes[i].Spell = 34
	s.LastSpell = 34
	s.Effects = []Effect{{Spell: 33, Player: 0, X: 30, Y: 30, Life: 10}}
	restored, err := Restore(b, s)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Heroes[i].Spell != Helen || restored.LastSpell != Helen || restored.Effects[0].Spell != Tsunami {
		t.Fatal("older numeric water IDs not migrated")
	}
	if s.Effects[0].Spell != 33 {
		t.Fatal("migration changed caller-owned snapshot")
	}
}

func TestNativeInitialGroupsAndStrength(t *testing.T) {
	b := testBundle(t)
	w, err := NewWorld(b, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Core.Peeps) != 13 || w.Core.Peeps[0].Population != 100 || w.Core.Peeps[10].Population != 50 || w.Core.Peeps[0].MovementSpeed != 20 {
		t.Fatal("native starting templates not applied")
	}
	if w.Core.Magnets[0].Mana != b.Levels[0].Players[0].InitialMana() {
		t.Fatal("native initial mana word was not applied")
	}
	for _, level := range b.Levels {
		if level.Players[0].InitialGroups() > 30 {
			w, err = NewWorld(b, level.Number, false)
			if err != nil {
				t.Fatal(err)
			}
			counts := [2]int{}
			for _, p := range w.Core.Peeps {
				counts[p.Player]++
			}
			if counts[0] != level.Players[0].InitialGroups() {
				t.Fatalf("world %d starting groups truncated: %d", level.Number, counts[0])
			}
			return
		}
	}
	t.Fatal("large-population native reference world missing")
}
