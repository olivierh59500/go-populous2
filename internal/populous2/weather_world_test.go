package populous2

import (
	"bytes"
	"testing"
)

func TestWorldStormCreatesCloudsAndRetainsCommandCallerAlias(t *testing.T) {
	w := lightningWorld(t, 4311)
	w.Core.Magnets[0].Mana = 1000000
	caller := 0xeb56
	alias := caller + 26
	if err := w.nativeCleanupMemory().Write16(alias, 0xbeef); err != nil {
		t.Fatal(err)
	}
	before := w.Core.Magnets[0].Mana
	cost := w.ManaCost(0, Storm)
	if !w.Cast(0, Storm, Target{X: 32, Y: 32}) || w.Core.Magnets[0].Mana != before-cost {
		t.Fatal("native storm admitted cast/debit differs")
	}
	value, err := w.nativeCleanupMemory().Read16(alias)
	if err != nil || value != 0 {
		t.Fatal("storm cleared a fabricated caller instead of native command memory")
	}
	clouds := 0
	for index, actor := range w.NativeEffects {
		if w.NativeEnvironment[index] == NativeEnvironmentStorm {
			clouds++
			if !actor.Active || actor.Kind != 0x36 || !w.Occupancy.Effects[index].Linked {
				t.Fatal("native storm cloud is not linked/owned")
			}
		}
	}
	if clouds == 0 || len(w.Effects) != 0 {
		t.Fatal("storm retained generic radius-damage effect")
	}
	restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	for range 30 {
		w.tickNativeEffects()
		restored.tickNativeEffects()
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
		t.Fatal("storm cloud/flash/command save continuation differs")
	}
}

func TestWorldStormPartialPoolCreationKeepsCloudsWithoutDebit(t *testing.T) {
	w := lightningWorld(t, 4311)
	w.Core.Magnets[0].Mana = 1000000
	for index := 1; index < NativeEffectCapacity; index++ {
		w.NativeEffects[index] = NativeEffectActor{Active: true, Kind: 0x22, Player: 0, X: 8320, Y: 8320, State: 2, Animation: 0x1a0, Speed: 16, Life: 200}
		w.linkEffect(index)
	}
	before := w.Core.Magnets[0].Mana
	if w.Cast(0, Storm, Target{X: 32, Y: 32}) || w.Core.Magnets[0].Mana != before || w.NativeEnvironment[0] != NativeEnvironmentStorm {
		t.Fatal("native partial storm admission/debit/created cloud differs")
	}
}

func TestWorldFireRainStartsUnlinkedThenImpactsThroughNativeDamage(t *testing.T) {
	w := lightningWorld(t, 4311)
	w.Core.Magnets[0].Mana = 1000000
	before := w.Core.Magnets[0].Mana
	cost := w.ManaCost(0, FireRain)
	if !w.Cast(0, FireRain, Target{X: 32, Y: 32}) || w.Core.Magnets[0].Mana != before-cost {
		t.Fatal("native fire rain admitted command/debit differs")
	}
	meteors := 0
	for index, actor := range w.NativeEffects {
		if w.NativeEnvironment[index] == NativeEnvironmentFireRain {
			meteors++
			if actor.State != 0x1c || w.Occupancy.Effects[index].Linked {
				t.Fatal("new native meteor acquired occupancy before delay")
			}
		}
	}
	if meteors == 0 || len(w.Effects) != 0 {
		t.Fatal("fire rain substituted a generic timed effect")
	}
	restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	for range 80 {
		w.tickNativeEffects()
		restored.tickNativeEffects()
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
		t.Fatal("meteor delay/falling/impact save continuation differs")
	}
	for _, controller := range w.NativeEnvironment {
		if controller == NativeEnvironmentFireRain {
			t.Fatal("native meteor terminal did not release its slot")
		}
	}
}

func TestNativeCommandImagePreservesBoundedAliasesAcrossSave(t *testing.T) {
	w := lightningWorld(t, 4311)
	memory := w.nativeCleanupMemory()
	if err := memory.Write32(0xeb70, 0x12345678); err != nil {
		t.Fatal(err)
	}
	if err := memory.Write32(0x1127e, 0xffffffff); err == nil {
		t.Fatal("command image allowed partial out-of-bounds write")
	}
	value, err := memory.Read32(0xeb70)
	if err != nil || value != 0x12345678 {
		t.Fatal("command alias was changed by rejected write")
	}
	restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	value, err = restored.nativeCleanupMemory().Read32(0xeb70)
	if err != nil || value != 0x12345678 {
		t.Fatal("native command alias bytes were regenerated during save")
	}
}

func TestWorldWeatherRunsInNormalSimulationLoop(t *testing.T) {
	w := lightningWorld(t, 4311)
	w.NativeGameMode = 8 // This isolated controller fixture has no followers.
	w.Core.Magnets[0].Mana = 1000000
	if !w.Cast(0, Storm, Target{X: 32, Y: 32}) || !w.Cast(0, FireRain, Target{X: 32, Y: 32}) {
		t.Fatal("native weather command setup failed")
	}
	w.Core.Computer[0].Mode, w.Core.Computer[1].Mode = 0, 0
	for range 300 {
		w.Tick()
	}
	for _, controller := range w.NativeEnvironment {
		if controller == NativeEnvironmentStorm || controller == NativeEnvironmentFireRain {
			t.Fatal("normal loop did not advance native weather to its terminal")
		}
	}
	if err := w.Occupancy.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestNativeMeteorIsHiddenDuringDelayAndUsesDecodedFallLayers(t *testing.T) {
	w := lightningWorld(t, 4311)
	w.Core.Magnets[0].Mana = 1000000
	if !w.Cast(0, FireRain, Target{X: 32, Y: 32}) {
		t.Fatal("native meteor setup failed")
	}
	if _, ok := w.EnvironmentalEffectFrame(0); ok {
		t.Fatal("unlinked delay phase rendered a meteor")
	}
	at := 0xc800
	if err := w.nativeCleanupMemory().Write16(at+20, 0); err != nil {
		t.Fatal(err)
	}
	w.tickNativeEnvironment(0)
	frame, ok := w.EnvironmentalEffectFrame(0)
	if !ok || len(frame.Layers) == 0 || frame.Layers[0].Y != -104 || !w.Occupancy.Effects[0].Linked {
		t.Fatalf("native first visible fall frame/offset differs: %+v", frame)
	}
}
