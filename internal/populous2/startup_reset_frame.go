package populous2

import "fmt"

// NativeStartupResetFrameCall is an actual source child with the surviving
// eight data registers and address-register labels. A pending operation owns
// its real inner phase; the caller does not restart an earlier reset prefix.
type NativeStartupResetFrameCall struct {
	Routine int
	Frame   *NativeFrameRegisterContext
	A       *[7]NativeRequesterAddress
}
type NativeStartupResetFrameCallbacks struct {
	Code, Memory FollowerCleanupMemory
	CodeBase     uint32
	Frame        *NativeFrameRegisterContext
	Call         func(NativeStartupResetFrameCall, *uint32) (NativeCommandFrameResult, error)
}
type NativeStartupResetFrameStep struct {
	Complete, Waiting bool
	PC                int
	Calls             []int
}

// ResetNativeStartup10F1A is the destructive original reset, not the later
// control-only initializer. It retains exactly God+$4e..$59 for both players
// and clears every byte in BSS$dc2..$eb22. Command/session bytes after that
// endpoint remain untouched. Only D0.W changes; its upper word survives.
func ResetNativeStartup10F1A(cb NativeStartupResetFrameCallbacks, a *[7]NativeRequesterAddress) error {
	if cb.Frame == nil || a == nil || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.Code) {
		return fmt.Errorf("native startup reset backing/frame missing")
	}
	m, c := cb.Memory, cb.Frame
	if e := m.Write32(0xdd8, 0); e != nil {
		return e
	}
	if e := cb.Code.Write16(0xa2a, 0); e != nil {
		return e
	}
	if e := m.Write16(0xddc, 0); e != nil {
		return e
	}
	var saved [2][12]uint8
	for side := 0; side < 2; side++ {
		for i := range saved[side] {
			v, e := m.Read8(0xe8f2 + side*314 + i)
			if e != nil {
				return e
			}
			saved[side][i] = v
		}
	}
	for at := 0xdc2; at < 0xeb22; at++ {
		if e := m.Write8(at, 0); e != nil {
			return e
		}
	}
	for side := 1; side >= 0; side-- {
		for _, span := range [][2]int{{8, 2}, {4, 4}, {0, 4}, {10, 2}} {
			for i := 0; i < span[1]; i++ {
				if e := m.Write8(0xe8f2+side*314+span[0]+i, saved[side][span[0]+i]); e != nil {
					return e
				}
			}
		}
	}
	a[0] = NativeRequesterAddress{Address: c.AddressBase + 0xe8a4}
	a[1] = NativeRequesterAddress{Address: c.AddressBase + 0xeb22}
	profile, e := m.Read16(0xeb42)
	if e != nil {
		return e
	}
	if profile == 0 {
		if e = m.Write16(0xeb42, 1); e != nil {
			return e
		}
		for _, v := range []struct {
			at    int
			value uint8
		}{{0xeb56, 1}, {0xeb60, 2}, {0xeb5e, 2}, {0xeb68, 4}} {
			if e = m.Write8(v.at, v.value); e != nil {
				return e
			}
		}
		if e = m.Write32(0xeb6a, c.AddressBase+0xeb56); e != nil {
			return e
		}
		profile = 1
	}
	c.Word(0, 4)
	transport, e := m.Read8(0xeb5e)
	if e != nil {
		return e
	}
	if int8(transport) >= 6 {
		c.Word(0, 2)
	}
	if e = m.Write16(0xe8be, 2); e != nil {
		return e
	}
	if e = m.Write16(0xe9f8, uint16(c.D[0])); e != nil {
		return e
	}
	if profile != 1 {
		if e = m.Write16(0xe8be, uint16(c.D[0])); e != nil {
			return e
		}
		if e = m.Write16(0xe9f8, 2); e != nil {
			return e
		}
	}
	for _, v := range []struct {
		at    int
		value uint16
	}{{0xe8bc, 1}, {0xe9f6, 2}, {0xe8b0, 14}, {0xe9ea, 14}, {0xeb18, 2}, {0xeb6e, 0}, {0xf0c, 8}} {
		if e = m.Write16(v.at, v.value); e != nil {
			return e
		}
	}
	return nil
}

// NativeStartupResetFrameState follows $10a8c/$10ad8 through every genuine
// initialization, presentation and transport child. The first checkpoint
// translates $10f1a directly; other children remain explicit operations until
// their complete register-aware bodies are supplied by the host.
type NativeStartupResetFrameState struct {
	Entry             int
	Started, Finished bool
	PC                int
	Registers         [8]uint32
	A                 [7]NativeRequesterAddress
	ChildActive       bool
	ChildRoutine      int
	ChildPhase        uint32
	savedFree         uint16
	failed            error
}

func (s *NativeStartupResetFrameState) Advance(cb NativeStartupResetFrameCallbacks) (out NativeStartupResetFrameStep, failure error) {
	if s == nil || cb.Frame == nil || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.Code) {
		return out, fmt.Errorf("native startup controller backing/frame missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		if s.Entry != 0x10a8c && s.Entry != 0x10ad8 {
			return out, fmt.Errorf("native startup entry%x unsupported", s.Entry)
		}
		s.Started = true
		s.PC = s.Entry
		s.Registers = cb.Frame.D
	}
	cb.Frame.D = s.Registers
	defer func() {
		s.Registers = cb.Frame.D
		out.PC = s.PC
		if failure != nil {
			s.failed = failure
		}
	}()
	if s.Finished {
		out.Complete = true
		return out, nil
	}
	m, c := cb.Memory, cb.Frame
	code := func(at int) NativeRequesterAddress {
		return NativeRequesterAddress{Address: cb.CodeBase + uint32(at), Code: true}
	}
	child := func(routine, next int) (bool, error) {
		if cb.Call == nil {
			return false, fmt.Errorf("native startup child%x missing", routine)
		}
		if !s.ChildActive {
			s.ChildActive = true
			s.ChildRoutine = routine
			out.Calls = append(out.Calls, routine)
		}
		if s.ChildRoutine != routine {
			return false, fmt.Errorf("native startup child changed while pending")
		}
		result, e := cb.Call(NativeStartupResetFrameCall{Routine: routine, Frame: c, A: &s.A}, &s.ChildPhase)
		if e != nil {
			return false, e
		}
		if !result.Complete {
			out.Waiting = true
			return false, nil
		}
		s.ChildActive = false
		s.ChildPhase = 0
		s.PC = next
		return true, nil
	}
	for transitions := 0; transitions < 128; transitions++ {
		switch s.PC {
		case 0x10a8c:
			mode, e := m.Read16(0xeb44)
			if e != nil {
				return out, e
			}
			s.PC = 0x10aae
			if mode == 6 {
				s.A[1], s.A[2] = code(0x86f0), code(0xaa0a)
				s.PC = 0x10aa2
			}
		case 0x10aa2:
			done, e := child(0x33b2, 0x10aa8)
			if e != nil || !done {
				return out, e
			}
		case 0x10aa8:
			done, e := child(0x1826e, 0x10aae)
			if e != nil || !done {
				return out, e
			}
		case 0x10aae:
			value, e := m.Read16(0xf0e)
			if e != nil {
				return out, e
			}
			s.savedFree = value
			if e = ResetNativeStartup10F1A(cb, &s.A); e != nil {
				return out, e
			}
			if e = m.Write16(0xf0e, s.savedFree); e != nil {
				return out, e
			}
			s.PC = 0x10ac0
		case 0x10ac0:
			done, e := child(0x3b64, 0x10ac6)
			if e != nil || !done {
				return out, e
			}
		case 0x10ac6:
			if int32(c.D[0]) > 0 {
				s.PC = 0x10b1c
			} else if int32(c.D[0]) < 0 {
				c.D[0] = 0xffffffff
				s.PC = 0
			} else {
				if e := m.Write32(0xeb28, 0x058028af); e != nil {
					return out, e
				}
				s.PC = 0x10ad8
			}
		case 0x10ad8:
			mode, e := m.Read16(0xeb44)
			if e != nil {
				return out, e
			}
			if mode == 2 {
				if e = m.Write16(0xf0e, 0); e != nil {
					return out, e
				}
			}
			value, e := m.Read16(0xf0e)
			if e != nil {
				return out, e
			}
			s.savedFree = value
			s.PC = 0x10aee
		case 0x10aee:
			done, e := child(0x10d6a, 0x10af4)
			if e != nil || !done {
				return out, e
			}
		case 0x10af4:
			if e := m.Write16(0xf0e, s.savedFree); e != nil {
				return out, e
			}
			if uint16(c.D[0]) == 0 {
				s.PC = 0x10a8c
			} else {
				s.PC = 0x10afe
			}
		case 0x10afe:
			done, e := child(0x1a32a, 0x10b04)
			if e != nil || !done {
				return out, e
			}
		case 0x10b04:
			done, e := child(0xcd22, 0x10b0a)
			if e != nil || !done {
				return out, e
			}
		case 0x10b0a:
			done, e := child(0xd9d8, 0x10b10)
			if e != nil || !done {
				return out, e
			}
		case 0x10b10:
			done, e := child(0xdbd4, 0x10b16)
			if e != nil || !done {
				return out, e
			}
		case 0x10b16:
			done, e := child(0x10b38, 0x10b1c)
			if e != nil || !done {
				return out, e
			}
		case 0x10b1c:
			target, e := m.Read32(0x22)
			if e != nil {
				return out, e
			}
			s.A[0] = NativeRequesterAddress{Address: target}
			s.PC = 0x10b22
		case 0x10b22:
			done, e := child(0xd8cc, 0x10b28)
			if e != nil || !done {
				return out, e
			}
		case 0x10b28:
			done, e := child(0x1da0, 0x10b2e)
			if e != nil || !done {
				return out, e
			}
		case 0x10b2e:
			done, e := child(0x18474, 0x10b34)
			if e != nil || !done {
				return out, e
			}
		case 0x10b34:
			c.D[0] = 0
			s.PC = 0
		case 0:
			s.Finished = true
			out.Complete = true
			return out, nil
		default:
			return out, fmt.Errorf("native startup PC%x unsupported", s.PC)
		}
	}
	// A source retry loop may contain many immediate child returns. Yield the
	// retained frame; this is a scheduling boundary, never a successful reset.
	out.Waiting = true
	return out, nil
}
