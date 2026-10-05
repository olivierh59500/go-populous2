package populous2

import (
	"bytes"
	"testing"

	legacy "go-populous2/internal/legacy"
)

func TestPlagueUsesNativeCellSelectionAndMergeInheritance(t *testing.T) {
	w := flatGroundWorld(t)
	w.Core.Peeps = []legacy.Peep{{Player: 1, Population: 100, AtPos: 2000, Flags: legacy.OnMove}, {Player: 1, Population: 100, AtPos: 2001, Flags: legacy.OnMove}, {Player: 0, Population: 100, AtPos: 2000, Flags: legacy.OnMove}}
	if !w.Cast(0, Plague, Target{X: 2000 % 64, Y: 2000 / 64}) {
		t.Fatal("enemy plague rejected")
	}
	if !w.Core.Peeps[0].Plague || w.Core.Peeps[1].Plague || w.Core.Peeps[2].Plague {
		t.Fatal("plague infected adjacent/friendly co-location")
	}
	ref := nativeActorReference(NativeFollowerPool, 0)
	a, _ := w.RecordImage.ReadFollowerEntry(ref)
	if a.Extra48 != 0xddc {
		t.Fatal("native overlay phase not initialized")
	}
	if frame, ok := w.PlagueFollowerFrame(0); !ok || len(frame.Layers) == 0 {
		t.Fatal("native plague overlay missing")
	}
	w.Core.Peeps[2].Player = 1
	if err := w.runNativeFollowerCall(func() error {
		return w.FollowerEntry.merge(ref, nativeActorReference(NativeFollowerPool, 2), w.nativeEntryCallbacks())
	}); err != nil {
		t.Fatal(err)
	}
	if !w.Core.Peeps[2].Plague || w.Core.Peeps[1].Plague {
		t.Fatal("actual friendly merge did not inherit plague")
	}
	if err := w.runNativeFollowerCall(func() error {
		_, err := w.CommonPrepass.Tick(nativeActorReference(NativeFollowerPool, 2), w.nativeCommonPrepassCallbacks())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	a, _ = w.RecordImage.ReadFollowerEntry(nativeActorReference(NativeFollowerPool, 2))
	if a.Extra48 != 0xde0 || a.Motion.Population != 200 {
		t.Fatal("native zero-damage plague clock differs")
	}
	if !w.Cast(0, Armageddon, Target{}) {
		t.Fatal("armageddon rejected")
	}
	owner, _ := w.RecordImage.Read8(nativeActorReference(NativeFollowerPool, 2), 12)
	if owner != 0 || !w.Heroes[1].Active || w.Core.War || w.NativeRaiseEnabled != 1 {
		t.Fatal("native plague cleanup/global hero conversion differs")
	}
}

func TestLegacyPlagueAndWarSaveMigrationUsesNativeState(t *testing.T) {
	w, err := NewWorld(testBundle(t), 0, true)
	if err != nil {
		t.Fatal(err)
	}
	s := w.Snapshot()
	s.Version = 21
	s.Core.War = true
	s.Core.Peeps[0].Plague = true
	copy, err := Restore(testBundle(t), s)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := copy.RecordImage.ReadFollowerEntry(52)
	if copy.Core.War || copy.NativeRaiseEnabled != 1 || a.Motion.Flags&16 == 0 || a.Extra48 != 0xddc || !s.Core.War {
		t.Fatal("legacy state did not migrate without mutating input")
	}
	before := encodeSnapshot(t, copy)
	copy, err = ReadSave(testBundle(t), bytes.NewReader(before))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, encodeSnapshot(t, copy)) {
		t.Fatal("native migrated plague/armageddon save changed")
	}
}
