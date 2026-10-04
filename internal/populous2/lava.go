package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

const NativeLavaKind uint8 = 0x38

type NativeLavaRules struct {
	Geometry                [256]uint8
	Animations              [16]int16
	FlatAnimations          [4]uint16
	Forward, Behind         [4]uint16
	Vectors                 [4][2]int16
	Burning                 [6]uint16
	DelayModulus, PushSpeed uint16
	Frames                  map[int]AnimationFrame
}

func DecodeNativeLavaRules(exe *amiga.Executable) (NativeLavaRules, error) {
	var r NativeLavaRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33612 {
		return r, fmt.Errorf("native lava tables missing")
	}
	code := exe.Hunks[0].Data
	copy(r.Geometry[:], code[0x33512:0x33612])
	for i := range r.Animations {
		r.Animations[i] = int16(binary.BigEndian.Uint16(code[0x1718a+i*2:]))
	}
	for i := range r.FlatAnimations {
		r.FlatAnimations[i] = binary.BigEndian.Uint16(code[0x171aa+i*2:])
		r.Forward[i] = binary.BigEndian.Uint16(code[0x171b6+i*2:])
		r.Behind[i] = binary.BigEndian.Uint16(code[0x171be+i*2:])
		r.Vectors[i] = [2]int16{int16(binary.BigEndian.Uint16(code[0x17126+i*4:])), int16(binary.BigEndian.Uint16(code[0x17128+i*4:]))}
	}
	for i := range r.Burning {
		r.Burning[i] = binary.BigEndian.Uint16(code[0x20a18+i*2:])
	}
	r.DelayModulus, r.PushSpeed = binary.BigEndian.Uint16(code[0x171b4:]), binary.BigEndian.Uint16(code[0x17124:])
	if r.DelayModulus == 0 {
		return r, fmt.Errorf("native lava delay divisor zero")
	}
	r.Frames = make(map[int]AnimationFrame)
	starts := []int{0x564, 0xf10}
	for _, pointer := range r.Animations {
		if pointer > 0 {
			starts = append(starts, int(pointer))
		}
	}
	for _, table := range [][]uint16{r.FlatAnimations[:], r.Burning[:]} {
		for _, pointer := range table {
			if pointer != 0 {
				starts = append(starts, int(pointer))
			}
		}
	}
	for _, start := range starts {
		frames, err := DecodeAnimation(exe, start)
		if err != nil {
			return NativeLavaRules{}, err
		}
		for i, frame := range frames {
			r.Frames[start+i*4] = frame
		}
	}
	return r, nil
}

type NativeLavaCallbacks struct {
	Memory       FollowerCleanupMemory
	Cell         func(NativePackedTile) (NativeOccupancyCell, error)
	Random       func() uint16
	Link, Unlink func(NativeRecordReference) error
	Move         func(NativeRecordReference, uint16, uint16) error
	// CreateBasalt is original $171ea with base lifetime, without Water XP.
	// The lava creator ignores its admission return and still returns1.
	CreateBasalt func(uint8, NativePackedTile, uint16) error
	Scorch       func(NativeRecordReference) error
	DestroyTown  func(NativeRecordReference) error
	Clock        uint16
}

func lavaDirection(direction uint16) (int, error) {
	if direction&1 != 0 || direction > 6 {
		return 0, fmt.Errorf("native lava direction outside table")
	}
	return int(direction / 2), nil
}

// Create translates $16f20. Its original return is1 for success/basalt,
// -1 for an existing lava actor or unsupported slope, and0 for bounds/pool
// rejection. Unsupported slopes clear the candidate's four XY bytes only.
func (r *NativeLavaRules) Create(owner uint8, packed NativePackedTile, direction uint16, cb NativeLavaCallbacks) (int16, error) {
	if r == nil || !winMemoryValid(cb.Memory) || cb.Cell == nil || cb.Random == nil || cb.Link == nil {
		return 0, fmt.Errorf("native lava creator callbacks missing")
	}
	dir, err := lavaDirection(direction)
	if err != nil {
		return 0, err
	}
	if uint16(packed)&0xc0c0 != 0 {
		return 0, nil
	}
	cell, err := cb.Cell(packed)
	if err != nil {
		return 0, err
	}
	found, err := r.containsLava(cell.Head, cb.Memory)
	if err != nil {
		return 0, err
	}
	if found {
		return -1, nil
	}
	shape := r.Geometry[cell.Tile] & 15
	if shape == 0 {
		if cb.CreateBasalt == nil {
			return 0, fmt.Errorf("native lava basalt callback missing")
		}
		return 1, cb.CreateBasalt(owner, packed, direction)
	}
	at, err := primitiveFreeRecord(cb.Memory, 0xc800, 0xe740, 32)
	if err != nil || at == 0 {
		return 0, err
	}
	if err := cb.Memory.Write8(at+6, uint8(packed)); err != nil {
		return 0, err
	}
	if err := cb.Memory.Write16(at+8, uint16(packed)); err != nil {
		return 0, err
	}
	animation := r.Animations[shape]
	if animation == 0 {
		return -1, cb.Memory.Write32(at+6, 0)
	}
	if animation < 0 {
		animation = int16(r.FlatAnimations[dir])
	}
	for _, w := range []struct {
		Offset int
		Value  uint16
	}{{10, uint16(animation)}, {26, direction}} {
		if err := cb.Memory.Write16(at+w.Offset, w.Value); err != nil {
			return 0, err
		}
	}
	for _, w := range []struct {
		Offset int
		Value  uint8
	}{{12, owner}, {7, 0}, {9, 0}, {0, NativeLavaKind}} {
		if err := cb.Memory.Write8(at+w.Offset, w.Value); err != nil {
			return 0, err
		}
	}
	delay := cb.Random()%r.DelayModulus + 1
	if err := cb.Memory.Write16(at+20, delay); err != nil {
		return 0, err
	}
	if err := cb.Memory.Write16(at+24, delay); err != nil {
		return 0, err
	}
	if err := cb.Memory.Write8(at+22, 0x34); err != nil {
		return 0, err
	}
	return 1, cb.Link(NativeRecordReference(at - 0x76c0))
}

func (r *NativeLavaRules) containsLava(ref NativeRecordReference, m FollowerCleanupMemory) (bool, error) {
	seen := map[NativeRecordReference]bool{}
	for ref != 0 {
		if seen[ref] {
			return false, fmt.Errorf("cyclic native lava cell chain")
		}
		seen[ref] = true
		at := 0x76c0 + int(int16(ref))
		kind, err := m.Read8(at)
		if err != nil {
			return false, err
		}
		if kind == NativeLavaKind {
			return true, nil
		}
		next, err := m.Read16(at + 2)
		if err != nil {
			return false, err
		}
		ref = NativeRecordReference(next)
	}
	return false, nil
}

type NativeLavaStep struct {
	Finished, Propagated bool
	ChildResult          int16
}

// Tick translates $1576a/$15898. Child allocation precedes the parent's life
// update. List traversal reads each target's next link after its native push,
// preserving the original traversal across changed cell chains.
func (r *NativeLavaRules) Tick(ref NativeRecordReference, cb NativeLavaCallbacks) (NativeLavaStep, error) {
	var step NativeLavaStep
	if r == nil || !winMemoryValid(cb.Memory) || cb.Cell == nil || cb.Unlink == nil {
		return step, fmt.Errorf("native lava controller callbacks missing")
	}
	at, err := volcanoAddress(ref)
	if err != nil {
		return step, err
	}
	phase, err := cb.Memory.Read8(at + 22)
	if err != nil {
		return step, err
	}
	finish := func() (NativeLavaStep, error) {
		step.Finished = true
		if err := cb.Memory.Write8(at+22, 0x36); err != nil {
			return step, err
		}
		if err := cb.Memory.Write8(at+12, 0); err != nil {
			return step, err
		}
		return step, cb.Unlink(ref)
	}
	if phase == 0x36 {
		step.Finished = true
		if err := cb.Memory.Write8(at+12, 0); err != nil {
			return step, err
		}
		return step, cb.Unlink(ref)
	}
	if phase != 0x34 {
		return step, fmt.Errorf("native lava phase outside controller")
	}
	x, err := cb.Memory.Read8(at + 6)
	if err != nil {
		return step, err
	}
	y, err := cb.Memory.Read8(at + 8)
	if err != nil {
		return step, err
	}
	origin := uint16(y)<<8 | uint16(x)
	direction, err := cb.Memory.Read16(at + 26)
	if err != nil {
		return step, err
	}
	dir, err := lavaDirection(direction)
	if err != nil {
		return step, err
	}
	timer, err := cb.Memory.Read16(at + 20)
	if err != nil {
		return step, err
	}
	timer--
	if err := cb.Memory.Write16(at+20, timer); err != nil {
		return step, err
	}
	if timer == 0 {
		owner, err := cb.Memory.Read8(at + 12)
		if err != nil {
			return step, err
		}
		step.Propagated = true
		step.ChildResult, err = r.Create(owner, NativePackedTile(origin+r.Forward[dir]), direction, cb)
		if err != nil {
			return step, err
		}
		if step.ChildResult != 0 {
			if err := cb.Memory.Write16(at+20, r.DelayModulus*2); err != nil {
				return step, err
			}
		}
	}
	cell, err := cb.Cell(NativePackedTile(origin))
	if err != nil {
		return step, err
	}
	animation := r.Animations[r.Geometry[cell.Tile]&15]
	if animation == 0 {
		return finish()
	}
	if animation < 0 {
		animation = int16(r.FlatAnimations[dir])
	}
	linear := int(x) + int(y)*64
	frame := uint16(animation) + (cb.Clock<<2+uint16(0xf44+linear*4))&4
	if err := cb.Memory.Write16(at+10, frame); err != nil {
		return step, err
	}
	life, err := cb.Memory.Read16(at + 24)
	if err != nil {
		return step, err
	}
	if err := cb.Memory.Write16(at+24, life-1); err != nil {
		return step, err
	}
	if int16(life) <= 1 {
		if cb.Scorch == nil {
			return step, fmt.Errorf("native lava scorch callback missing")
		}
		if err := cb.Scorch(ref); err != nil {
			return step, err
		}
		if err := cb.Memory.Write16(at+24, 3); err != nil {
			return step, err
		}
		behind := origin + r.Behind[dir]
		if behind&0xc0c0 != 0 {
			return finish()
		}
		back, err := cb.Cell(NativePackedTile(behind))
		if err != nil {
			return step, err
		}
		if back.Tile != 0xdc {
			found, err := r.containsLava(back.Head, cb.Memory)
			if err != nil {
				return step, err
			}
			if !found {
				return finish()
			}
		}
	}
	current := cell.Head
	for visits := 0; current != 0; visits++ {
		if visits > 1053 {
			return step, fmt.Errorf("native lava target traversal outside bounded pools")
		}
		if err := r.push(current, dir, cb); err != nil {
			return step, err
		}
		next, err := cb.Memory.Read16(0x76c0 + int(int16(current)) + 2)
		if err != nil {
			return step, err
		}
		current = NativeRecordReference(next)
	}
	return step, nil
}

func (r *NativeLavaRules) push(ref NativeRecordReference, dir int, cb NativeLavaCallbacks) error {
	m, at := cb.Memory, 0x76c0+int(int16(ref))
	kind, err := m.Read8(at)
	if err != nil {
		return err
	}
	switch {
	case kind == 4:
		state, err := m.Read8(at + 22)
		if err != nil {
			return err
		}
		if state != 0x3c {
			if cb.DestroyTown == nil {
				return fmt.Errorf("native lava town destruction callback missing")
			}
			if err := cb.DestroyTown(ref); err != nil {
				return err
			}
			if err := m.Write8(at+22, 0x3c); err != nil {
				return err
			}
		}
	case kind >= 2 && kind <= 0x12 && kind&1 == 0:
		state, err := m.Read8(at + 22)
		if err != nil {
			return err
		}
		if state == 0x3a {
			return nil
		}
		if state != 0x3c {
			animation := uint16(0x564)
			flags, err := m.Read8(at + 13)
			if err != nil {
				return err
			}
			if flags&2 != 0 {
				hero, err := m.Read16(at + 40)
				if err != nil {
					return err
				}
				if hero&1 != 0 || hero > 10 {
					return fmt.Errorf("native lava burning hero outside table")
				}
				animation = r.Burning[hero/2]
				if animation == 0 {
					return nil
				}
			}
			if err := m.Write16(at+10, animation); err != nil {
				return err
			}
			if err := m.Write8(at+22, 0x3c); err != nil {
				return err
			}
		}
	case kind == 0x16:
		if err := m.Write8(at, 0x1e); err != nil {
			return err
		}
		if err := m.Write16(at+10, 0xf10); err != nil {
			return err
		}
	case kind == 0x14 || kind == 0x18 || kind == 0x1a || kind == 0x1c || kind == 0x1e:
	case kind >= 0x20 && kind <= 0x40 && kind&1 == 0:
		return nil
	default:
		return fmt.Errorf("native lava contact kind outside dispatch table")
	}
	x, err := m.Read16(at + 6)
	if err != nil {
		return err
	}
	y, err := m.Read16(at + 8)
	if err != nil {
		return err
	}
	x += uint16(uint32(uint16(r.Vectors[dir][0])) * uint32(r.PushSpeed))
	y += uint16(uint32(uint16(r.Vectors[dir][1])) * uint32(r.PushSpeed))
	if int16(x) < 0 || int16(y) < 0 || int16(x) >= 0x4000 || int16(y) >= 0x4000 {
		if kind == 0x14 {
			return nil
		}
		if cb.Unlink == nil {
			return fmt.Errorf("native lava target unlink callback missing")
		}
		if err := m.Write8(at+12, 0); err != nil {
			return err
		}
		return cb.Unlink(ref)
	}
	if cb.Move == nil {
		return fmt.Errorf("native lava target movement callback missing")
	}
	return cb.Move(ref, x, y)
}
