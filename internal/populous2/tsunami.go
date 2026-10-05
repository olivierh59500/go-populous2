package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type TsunamiRules struct {
	Origins       [4]uint16
	Animations    [4]uint16
	Vectors       [4][2]int16
	Sides, Fronts [4][2]uint16
	Life, Speed   uint16
	Properties    [256]uint16
	ImageWords    []int16
	Frames        map[int]AnimationFrame
}

func DecodeTsunamiRules(exe *amiga.Executable) (TsunamiRules, error) {
	var rules TsunamiRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33512 {
		return rules, fmt.Errorf("native tsunami tables missing")
	}
	code := exe.Hunks[0].Data
	rules.Life = binary.BigEndian.Uint16(code[0x20d74:])
	rules.Speed = binary.BigEndian.Uint16(code[0x20d76:])
	for index := range rules.Origins {
		rules.Origins[index] = binary.BigEndian.Uint16(code[0x20d94+index*2:])
		rules.Animations[index] = binary.BigEndian.Uint16(code[0x20d9e+index*2:])
		for axis := range 2 {
			rules.Vectors[index][axis] = int16(binary.BigEndian.Uint16(code[0x20da6+index*4+axis*2:]))
			rules.Sides[index][axis] = binary.BigEndian.Uint16(code[0x20db6+index*4+axis*2:])
			rules.Fronts[index][axis] = binary.BigEndian.Uint16(code[0x20dc6+index*4+axis*2:])
		}
	}
	if binary.BigEndian.Uint16(code[0x20d9c:]) != 0xff9d {
		return TsunamiRules{}, fmt.Errorf("native tsunami origin sentinel missing")
	}
	for index := range rules.Properties {
		rules.Properties[index] = binary.BigEndian.Uint16(code[0x33312+index*2:])
	}
	for at := 0x23d1a; at < 0x26956; at += 2 {
		rules.ImageWords = append(rules.ImageWords, int16(binary.BigEndian.Uint16(code[at:])))
	}
	rules.Frames = make(map[int]AnimationFrame)
	for _, start := range rules.Animations {
		frames, err := DecodeAnimation(exe, int(start))
		if err != nil {
			return TsunamiRules{}, err
		}
		for index, frame := range frames {
			rules.Frames[int(start)+index*4] = frame
		}
	}
	return rules, nil
}

type TsunamiCallbacks struct {
	Memory       FollowerCleanupMemory
	Link, Unlink func(NativeRecordReference) error
	Move         func(NativeRecordReference, uint16, uint16) (bool, error) // Native D0 is1 when the tile changes.
	Lower        func(uint8, uint8) error                                  // Direct $d7f0, including propagation.
}

type TsunamiCreation struct {
	References []NativeRecordReference
	PoolFull   bool
}

func tsunamiGrid(packed uint16) int {
	return 0xf44 + int(int16(packed&0xff00|uint16(uint8(packed)<<2)))
}

// Create is command56/spell34's $16af4. Four adjacent water-property parcels
// start independent cardinal fronts. The origin need not be water. No RNG or
// direction input occurs, and the action handler always proceeds to its debit.
func (rules TsunamiRules) Create(owner uint16, x, y uint8, cb TsunamiCallbacks) (TsunamiCreation, error) {
	step := TsunamiCreation{References: []NativeRecordReference{}}
	m := cb.Memory
	if !winMemoryValid(m) || cb.Link == nil {
		return step, fmt.Errorf("native tsunami creator callbacks missing")
	}
	origin := uint16(y)<<8 | uint16(x)
	for direction, offset := range rules.Origins {
		packed := origin + offset
		if packed&0xc0c0 != 0 {
			continue
		}
		tile, err := m.Read8(tsunamiGrid(packed) + 1)
		if err != nil {
			return step, err
		}
		if rules.Properties[tile]&8 == 0 {
			continue
		}
		address, err := primitiveFreeRecord(m, 0xc800, 0xe740, 32)
		if err != nil {
			return step, err
		}
		if address == 0 {
			step.PoolFull = true
			return step, nil
		}
		if err := m.Write8(address+12, uint8(owner)); err != nil {
			return step, err
		}
		if err := m.Write8(address+6, uint8(packed)); err != nil {
			return step, err
		}
		if err := m.Write16(address+8, packed); err != nil {
			return step, err
		}
		if err := m.Write8(address+7, 0); err != nil {
			return step, err
		}
		if err := m.Write8(address+9, 0); err != nil {
			return step, err
		}
		if err := m.Write8(address, 0x32); err != nil {
			return step, err
		}
		if err := m.Write16(address+10, rules.Animations[direction]); err != nil {
			return step, err
		}
		if err := m.Write16(address+26, uint16(direction*4)); err != nil {
			return step, err
		}
		if err := m.Write8(address+22, 0x2a); err != nil {
			return step, err
		}
		if err := m.Write16(address+24, rules.Life); err != nil {
			return step, err
		}
		if int16(owner) <= 2 {
			// $16bb4 reads WaterXP and shifts it, then $16bba replaces
			// that result with fixed speed. Preserve the native read boundary.
			if _, err := m.Read8(primitiveDeityAddress(owner) + 0x57); err != nil {
				return step, err
			}
		}
		for axis, vector := range rules.Vectors[direction] {
			if int8(uint8(vector)) < 0 {
				if err := m.Write8(address+7+axis*2, uint8(vector)); err != nil {
					return step, err
				}
			}
			if err := m.Write16(address+14+axis*2, uint16(vector*int16(rules.Speed))); err != nil {
				return step, err
			}
		}
		ref := NativeRecordReference(uint16(address - 0x76c0))
		if err := cb.Link(ref); err != nil {
			return step, err
		}
		step.References = append(step.References, ref)
	}
	return step, nil
}

type TsunamiStep struct {
	Skipped, Removed, Moved, CellChanged bool
	Lowered                              [][2]uint8
	Children                             []NativeRecordReference
}

func (rules TsunamiRules) nextAnimation(animation uint16) (uint16, error) {
	next := uint16(animation + 4)
	if next&1 != 0 || int(next)/2 >= len(rules.ImageWords) {
		return 0, fmt.Errorf("native tsunami animation outside bounded bank")
	}
	if loop := rules.ImageWords[next/2]; loop < 0 {
		next += uint16(loop)
	}
	return next, nil
}

// Tick translates states28/2a at $15470/$1547a. A later-slot newborn first
// changes28 to2a without moving. Active fronts retain the unused life field,
// lower shallow shores, and spread sideways only within the same tile step.
// No direct follower push, damage or wave-specific hero test is added.
func (rules TsunamiRules) Tick(ref NativeRecordReference, cb TsunamiCallbacks) (TsunamiStep, error) {
	step := TsunamiStep{Lowered: [][2]uint8{}, Children: []NativeRecordReference{}}
	m := cb.Memory
	if !winMemoryValid(m) || cb.Link == nil || cb.Unlink == nil || cb.Move == nil || cb.Lower == nil {
		return step, fmt.Errorf("native tsunami runtime callbacks missing")
	}
	location, ok := LocateNativeRecord(ref)
	if !ok || location.Pool != NativeEffectPool {
		return step, fmt.Errorf("native tsunami reference outside shared pool")
	}
	address := cleanupRecordAddress(ref)
	state, err := m.Read8(address + 22)
	if err != nil {
		return step, err
	}
	if state == 0x28 {
		step.Skipped = true
		return step, m.Write8(address+22, 0x2a)
	}
	if state != 0x2a {
		return step, fmt.Errorf("native tsunami state%x unsupported", state)
	}
	animation, err := m.Read16(address + 10)
	if err != nil {
		return step, err
	}
	next, err := rules.nextAnimation(animation)
	if err != nil {
		return step, err
	}
	if err := m.Write16(address+10, next); err != nil {
		return step, err
	}
	x, err := m.Read16(address + 6)
	if err != nil {
		return step, err
	}
	y, err := m.Read16(address + 8)
	if err != nil {
		return step, err
	}
	vx, err := m.Read16(address + 14)
	if err != nil {
		return step, err
	}
	vy, err := m.Read16(address + 16)
	if err != nil {
		return step, err
	}
	x, y = x+vx, y+vy
	remove := func() error {
		if err := m.Write8(address+12, 0); err != nil {
			return err
		}
		step.Removed = true
		return cb.Unlink(ref)
	}
	if int16(x) < 0 || int16(y) < 0 || int16(x) >= 0x4000 || int16(y) >= 0x4000 {
		return step, remove()
	}
	step.CellChanged, err = cb.Move(ref, x, y)
	if err != nil {
		return step, err
	}
	step.Moved = true
	direction, err := m.Read16(address + 26)
	if err != nil {
		return step, err
	}
	if direction%4 != 0 || direction/4 >= 4 {
		return step, fmt.Errorf("native tsunami direction outside four-entry tables")
	}
	index := direction / 4
	packed := y&0xff00 | x>>8
	front := packed + rules.Fronts[index][0]
	if front&0xc0c0 == 0 {
		grid := tsunamiGrid(front)
		header, err := m.Read8(grid)
		if err != nil {
			return step, err
		}
		tile, err := m.Read8(grid + 1)
		if err != nil {
			return step, err
		}
		if tile == 0xe0 || header&7 != 0 {
			return step, remove()
		}
	}
	front = packed + rules.Fronts[index][1]
	if front&0xc0c0 == 0 {
		tile, err := m.Read8(tsunamiGrid(front) + 1)
		if err != nil {
			return step, err
		}
		if rules.Properties[tile]&8 == 0 {
			lower := func(target uint16) error {
				xx, yy := uint8(target), uint8(target>>8)
				step.Lowered = append(step.Lowered, [2]uint8{xx, yy})
				return cb.Lower(xx, yy)
			}
			if err := lower(front); err != nil {
				return step, err
			}
			tile, err = m.Read8(tsunamiGrid(packed) + 1)
			if err != nil {
				return step, err
			}
			if rules.Properties[tile]&8 == 0 {
				if err := lower(packed); err != nil {
					return step, err
				}
			}
		}
	}
	if step.CellChanged {
		return step, nil
	}
	for _, offset := range rules.Sides[index] {
		neighbor := packed + offset
		if neighbor&0xc0c0 != 0 {
			continue
		}
		grid := tsunamiGrid(neighbor)
		tile, err := m.Read8(grid + 1)
		if err != nil {
			return step, err
		}
		if rules.Properties[tile]&8 == 0 {
			continue
		}
		head, err := m.Read16(grid + 2)
		if err != nil {
			return step, err
		}
		blocked := false
		seen := map[uint16]bool{}
		for head != 0 {
			if seen[head] {
				return step, fmt.Errorf("cyclic native tsunami clone-admission chain")
			}
			seen[head] = true
			a := cleanupRecordAddress(NativeRecordReference(head))
			kind, err := m.Read8(a)
			if err != nil {
				return step, err
			}
			if kind == 0x32 {
				blocked = true
				break
			}
			head, err = m.Read16(a + 2)
			if err != nil {
				return step, err
			}
		}
		if blocked {
			continue
		}
		child, err := primitiveFreeRecord(m, 0xc800, 0xe740, 32)
		if err != nil {
			return step, err
		}
		if child == 0 {
			continue
		}
		animation, err := m.Read16(address + 10)
		if err != nil {
			return step, err
		}
		animation, err = rules.nextAnimation(animation)
		if err != nil {
			return step, err
		}
		if err := m.Write16(child+10, animation); err != nil {
			return step, err
		}
		for _, field := range []int{12, 0} {
			value, err := m.Read8(address + field)
			if err != nil {
				return step, err
			}
			if err := m.Write8(child+field, value); err != nil {
				return step, err
			}
		}
		// D2 now holds the four-byte grid offset. Only X is shifted back;
		// the following copied Y fraction replaces its low offset byte.
		offsetWord := neighbor&0xff00 | uint16(uint8(neighbor)<<2)
		if err := m.Write8(child+6, uint8(offsetWord)>>2); err != nil {
			return step, err
		}
		if err := m.Write16(child+8, offsetWord); err != nil {
			return step, err
		}
		for _, field := range []int{7, 9, 22} {
			value, err := m.Read8(address + field)
			if err != nil {
				return step, err
			}
			if err := m.Write8(child+field, value); err != nil {
				return step, err
			}
		}
		velocity, err := m.Read32(address + 14)
		if err != nil {
			return step, err
		}
		if err := m.Write32(child+14, velocity); err != nil {
			return step, err
		}
		for _, field := range []int{26, 24} {
			value, err := m.Read16(address + field)
			if err != nil {
				return step, err
			}
			if err := m.Write16(child+field, value); err != nil {
				return step, err
			}
		}
		childRef := NativeRecordReference(uint16(child - 0x76c0))
		if err := cb.Link(childRef); err != nil {
			return step, err
		}
		if child >= address {
			if err := m.Write8(child+22, 0x28); err != nil {
				return step, err
			}
		}
		step.Children = append(step.Children, childRef)
	}
	return step, nil
}
