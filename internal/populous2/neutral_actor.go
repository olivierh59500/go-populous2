package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeNeutralRules struct {
	Raster          [256]uint8
	NeighborOffsets []uint16
	ImageWords      []int16
	Frames          map[int]AnimationFrame
	VictimTimer     uint16
}

func DecodeNativeNeutralRules(exe *amiga.Executable) (NativeNeutralRules, error) {
	var rules NativeNeutralRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33612 {
		return rules, fmt.Errorf("native neutral actor tables missing")
	}
	code := exe.Hunks[0].Data
	copy(rules.Raster[:], code[0x33512:0x33612])
	rules.VictimTimer = binary.BigEndian.Uint16(code[0x20d72:])
	for at := 0x1239a; at < 0x123b4; at += 2 {
		value := binary.BigEndian.Uint16(code[at:])
		if value == 0xff9d {
			break
		}
		rules.NeighborOffsets = append(rules.NeighborOffsets, value)
	}
	if len(rules.NeighborOffsets) != 3 {
		return rules, fmt.Errorf("native neutral neighborhood table differs")
	}
	for at := 0x23d1a; at < 0x26956; at += 2 {
		rules.ImageWords = append(rules.ImageWords, int16(binary.BigEndian.Uint16(code[at:])))
	}
	rules.Frames = make(map[int]AnimationFrame)
	for _, start := range []int{0x2cc, 0x53c, 0x550, 0xa98, 0xab4, 0x2bfc, 0x2c18, 0x2c34} {
		frames, err := DecodeAnimation(exe, start)
		if err != nil {
			return NativeNeutralRules{}, err
		}
		for index, frame := range frames {
			rules.Frames[start+index*4] = frame
		}
	}
	return rules, nil
}

type NativeNeutralActorCallbacks struct {
	Memory           FollowerCleanupMemory
	Move             func(NativeRecordReference, uint16, uint16) error
	Unlink           func(NativeRecordReference) error
	Head             func(NativePackedTile) (NativeRecordReference, error)
	Tile             func(NativePackedTile) (uint8, error)
	WriteTile        func(NativePackedTile, uint8) error
	Random           func() int
	Lower            func(uint8, uint8) error         // Original direct $d7f0.
	CreateWhirlwind  func(uint8, uint8, uint16) error // Original $15c3e.
	PlantTree        func(uint8, uint8, uint16) error // Original $db26.
	CreateFireColumn func(uint8, uint8, uint16) error // Original $15b7c.
	Cleanup          func(NativeRecordReference, uint16) error
}

type NativeNeutralStep struct {
	Removed, Moved bool
	Effect         uint16
}

// Tick translates $12190/$121e4's neutral-owner movement and six effects.
// Population remains zero and the caller ends this actor at $12462. Native
// terrain/tree/fungus operations keep explicit call boundaries and owner3.
func (rules *NativeNeutralRules) Tick(ref NativeRecordReference, cb NativeNeutralActorCallbacks) (NativeNeutralStep, error) {
	var step NativeNeutralStep
	if rules == nil || !winMemoryValid(cb.Memory) || cb.Move == nil || cb.Unlink == nil {
		return step, fmt.Errorf("native neutral actor callbacks missing")
	}
	m, at := cb.Memory, cleanupRecordAddress(ref)
	animation, err := m.Read16(at + 10)
	if err != nil {
		return step, err
	}
	next := uint16(animation + 4)
	if next&1 != 0 || int(next)/2 >= len(rules.ImageWords) {
		return step, fmt.Errorf("native neutral animation outside bank")
	}
	if loop := rules.ImageWords[next/2]; loop < 0 {
		next += uint16(loop)
	}
	if err := m.Write16(at+10, next); err != nil {
		return step, err
	}
	x, err := m.Read16(at + 6)
	if err != nil {
		return step, err
	}
	y, err := m.Read16(at + 8)
	if err != nil {
		return step, err
	}
	vx, err := m.Read16(at + 14)
	if err != nil {
		return step, err
	}
	vy, err := m.Read16(at + 16)
	if err != nil {
		return step, err
	}
	newX, newY := uint16(x+vx), uint16(y+vy)
	if int16(newX) < 0 || int16(newY) < 0 || int16(newX) >= 0x4000 || int16(newY) >= 0x4000 {
		if err := m.Write8(at+12, 0); err != nil {
			return step, err
		}
		if err := m.Write32(at+26, 0); err != nil {
			return step, err
		}
		step.Removed = true
		return step, cb.Unlink(ref)
	}
	if err := cb.Move(ref, newX, newY); err != nil {
		return step, err
	}
	step.Moved = true
	selector, err := m.Read16(at + 40)
	if err != nil {
		return step, err
	}
	if selector&1 != 0 || selector > 12 {
		return step, fmt.Errorf("native neutral effect selector outside table")
	}
	step.Effect = selector
	packed := NativePackedTile(newY&0xff00 | newX>>8)
	xb, yb := uint8(newX>>8), uint8(newY>>8)
	switch selector {
	case 4:
		if cb.Lower == nil {
			return step, fmt.Errorf("native neutral direct lowering missing")
		}
		if err := cb.Lower(xb, yb); err != nil {
			return step, err
		}
		if xb+1 < 64 {
			return step, cb.Lower(xb+1, yb)
		}
	case 2:
		if cb.Tile == nil || cb.WriteTile == nil {
			return step, fmt.Errorf("native neutral tile callbacks missing")
		}
		tile, err := cb.Tile(packed)
		if err != nil {
			return step, err
		}
		if rules.Raster[tile]&15 == 15 {
			return step, cb.WriteTile(packed, 0xae)
		}
	case 6:
		if cb.Random == nil || cb.CreateWhirlwind == nil {
			return step, fmt.Errorf("native neutral whirlwind callback missing")
		}
		if cb.Random()&31 == 0 {
			return step, cb.CreateWhirlwind(xb, yb, 3)
		}
	case 8:
		if cb.Tile == nil || cb.Head == nil || cb.PlantTree == nil {
			return step, fmt.Errorf("native neutral tree callback missing")
		}
		tile, err := cb.Tile(packed)
		if err != nil {
			return step, err
		}
		if rules.Raster[tile] == 0 {
			return step, nil
		}
		head, err := cb.Head(packed)
		if err != nil {
			return step, err
		}
		if head == 0 {
			return step, nil
		}
		for count := 0; head != 0; count++ {
			if count >= NativeRecordImageSize {
				return step, fmt.Errorf("native neutral tree scan exceeds bounded chain")
			}
			kind, err := m.Read8(cleanupRecordAddress(head))
			if err != nil {
				return step, err
			}
			if kind == 0x16 {
				return step, nil
			}
			next, err := m.Read16(cleanupRecordAddress(head) + 2)
			if err != nil {
				return step, err
			}
			head = NativeRecordReference(next)
		}
		return step, cb.PlantTree(xb, yb, 3)
	case 10:
		if cb.Tile == nil || cb.Random == nil || cb.CreateFireColumn == nil {
			return step, fmt.Errorf("native neutral fire-column callback missing")
		}
		tile, err := cb.Tile(packed)
		if err != nil {
			return step, err
		}
		if tile != 0 && cb.Random()&31 == 0 {
			return step, cb.CreateFireColumn(xb, yb, 3)
		}
	case 12:
		if cb.Head == nil || cb.Cleanup == nil {
			return step, fmt.Errorf("native neutral victim callbacks missing")
		}
		for _, delta := range rules.NeighborOffsets {
			cell := uint16(packed) + delta
			if cell&0xc0c0 != 0 {
				continue
			}
			head, err := cb.Head(NativePackedTile(cell))
			if err != nil {
				return step, err
			}
			for count := 0; head != 0; count++ {
				if count >= NativeRecordImageSize {
					return step, fmt.Errorf("native neutral victim scan exceeds bounded chain")
				}
				victim := cleanupRecordAddress(head)
				kind, err := m.Read8(victim)
				if err != nil {
					return step, err
				}
				if kind == 2 || kind == 4 {
					if err := cb.Cleanup(head, 1); err != nil {
						return step, err
					}
					if err := m.Write16(victim+20, rules.VictimTimer); err != nil {
						return step, err
					}
					if err := m.Write8(victim+22, 0x46); err != nil {
						return step, err
					}
					if err := m.Write16(victim+10, 0x2c34); err != nil {
						return step, err
					}
				}
				next, err := m.Read16(victim + 2)
				if err != nil {
					return step, err
				}
				head = NativeRecordReference(next)
			}
		}
	case 0:
		// Native selector zero jumps directly to the effect-return boundary.
	}
	return step, nil
}
