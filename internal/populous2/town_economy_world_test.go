package populous2

import (
	"bytes"
	"testing"

	legacy "go-populous2/internal/legacy"
)

func TestNativeWorldTownWorkProducesOriginalManaAndGrowth(t *testing.T) {
	w := lightningWorld(t, 4311)
	w.bindNativeTownEvaluator()
	w.Core.Magnets[0].Carried, w.Core.Magnets[1].Carried = 0, 0
	pos := 32 + 32*64
	stage := 18
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 1000, AtPos: pos, Flags: legacy.InTown, MovementSpeed: 20, TownStage: stage, TownWork: int(w.TownEconomy.WorkTicks[stage]) - 1}}
	w.initializeNativeFollower(0)
	beforeMana := w.Core.Magnets[0].Mana
	w.Core.TickWithComputer([2]bool{})
	if w.Core.Peeps[0].Population != 1000+int(w.TownEconomy.PopulationAdd[stage]) || w.Core.Peeps[0].TownWork != 0 || w.Core.Magnets[0].Mana != beforeMana+int(w.TownEconomy.ManaAdd[stage]) {
		t.Fatalf("native town production differs: %+v", w.Core.Peeps[0])
	}
	deity, _ := NativeDeityAddress(1)
	count, _ := w.runtimeMemory().Read16(deity + 0x24)
	total, _ := w.runtimeMemory().Read32(deity + 4)
	if count != 1 || total != uint32(w.Core.Peeps[0].Population) {
		t.Fatal("native town statistics were not written at dispatch boundaries")
	}
	copy, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	for range 10 {
		w.Core.TickWithComputer([2]bool{})
		copy.Core.TickWithComputer([2]bool{})
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, copy)) {
		t.Fatal("native town work/save continuation differs")
	}
}

func TestNativeWorldTownEmigrantRunsHigherSlotDuringSamePass(t *testing.T) {
	w := lightningWorld(t, 4311)
	w.bindNativeTownEvaluator()
	w.Core.Magnets[0].Carried, w.Core.Magnets[1].Carried = 0, 0
	w.Core.FollowerAttrition = func(int, bool) int { return 1 }
	stage := 18
	pos := 32 + 32*64
	population := int(w.TownEconomy.PopulationLimit[stage]) + 100
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: population, AtPos: pos, Flags: legacy.InTown, MovementSpeed: 20, TownStage: stage, TownWork: int(w.TownEconomy.WorkTicks[stage]) - 1}}
	w.initializeNativeFollower(0)
	w.Core.TickWithComputer([2]bool{})
	quota := int(nativeCombatQuotient(int32(population+int(w.TownEconomy.PopulationAdd[stage])), w.TownEconomy.EmigrationDivisor[stage]))
	if len(w.Core.Peeps) < 2 || w.Core.Peeps[0].Population != population-quota || w.Core.Peeps[1].Population != quota-1 || w.Core.Peeps[1].Weapons != stage || w.Core.Peeps[1].IQ != stage*2 {
		t.Fatalf("native same-pass birth differs: %+v", w.Core.Peeps)
	}
	if w.NativeFollowers[1].Actor.Animation != 4 || w.NativeFollowers[1].Actor.State != 4 {
		t.Fatal("higher native newborn slot did not run its first movement in the birth pass")
	}
}

func TestNativeWorldRareBirthCreatesRealNeutralActorAndDeadline(t *testing.T) {
	w := lightningWorld(t, 4311)
	w.bindNativeTownEvaluator()
	w.Core.Magnets[0].Carried, w.Core.Magnets[1].Carried = 0, 0
	stage := 18
	pos := 32 + 32*64
	w.Core.Peeps = make([]legacy.Peep, 249)
	for index := range w.Core.Peeps {
		w.Core.Peeps[index] = legacy.Peep{Player: 0, Population: 100, AtPos: pos, Flags: legacy.OnMove, MovementSpeed: 20, IQ: 2}
		w.initializeNativeFollower(index)
	}
	population := int(w.TownEconomy.PopulationLimit[stage]) + 100
	w.Core.Peeps[0] = legacy.Peep{Player: 0, Population: population, AtPos: pos, Flags: legacy.InTown, MovementSpeed: 20, TownStage: stage, TownWork: int(w.TownEconomy.WorkTicks[stage]) - 1}
	w.NativeEntries[0] = NativeFollowerEntry{}
	w.Core.GameTurn = 100
	if err := w.runNativeFollowerCall(func() error { _, err := w.nativeTownEconomyTick(52); return err }); err != nil {
		t.Fatal(err)
	}
	if len(w.Core.Peeps) < 251 || w.NativeCreatureDeadline != 150 {
		t.Fatal("rare native birth did not allocate child/neutral records and deadline")
	}
	child, err := w.RecordImage.ReadFollowerEntry(NativeRecordReference(250 * 52))
	if err != nil {
		t.Fatal(err)
	}
	neutral, err := w.RecordImage.ReadFollowerEntry(NativeRecordReference(251 * 52))
	if err != nil {
		t.Fatal(err)
	}
	if child.Owner != 1 || neutral.Owner != 3 || neutral.Motion.Kind != 0x3c || neutral.Motion.State != 0x44 || neutral.Hero40 < 2 || neutral.Hero40 > 12 || !w.Occupancy.Followers[250].Linked {
		t.Fatalf("rare callback substituted a generic actor: child%+v neutral%+v", child, neutral)
	}
	if _, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w))); err != nil {
		t.Fatal(err)
	}
}

func TestNativeEconomyCampaignLandscapesAndSavedContinuation(t *testing.T) {
	b := testBundle(t)
	var selected [4]bool
	for index, level := range b.Levels {
		if level.Terrain < 0 || level.Terrain > 3 || selected[level.Terrain] {
			continue
		}
		selected[level.Terrain] = true
		t.Run(level.Code, func(t *testing.T) {
			w, err := NewWorld(b, index, false)
			if err != nil {
				t.Fatal(err)
			}
			w.Demo = true
			for range 600 {
				w.Tick()
			}
			restored, err := ReadSave(b, bytes.NewReader(encodeSnapshot(t, w)))
			if err != nil {
				t.Fatal(err)
			}
			for range 30 {
				w.Tick()
				restored.Tick()
			}
			if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
				t.Fatal("native economy landscape save continuation diverged")
			}
		})
	}
	for landscape, found := range selected {
		if !found {
			t.Fatalf("no original campaign world tested for landscape%d", landscape)
		}
	}
}
