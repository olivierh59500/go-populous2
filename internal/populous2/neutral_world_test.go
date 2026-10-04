package populous2

import (
	"bytes"
	legacy "go-populous2/internal/legacy"
	"testing"
)

func TestNativeWorldNeutralOwnerRunsAndSurvivesSave(t *testing.T) {
	w := lightningWorld(t, 4311)
	w.bindNativeTownEvaluator()
	w.Core.Peeps = nil
	w.reconcileActorGraph()
	var created NativeNeutralCreation
	if err := w.runNativeFollowerCall(func() error {
		var err error
		created, err = CreateNativeNeutral(FollowerCleanupRegisters{D0: 32, D1: 32, D2: 2}, NativeNeutralCallbacks{Memory: w.nativeCleanupMemory(), Insert: w.nativeRuntimeInsert})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if !created.Created || len(w.Core.Peeps) != 1 || w.Core.Peeps[0].Player != 2 || w.Core.Peeps[0].Population != 0 || !w.Core.FollowerReserved(0) {
		t.Fatal("neutral owner was collapsed into ordinary player/allocation state")
	}
	w.Core.TickWithComputer([2]bool{})
	raw, err := w.RecordImage.ReadFollowerEntry(52)
	if err != nil {
		t.Fatal(err)
	}
	if raw.Owner != 3 || raw.Motion.State != 0x44 || raw.Motion.X != 32*256+16 || w.Core.Magnets[0].Population != 0 || w.Core.Magnets[1].Population != 0 {
		t.Fatalf("neutral actor dispatch/totals differs: %+v", raw)
	}
	copy, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	for range 5 {
		w.Core.TickWithComputer([2]bool{})
		copy.Core.TickWithComputer([2]bool{})
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, copy)) {
		t.Fatal("native neutral owner movement/save continuation differs")
	}
}

func TestNativeWorldNeutralEffectCreatesOwnerThreeWhirlwind(t *testing.T) {
	w := lightningWorld(t, 4311)
	w.bindNativeTownEvaluator()
	w.Core.Peeps = []legacy.Peep{}
	w.reconcileActorGraph()
	if err := w.runNativeFollowerCall(func() error {
		_, err := w.PrimitiveCreators.CreateWhirlwind(3, 32, 32, w.nativePrimitiveCallbacks())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	actor := w.NativeEffects[0]
	if !actor.Active || actor.Player != 2 || actor.Kind != 0x20 || actor.Life != int16(w.PrimitiveCreators.WhirlwindLife) {
		t.Fatal("neutral raw whirlwind owner/experience mapping differs")
	}
	if _, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w))); err != nil {
		t.Fatal(err)
	}
}

func TestNativeWorldNeutralWhirlpoolHasNoPlayerExperience(t *testing.T) {
	w := oceanWorld(t, 4311)
	if !w.castWhirlpool(2, 32, 32) {
		t.Fatal("native neutral four-water creation failed")
	}
	if w.NativeEffects[0].Player != 2 || w.NativeEffects[0].Life != int16(w.Whirlpools.BaseLife) {
		t.Fatal("neutral whirlpool read nonexistent deity experience")
	}
	if _, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w))); err != nil {
		t.Fatal(err)
	}
}
