package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeWallRules struct {
	Art      WallRules
	Crossing FollowerCrossingRules
	Hero     FollowerHeroRules
	Motion   FollowerMotionRules
	code     []byte
}

func DecodeNativeWallRules(exe *amiga.Executable) (NativeWallRules, error) {
	var r NativeWallRules
	var err error
	if exe == nil || len(exe.Hunks) == 0 {
		return r, fmt.Errorf("native wall CODE missing")
	}
	r.Art, err = DecodeWallRules(exe)
	if err != nil {
		return r, err
	}
	r.Crossing, err = DecodeFollowerCrossingRules(exe)
	if err != nil {
		return r, err
	}
	r.Hero, err = DecodeFollowerHeroRules(exe)
	if err != nil {
		return r, err
	}
	r.Motion, err = DecodeFollowerMotionRules(exe)
	if err != nil {
		return r, err
	}
	r.code = exe.Hunks[0].Data
	return r, nil
}

// NativeWallPlacementState retains the four original CODE:$1647c scratch
// pointers. A linked wall whose coordinates mismatch does not overwrite its
// scratch entry, so a later cast may still mutate the previously selected wall.
type NativeWallPlacementState struct{ Neighbors [4]NativeRecordReference }

type NativeWallCallbacks struct {
	Memory       FollowerCleanupMemory
	SourceD7     uint16 // $16270 replaces only the low mask byte.
	Link, Unlink func(NativeRecordReference) error
	Move         func(NativeRecordReference, uint16, uint16) (bool, error)
	Enter        func(NativeRecordReference) error // Complete $1275a after a committed crossing.
}

type NativeWallCreation struct {
	Reference            NativeRecordReference
	Created, HeadChanged bool
}
type NativeWallPass struct{ Visited, Advanced, Removed int }
type NativeWallFollowerStep struct {
	Moving, Crossed, Blocked, WallBroken, ReadyNextUpdate, Redispatch bool
	ReturnState                                                       uint8
}

func (r *NativeWallRules) word(at int) (uint16, error) {
	if r == nil || at < 0 || at&1 != 0 || at+2 > len(r.code) {
		return 0, fmt.Errorf("native wall CODE word outside retained image: %x", at)
	}
	return binary.BigEndian.Uint16(r.code[at:]), nil
}

// Create translates $1626c, including pre-admission deity head changes, actual
// linked neighbor searches, stale scratch pointers, stage changes and gates.
func (r *NativeWallRules) Create(owner uint16, x, y uint8, state *NativeWallPlacementState, cb NativeWallCallbacks) (NativeWallCreation, error) {
	var result NativeWallCreation
	if r == nil || state == nil || !winMemoryValid(cb.Memory) || cb.Link == nil {
		return result, fmt.Errorf("native wall creation callbacks missing")
	}
	m := nativeWhirlwindMemory{m: cb.Memory}
	packed := uint16(y)<<8 | uint16(x)
	grid := nativeWhirlwindGrid(packed)
	tile := m.byte(grid + 1)
	mask := cb.SourceD7&0xff00 | 0x45
	if uint8(owner) == 1 {
		mask = cb.SourceD7&0xff00 | 0x43
	}
	if r.Art.Properties[tile]&mask == 0 {
		return result, m.err
	}
	seen := map[uint16]bool{}
	for head := m.word(grid + 2); head != 0 && m.err == nil; {
		if seen[head] {
			return result, fmt.Errorf("cyclic native wall placement chain")
		}
		seen[head] = true
		at := cleanupRecordAddress(NativeRecordReference(head))
		if m.byte(at) == 0x1a {
			return result, m.err
		}
		head = m.word(at + 2)
	}
	if m.err != nil {
		return result, m.err
	}
	at, err := primitiveFreeRecord(cb.Memory, 0x5f50, 0x6bd0, 16)
	if err != nil || at == 0 {
		return result, err
	}
	ref := NativeRecordReference(uint16(at - 0x76c0))
	result.Reference = ref
	god := heroGodAddress(uint8(owner))
	m.putWord(at+14, m.word(god+16))
	m.putWord(god+16, uint16(ref))
	result.HeadChanged = m.err == nil
	// The two byte doublings and later byte shift retain X modulo64.
	packed = packed&0xff00 | uint16(x&63)
	connections := uint16(0)
	cursor := 0
	for _, delta := range r.Art.Offsets {
		connections <<= 1
		candidate := packed + uint16(int16(delta[0]+delta[1]*256))
		if candidate&0xc0c0 != 0 {
			state.Neighbors[cursor] = 0
			cursor++
			continue
		}
		head := m.word(nativeWhirlwindGrid(candidate) + 2)
		seen = map[uint16]bool{}
		matched := false
		for head != 0 && m.err == nil {
			if seen[head] {
				return result, fmt.Errorf("cyclic native wall neighbor chain")
			}
			seen[head] = true
			target := cleanupRecordAddress(NativeRecordReference(head))
			if m.byte(target) == 0x1a {
				actual := m.word(target+8)&0xff00 | uint16(m.byte(target+6))
				if actual == candidate {
					connections |= 1
					state.Neighbors[cursor] = NativeRecordReference(head)
					cursor++
				}
				matched = true
				break
			}
			head = m.word(target + 2)
		}
		if !matched {
			state.Neighbors[cursor] = 0
			cursor++
		}
	}
	if m.err != nil {
		return result, m.err
	}
	if connections == 0 && m.word(at+14) != 0 {
		return result, m.err
	}
	art := r.Art.Art[connections]
	m.putByte(at, 0x1a)
	m.putByte(at+1, art.Variant)
	m.putByte(at+12, uint8(owner))
	m.putWord(at+10, uint16(art.Animation))
	m.putWord(at+8, packed)
	m.putByte(at+6, uint8(packed))
	m.putByte(at+7, 128)
	m.putByte(at+9, 128)
	m.putWord(0xf2e, m.word(0xf2e)+1)
	if m.err != nil {
		return result, m.err
	}
	if err := cb.Link(ref); err != nil {
		return result, err
	}
	for _, neighbor := range state.Neighbors {
		if neighbor == 0 || m.byte(at+1) == 0 {
			continue
		}
		target := cleanupRecordAddress(neighbor)
		switch m.byte(target + 1) {
		case 2:
			if m.word(at+10) == 0x5cc {
				continue
			}
		case 4:
			if m.word(at+10) == 0x5dc {
				continue
			}
		default:
			continue
		}
		m.putWord(target+10, 0x5bc)
		m.putByte(target+1, 8)
	}
	tile = m.byte(nativeWhirlwindGrid(packed) + 1)
	if r.Art.Properties[tile]&0x40 != 0 {
		m.putWord(at+10, 0xb44)
		m.putByte(at+1, 8)
		roadAt := 0x1691c + int(int16(uint16(tile)-197))
		if roadAt < 0 || roadAt >= len(r.code) {
			return result, fmt.Errorf("native wall road byte outside CODE")
		}
		if r.code[roadAt]&5 != 0 {
			m.putWord(at+10, 0xb54)
			m.putByte(at+1, 6)
		}
	}
	result.Created = m.err == nil
	return result, m.err
}

// TickPool is complete $161cc. Broken walls retain their terminal phase and
// graph allocation on suitable terrain. Removal adjusts only a matching deity
// head; it never repairs an interior or stale per-owner list reference.
func (r *NativeWallRules) TickPool(cb NativeWallCallbacks) (NativeWallPass, error) {
	var step NativeWallPass
	if r == nil || !winMemoryValid(cb.Memory) || cb.Unlink == nil {
		return step, fmt.Errorf("native wall pool callbacks missing")
	}
	m := nativeWhirlwindMemory{m: cb.Memory}
	for at := 0x5f50; at < 0x6bd0; at += 16 {
		owner := m.byte(at + 12)
		if m.err != nil {
			return step, m.err
		}
		if owner == 0 {
			continue
		}
		step.Visited++
		packed := m.word(at+8)&0xff00 | uint16(m.byte(at+6))
		if r.Art.Properties[m.byte(nativeWhirlwindGrid(packed)+1)]&0x77 == 0 {
			m.putWord(0xf2e, m.word(0xf2e)+1)
			god := heroGodAddress(owner)
			ref := NativeRecordReference(uint16(at - 0x76c0))
			if m.word(god+16) == uint16(ref) {
				m.putWord(god+16, m.word(at+14))
			}
			m.putByte(at+12, 0)
			if m.err != nil {
				return step, m.err
			}
			if err := cb.Unlink(ref); err != nil {
				return step, err
			}
			step.Removed++
			continue
		}
		next := m.word(at+10) + 4
		word, err := r.word(0x23d1a + int(int16(next)))
		if m.err != nil {
			return step, m.err
		}
		if err != nil {
			return step, err
		}
		if int16(word) >= 0 {
			m.putWord(at+10, next)
			step.Advanced++
		}
		if m.err != nil {
			return step, m.err
		}
	}
	return step, m.err
}

func (r *NativeWallRules) admit(ref NativeRecordReference, x, y uint16, cb NativeWallCallbacks) (FollowerCrossingStep, error) {
	// The proven crossing helper owns ordinary admission. Preserve the earlier
	// kind write on an odd wall-stage fault, which occurs before the CODE read.
	step, err := r.Crossing.Admit(ref, x, y, FollowerCrossingCallbacks{Memory: cb.Memory})
	if err == nil {
		return step, nil
	}
	at := cleanupRecordAddress(ref)
	state, e := cb.Memory.Read8(at + 22)
	if e != nil || state != 0x2a {
		return step, err
	}
	head, e := cb.Memory.Read16(nativeWhirlwindGrid(nativeOccupancyPackedTile(x, y)) + 2)
	if e != nil {
		return step, err
	}
	seen := map[uint16]bool{}
	for head != 0 && !seen[head] {
		seen[head] = true
		target := cleanupRecordAddress(NativeRecordReference(head))
		kind, e := cb.Memory.Read8(target)
		if e != nil {
			return step, err
		}
		if kind == 0x1a {
			stage, e := cb.Memory.Read8(target + 1)
			if e == nil && stage&1 != 0 {
				_ = cb.Memory.Write8(target, 0x1c)
			}
			return step, err
		}
		head, e = cb.Memory.Read16(target + 2)
		if e != nil {
			return step, err
		}
	}
	return step, err
}

// TickFollower executes ordinary state4 crossing/climbing and state2a attack.
// Climbing has the ordinary walking image bank and coordinates; no synthetic
// altitude or dedicated state is introduced. Attack leaves velocity and XY
// intact and advances its first native frame in the breaking update.
func (r *NativeWallRules) TickFollower(ref NativeRecordReference, cb NativeWallCallbacks) (NativeWallFollowerStep, error) {
	var step NativeWallFollowerStep
	if r == nil || !winMemoryValid(cb.Memory) || cb.Move == nil || cb.Enter == nil {
		return step, fmt.Errorf("native wall follower callbacks missing")
	}
	m := nativeWhirlwindMemory{m: cb.Memory}
	at := cleanupRecordAddress(ref)
	if m.byte(at+12) == 0 {
		return step, m.err
	}
	attack := func() error {
		next := m.word(at+10) + 4
		marker, err := r.word(0x23d1a + int(int16(next)))
		if m.err != nil {
			return m.err
		}
		if err != nil {
			return err
		}
		if int16(marker) < 0 {
			m.putByte(at, 2)
			m.putWord(at+10, 0)
			m.putByte(at+22, 2)
			step.ReadyNextUpdate = true
		} else {
			m.putWord(at+10, next)
		}
		return m.err
	}
	state := m.byte(at + 22)
	if state == 0x2a {
		err := attack()
		return step, err
	}
	if state != 4 {
		return step, fmt.Errorf("native wall follower state %02x unsupported", state)
	}
	next := m.word(at+10) + 4
	marker, err := r.word(0x23d1a + int(int16(next)))
	if m.err != nil {
		return step, m.err
	}
	if err != nil {
		return step, err
	}
	if int16(marker) < 0 {
		next = 0
	}
	m.putWord(at+10, next)
	timer := m.word(at + 20)
	m.putWord(at+20, timer-1)
	if int16(timer) < 1 {
		step.ReturnState = m.byte(at + 23)
		m.putByte(at+22, step.ReturnState)
		step.Redispatch = true
		return step, m.err
	}
	oldX, oldY := m.word(at+6), m.word(at+8)
	x, y := oldX+m.word(at+14), oldY+m.word(at+16)
	if m.err != nil {
		return step, m.err
	}
	if nativeOccupancyPackedTile(x, y) == nativeOccupancyPackedTile(oldX, oldY) {
		m.putWord(at+6, x)
		m.putWord(at+8, y)
		step.Moving = true
		return step, m.err
	}
	admission, err := r.admit(ref, x, y, cb)
	if err != nil {
		return step, err
	}
	if admission.WallBroken {
		step.WallBroken = true
		err := attack()
		return step, err
	}
	if !admission.Admitted {
		vx, vy := int16(m.word(at+14)), int16(m.word(at+16))
		speed := int16(m.byte(at + 18))
		direction := r.Motion.Bounce[(followerMotionSign(int(vy))+1)*3+followerMotionSign(int(vx))+1]
		m.putWord(at+14, uint16(int16(direction[0])*speed))
		m.putWord(at+16, uint16(int16(direction[1])*speed))
		step.Blocked = true
		return step, m.err
	}
	if _, err := cb.Move(ref, x, y); err != nil {
		return step, err
	}
	step.Moving, step.Crossed = true, true
	return step, cb.Enter(ref)
}

// Frame retains the original wall/attack image offset or the ordinary walking
// bank chosen by $e65c. Wall climbing uses these same walking layers and cues.
func (r *NativeWallRules) Frame(ref NativeRecordReference, memory FollowerCleanupMemory) (AnimationFrame, uint16, error) {
	if r == nil || !winMemoryValid(memory) {
		return AnimationFrame{}, 0, fmt.Errorf("native wall frame memory missing")
	}
	m := nativeWhirlwindMemory{m: memory}
	at := cleanupRecordAddress(ref)
	offset := m.word(at + 10)
	kind, state := m.byte(at), m.byte(at+22)
	if kind == 2 && state != 0x2a {
		direction := int(r.Motion.Angle(int16(m.word(at+14)), int16(m.word(at+16))) >> 5)
		bank := 0x209e0
		if m.byte(at+12) != 1 {
			bank = 0x209f0
		}
		selector := m.word(at + 50)
		if m.byte(at+13)&2 != 0 {
			bank = 0x20a00
			selector = m.word(at + 40)
		}
		base, err := r.word(bank + int(int16(selector)))
		if m.err != nil {
			return AnimationFrame{}, 0, m.err
		}
		if err != nil {
			return AnimationFrame{}, 0, err
		}
		offset += base + uint16(r.Motion.DirectionOffsets[direction])
	}
	if m.err != nil {
		return AnimationFrame{}, 0, m.err
	}
	image, err := r.word(0x23d1a + int(int16(offset)))
	if err != nil {
		return AnimationFrame{}, offset, err
	}
	if int16(image) < 0 {
		return AnimationFrame{}, offset, fmt.Errorf("native wall image points at animation marker")
	}
	layers, err := decodeImageLayers(r.code, image)
	if err != nil {
		return AnimationFrame{}, offset, err
	}
	cue, err := r.word(0x23d1a + int(int16(offset)) + 2)
	if err != nil {
		return AnimationFrame{}, offset, err
	}
	if cue >= 0x532 {
		cue = 0
	}
	if cue%10 != 0 {
		return AnimationFrame{}, offset, fmt.Errorf("native wall image cue is unaligned")
	}
	return AnimationFrame{Layers: layers, SoundCue: int(cue) / 10}, offset, nil
}
