package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type HurricaneRules struct {
	Life       uint16
	Speed      uint8
	Directions [4]uint8
	Axes       [4][2]uint8
}

func DecodeHurricaneRules(exe *amiga.Executable) (HurricaneRules, error) {
	var r HurricaneRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x20d72 {
		return r, fmt.Errorf("native hurricane tables missing")
	}
	c := exe.Hunks[0].Data
	r.Life = binary.BigEndian.Uint16(c[0x20d6e:])
	r.Speed = c[0x20d71]
	copy(r.Directions[:], c[0x17bb6:0x17bba])
	for d := range r.Axes {
		copy(r.Axes[d][:], c[0x17306+d*2:0x17308+d*2])
	}
	return r, nil
}

type HurricaneCallbacks struct {
	Memory       FollowerCleanupMemory
	Move         func(NativeRecordReference, uint16, uint16) error
	Cleanup      func(NativeRecordReference, uint16) error
	Unlink       func(NativeRecordReference) error
	WriteOverlay func(int, uint8) error
	// PointerBase maps a BSS offset to the original relocated native pointer.
	// $15a94 compares that pointer with the VALUE stored at deity0+$00, not
	// with a literal end-of-marker address. Preserve this alias explicitly.
	PointerBase uint32
}

type HurricaneCreation struct {
	Reference NativeRecordReference
	Admitted  bool
}
type HurricaneStep struct {
	Handled, Finished                bool
	Cells, Moves, Removed, Protected int
}

// Create translates $172a4. It allocates by raw owner only, clears all four
// XY bytes, then copies only the direction's selected input axis. No bounds,
// randomness, experience, kind reset or map insertion occurs in this routine.
func (r HurricaneRules) Create(owner, x, y uint8, direction uint16, cb HurricaneCallbacks) (HurricaneCreation, error) {
	var step HurricaneCreation
	if cb.Memory.Read8 == nil || cb.Memory.Write8 == nil || cb.Memory.Write16 == nil || cb.Memory.Write32 == nil || direction > 6 || direction&1 != 0 {
		return step, fmt.Errorf("native hurricane creator callbacks/direction missing")
	}
	address, err := primitiveFreeRecord(cb.Memory, 0xc800, 0xe740, 32)
	if err != nil || address == 0 {
		return step, err
	}
	if err := cb.Memory.Write8(address+12, owner); err != nil {
		return step, err
	}
	if err := cb.Memory.Write32(address+6, 0); err != nil {
		return step, err
	}
	if r.Axes[direction/2][0] != 0 {
		if err := cb.Memory.Write8(address+6, x); err != nil {
			return step, err
		}
	}
	if r.Axes[direction/2][1] != 0 {
		if err := cb.Memory.Write8(address+8, y); err != nil {
			return step, err
		}
	}
	if err := cb.Memory.Write8(address+22, 0x3c); err != nil {
		return step, err
	}
	if err := cb.Memory.Write16(address+26, direction); err != nil {
		return step, err
	}
	if err := cb.Memory.Write16(address+24, r.Life); err != nil {
		return step, err
	}
	if err := cb.Memory.Write8(address+18, r.Speed); err != nil {
		return step, err
	}
	step.Reference, step.Admitted = NativeRecordReference(address-0x76c0), true
	return step, nil
}

func hurricaneRemove(ref NativeRecordReference, cb HurricaneCallbacks) (bool, error) {
	address := cleanupRecordAddress(ref)
	if address >= 0xe740 {
		upper, err := cb.Memory.Read32(0xe76a)
		if err != nil {
			return false, err
		}
		if cb.PointerBase+uint32(address) < upper {
			return false, nil
		}
	}
	if address >= 0x76c0 && address < 0xc800 {
		if cb.Cleanup == nil {
			return false, fmt.Errorf("native hurricane follower cleanup missing")
		}
		return true, cb.Cleanup(ref, 0)
	}
	if cb.Unlink == nil {
		return false, fmt.Errorf("native hurricane generic unlink missing")
	}
	if err := cb.Memory.Write8(address+12, 0); err != nil {
		return false, err
	}
	return true, cb.Unlink(ref)
}

// Tick translates $15916 and all four scan loops. It clears each visited
// overlay and pushes every linked record, with no owner/kind/hero filtering.
// Native traversal reads the actor's next word AFTER $12518. A boundary
// removal ends the current cell immediately, leaving its other occupants for
// a later pass; no independent snapshot of the old list is used.
func (r HurricaneRules) Tick(ref NativeRecordReference, cb HurricaneCallbacks) (HurricaneStep, error) {
	var step HurricaneStep
	if !winMemoryValid(cb.Memory) || cb.Move == nil || cb.WriteOverlay == nil {
		return step, fmt.Errorf("native hurricane runtime callbacks missing")
	}
	address, err := volcanoAddress(ref)
	if err != nil {
		return step, err
	}
	owner, err := cb.Memory.Read8(address + 12)
	if err != nil {
		return step, err
	}
	if owner == 0 {
		return step, nil
	}
	state, err := cb.Memory.Read8(address + 22)
	if err != nil {
		return step, err
	}
	if state != 0x3c {
		return step, nil
	}
	step.Handled = true
	life, err := cb.Memory.Read16(address + 24)
	if err != nil {
		return step, err
	}
	if err := cb.Memory.Write16(address+24, life-1); err != nil {
		return step, err
	}
	if int16(life) <= 1 {
		step.Finished = true
		return step, cb.Memory.Write8(address+12, 0)
	}
	x, err := cb.Memory.Read8(address + 6)
	if err != nil {
		return step, err
	}
	y, err := cb.Memory.Read8(address + 8)
	if err != nil {
		return step, err
	}
	direction, err := cb.Memory.Read16(address + 26)
	if err != nil {
		return step, err
	}
	if direction > 6 || direction&1 != 0 {
		return step, fmt.Errorf("native hurricane direction outside table")
	}
	speed, err := cb.Memory.Read8(address + 18)
	if err != nil {
		return step, err
	}
	cell := func(index int) error {
		if index < 0 || index >= 4096 {
			return fmt.Errorf("native hurricane map scan outside bounded image")
		}
		step.Cells++
		if err := cb.WriteOverlay(index, 0); err != nil {
			return err
		}
		head, err := cb.Memory.Read16(0xf44 + index*4 + 2)
		if err != nil {
			return err
		}
		for visits := 0; head != 0; visits++ {
			if visits > 1053 {
				return fmt.Errorf("native hurricane traversal exceeds bounded actor pools")
			}
			actor := NativeRecordReference(head)
			at := cleanupRecordAddress(actor)
			px, err := cb.Memory.Read16(at + 6)
			if err != nil {
				return err
			}
			py, err := cb.Memory.Read16(at + 8)
			if err != nil {
				return err
			}
			nx, ny := px, py
			inside := true
			switch direction {
			case 0:
				ny = py - uint16(speed)
				inside = int16(py) >= int16(speed)
			case 2:
				nx = px + uint16(speed)
				inside = int16(nx) < 0x4000
			case 4:
				ny = py + uint16(speed)
				inside = int16(ny) < 0x4000
			case 6:
				nx = px - uint16(speed)
				inside = int16(px) >= int16(speed)
			}
			if !inside {
				removed, err := hurricaneRemove(actor, cb)
				if err != nil {
					return err
				}
				if removed {
					step.Removed++
				} else {
					step.Protected++
				}
				return nil
			}
			if err := cb.Move(actor, nx, ny); err != nil {
				return err
			}
			step.Moves++
			head, err = cb.Memory.Read16(at + 2)
			if err != nil {
				return err
			}
		}
		return nil
	}
	switch direction {
	case 0:
		for index := int(x) + int(y)*64; index >= 0; index-- {
			if err := cell(index); err != nil {
				return step, err
			}
		}
	case 4:
		for index := int(x) + int(y)*64; index < 4096; index++ {
			if err := cell(index); err != nil {
				return step, err
			}
		}
	case 2:
		for xx := int(x); xx < 64; xx++ {
			for yy := 0; yy < 64; yy++ {
				if err := cell(xx + yy*64); err != nil {
					return step, err
				}
			}
		}
	case 6:
		for xx := int(x); xx >= 0; xx-- {
			for yy := 0; yy < 64; yy++ {
				if err := cell(xx + yy*64); err != nil {
					return step, err
				}
			}
		}
	}
	return step, nil
}
