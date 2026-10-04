package populous2

import "fmt"

// FollowerCleanupMemory addresses BSS offsets, not relocated host pointers.
// It must cover actor pools, the three14-byte magnet records at $e740, deity
// records at $e76a, and any explicitly supported aliases. Unknown addresses
// return errors; they must not silently read zero or repair native references.
type FollowerCleanupMemory struct {
	Read8   func(int) (uint8, error)
	Read16  func(int) (uint16, error)
	Read32  func(int) (uint32, error)
	Write8  func(int, uint8) error
	Write16 func(int, uint16) error
	Write32 func(int, uint32) error
}

type FollowerCleanupCallbacks struct {
	Memory FollowerCleanupMemory
	// These are original $125da/$125a0 primitive operations, including marker
	// references beyond the four ordinary actor pools. Insert adds no pressure.
	Unlink func(NativeRecordReference) error
	Insert func(NativeRecordReference) error
	// ClearFarms is the complete $135ca compositor, with replacement tile15.
	// It preserves caller D0/D1/D2 and the effective A0 context.
	ClearFarms func(NativeRecordReference, uint8) error
}

type FollowerCleanupRegisters struct {
	D0, D1, D2 uint32
}

type FollowerCleanupStep struct {
	Registers                      FollowerCleanupRegisters
	GodAddress, HeroContextAddress int
	Owner                          uint8
	Removed, Leader                bool
	CaptivesVisited                int
}

func cleanupRecordAddress(reference NativeRecordReference) int {
	return 0x76c0 + int(int16(reference))
}

func (cb FollowerCleanupCallbacks) validate() error {
	m := cb.Memory
	if m.Read8 == nil || m.Read16 == nil || m.Read32 == nil || m.Write8 == nil || m.Write16 == nil || m.Write32 == nil || cb.Unlink == nil || cb.Insert == nil || cb.ClearFarms == nil {
		return fmt.Errorf("incomplete native follower cleanup callbacks")
	}
	return nil
}

// CleanupFollower translates complete $124a2, including $140ae/$13fe4 leader
// relocation and the exact mixed A0/A2 context of $14654. It returns register
// values needed by Whirlwind release. Input/output A0-A2 are preserved by the
// original wrapper, so the API addresses the source explicitly instead.
//
// D0's low byte selects retention: zero removes owner and graph membership;
// every mode clears population. Byte $13 means decimal19, not flags at $0d.
func CleanupFollower(reference NativeRecordReference, registers FollowerCleanupRegisters, cb FollowerCleanupCallbacks) (FollowerCleanupStep, error) {
	step := FollowerCleanupStep{Registers: registers}
	if err := cb.validate(); err != nil {
		return step, err
	}
	m := cb.Memory
	source := cleanupRecordAddress(reference)
	owner, err := m.Read8(source + 0x0c)
	if err != nil {
		return step, err
	}
	step.Owner = owner
	// EXT.W after MOVE.B sign-extends the owner, then MULU uses that low word.
	step.Registers.D1 = uint32(uint16(int16(int8(owner)))) * 314
	step.GodAddress = 0xe76a + int(int16(uint16(step.Registers.D1)))
	context := step.GodAddress
	metric, err := m.Read16(context + 0x44)
	if err != nil {
		return step, err
	}
	if err := m.Write16(context+0x44, metric-2); err != nil {
		return step, err
	}
	flags, err := m.Read8(source + 0x0d)
	if err != nil {
		return step, err
	}
	if flags&1 != 0 {
		step.Leader = true
		loss, err := m.Read16(context + 0x46)
		if err != nil {
			return step, err
		}
		if err := m.Write16(context+0x46, loss+1); err != nil {
			return step, err
		}
		metric, err := m.Read16(context + 0x44)
		if err != nil {
			return step, err
		}
		if err := m.Write16(context+0x44, metric-10); err != nil {
			return step, err
		}
		context = source // MOVEA.L A2,A0 remains effective after $140ae.
		// $140b2 preserves D2's upper word while EXT.W replaces its low word.
		step.Registers.D2 = registers.D2&0xffff0000 | uint32(uint16(int16(int8(owner))))
		if err := m.Write8(source+0x0d, flags&^1); err != nil {
			return step, err
		}
		if err := m.Write16(step.GodAddress+8, 0); err != nil {
			return step, err
		}
		x, err := m.Read8(source + 6)
		if err != nil {
			return step, err
		}
		y, err := m.Read8(source + 8)
		if err != nil {
			return step, err
		}
		// MOVE.B Y,D1 occurs before the clamping inside $13fe4, and its
		// register-preserving wrapper restores this original unclamped value.
		step.Registers.D1 = step.Registers.D1&0xffffff00 | uint32(y)
		step.Registers.D2 = step.Registers.D2&0xffffff00 | uint32(owner)
		marker, err := m.Read16(step.GodAddress + 0x0a)
		if err != nil {
			return step, err
		}
		if err := cb.Unlink(NativeRecordReference(marker)); err != nil {
			return step, err
		}
		clamp := func(value uint8) uint8 {
			if int8(value) < 0 {
				return 0
			}
			if value >= 64 {
				return 63
			}
			return value
		}
		markerAddress := cleanupRecordAddress(NativeRecordReference(marker))
		for _, field := range []struct {
			offset int
			value  uint8
		}{{6, clamp(x)}, {7, 128}, {8, clamp(y)}, {9, 128}} {
			if err := m.Write8(markerAddress+field.offset, field.value); err != nil {
				return step, err
			}
		}
		if err := cb.Insert(NativeRecordReference(marker)); err != nil {
			return step, err
		}
	}
	kind, err := m.Read8(source)
	if err != nil {
		return step, err
	}
	if kind == 4 {
		step.Registers.D2 = step.Registers.D2&0xffffff00 | 15
		if err := cb.ClearFarms(reference, 15); err != nil {
			return step, err
		}
	}
	if err := m.Write8(source+0x13, 0); err != nil {
		return step, err
	}
	step.HeroContextAddress = context
	visits, err := cleanupHeroLinksInContext(source, context, m)
	step.CaptivesVisited = visits
	if err != nil {
		return step, err
	}
	if uint8(registers.D0) == 0 {
		if err := m.Write8(source+0x0c, 0); err != nil {
			return step, err
		}
		if err := m.Write32(source+0x1a, 0); err != nil {
			return step, err
		}
		if err := cb.Unlink(reference); err != nil {
			return step, err
		}
		step.Removed = true
	}
	return step, m.Write32(source+0x1a, 0)
}

func cleanupHeroLinksInContext(source, context int, m FollowerCleanupMemory) (int, error) {
	visits := 0
	flags, err := m.Read8(source + 0x0d)
	if err != nil {
		return visits, err
	}
	sourceReference := uint16(source - 0x76c0)
	if flags&2 != 0 {
		association, err := m.Read16(source + 0x22)
		if err != nil {
			return visits, err
		}
		if association != 0 {
			target := cleanupRecordAddress(NativeRecordReference(association))
			back, err := m.Read16(target + 0x24)
			if err != nil {
				return visits, err
			}
			if back == sourceReference {
				if err := m.Write16(target+0x24, 0); err != nil {
					return visits, err
				}
			}
			if err := m.Write16(context+0x22, 0); err != nil {
				return visits, err
			}
		}
		hero, err := m.Read16(context + 0x28)
		if err != nil {
			return visits, err
		}
		if hero == 10 {
			head, err := m.Read16(context + 0x2a)
			if err != nil {
				return visits, err
			}
			for head != 0 {
				if visits >= NativeRecordImageSize*2 {
					return visits, fmt.Errorf("native contextual captive cleanup exceeds bounded iteration window")
				}
				target := cleanupRecordAddress(NativeRecordReference(head))
				next, err := m.Read16(target + 0x2a)
				if err != nil {
					return visits, err
				}
				if err := m.Write16(target+0x2a, 0); err != nil {
					return visits, err
				}
				flags, err := m.Read8(target + 0x0d)
				if err != nil {
					return visits, err
				}
				if err := m.Write8(target+0x0d, flags&^8); err != nil {
					return visits, err
				}
				if err := m.Write8(target+0x16, 2); err != nil {
					return visits, err
				}
				visits++
				head = next
			}
		}
	}
	back, err := m.Read16(source + 0x24)
	if err != nil || back == 0 {
		return visits, err
	}
	target := cleanupRecordAddress(NativeRecordReference(back))
	forward, err := m.Read16(target + 0x22)
	if err != nil {
		return visits, err
	}
	if forward == sourceReference {
		err = m.Write16(target+0x22, 0)
	}
	return visits, err
}
