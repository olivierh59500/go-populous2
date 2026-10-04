package populous2

import "testing"

func TestEarlierWaterSlotSavesMigrateWithoutChangingCallerData(t *testing.T) {
	w := flatGroundWorld(t)
	snapshot := w.Snapshot()
	snapshot.Version = 11
	snapshot.LastSpell = 30 // Earlier prototypes labeled this slot Whirlpool.
	snapshot.Effects = []Effect{{Spell: 30, Player: 0, X: 32, Y: 32, Life: 10}}
	snapshot.Marks[2000] = Mark{Spell: 31, Player: 0, Life: 2000}
	restored, err := Restore(testBundle(t), snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if restored.LastSpell != Whirlpool || len(restored.Effects) != 1 || restored.Effects[0].Spell != Whirlpool || restored.Marks[2000].Spell != Basalt {
		t.Fatal("earlier water effects changed semantic meaning during migration")
	}
	if snapshot.LastSpell != 30 || snapshot.Effects[0].Spell != 30 || snapshot.Marks[2000].Spell != 31 {
		t.Fatal("water migration changed caller-owned snapshot data")
	}
}

func TestCurrentWaterSlotSavesPreserveNativeIDs(t *testing.T) {
	w := flatGroundWorld(t)
	w.LastSpell = Whirlpool
	w.Effects = []Effect{{Spell: Whirlpool, Player: 0, X: 32, Y: 32, Life: 10}}
	w.Marks[2000] = Mark{Spell: Basalt, Player: 0, Life: 2000}
	restored, err := Restore(testBundle(t), w.Snapshot())
	if err != nil {
		t.Fatal(err)
	}
	if restored.LastSpell != Whirlpool || restored.Effects[0].Spell != Whirlpool || restored.Marks[2000].Spell != Basalt {
		t.Fatal("current native water IDs were swapped again")
	}
}
