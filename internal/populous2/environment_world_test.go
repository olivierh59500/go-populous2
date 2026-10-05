package populous2

import (
	"bytes"
	"testing"
)

func TestWorldEarthquakeRetainsRawKindAndDirectedControllerThroughSave(t *testing.T) {
	w := lightningWorld(t, 4311)
	w.Core.Magnets[0].Mana = 1000000
	ref := nativeActorReference(NativeEffectPool, 0)
	w.patchNativeByte(ref, 0, 0x22)
	w.patchNativeWord(ref, 14, 91)
	w.patchNativeWord(ref, 16, 0xffe5)
	cost := w.ManaCost(0, Earthquake)
	if !w.Cast(0, Earthquake, Target{X: 32, Y: 32, Direction: 1}) || w.Core.Magnets[0].Mana != 1000000-cost {
		t.Fatal("native quake cast/debit failed")
	}
	kind, _ := w.RecordImage.Read8(ref, 0)
	vx, _ := w.RecordImage.Read16(ref, 14)
	direction, _ := w.RecordImage.Read8(ref, 26)
	if kind != 0x22 || vx != 91 || direction != w.EarthquakeRules.Directions[1] || w.NativeEnvironment[0] != NativeEnvironmentQuake || w.Occupancy.Effects[0].Linked {
		t.Fatal("quake regenerated stale bytes or selected a linked fire controller")
	}
	restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	for range 120 {
		w.tickNativeEffects()
		restored.tickNativeEffects()
	}
	if w.NativeEnvironmentDirty == 0 {
		t.Fatal("native quake runtime never painted/faded its terrain")
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
		t.Fatal("raw directed quake save continuation differs")
	}
}

func TestNativeQuakeFullPoolStillChargesCast(t *testing.T) {
	w := lightningWorld(t, 4311)
	w.Core.Magnets[0].Mana = 1000000
	for index := range w.NativeEffects {
		w.NativeEffects[index] = NativeEffectActor{Active: true, Kind: 0x22, Player: 0, X: 8320, Y: 8320, State: 2, Animation: 0x1a0, Speed: 16, Life: 200}
		w.linkEffect(index)
	}
	before := w.Core.Magnets[0].Mana
	cost := w.ManaCost(0, Earthquake)
	if !w.Cast(0, Earthquake, Target{X: 32, Y: 32}) || w.Core.Magnets[0].Mana != before-cost {
		t.Fatal("native quake full-pool command lost its debit")
	}
	for _, controller := range w.NativeEnvironment {
		if controller != NativeEnvironmentNone {
			t.Fatal("full-pool quake stole an occupied slot")
		}
	}
}

func TestWorldVolcanoEruptsViaRealRawFireAndLavaCreators(t *testing.T) {
	w := lightningWorld(t, 4311)
	w.Core.Magnets[0].Mana = 1000000
	before := w.Core.Magnets[0].Mana
	cost := w.ManaCost(0, Volcano)
	if !w.Cast(0, Volcano, Target{X: 32, Y: 32}) || w.Core.Magnets[0].Mana != before-cost || w.NativeEnvironment[0] != NativeEnvironmentVolcano {
		t.Fatal("native volcano command/owner failed")
	}
	if w.Occupancy.Effects[0].Linked {
		t.Fatal("native volcano controller acquired invented occupancy")
	}
	restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	for range 6 {
		w.tickNativeEffects()
		restored.tickNativeEffects()
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
		t.Fatal("native volcano eruption save continuation differs")
	}
	columns, lava := 0, 0
	for index, actor := range w.NativeEffects {
		if actor.Active && actor.Kind == 0x22 && w.NativeEnvironment[index] == NativeEnvironmentNone {
			columns++
		}
		if w.NativeEnvironment[index] == NativeEnvironmentLava {
			lava++
		}
	}
	if columns == 0 || lava == 0 {
		t.Fatalf("native volcano omitted original eruption children: columns%d lava%d controller%d rawactor%+v", columns, lava, w.NativeEnvironment[0], w.NativeEffects[0])
	}
	if err := w.Occupancy.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestQuakeSaveIgnoresRecycledBasaltAndFungusKind(t *testing.T) {
	for _, kind := range []uint8{BasaltActorKind, FungusActorKind} {
		t.Run(string(rune(kind)), func(t *testing.T) {
			w := lightningWorld(t, 4311)
			ref := nativeActorReference(NativeEffectPool, 0)
			w.patchNativeByte(ref, 0, kind)
			w.patchNativeByte(ref, 27, 255)
			if !w.castNativeEarthquake(0, 32, 32, 1) {
				t.Fatal("controlled quake did not allocate")
			}
			if _, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w))); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestInactiveWaterQuakeDoesNotLeaveOwnedControllerTag(t *testing.T) {
	w := oceanWorld(t, 4311)
	if w.castNativeEarthquake(0, 32, 32, 0) || w.NativeEnvironment[0] != NativeEnvironmentNone {
		t.Fatal("water-rejected native quake left an active tag")
	}
	if _, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w))); err != nil {
		t.Fatal(err)
	}
}

func TestEnvironmentalSaveRejectsControllerStateMismatch(t *testing.T) {
	w := lightningWorld(t, 4311)
	if !w.castNativeEarthquake(0, 32, 32, 0) {
		t.Fatal("controlled quake allocation failed")
	}
	snapshot := w.Snapshot()
	snapshot.NativeEnvironment[0] = NativeEnvironmentVolcano
	if _, err := Restore(testBundle(t), snapshot); err == nil {
		t.Fatal("save accepted quake bytes assigned to a volcano controller")
	}
}
