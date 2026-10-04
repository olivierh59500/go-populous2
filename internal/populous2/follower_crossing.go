package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type FollowerCrossingRules struct {
	Properties     [256]uint16
	Raster         [256]uint8
	WallThresholds [2]uint32
	WallAnimations [256]uint16
}

func DecodeFollowerCrossingRules(exe *amiga.Executable) (FollowerCrossingRules, error) {
	var r FollowerCrossingRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33612 {
		return r, fmt.Errorf("native crossing tables missing")
	}
	c := exe.Hunks[0].Data
	for index := range r.Properties {
		r.Properties[index] = binary.BigEndian.Uint16(c[0x33312+index*2:])
	}
	copy(r.Raster[:], c[0x33512:0x33612])
	for index := range r.WallThresholds {
		r.WallThresholds[index] = binary.BigEndian.Uint32(c[0x20d5c+index*4:])
	}
	for index := range r.WallAnimations {
		r.WallAnimations[index] = binary.BigEndian.Uint16(c[0x20f1e+int(int8(uint8(index))):])
	}
	return r, nil
}

type FollowerCrossingCallbacks struct{ Memory FollowerCleanupMemory }
type FollowerCrossingStep struct{ Admitted, Blocked, WallBroken, LandRequested bool }

// Admit translates $115c6 through the admission/wall decisions at $116cc.
// The caller retains the separately verified motion bounce, graph $12518 and
// committed $1275a entry. A broken wall ends in $11ce8 before any move.
func (r *FollowerCrossingRules) Admit(ref NativeRecordReference, x, y uint16, cb FollowerCrossingCallbacks) (FollowerCrossingStep, error) {
	var step FollowerCrossingStep
	m := cb.Memory
	if r == nil || !winMemoryValid(m) {
		return step, fmt.Errorf("native crossing memory missing")
	}
	packed := y&0xff00 | x>>8
	if packed&0xc0c0 != 0 {
		step.Blocked = true
		return step, nil
	}
	source := cleanupRecordAddress(ref)
	owner, err := m.Read8(source + 12)
	if err != nil {
		return step, err
	}
	god := heroGodAddress(owner)
	cell := 0xf44 + int(packed&0xff00) + int(uint8(packed))*4
	header, err := m.Read8(cell)
	if err != nil {
		return step, err
	}
	var index uint8
	if header&7 == 0 {
		tile, err := m.Read8(cell + 1)
		if err != nil {
			return step, err
		}
		index = tile
		if r.Raster[tile]&1 == 0 {
			if err := m.Write16(god+0x32, uint16(ref)); err != nil {
				return step, err
			}
			if err := m.Write16(god+0x34, packed); err != nil {
				return step, err
			}
			step.LandRequested = true
		}
	} else {
		index = header & 7
	}
	// Native doubles D0's low byte before the property-word test. The
	// word table therefore sees the wrapped seven-bit index, not raw tile.
	if r.Properties[int(uint8(index<<1))/2]&8 != 0 {
		step.Blocked = true
		return step, nil
	}
	head, err := m.Read16(cell + 2)
	if err != nil {
		return step, err
	}
	for count := 0; head != 0; count++ {
		if count >= NativeRecordImageSize {
			return step, fmt.Errorf("native crossing chain exceeds bounded window")
		}
		at := cleanupRecordAddress(NativeRecordReference(head))
		kind, err := m.Read8(at)
		if err != nil {
			return step, err
		}
		if kind == 0x18 {
			step.Blocked = true
			return step, nil
		}
		if kind == 0x1a {
			other, err := m.Read8(at + 12)
			if err != nil {
				return step, err
			}
			if other == owner {
				step.Admitted = true
				return step, nil
			}
			xp, err := m.Read8(god + 0x54)
			if err != nil {
				return step, err
			}
			population, err := m.Read32(source + 26)
			if err != nil {
				return step, err
			}
			breakThreshold := uint32(xp)<<7 + r.WallThresholds[1]
			if int32(breakThreshold) < int32(population) {
				if err := m.Write8(source+22, 0x2a); err != nil {
					return step, err
				}
				animation := uint16(0x7cc)
				if owner != 1 {
					animation = 0x7d4
				}
				if err := m.Write16(source+10, animation); err != nil {
					return step, err
				}
				stage, err := m.Read8(at + 1)
				if err != nil {
					return step, err
				}
				if stage&1 != 0 {
					return step, fmt.Errorf("native crossing wall stage causes an odd word address")
				}
				if err := m.Write8(at, 0x1c); err != nil {
					return step, err
				}
				if err := m.Write16(at+10, r.WallAnimations[stage]); err != nil {
					return step, err
				}
				step.WallBroken = true
				return step, nil
			}
			step.Admitted = uint32(xp)<<7+r.WallThresholds[0] <= population
			step.Blocked = !step.Admitted
			return step, nil
		}
		head, err = m.Read16(at + 2)
		if err != nil {
			return step, err
		}
	}
	step.Admitted = true
	return step, nil
}
