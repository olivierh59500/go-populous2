package populous2

import (
	"encoding/binary"
	"fmt"
	"go-populous2/internal/amiga"
)

const NativeEffectCapacity = 250

// NativeEffectActor retains the simulation fields of a native 32-byte effect
// record. Position and velocity are signed 8.8 values, not whole tile deltas.
type NativeEffectActor struct {
	Active      bool
	Kind        uint8
	Player      uint8
	X, Y        int16
	VX, VY      int16
	Speed       uint8
	Timer, Life int16
	State       uint8
	Animation   int
}

type FireColumnRules struct {
	BaseLife        int
	Speed           uint8
	Jitter          [9][2]int
	Neighbors       [16][2]int
	Vectors         [16][2]int16
	Frames          map[int]AnimationFrame
	SequenceLengths map[int]int
	HeroDeath       [6]int
	TownDeath       [TownStages]int
	TownClearCount  [TownStages]uint8
}

func DecodeFireColumnRules(exe *amiga.Executable) (FireColumnRules, error) {
	var r FireColumnRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x23d1a+0x2bd4+4 {
		return r, fmt.Errorf("native fire column tables missing")
	}
	code := exe.Hunks[0].Data
	r.BaseLife = int(binary.BigEndian.Uint16(code[0x20d44:]))
	r.Speed = code[0x20d47]
	if r.BaseLife < 1 || r.BaseLife > 32000 || r.Speed == 0 {
		return r, fmt.Errorf("invalid native fire column lifetime/speed")
	}
	decodeOffset := func(at int) [2]int {
		n := int(int16(binary.BigEndian.Uint16(code[at:])))
		x := int(int8(byte(n)))
		return [2]int{x, (n - x) / 256}
	}
	for i := range r.Jitter {
		r.Jitter[i] = decodeOffset(0x20d7a + i*2)
	}
	for i := range r.Neighbors {
		r.Neighbors[i] = decodeOffset(0x20dd6 + i*2)
		r.Vectors[i] = [2]int16{int16(binary.BigEndian.Uint16(code[0x20df6+i*4:])), int16(binary.BigEndian.Uint16(code[0x20df6+i*4+2:]))}
	}
	r.Frames = make(map[int]AnimationFrame)
	r.SequenceLengths = make(map[int]int)
	for _, start := range []int{0x1a0, 0x4b8, 0x660} {
		frames, err := DecodeAnimation(exe, start)
		if err != nil {
			return FireColumnRules{}, err
		}
		r.SequenceLengths[start] = len(frames)
		for i, frame := range frames {
			r.Frames[start+i*4] = frame
		}
	}
	for i := range r.HeroDeath {
		r.HeroDeath[i] = int(binary.BigEndian.Uint16(code[0x20a24+i*2:]))
	}
	for i := range r.TownDeath {
		r.TownDeath[i] = int(binary.BigEndian.Uint16(code[0x20b3a+i*2:]))
		r.TownClearCount[i] = code[0x13654+i] + 1
	}
	for _, start := range append([]int{0x178, 0x2bd4}, r.TownDeath[:]...) {
		if _, ok := r.SequenceLengths[start]; ok {
			continue
		}
		frames, err := DecodeAnimation(exe, start)
		if err != nil {
			return FireColumnRules{}, err
		}
		r.SequenceLengths[start] = len(frames)
		for i, frame := range frames {
			r.Frames[start+i*4] = frame
		}
	}
	return r, nil
}

// castFireColumn follows $15b7c: a jittered 3x3 position, first free shared
// slot and two random draws on success. Occupied or water tiles are valid.
func (w *World) castFireColumn(player, x, y int) bool {
	created := false
	err := w.runNativeFollowerCall(func() error {
		step, err := w.PrimitiveCreators.CreateFireColumn(uint16(player+1), uint8(x), uint8(y), w.nativePrimitiveCallbacks())
		created = step.Created
		return err
	})
	if err != nil {
		panic(err)
	}
	return created
}

func (w *World) tickNativeEffects() {
	for i := range w.NativeEffects {
		a := &w.NativeEffects[i]
		if w.NativeEnvironment[i] != NativeEnvironmentNone {
			if err := w.tickNativeEnvironment(i); err != nil {
				panic(err)
			}
			continue
		}
		if !a.Active {
			continue
		}
		wasLinked, _ := w.Occupancy.Linked(nativeActorReference(NativeEffectPool, i))
		switch a.Kind {
		case 0x22:
			if err := w.tickRawFireColumn(i); err != nil {
				panic(err)
			}
			continue
		case 0x20:
			if err := w.tickRawWhirlwind(i); err != nil {
				panic(err)
			}
			continue // The complete native call hydrated its record and children.
		case FungusActorKind:
			if err := w.tickRawFungus(i); err != nil {
				panic(err)
			}
			continue
		case WhirlpoolActorKind:
			w.tickWhirlpool(a)
		case BasaltActorKind:
			w.tickBasalt(i)
		case 0x28, 0x2a:
			if err := w.LightningRules.Tick(&w.LightningState, &w.NativeEffects, i, w.lightningCallbacks()); err != nil {
				panic(err)
			}
			if a.Active {
				w.moveActor(NativeEffectPool, i, uint16(a.X), uint16(a.Y))
			}
		}
		if wasLinked && (a.Kind == 0x20 || a.Kind == 0x22) {
			if !a.Active {
				w.unlinkActor(NativeEffectPool, i)
			} else {
				w.moveActor(NativeEffectPool, i, uint16(a.X), uint16(a.Y))
			}
		}
		w.projectNativeEffectRecord(i)
	}
}
