package populous2

import "fmt"

type FollowerContactCallbacks struct {
	Memory     FollowerCleanupMemory
	ClearFarms func(NativeRecordReference, uint8) error
	Sound      func(uint16) error
}

type FollowerContactStep struct {
	Aggressor, Defender NativeRecordReference
	Swapped, Captured   bool
}

// PrepareFollowerContact translates complete $12ade with original A0/A1
// participants. Raw reads after every write preserve aliases between actors,
// target backlinks and captive-chain records. No owner/population admission or
// general death cleanup is added; the native caller supplies those decisions.
func PrepareFollowerContact(source, target NativeRecordReference, cb FollowerContactCallbacks) (FollowerContactStep, error) {
	step := FollowerContactStep{Aggressor: source, Defender: target}
	m := cb.Memory
	if !winMemoryValid(m) || cb.ClearFarms == nil || cb.Sound == nil {
		return step, fmt.Errorf("native follower contact callbacks missing")
	}
	a, b := cleanupRecordAddress(source), cleanupRecordAddress(target)
	flags, err := m.Read8(a + 13)
	if err != nil {
		return step, err
	}
	if flags&2 == 0 {
		other, err := m.Read8(b + 13)
		if err != nil {
			return step, err
		}
		if other&2 != 0 {
			a, b, source, target = b, a, target, source
			flags, step.Swapped = other, true
			step.Aggressor, step.Defender = source, target
		}
	}
	if flags&2 != 0 {
		association, err := m.Read16(a + 34)
		if err != nil {
			return step, err
		}
		if association != 0 {
			associated := cleanupRecordAddress(NativeRecordReference(association))
			if err := m.Write16(a+34, 0); err != nil {
				return step, err
			}
			back, err := m.Read16(associated + 36)
			if err != nil {
				return step, err
			}
			if back == uint16(source) {
				if err := m.Write16(associated+36, 0); err != nil {
					return step, err
				}
			}
		}
		hero, err := m.Read16(a + 40)
		if err != nil {
			return step, err
		}
		if hero == 10 {
			otherFlags, err := m.Read8(b + 13)
			if err != nil {
				return step, err
			}
			if err := m.Write8(b+13, otherFlags|8); err != nil {
				return step, err
			}
			kind, err := m.Read8(b)
			if err != nil {
				return step, err
			}
			if kind == 4 {
				if err := cb.ClearFarms(target, 15); err != nil {
					return step, err
				}
			}
			previous, err := m.Read16(a + 42)
			if err != nil {
				return step, err
			}
			if previous != 0 {
				if err := m.Write16(cleanupRecordAddress(NativeRecordReference(previous))+42, uint16(target)); err != nil {
					return step, err
				}
			}
			if err := m.Write16(a+42, uint16(target)); err != nil {
				return step, err
			}
			if err := m.Write16(b+44, uint16(source)); err != nil {
				return step, err
			}
			if err := cb.Sound(0x35c); err != nil {
				return step, err
			}
			// $184f6 preserves D0: the victim stores the sound argument
			// $35c in its word42, even when that aliases the source chain.
			if err := m.Write16(b+42, 0x35c); err != nil {
				return step, err
			}
			if err := m.Write8(b+22, 0x34); err != nil {
				return step, err
			}
			if err := m.Write8(b, 2); err != nil {
				return step, err
			}
			if err := m.Write16(b+10, 0); err != nil {
				return step, err
			}
			if err := m.Write8(a+22, 0x24); err != nil {
				return step, err
			}
			if err := m.Write16(b+36, 0); err != nil {
				return step, err
			}
			if err := m.Write16(a+34, 0); err != nil {
				return step, err
			}
			step.Captured = true
			return step, nil
		}
	}
	if err := m.Write16(a+30, uint16(target)); err != nil {
		return step, err
	}
	if err := m.Write16(b+30, uint16(source)); err != nil {
		return step, err
	}
	if err := m.Write8(b+22, 0x10); err != nil {
		return step, err
	}
	if err := m.Write8(a+22, 0x0e); err != nil {
		return step, err
	}
	if err := m.Write16(a+10, 0x1c8); err != nil {
		return step, err
	}
	kind, err := m.Read8(a)
	if err != nil {
		return step, err
	}
	if kind != 4 {
		if err := m.Write8(a+7, 128); err != nil {
			return step, err
		}
		if err := m.Write8(a+9, 128); err != nil {
			return step, err
		}
	}
	return step, nil
}
