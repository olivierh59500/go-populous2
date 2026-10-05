package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type ArmageddonRules struct {
	Eligible   [36]bool
	ValidState [36]bool
}

func DecodeArmageddonRules(exe *amiga.Executable) (ArmageddonRules, error) {
	var r ArmageddonRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x13094 {
		return r, fmt.Errorf("native armageddon state table missing")
	}
	code := exe.Hunks[0].Data
	for i := range r.Eligible {
		handler := 0x1304c + int(int16(binary.BigEndian.Uint16(code[0x1304c+i*2:])))
		switch handler {
		case 0x13094:
			r.ValidState[i], r.Eligible[i] = true, true
		case 0x130c6:
			r.ValidState[i] = true
		case 0x1304c:
		default:
			return ArmageddonRules{}, fmt.Errorf("native armageddon dispatch outside table")
		}
	}
	return r, nil
}

type ArmageddonCallbacks struct {
	Memory  FollowerCleanupMemory
	Random  func() uint16
	Cleanup func(NativeRecordReference, uint16) error
	// Convert is complete $142fe with hero byte0/2/4/6. It includes real
	// ClearLeader, town farms, native hero sound/population/speed changes.
	Convert func(NativeRecordReference, uint16) error
	Sound   func(uint16) error
	// Debit is the original shared $17e38 boundary with power-offset8. Its
	// global/editor/neutral admission, XP and mana rules remain caller-owned.
	Debit func(uint16, uint16) error
}

type ArmageddonStep struct {
	AlreadyEnabled, Enabled, Admitted bool
	Converted, Removed                []NativeRecordReference
	RandomDraws                       int
}

// Cast translates $13022. The original global$f12 enables heroes' direct
// terrain raising, not a new follower countdown or a center relocation.
// Eligible infected actors receive cleanup0; every other eligible allocated
// follower becomes a random one of the first four heroes. Population is not
// an admission filter. Existing hero/motion/battle handlers own later updates.
func (r *ArmageddonRules) Cast(cb ArmageddonCallbacks) (ArmageddonStep, error) {
	step := ArmageddonStep{Converted: []NativeRecordReference{}, Removed: []NativeRecordReference{}}
	m := cb.Memory
	if r == nil || !winMemoryValid(m) {
		return step, fmt.Errorf("native armageddon memory missing")
	}
	enabled, err := m.Read16(0xf12)
	if err != nil {
		return step, err
	}
	step.Admitted = true
	if enabled != 0 {
		step.AlreadyEnabled = true
		return step, nil
	}
	for at := 0x76f4; at < 0xc800; at += 52 {
		owner, err := m.Read8(at + 12)
		if err != nil {
			return step, err
		}
		if owner == 0 {
			continue
		}
		state, err := m.Read8(at + 22)
		if err != nil {
			return step, err
		}
		if state&1 != 0 || int(state/2) >= len(r.Eligible) || !r.ValidState[state/2] {
			return step, fmt.Errorf("native armageddon allocated state outside table")
		}
		if !r.Eligible[state/2] {
			continue
		}
		ref := NativeRecordReference(at - 0x76c0)
		flags, err := m.Read8(at + 13)
		if err != nil {
			return step, err
		}
		if flags&16 != 0 {
			if cb.Cleanup == nil {
				return step, fmt.Errorf("native armageddon plague cleanup missing")
			}
			if err := cb.Cleanup(ref, 0); err != nil {
				return step, err
			}
			step.Removed = append(step.Removed, ref)
			continue
		}
		if cb.Random == nil || cb.Convert == nil {
			return step, fmt.Errorf("native armageddon hero callbacks missing")
		}
		hero := (cb.Random() % 4) * 2
		step.RandomDraws++
		if err := cb.Convert(ref, hero); err != nil {
			return step, err
		}
		step.Converted = append(step.Converted, ref)
	}
	if err := m.Write16(0xf12, 1); err != nil {
		return step, err
	}
	step.Enabled = true
	return step, nil
}

// Command is the exact $17b02 wrapper through the external $17e38 debit
// boundary. Sound$051e plays even on a repeated cast. An already nonzero$f12
// leaves the caller's nonzero condition, so that no-op is still admitted.
func (r *ArmageddonRules) Command(owner uint16, cb ArmageddonCallbacks) (ArmageddonStep, error) {
	if cb.Sound == nil || cb.Debit == nil {
		return ArmageddonStep{}, fmt.Errorf("native armageddon command callbacks missing")
	}
	if err := cb.Sound(0x51e); err != nil {
		return ArmageddonStep{}, err
	}
	step, err := r.Cast(cb)
	if err != nil {
		return step, err
	}
	if step.Admitted {
		err = cb.Debit(owner, 8)
	}
	return step, err
}
