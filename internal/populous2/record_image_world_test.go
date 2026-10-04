package populous2

import (
	"testing"

	legacy "go-populous2/internal/legacy"
)

func TestNativeFollowerImagePatchSurvivesOrdinaryControllerRead(t *testing.T) {
	w := lightningWorld(t, 4311)
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 1000, AtPos: 32 + 32*64, Flags: legacy.OnMove, MovementSpeed: 20}}
	w.initializeNativeFollower(0)
	w.refreshNativeRecordImage()
	ref := nativeActorReference(NativeFollowerPool, 0)
	for _, field := range []struct {
		offset int
		value  uint16
	}{{10, 0x6a0}, {14, 0xffec}, {16, 20}, {20, 7}, {50, 12}} {
		patch, err := w.RecordImage.Write16(ref, field.offset, field.value)
		if err != nil {
			t.Fatal(err)
		}
		w.hydrateNativePatch(patch)
	}
	patch, err := w.RecordImage.Write8(ref, 22, 4)
	if err != nil {
		t.Fatal(err)
	}
	w.hydrateNativePatch(patch)
	w.refreshNativeRecordImage()
	actor, err := w.readEntryRecord(ref)
	if err != nil {
		t.Fatal(err)
	}
	if actor.Motion.Animation != 0x6a0 || actor.Motion.VX != -20 || actor.Motion.VY != 20 || actor.Motion.Timer != 7 || actor.Motion.Variant != 12 || actor.Motion.State != 4 {
		t.Fatal("ordinary controller restored stale fields over a retained image patch")
	}
	raw, err := w.RecordImage.ReadFollowerEntry(ref)
	if err != nil || raw.Motion != actor.Motion {
		t.Fatalf("retained image differs after controller refresh: %v", err)
	}
}

func TestNativeEffectImageRetainsFinalBasaltWrites(t *testing.T) {
	w := oceanWorld(t, 4311)
	if !w.castBasalt(0, 32, 32, 0) {
		t.Fatal("controlled basalt creation failed")
	}
	w.NativeEffects[0].Life = 1
	w.refreshNativeRecordImage()
	w.tickNativeEffects()
	w.refreshNativeRecordImage()
	ref := nativeActorReference(NativeEffectPool, 0)
	life, err := w.RecordImage.Read16(ref, 24)
	if err != nil {
		t.Fatal(err)
	}
	state, err := w.RecordImage.Read8(ref, 22)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := w.RecordImage.Read8(ref, 12)
	if err != nil {
		t.Fatal(err)
	}
	if w.NativeEffects[0].Active || life != 0 || state != 0x3a || owner != 0 {
		t.Fatalf("retired basalt image lost final writes: life %d state %x owner %d", life, state, owner)
	}
}

func TestNativeEntryProjectionTracksDemotedAndStruckTown(t *testing.T) {
	w := lightningWorld(t, 4311)
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 1000, AtPos: 32 + 32*64, Flags: legacy.InTown, MovementSpeed: 20, TownStage: 18}}
	w.initializeNativeFollower(0)
	ref := nativeActorReference(NativeFollowerPool, 0)
	w.refreshNativeRecordImage()
	v, ok := w.lightningRecord(ref)
	if !ok {
		t.Fatal("town record missing")
	}
	v.Kind, v.State, v.Animation = 4, 0x1e, 0x744
	w.setLightningRecord(ref, v)
	w.refreshNativeRecordImage()
	a, err := w.RecordImage.ReadFollowerEntry(ref)
	if err != nil || a.Motion.Kind != 4 || a.Motion.State != 0x1e || a.Motion.Animation != 0x744 {
		t.Fatalf("retained image lost the lightning town state: %v", err)
	}
	w.LightningVictims[0].Active = false
	w.Core.Peeps[0].Flags = legacy.OnMove
	w.refreshNativeRecordImage()
	a, err = w.RecordImage.ReadFollowerEntry(ref)
	if err != nil || a.Motion.Kind != 2 || a.Motion.State != 2 || a.Byte1 != 0 {
		t.Fatalf("retained image still exposes a demoted town: %v", err)
	}
}

func TestNativeEntryProjectionResetsWhenFollowerSlotIsReused(t *testing.T) {
	w := lightningWorld(t, 4311)
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 1000, AtPos: 32 + 32*64, Flags: legacy.OnMove, MovementSpeed: 20}, {Player: 0}}
	w.NativeEntries[1] = NativeFollowerEntry{Initialized: true, Managed: true, Actor: FollowerEntryActor{Owner: 1, Contact30: 123, Motion: FollowerMotionActor{Kind: 4, State: 12}}}
	if slot := w.Core.AllocateHeroClone(0); slot != 1 {
		t.Fatal("expected the inactive follower slot to be reused")
	}
	a, err := w.readEntryRecord(nativeActorReference(NativeFollowerPool, 1))
	if err != nil {
		t.Fatal(err)
	}
	if a.Motion.Kind != 2 || a.Contact30 != 0 || w.NativeEntries[1].Managed {
		t.Fatal("reused follower retained another generation's entry state")
	}
}
