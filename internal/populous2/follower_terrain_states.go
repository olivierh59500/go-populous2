package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

// FollowerTerrainRules contains the original $11d1a, $11e3a and $11caa
// animation banks and constants. The caller executes $12c3c once before
// dispatching these states; these handlers do not repeat that prepass.
type FollowerTerrainRules struct {
	Properties     [256]uint16
	WaterDeath     [6]uint16
	BurnShift      uint16
	Frames         map[int]AnimationFrame
	AnimationWords []uint16
	Aftermath      FollowerAftermathRules
}

func DecodeFollowerTerrainRules(exe *amiga.Executable) (FollowerTerrainRules, error) {
	var r FollowerTerrainRules
	p, err := DecodeCommonPrepassRules(exe)
	if err != nil {
		return r, err
	}
	r.Properties, r.WaterDeath, r.AnimationWords = p.Properties, p.Swimming, p.AnimationWords
	r.BurnShift = binary.BigEndian.Uint16(exe.Hunks[0].Data[0x20d54:])
	r.Aftermath, err = DecodeFollowerAftermathRules(exe)
	if err != nil {
		return r, err
	}
	r.Frames = make(map[int]AnimationFrame)
	starts := []int{0x7dc, 0x196c, 0xbd8, 0xc0c, 0x564}
	for _, table := range [][6]uint16{p.Drowning, p.Swimming, p.Conversion, p.Burning} {
		for _, pointer := range table {
			if pointer != 0 {
				starts = append(starts, int(pointer))
			}
		}
	}
	for _, start := range starts {
		frames, err := DecodeAnimation(exe, start)
		if err != nil {
			return FollowerTerrainRules{}, err
		}
		for index, frame := range frames {
			r.Frames[start+index*4] = frame
		}
	}
	return r, nil
}

type FollowerTerrainCallbacks struct {
	Read     func(NativeRecordReference) (FollowerEntryActor, error)
	Write    func(NativeRecordReference, FollowerEntryActor) error
	Tile     func(NativePackedTile) (uint8, error)
	Scenario func(uint8) (uint16, error)
	// Attrition reads deity long$14(decimal20). Its signed SUB.L flags differ
	// from $130e4, which stores with MOVE.L before testing the result.
	Attrition func(uint8) (uint32, error)
	// SetWaterReference writes deity word$36(decimal54), even for a hero.
	SetWaterReference func(uint8, NativeRecordReference) error
	Cleanup           func(NativeRecordReference, uint16) error
	ClearLeader       func(NativeRecordReference) error
	// Move is $12518 with the complete fixed-point coordinates. It owns the
	// original graph splice and destination-header pressure increment.
	Move func(NativeRecordReference, uint16, uint16) error
}

type FollowerTerrainStep struct {
	CurrentTotal, NextFollower, Search, Removed, RetainedDeath bool
}

func (r *FollowerTerrainRules) next(animation int) (uint16, int16, error) {
	next := uint16(animation + 4)
	if next&1 != 0 || int(next)/2 >= len(r.AnimationWords) {
		return 0, 0, fmt.Errorf("native terrain animation outside bounded bank")
	}
	return next, int16(r.AnimationWords[int(next)/2]), nil
}

// TickWater translates state$16/$11d1a. A survivor on land branches to
// ordinary search in this update. A fatal swimmer starts the retained death
// and executes its first $1199e animation step in this same update.
func (r *FollowerTerrainRules) TickWater(ref NativeRecordReference, cb FollowerTerrainCallbacks) (FollowerTerrainStep, error) {
	var step FollowerTerrainStep
	if r == nil || cb.Read == nil || cb.Write == nil || cb.Scenario == nil {
		return step, fmt.Errorf("native water callbacks missing")
	}
	a, err := cb.Read(ref)
	if err != nil {
		return step, err
	}
	next, word, err := r.next(a.Motion.Animation)
	if err != nil {
		return step, err
	}
	if word < 0 {
		next += uint16(word)
	}
	a.Motion.Animation = int(next)
	if err := cb.Write(ref, a); err != nil {
		return step, err
	}
	scenario, err := cb.Scenario(a.Owner)
	if err != nil {
		return step, err
	}
	alive := false
	var population uint32
	if scenario&0x20 == 0 {
		if cb.Attrition == nil {
			return step, fmt.Errorf("native water attrition callback missing")
		}
		amount, err := cb.Attrition(a.Owner)
		if err != nil {
			return step, err
		}
		population = uint32(a.Motion.Population) - amount
		alive = int64(a.Motion.Population)-int64(int32(amount)) > 0
	}
	if !alive {
		a.Motion.State, a.Motion.Animation = 0x2e, 0x196c
		if a.Motion.Flags&2 != 0 {
			pointer, err := prepassHeroAnimation(a, 0x196c, r.WaterDeath)
			if err != nil {
				return step, err
			}
			a.Motion.Animation = int(pointer)
		}
		if err := cb.Write(ref, a); err != nil {
			return step, err
		}
		if cb.Cleanup == nil {
			return step, fmt.Errorf("native water retained cleanup missing")
		}
		if err := cb.Cleanup(ref, 1); err != nil {
			return step, err
		}
		after, err := r.Aftermath.Tick(ref, FollowerAftermathCallbacks{Read: cb.Read, Write: cb.Write, Cleanup: cb.Cleanup})
		step.CurrentTotal, step.NextFollower, step.Removed, step.RetainedDeath = after.CurrentTotal, after.NextFollower, after.Removed, true
		return step, err
	}
	a.Motion.Population = int32(population)
	if err := cb.Write(ref, a); err != nil {
		return step, err
	}
	if cb.Tile == nil {
		return step, fmt.Errorf("native water tile callback missing")
	}
	tile, err := cb.Tile(entryTile(a))
	if err != nil {
		return step, err
	}
	if r.Properties[tile]&8 == 0 {
		a.Motion.Kind, a.Motion.State, a.Motion.Animation = 2, 2, 0
		step.Search = true
		return step, cb.Write(ref, a)
	}
	if cb.SetWaterReference == nil {
		return step, fmt.Errorf("native water deity-reference callback missing")
	}
	step.CurrentTotal = true
	return step, cb.SetWaterReference(a.Owner, ref)
}

func conversionAxis(position uint16, velocity int16) uint16 {
	delta := int8(0)
	if velocity > 0 {
		delta = 1
	} else if velocity < 0 {
		delta = -1
	}
	cell := uint8(position >> 8)
	next := cell + uint8(delta)
	if int8(next) < 0 || int8(next) >= 64 {
		next = cell
	}
	return uint16(next)<<8 | position&0xff
}

// TickConversion translates state$36/$11e3a. Completion flips the owner,
// advances each cell coordinate by its velocity sign while retaining both
// fractions, then executes native graph movement and ordinary search.
func (r *FollowerTerrainRules) TickConversion(ref NativeRecordReference, cb FollowerTerrainCallbacks) (FollowerTerrainStep, error) {
	var step FollowerTerrainStep
	if r == nil || cb.Read == nil || cb.Write == nil {
		return step, fmt.Errorf("native conversion callbacks missing")
	}
	a, err := cb.Read(ref)
	if err != nil {
		return step, err
	}
	next, word, err := r.next(a.Motion.Animation)
	if err != nil {
		return step, err
	}
	if word >= 0 {
		a.Motion.Animation = int(next)
		step.NextFollower = true
		return step, cb.Write(ref, a)
	}
	if a.Motion.Flags&1 != 0 {
		if cb.ClearLeader == nil {
			return step, fmt.Errorf("native conversion leader callback missing")
		}
		if err := cb.ClearLeader(ref); err != nil {
			return step, err
		}
		a, err = cb.Read(ref)
		if err != nil {
			return step, err
		}
	}
	if a.Owner == 1 {
		a.Owner = 2
	} else {
		a.Owner = 1
	}
	a.Motion.Kind, a.Motion.State, a.Motion.Animation = 2, 2, 0
	if err := cb.Write(ref, a); err != nil {
		return step, err
	}
	if cb.Move == nil {
		return step, fmt.Errorf("native conversion movement callback missing")
	}
	x, y := conversionAxis(uint16(a.Motion.X), a.Motion.VX), conversionAxis(uint16(a.Motion.Y), a.Motion.VY)
	step.Search = true
	return step, cb.Move(ref, x, y)
}

// TickBurning translates state$3c/$11caa. The original damage is a logical
// population shift followed by ADD.W, preserving the shifted high word. Death
// performs complete cleanup0; both paths still reach $123b4 current total.
func (r *FollowerTerrainRules) TickBurning(ref NativeRecordReference, cb FollowerTerrainCallbacks) (FollowerTerrainStep, error) {
	var step FollowerTerrainStep
	if r == nil || cb.Read == nil || cb.Write == nil {
		return step, fmt.Errorf("native burning callbacks missing")
	}
	a, err := cb.Read(ref)
	if err != nil {
		return step, err
	}
	next, word, err := r.next(a.Motion.Animation)
	if err != nil {
		return step, err
	}
	if word < 0 {
		next += uint16(word)
	}
	a.Motion.Animation = int(next)
	if err := cb.Write(ref, a); err != nil {
		return step, err
	}
	count := r.BurnShift & 63
	damage := uint32(a.Motion.Population) >> count
	damage = damage&0xffff0000 | uint32(uint16(damage)+r.BurnShift)
	previous := a.Motion.Population
	a.Motion.Population = int32(uint32(previous) - damage)
	if err := cb.Write(ref, a); err != nil {
		return step, err
	}
	step.CurrentTotal = true
	if int64(previous)-int64(int32(damage)) <= 0 {
		if cb.Cleanup == nil {
			return step, fmt.Errorf("native burning cleanup missing")
		}
		step.Removed = true
		return step, cb.Cleanup(ref, 0)
	}
	return step, nil
}
