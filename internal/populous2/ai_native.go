package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeAIRules struct {
	Code             []byte
	Raster           [256]uint8
	Properties       [256]uint16
	ExpansionDelay   uint16
	ExpansionOffsets []uint16
}

func DecodeNativeAIRules(exe *amiga.Executable) (NativeAIRules, error) {
	var r NativeAIRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33612 {
		return r, fmt.Errorf("native AI tables missing")
	}
	r.Code = append([]byte(nil), exe.Hunks[0].Data...)
	r.ExpansionDelay = binary.BigEndian.Uint16(r.Code[0x207f6:])
	copy(r.Raster[:], r.Code[0x33512:0x33612])
	for i := range r.Properties {
		r.Properties[i] = binary.BigEndian.Uint16(r.Code[0x33312+i*2:])
	}
	for at := 0x207f8; at < 0x20872; at += 2 {
		v := binary.BigEndian.Uint16(r.Code[at:])
		if v == 0xff9d {
			break
		}
		r.ExpansionOffsets = append(r.ExpansionOffsets, v)
	}
	if len(r.ExpansionOffsets) == 0 {
		return r, fmt.Errorf("native AI expansion table empty")
	}
	return r, nil
}

type NativeAIPolicy uint8

const (
	NativeAIReleaseTown NativeAIPolicy = iota
	NativeAIOffensivePower
	NativeAIMagnetMode
)

type NativeAICallbacks struct {
	Memory FollowerCleanupMemory
	Random func() uint16
	// Policy supplies original $13ba4/$13c1c/$13dde until those separate
	// controllers are bound. A missing required policy is an explicit error,
	// not a silent no-op or an inherited fixed list of powers.
	Policy func(NativeAIPolicy, int, int) (bool, error)
}

type NativeAIStep struct {
	SidesProcessed  int
	CommandsPending [2]bool
}

// Tick translates original $1383c ordering. Decisions update side1/side2's
// ten-byte records at$eb56/$eb60; $1744c executes them later in the original
// update. Control$12 runs terrain/release decisions without offensive power
// or magnet policy. Control4 uses its own reaction word and template+$68.
func (r *NativeAIRules) Tick(cb NativeAICallbacks) (NativeAIStep, error) {
	var step NativeAIStep
	m := cb.Memory
	if r == nil || !winMemoryValid(m) {
		return step, fmt.Errorf("native AI dispatcher memory missing")
	}
	for side := 0; side < 2; side++ {
		god, command := 0xe8a4+side*314, 0xeb56+side*10
		control, err := m.Read16(god + 0x1a)
		if err != nil {
			return step, err
		}
		if control != 4 && control != 0x12 {
			continue
		}
		step.SidesProcessed++
		if control == 4 {
			reaction, err := m.Read16(god + 0x4c)
			if err != nil {
				return step, err
			}
			reaction--
			if err := m.Write16(god+0x4c, reaction); err != nil {
				return step, err
			}
			if _, err := r.Urgent(god, command, cb); err != nil {
				return step, err
			}
			reaction, err = m.Read16(god + 0x4c)
			if err != nil {
				return step, err
			}
			if int16(reaction) > 0 {
				continue
			}
			delay, err := m.Read16(god + 0x68)
			if err != nil {
				return step, err
			}
			if err := m.Write16(god+0x4c, delay); err != nil {
				return step, err
			}
			pending, err := m.Read8(command + 1)
			if err != nil {
				return step, err
			}
			if pending != 0 {
				step.CommandsPending[side] = true
				continue
			}
		}
		chosen, err := r.Expand(god, command, cb)
		if err != nil {
			return step, err
		}
		if !chosen {
			chosen, err = r.ReleaseTown(god, command, cb)
			if err != nil {
				return step, err
			}
		}
		if !chosen && control != 0x12 {
			if cb.Policy == nil {
				return step, fmt.Errorf("native AI offensive policy missing")
			}
			chosen, err = cb.Policy(NativeAIOffensivePower, god, command)
			if err != nil {
				return step, err
			}
			if !chosen {
				chosen, err = cb.Policy(NativeAIMagnetMode, god, command)
				if err != nil {
					return step, err
				}
			}
		}
		step.CommandsPending[side] = chosen
	}
	return step, nil
}

// ReleaseTown translates $13ba4: its own cooldown, baseline sculpt price and
// original town-stage population threshold select right-click command4. It
// does not release a Go follower itself; command execution remains later.
func (r *NativeAIRules) ReleaseTown(god, command int, cb NativeAICallbacks) (bool, error) {
	m := cb.Memory
	if r == nil || !winMemoryValid(m) {
		return false, fmt.Errorf("native AI town-release memory missing")
	}
	present, err := m.Read16(god + 0x1c)
	if err != nil || present == 0 {
		return false, err
	}
	timer, err := m.Read16(god + 0x30)
	if err != nil {
		return false, err
	}
	if err := m.Write16(god+0x30, timer-1); err != nil {
		return false, err
	}
	if int16(timer) > 1 {
		return false, nil
	}
	delay, err := r.codeWord(0x2075e)
	if err != nil {
		return false, err
	}
	if err := m.Write16(god+0x30, delay); err != nil {
		return false, err
	}
	price, err := r.codeWord(0x21238)
	if err != nil {
		return false, err
	}
	mana, err := m.Read32(god)
	if err != nil || int32(uint32(price)*4) > int32(mana) {
		return false, err
	}
	ref, err := m.Read16(god + 0x1e)
	if err != nil {
		return false, err
	}
	thresholdIndex, err := r.codeWord(0x20760)
	if err != nil {
		return false, err
	}
	mode, err := m.Read16(god + 0x0c)
	if err != nil {
		return false, err
	}
	if mode == 0x10 {
		bonus, err := r.codeWord(0x20762)
		if err != nil {
			return false, err
		}
		thresholdIndex += bonus
	}
	threshold, err := r.codeWord(0x336a6 + int(int16(thresholdIndex)))
	if err != nil {
		return false, err
	}
	population, err := m.Read32(aiReferenceAddress(ref) + 26)
	if err != nil || int32(uint32(threshold)) >= int32(population) {
		return false, err
	}
	return true, r.commandAtActor(command, 4, aiReferenceAddress(ref), m)
}

// CompileChoices translates $10e90. Original command/type pairs are copied
// only for positive signed power flags. The offensive list reserves index0
// as a no-choice entry; the additional leader list follows its first count.
// Untouched bytes of a previously larger list are retained.
func (r *NativeAIRules) CompileChoices(god int, m FollowerCleanupMemory) error {
	if r == nil || !winMemoryValid(m) {
		return fmt.Errorf("native AI power-list memory missing")
	}
	destination, count := god+0x9c, uint16(1)
	copyChoice := func(at int) error {
		command, err := r.codeWord(at)
		if err != nil {
			return err
		}
		kind, err := r.codeWord(at + 2)
		if err != nil {
			return err
		}
		powerOffset, err := r.codeWord(0x210b0 + int(int16(command)))
		if err != nil {
			return err
		}
		flag, err := m.Read8(god + 0x70 + int(powerOffset>>1))
		if err != nil {
			return err
		}
		if int8(flag) <= 0 {
			return nil
		}
		count++
		if err := m.Write16(destination, command); err != nil {
			return err
		}
		if err := m.Write16(destination+2, kind); err != nil {
			return err
		}
		destination += 4
		return nil
	}
	for at := 0x20768; at < 0x207c8; at += 4 {
		if err := copyChoice(at); err != nil {
			return err
		}
	}
	if err := m.Write16(god+0x94, count); err != nil {
		return err
	}
	count = 0
	for at := 0x207c8; at < 0x207e8; at += 4 {
		if err := copyChoice(at); err != nil {
			return err
		}
	}
	if err := m.Write16(god+0x96, count); err != nil {
		return err
	}
	return m.Write16(god+0x26, 0)
}

func (r *NativeAIRules) codeWord(a int) (uint16, error) {
	if a < 0 || a+2 > len(r.Code) {
		return 0, fmt.Errorf("native AI code table outside executable")
	}
	return binary.BigEndian.Uint16(r.Code[a : a+2]), nil
}
func (r *NativeAIRules) basePrice(command uint16) (uint32, error) {
	offset, err := r.codeWord(0x210b0 + int(int16(command)))
	if err != nil {
		return 0, err
	}
	if command == 0x32 {
		offset = 0x48
	}
	price, err := r.codeWord(0x21238 + int(int16(offset)))
	return uint32(price) * 4, err
}
func aiReferenceAddress(ref uint16) int { return 0x76c0 + int(int16(ref)) }

// Urgent translates $138c0: repair the observed water-follower cell, emit a
// prepared affordable power, or repair the observed hazardous tile. It uses
// native base prices and raw references rather than player admission or XP.
func (r *NativeAIRules) Urgent(god, command int, cb NativeAICallbacks) (bool, error) {
	m := cb.Memory
	if r == nil || !winMemoryValid(m) {
		return false, fmt.Errorf("native AI urgent memory missing")
	}
	ref, err := m.Read16(god + 0x36)
	if err != nil {
		return false, err
	}
	reaction, err := m.Read16(god + 0x4c)
	if err != nil {
		return false, err
	}
	if ref != 0 && int16(reaction) <= 0 {
		return true, r.commandAtActor(command, 2, aiReferenceAddress(ref), m)
	}
	prepared, err := m.Read16(god + 0x38)
	if err != nil {
		return false, err
	}
	if prepared != 0 {
		price, err := r.basePrice(prepared)
		if err != nil {
			return false, err
		}
		mana, err := m.Read32(god)
		if err != nil {
			return false, err
		}
		if int32(price) < int32(mana) {
			packed, err := m.Read16(god + 0x3a)
			if err != nil {
				return false, err
			}
			if err := m.Write8(command+1, uint8(prepared)); err != nil {
				return false, err
			}
			return true, m.Write16(command+2, packed)
		}
		if err := m.Write16(god+0x38, 0); err != nil {
			return false, err
		}
		return false, m.Write16(god+0x3a, 0)
	}
	hazards, err := m.Read16(god + 0x32)
	if err != nil {
		return false, err
	}
	if hazards == 0 {
		return false, nil
	}
	if err := m.Write8(command+1, 2); err != nil {
		return false, err
	}
	x, err := m.Read8(god + 0x35)
	if err != nil {
		return false, err
	}
	y, err := m.Read8(god + 0x34)
	if err != nil {
		return false, err
	}
	if err := m.Write8(command+2, x); err != nil {
		return false, err
	}
	return true, m.Write8(command+3, y)
}

func (r *NativeAIRules) commandAtActor(command int, kind uint8, actor int, m FollowerCleanupMemory) error {
	if err := m.Write8(command+1, kind); err != nil {
		return err
	}
	x, err := m.Read8(actor + 6)
	if err != nil {
		return err
	}
	y, err := m.Read8(actor + 8)
	if err != nil {
		return err
	}
	if err := m.Write8(command+2, x); err != nil {
		return err
	}
	return m.Write8(command+3, y)
}

// Expand translates $1395a and its exact sentinel-terminated parcel order.
// Friendly nontown occupants reject a parcel; towns do not. Property masks,
// random hazardous-ground choice and byte comparisons retain native ordering.
func (r *NativeAIRules) Expand(god, command int, cb NativeAICallbacks) (bool, error) {
	m := cb.Memory
	if r == nil || !winMemoryValid(m) {
		return false, fmt.Errorf("native AI expansion memory missing")
	}
	ref, err := m.Read16(god + 0x2e)
	if err != nil {
		return false, err
	}
	if ref == 0 {
		return false, nil
	}
	timer, err := m.Read16(god + 0x2c)
	if err != nil {
		return false, err
	}
	if err := m.Write16(god+0x2c, timer-1); err != nil {
		return false, err
	}
	if int16(timer) > 1 {
		return false, nil
	}
	if err := m.Write16(god+0x2c, r.ExpansionDelay); err != nil {
		return false, err
	}
	price, err := r.codeWord(0x21238)
	if err != nil {
		return false, err
	}
	mana, err := m.Read32(god)
	if err != nil {
		return false, err
	}
	if int32(uint32(price)*4+2) > int32(mana) {
		return false, nil
	}
	actor := aiReferenceAddress(ref)
	x, err := m.Read8(actor + 6)
	if err != nil {
		return false, err
	}
	y, err := m.Read8(actor + 8)
	if err != nil {
		return false, err
	}
	origin := uint16(y)<<8 | uint16(x)
	originHeight, err := r.height(origin, m)
	if err != nil {
		return false, err
	}
	identity, err := m.Read8(god + 0x19)
	if err != nil {
		return false, err
	}
	for _, offset := range r.ExpansionOffsets {
		packed := origin + offset
		if packed&0xc0c0 != 0 {
			continue
		}
		grid := 0xf44 + int(int16(packed&0xff00|uint16(uint8(packed)<<2)))
		head, err := m.Read16(grid + 2)
		if err != nil {
			return false, err
		}
		skip := false
		seen := map[uint16]bool{}
		for head != 0 {
			if seen[head] {
				return false, fmt.Errorf("cyclic native AI parcel chain")
			}
			seen[head] = true
			at := aiReferenceAddress(head)
			kind, err := m.Read8(at)
			if err != nil {
				return false, err
			}
			owner, err := m.Read8(at + 12)
			if err != nil {
				return false, err
			}
			if kind != 4 && owner == identity {
				skip = true
				break
			}
			head, err = m.Read16(at + 2)
			if err != nil {
				return false, err
			}
		}
		if skip {
			continue
		}
		height, err := r.height(packed, m)
		if err != nil {
			return false, err
		}
		tile, err := m.Read8(grid + 1)
		if err != nil {
			return false, err
		}
		properties := r.Properties[tile]
		forceRaise := properties&0x20 != 0
		if !forceRaise && properties&0x390 != 0 {
			if cb.Random == nil {
				return false, fmt.Errorf("native AI hazard RNG missing")
			}
			forceRaise = cb.Random()&15 <= 3
		}
		if !forceRaise && height == originHeight {
			continue
		}
		kind := uint8(2)
		if !forceRaise && int8(height) > int8(originHeight) {
			kind = 4
		}
		if err := m.Write8(command+1, kind); err != nil {
			return false, err
		}
		if err := m.Write8(command+2, uint8(packed)); err != nil {
			return false, err
		}
		return true, m.Write8(command+3, uint8(packed>>8))
	}
	stage, err := m.Read8(actor + 1)
	if err != nil {
		return false, err
	}
	return false, m.Write8(actor+0x13, stage)
}

func (r *NativeAIRules) height(packed uint16, m FollowerCleanupMemory) (uint8, error) {
	grid := 0xf44 + int(int16(packed&0xff00|uint16(uint8(packed)<<2)))
	header, err := m.Read8(grid)
	if err != nil {
		return 0, err
	}
	tile, err := m.Read8(grid + 1)
	if err != nil {
		return 0, err
	}
	return header&7 + r.Raster[tile]&1, nil
}
