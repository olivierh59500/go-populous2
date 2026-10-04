package populous2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	legacy "go-populous2/internal/legacy"
)

func TestWhirlwindWorldUsesOriginalController(t *testing.T) {
	data, err := os.ReadFile("testdata/whirlwind_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Fixtures []nativeWhirlwindFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Fixtures {
		t.Run(fmt.Sprintf("%s-xp%d", fixture.Name, fixture.Experience), func(t *testing.T) {
			w := flatGroundWorld(t)
			w.NativeEffects = [NativeEffectCapacity]NativeEffectActor{}
			w.Scenery = [SceneryCapacity]SceneryActor{}
			w.rebuildSceneryIndex()
			w.Marks = [4096]Mark{}
			if fixture.Name == "water" {
				w.Core.Alt = [legacy.EndWidth * legacy.EndWidth]int{}
				w.Core.MapBlk = [4096]byte{}
			} else if fixture.Name == "uphill" {
				for y := 0; y <= 64; y++ {
					for x := 0; x <= 64; x++ {
						w.Core.Alt[x+y*65] = 1 + clamp(x-28, 0, 7)
					}
				}
			}
			core := w.Core.Snapshot()
			core.RNG = fixture.Seed
			w.Core = legacy.WorldFromSnapshot(core, w.Core.Rules)
			w.Experience[0][Air] = fixture.Experience
			before := w.Core.Magnets[0].Mana
			cost := w.ManaCost(0, Whirlwind)
			if !w.Cast(0, Whirlwind, Target{X: fixture.Target[0], Y: fixture.Target[1]}) {
				t.Fatal("native creation was not dispatched")
			}
			if w.Core.Magnets[0].Mana != before-cost || len(w.Effects) != 0 {
				t.Fatal("native cast debit or shared effect allocation is wrong")
			}
			assertNativeFireState(t, w, fixture.Initial, fixture.InitialRNG, fixture.InitialTilesSHA256, 0)
			for _, step := range fixture.Trace {
				w.tickNativeEffects()
				assertNativeFireState(t, w, step.Actor, step.RNG, step.TilesSHA256, step.Tick)
			}
		})
	}
}

func TestWhirlwindDoesNotSubstituteAreaDamageOrFire(t *testing.T) {
	w := flatGroundWorld(t)
	w.Scenery = [SceneryCapacity]SceneryActor{}
	w.rebuildSceneryIndex()
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 100, AtPos: 32 + 32*64, Flags: legacy.OnMove}, {Player: 1, Population: 250, AtPos: 32 + 32*64, Flags: legacy.InTown}}
	people := append([]legacy.Peep(nil), w.Core.Peeps...)
	altitudes, blocks, marks := w.Core.Alt, w.Core.MapBlk, w.Marks
	if !w.Cast(0, Whirlwind, Target{X: 32, Y: 32}) {
		t.Fatal("funded whirlwind rejected")
	}
	for range 80 {
		w.tickEffects()
		w.tickNativeEffects()
	}
	if !reflect.DeepEqual(people, w.Core.Peeps) || w.Core.Alt != altitudes || w.Core.MapBlk != blocks || w.Marks != marks || len(w.FlameDeaths) != 0 {
		t.Fatal("unported pickup/town interactions were replaced with generic damage or burning")
	}
	if len(w.Effects) != 0 || w.NativeEffects[0].Kind != 0x20 {
		t.Fatal("whirlwind retained the provisional moving-effect controller")
	}
}

func TestWhirlwindSharedPoolFailureIsAtomic(t *testing.T) {
	w := flatGroundWorld(t)
	for index := range w.NativeEffects {
		w.NativeEffects[index] = NativeEffectActor{Active: true, Kind: 0x22, Player: uint8(index % 2), X: 8320, Y: 8320, State: 2, Animation: 0x1a0, Speed: 16, Life: 200}
	}
	actors, mana, rng, serial := w.NativeEffects, w.Core.Magnets[0].Mana, w.Core.Snapshot().RNG, w.SpellSerial
	if w.Cast(0, Whirlwind, Target{X: 32, Y: 32}) || w.NativeEffects != actors || w.Core.Magnets[0].Mana != mana || w.Core.Snapshot().RNG != rng || w.SpellSerial != serial {
		t.Fatal("full shared pool consumed mana, randomness, or existing actors")
	}
	w.NativeEffects[23].Active = false
	w.NativeEffects[23].VX, w.NativeEffects[23].VY = -19, 71
	if !w.Cast(0, Whirlwind, Target{X: 32, Y: 32}) || w.NativeEffects[23].Kind != 0x20 || w.NativeEffects[23].VX != -19 || w.NativeEffects[23].VY != 71 {
		t.Fatal("first free slot did not preserve the original velocity words")
	}
}

func TestWhirlwindAndFlameSavedContinuation(t *testing.T) {
	w := flatGroundWorld(t)
	if !w.Cast(0, Whirlwind, Target{X: 32, Y: 32}) || !w.Cast(0, FireColumn, Target{X: 40, Y: 40}) {
		t.Fatal("mixed native effect creation rejected")
	}
	for range 37 {
		w.tickNativeEffects()
	}
	restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	for range 140 {
		w.tickNativeEffects()
		restored.tickNativeEffects()
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
		t.Fatal("save/load changed mixed native motion, frames, timer, or randomness")
	}
}

func TestWhirlwindNativeAnimationBanksAndSaveValidation(t *testing.T) {
	b := testBundle(t)
	for animation, expected := range b.Whirlwinds.Frames {
		frame, ok := b.NativeEffectFrame(NativeEffectActor{Kind: 0x20, Animation: animation})
		if !ok || !reflect.DeepEqual(frame, expected) || frame.SoundCue < 0 || frame.SoundCue >= len(b.Audio.Patterns) {
			t.Fatal("renderer/audio did not select the decoded whirlwind bank")
		}
		for landscape := range b.Sprites {
			for _, layer := range frame.Layers {
				if layer.Sprite < 0 || layer.Sprite >= len(b.Sprites[landscape]) {
					t.Fatal("native effect layer does not exist in a landscape sprite bank")
				}
			}
		}
	}
	w := flatGroundWorld(t)
	if !w.Cast(0, Whirlwind, Target{X: 32, Y: 32}) {
		t.Fatal("native cast rejected")
	}
	for _, change := range []func(*NativeEffectActor){
		func(a *NativeEffectActor) { a.Animation = 0x4b8 },
		func(a *NativeEffectActor) { a.State = 4 },
		func(a *NativeEffectActor) { a.State = 12 },
		func(a *NativeEffectActor) { a.Kind = 0x22 },
		func(a *NativeEffectActor) { a.X = -1 },
	} {
		s := w.Snapshot()
		change(&s.NativeEffects[0])
		if _, err := Restore(b, s); err == nil {
			t.Fatal("invalid native phase, animation bank, or coordinate was accepted")
		}
	}
}

func TestWhirlwindPrototypeSaveMigration(t *testing.T) {
	w := flatGroundWorld(t)
	s := w.Snapshot()
	s.Version = 8
	s.Effects = []Effect{{Spell: Whirlwind, Player: 1, X: 32, Y: 32, DX: -1, DY: 1, Life: 15}, {Spell: Storm, Player: 0, X: 32, Y: 32, Life: 10}}
	before := append([]Effect(nil), s.Effects...)
	restored, err := Restore(testBundle(t), s)
	if err != nil {
		t.Fatal(err)
	}
	a := restored.NativeEffects[0]
	if len(restored.Effects) != 1 || restored.Effects[0].Spell != Storm || !a.Active || a.Kind != 0x20 || a.State != 10 || a.Player != 1 || a.Life != 15 || a.VX != -24 || a.VY != 24 || !reflect.DeepEqual(before, s.Effects) {
		t.Fatal("prototype migration changed the caller's data or lost motion/life")
	}
	for index := range s.NativeEffects {
		s.NativeEffects[index] = NativeEffectActor{Active: true, Kind: 0x22, X: 8320, Y: 8320, Speed: 16, State: 2, Animation: 0x1a0, Life: 200}
	}
	if _, err := Restore(testBundle(t), s); err == nil {
		t.Fatal("prototype migration silently discarded an effect beyond pool capacity")
	}
	s.Version = SaveVersion
	if _, err := Restore(testBundle(t), s); err == nil {
		t.Fatal("current save format accepted the obsolete generic whirlwind controller")
	}
}
