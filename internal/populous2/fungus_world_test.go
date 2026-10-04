package populous2

import (
	"bytes"
	"testing"

	legacy "go-populous2/internal/legacy"
)

func TestFungusCastPlantsOneNativeSeedAndRetainsPendingWait(t *testing.T) {
	w := flatGroundWorld(t)
	w.Experience[0][Plants] = 96
	before, rng := w.Core.Magnets[0].Mana, w.Core.Snapshot().RNG
	cost := w.ManaCost(0, Fungus)
	if !w.Cast(0, Fungus, Target{X: 32, Y: 32}) {
		t.Fatal("funded fungus cast rejected")
	}
	if w.Core.Magnets[0].Mana != before-cost || w.Core.Snapshot().RNG != rng || w.nativeTileAt(32, 32) != 145 {
		t.Fatal("native fungus seed/debit/randomness differs")
	}
	seeds := 0
	for _, mark := range w.Marks {
		if mark.NativeTile == 145 {
			seeds++
		}
	}
	if seeds != 1 || w.FungusState.Pending[0] != 1 || w.NativeEffects[0].Speed != 7 {
		t.Fatal("fungus became a filled disk or lost its experience cadence")
	}
	for range 25 {
		w.tickNativeEffects()
	}
	wait := w.NativeEffects[0].Timer
	w.Experience[0][Plants] = 255
	if !w.Cast(0, Fungus, Target{X: 33, Y: 32}) || w.NativeEffects[0].Timer != wait || w.NativeEffects[0].Speed != 7 || w.NativeEffects[1].Active {
		t.Fatal("recasting reset the pending controller or allocated another one")
	}
	if _, ok := testBundle(t).NativeEffectFrame(w.NativeEffects[0]); ok {
		t.Fatal("map-only fungus controller was rendered as a moving actor")
	}
}

func TestFungusCollectionAndNativeBlinkerThroughWorld(t *testing.T) {
	w := flatGroundWorld(t)
	for _, x := range []int{31, 32, 33} {
		if !w.Cast(0, Fungus, Target{X: x, Y: 32}) {
			t.Fatal("blinker planting rejected")
		}
	}
	for range 99 {
		w.tickNativeEffects()
	}
	if w.NativeEffects[0].State != FungusCollecting || w.FungusState.Pending[0] != 1 {
		t.Fatal("native collection ended early")
	}
	w.tickNativeEffects()
	if w.NativeEffects[0].State != FungusEvolving || w.FungusState.Pending[0] != 0 || w.nativeTileAt(32, 32) != 146 {
		t.Fatal("collection transition did not fall through into first aging")
	}
	for range 10 {
		w.tickNativeEffects()
	}
	for _, want := range []struct {
		x, y int
		code uint8
	}{{32, 31, 145}, {32, 32, 149}, {32, 33, 145}, {31, 32, 150}, {33, 32, 150}} {
		if w.nativeTileAt(want.x, want.y) != want.code {
			t.Fatalf("native blinker differs at %d,%d: got %d want %d", want.x, want.y, w.nativeTileAt(want.x, want.y), want.code)
		}
	}
	if !w.Cast(0, Fungus, Target{X: 45, Y: 45}) || w.FungusState.Pending[0] != 2 || !w.NativeEffects[0].Active || !w.NativeEffects[1].Active {
		t.Fatal("evolving controller prevented another collection phase")
	}
}

func TestFungusFullPoolAndIneligibleTerrainStillConsumeNativeCast(t *testing.T) {
	w := flatGroundWorld(t)
	for index := range w.NativeEffects {
		w.NativeEffects[index] = NativeEffectActor{Active: true, Kind: 0x22, Player: 0, X: 8320, Y: 8320, Speed: 16, State: 2, Animation: 0x1a0, Life: 200}
	}
	pool, before := w.NativeEffects, w.Core.Magnets[0].Mana
	cost := w.ManaCost(0, Fungus)
	if !w.Cast(0, Fungus, Target{X: 32, Y: 32}) || w.nativeTileAt(32, 32) != 145 || w.NativeEffects != pool || w.Core.Magnets[0].Mana != before-cost {
		t.Fatal("full pool prevented native pre-allocation planting/debit")
	}
	w.Core.Alt = [65 * 65]int{}
	w.Marks = [4096]Mark{}
	before = w.Core.Magnets[0].Mana
	if !w.Cast(0, Fungus, Target{X: 32, Y: 32}) || w.Marks[32+32*64].NativeTile != 0 || w.Core.Magnets[0].Mana != before-cost {
		t.Fatal("native command changed water or made mana conditional on success")
	}
}

func TestFungusDoesNotUseRandomAreaDamage(t *testing.T) {
	w := flatGroundWorld(t)
	w.Core.Peeps = []legacy.Peep{{Player: 1, Population: 100, AtPos: 32 + 32*64, Flags: legacy.OnMove}}
	if !w.Cast(0, Fungus, Target{X: 32, Y: 32}) {
		t.Fatal("funded cast rejected")
	}
	rng := w.Core.Snapshot().RNG
	for range 140 {
		w.tickEffects()
		w.tickNativeEffects()
	}
	if w.Core.Peeps[0].Population != 100 || w.Core.Snapshot().RNG != rng {
		t.Fatal("fungus automaton substituted generic random damage/spread")
	}
}

func TestFungusSavedContinuationAndReferenceValidation(t *testing.T) {
	w := flatGroundWorld(t)
	for _, x := range []int{31, 32, 33} {
		w.Cast(0, Fungus, Target{X: x, Y: 32})
	}
	for range 42 {
		w.tickNativeEffects()
	}
	restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	for range 180 {
		w.tickNativeEffects()
		restored.tickNativeEffects()
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
		t.Fatal("fungus wait, bounds, pending links or native tile stages changed after load")
	}
	for _, mutate := range []func(*Snapshot){
		func(s *Snapshot) { s.FungusState.Pending[0] = NativeEffectCapacity + 1 },
		func(s *Snapshot) { s.FungusState.Pending[1] = 1 },
		func(s *Snapshot) { s.NativeEffects[0].Speed = 0 },
		func(s *Snapshot) { s.NativeEffects[0].State = 4 },
	} {
		s := w.Snapshot()
		mutate(&s)
		if _, err := Restore(testBundle(t), s); err == nil {
			t.Fatal("invalid native fungus controller accepted")
		}
	}
}

func TestFungusPrototypeMarksMigrateWithoutRandomness(t *testing.T) {
	w := flatGroundWorld(t)
	s := w.Snapshot()
	s.Version = 9
	s.Marks[32+32*64] = Mark{Spell: Fungus, Player: 0, Life: 240}
	s.Marks[33+32*64] = s.Marks[32+32*64]
	rng := s.Core.RNG
	restored, err := Restore(testBundle(t), s)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Core.Snapshot().RNG != rng || restored.FungusState.Pending[0] != 1 || restored.nativeTileAt(32, 32) != 145 || restored.nativeTileAt(33, 32) != 145 || s.Marks[32+32*64].NativeTile != 0 {
		t.Fatal("prototype migration lost seeds, consumed randomness or mutated caller data")
	}
}
