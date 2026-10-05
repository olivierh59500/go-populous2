package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type RenewNativeRules struct {
	CountModulus uint16
	Offsets      [45]uint16
	Raster       [256]uint8
}

func DecodeRenewNativeRules(exe *amiga.Executable) (RenewNativeRules, error) {
	var rules RenewNativeRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33612 {
		return rules, fmt.Errorf("native renew-land tables missing")
	}
	code := exe.Hunks[0].Data
	rules.CountModulus = binary.BigEndian.Uint16(code[0x20fa4:])
	if rules.CountModulus == 0 {
		return RenewNativeRules{}, fmt.Errorf("native renew-land divisor is zero")
	}
	for index := range rules.Offsets {
		rules.Offsets[index] = binary.BigEndian.Uint16(code[0x20fa8+index*2:])
	}
	copy(rules.Raster[:], code[0x33512:0x33612])
	return rules, nil
}

type RenewNativeCallbacks struct {
	Memory FollowerCleanupMemory
	Random func() uint16
}
type RenewNativePlacement struct{ Attempts, Painted, RandomDraws int }

// Create translates $16a62, command80/slot7. It paints sampled raster-shape15
// parcels with tile245. Ownership, occupancy, hero fields and deity metrics
// are not inspected or modified; the action handler always debits its cast.
func (rules RenewNativeRules) Create(owner uint16, x, y uint8, cb RenewNativeCallbacks) (RenewNativePlacement, error) {
	var step RenewNativePlacement
	m := cb.Memory
	if !winMemoryValid(m) || cb.Random == nil {
		return step, fmt.Errorf("native renew-land callbacks missing")
	}
	xp := uint16(0)
	retained := uint32(0)
	if int8(uint8(owner)) <= 2 {
		retained = uint32(owner) * 314
		value, err := m.Read8(primitiveDeityAddress(owner) + 0x53)
		if err != nil {
			return step, err
		}
		xp = uint16(value >> 5)
	}
	origin := uint16(y)<<8 | uint16(x)
	// MOVE.W RNG replaces only D2's low word after its owner MULU. DIVU
	// overflow leaves the full dividend unchanged before the native SWAP.
	dividend := retained&0xffff0000 | uint32(cb.Random())
	remainder := uint16(dividend % uint32(rules.CountModulus))
	if dividend/uint32(rules.CountModulus) > 0xffff {
		remainder = uint16(dividend >> 16)
	}
	count := remainder + rules.CountModulus/2 + xp
	step.RandomDraws++
	for attempt := 0; attempt <= int(count); attempt++ {
		step.Attempts++
		bits := cb.Random()
		step.RandomDraws++
		packed := origin + rules.Offsets[(bits%90)/2]
		if packed&0xc0c0 != 0 {
			continue
		}
		grid := tsunamiGrid(packed)
		tile, err := m.Read8(grid + 1)
		if err != nil {
			return step, err
		}
		if rules.Raster[tile]&15 == 15 {
			if err := m.Write8(grid+1, 245); err != nil {
				return step, err
			}
			step.Painted++
		}
	}
	return step, nil
}
