package populous2

import (
	"bytes"
	"testing"

	legacy "go-populous2/internal/legacy"
)

func nativeBattleWorld(t *testing.T, attacker, defender int) *World {
	t.Helper()
	w := lightningWorld(t, 4311)
	w.Core.Magnets[0].Carried, w.Core.Magnets[1].Carried = 0, 0
	pos := 32 + 32*64
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: attacker, AtPos: pos, Flags: legacy.OnMove, MovementSpeed: 20, Weapons: 3}, {Player: 1, Population: defender, AtPos: pos, Flags: legacy.OnMove, MovementSpeed: 20, Weapons: 7}}
	w.initializeNativeFollower(0)
	w.initializeNativeFollower(1)
	w.reconcileActorGraph()
	if err := w.runNativeFollowerCall(func() error {
		return w.FollowerEntry.prepareBattle(52, 104, w.nativeEntryCallbacks())
	}); err != nil {
		t.Fatal(err)
	}
	return w
}

func TestNativeWorldBattleUsesAggressorQuotientAndPassiveDefender(t *testing.T) {
	w := nativeBattleWorld(t, 1000, 1000)
	if !w.updateNativeManagedFollower(0) || w.Core.Peeps[0].Population != 920 || w.Core.Peeps[1].Population != 960 {
		t.Fatal("World battle did not apply native reciprocal damage")
	}
	rng := w.Random
	if !w.updateNativeManagedFollower(1) || w.Random != rng || w.Core.Peeps[0].Population != 920 || w.Core.Peeps[1].Population != 960 {
		t.Fatal("passive defender consumed RNG or applied another damage pass")
	}
	if err := w.Occupancy.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestNativeWorldBattleWinnerAndRetainedDeathSurviveSave(t *testing.T) {
	w := nativeBattleWorld(t, 1000, 1)
	if !w.updateNativeManagedFollower(0) {
		t.Fatal("native aggressor was not dispatched")
	}
	winner, err := w.RecordImage.ReadFollowerEntry(52)
	if err != nil {
		t.Fatal(err)
	}
	loser, err := w.RecordImage.ReadFollowerEntry(104)
	if err != nil {
		t.Fatal(err)
	}
	if winner.Motion.State != 0x42 || loser.Motion.State != 0x18 || loser.Owner != 2 || loser.Motion.Population != 0 || !w.Core.FollowerReserved(1) {
		t.Fatal("winner/death records did not retain native states and allocation")
	}
	restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	for range 20 {
		w.updateNativeManagedFollower(1)
		restored.updateNativeManagedFollower(1)
		if !w.NativeEntries[1].Managed {
			break
		}
	}
	if w.Core.FollowerReserved(1) || !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
		t.Fatal("retained native death continuation diverged or remained allocated")
	}
}

func TestNativeWorldFriendlyContactHomesBeforeMerging(t *testing.T) {
	w := lightningWorld(t, 4311)
	w.Core.Magnets[0].Carried, w.Core.Magnets[1].Carried = 0, 0
	pos := 32 + 32*64
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 700, AtPos: pos, Flags: legacy.OnMove, MovementSpeed: 20, Weapons: 3}, {Player: 0, Population: 300, AtPos: pos, Flags: legacy.OnMove, MovementSpeed: 20, Weapons: 7}}
	w.initializeNativeFollower(0)
	w.initializeNativeFollower(1)
	w.NativeFollowers[0].Actor.X, w.NativeFollowers[1].Actor.X = 32*256+40, 32*256+200
	w.NativeFollowers[0].Actor.State = 4
	w.reconcileActorGraph()
	step := w.enterNativeFollowerCell(0)
	if step.Outcome != FollowerEntryHoming || w.Core.Peeps[0].Population != 700 || w.Core.Peeps[1].Population != 300 || !w.NativeEntries[0].Managed {
		t.Fatalf("friendly contact homing mismatch: step %+v, populations %d/%d, entry %+v", step, w.Core.Peeps[0].Population, w.Core.Peeps[1].Population, w.NativeEntries[0])
	}
	for range 30 {
		if !w.NativeEntries[0].Managed {
			break
		}
		w.updateNativeManagedFollower(0)
	}
	if w.Core.Peeps[0].Population != 0 || w.Core.Peeps[1].Population != 1000 || w.Core.Peeps[1].Weapons != 7 || w.Core.FollowerReserved(0) {
		t.Fatal("native completed friendly contact did not merge and release its source")
	}
	if err := w.Occupancy.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestNativeWorldWaitingExpiryStartsSearchDuringSameUpdate(t *testing.T) {
	w := lightningWorld(t, 4311)
	w.Core.Magnets[0].Carried, w.Core.Magnets[1].Carried = 0, 0
	w.Core.FollowerAttrition = func(int, bool) int { return 0 }
	pos := 32 + 32*64
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 700, AtPos: pos, Flags: legacy.OnMove, MovementSpeed: 20, IQ: 2}}
	w.initializeNativeFollower(0)
	w.refreshNativeRecordImage()
	a, err := w.RecordImage.ReadFollowerEntry(52)
	if err != nil {
		t.Fatal(err)
	}
	a.Motion.State, a.Motion.Animation, a.Motion.Timer = 10, 0xccc, 1
	if _, err := w.RecordImage.PatchFollowerEntry(52, w.NativeEntries[0].Actor, a); err != nil {
		t.Fatal(err)
	}
	w.hydrateNativeRuntimeRecords()
	if !w.updateNativeManagedFollower(0) {
		t.Fatal("native waiter was not handled")
	}
	a, err = w.RecordImage.ReadFollowerEntry(52)
	if err != nil || a.Motion.State != 4 || a.Motion.Animation != 4 || a.Motion.Population != 700 {
		t.Fatalf("native wait did not fall through into one motion update: %+v, %v", a, err)
	}
}

func TestNativeBattleIsDispatchedByNormalWorldLoop(t *testing.T) {
	w := nativeBattleWorld(t, 1000, 1000)
	w.Core.Computer[0].Mode, w.Core.Computer[1].Mode = 0, 0
	w.Tick()
	if w.Core.Peeps[0].Population != 920 || w.Core.Peeps[1].Population != 960 {
		t.Fatal("normal World loop bypassed native attacker/defender dispatch")
	}
}

func TestNativeFollowerControlLatchesSurviveSave(t *testing.T) {
	w := nativeBattleWorld(t, 1000, 1000)
	w.NativeRaiseEnabled, w.NativeBirthBlocked = 1, true
	restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	if restored.NativeRaiseEnabled != 1 || !restored.NativeBirthBlocked {
		t.Fatal("save lost native direct-raising or allocation-inhibition latch")
	}
}

func TestOrdinaryAttritionRetainsNativeDeathRecord(t *testing.T) {
	w := lightningWorld(t, 4311)
	w.Core.Magnets[0].Carried, w.Core.Magnets[1].Carried = 0, 0
	w.Core.FollowerAttrition = func(int, bool) int { return 10 }
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 5, AtPos: 32 + 32*64, Flags: legacy.OnMove, MovementSpeed: 20, IQ: 2}}
	w.initializeNativeFollower(0)
	w.updateNativeFollower(0)
	a, err := w.RecordImage.ReadFollowerEntry(52)
	if err != nil || a.Owner != 1 || a.Motion.Population != 0 || a.Motion.State != 0x2c || a.Motion.Animation != 0x7f4 || !w.Core.FollowerReserved(0) {
		t.Fatalf("ordinary attrition freed its native death actor: %+v, %v", a, err)
	}
	if !w.Occupancy.Followers[0].Linked {
		t.Fatal("native attrition death lost occupancy before its terminal")
	}
	for range 32 {
		if !w.NativeEntries[0].Managed {
			break
		}
		w.updateNativeManagedFollower(0)
	}
	if w.Core.FollowerReserved(0) || w.Occupancy.Followers[0].Linked {
		t.Fatal("native attrition death failed to reach complete terminal cleanup")
	}
}
