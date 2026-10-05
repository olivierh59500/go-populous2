package populous2

import "fmt"

// LoadNativeStartupTemplates follows$10df2 using the actual mutable250-byte
// CODE record at20536. Its final D1 is a long load of the terrain/seed words,
// not a normalized terrain identifier or synthesized constructor return.
func LoadNativeStartupTemplates(cb NativeStartupResetFrameCallbacks, a *[7]NativeRequesterAddress) error {
	if cb.Frame == nil || a == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) {
		return fmt.Errorf("native startup template backing missing")
	}
	c, m := cb.Frame, cb.Memory
	a[0] = NativeRequesterAddress{Address: cb.CodeBase + 0x20536, Code: true}
	copyBytes := func(source, target, count int) error {
		a[2] = NativeRequesterAddress{Address: cb.CodeBase + uint32(source), Code: true}
		a[1] = NativeRequesterAddress{Address: c.AddressBase + uint32(target)}
		c.Word(1, uint16(count-1))
		for i := 0; i < count; i++ {
			value, e := cb.Code.Read8(source + i)
			if e != nil {
				return e
			}
			if e = m.Write8(target+i, value); e != nil {
				return e
			}
			a[2].Address++
			a[1].Address++
			c.Word(1, uint16(c.D[1])-1)
		}
		return nil
	}
	for side := 0; side < 2; side++ {
		if e := copyBytes(0x20536+side*58, 0xe8fe+side*314, 58); e != nil {
			return e
		}
		mode, e := m.Read16(0xeb44)
		if e != nil {
			return e
		}
		control, e := m.Read16(0xe8be + side*314)
		if e != nil {
			return e
		}
		if mode != 6 && control == 4 {
			for i := 0; i < 2; i++ {
				v, e := cb.Code.Read32(0x20536 + 116 + i*4)
				if e != nil {
					return e
				}
				if e = m.Write32(0xe8f6+side*314+i*4, v); e != nil {
					return e
				}
			}
		}
	}
	if e := copyBytes(0x20536+122, 0xdde, 60); e != nil {
		return e
	}
	if e := m.Write16(0xf0a, 0); e != nil {
		return e
	}
	terrain, e := cb.Code.Read16(0x20536 + 182)
	if e != nil {
		return e
	}
	if e = m.Write16(0xeb22, terrain); e != nil {
		return e
	}
	value, e := cb.Code.Read32(0x20536 + 182)
	if e != nil {
		return e
	}
	c.D[1] = value
	return nil
}

// CompileNativeStartupChoices translates$10e90 directly. Signed power bytes,
// raw command-table word indexing and every MOVE.W upper-register bit remain
// visible. This is separate from copying the templates at$10df2.
func CompileNativeStartupChoices(cb NativeStartupResetFrameCallbacks, a *[7]NativeRequesterAddress) error {
	if cb.Frame == nil || a == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) {
		return fmt.Errorf("native startup choice backing missing")
	}
	c, m := cb.Frame, cb.Memory
	for god := 0xe8a4; god < 0xeb18; god += 314 {
		a[1] = NativeRequesterAddress{Address: c.AddressBase + uint32(god)}
		out := god + 0x9c
		a[2] = NativeRequesterAddress{Address: c.AddressBase + uint32(out)}
		for bank, span := range [][2]int{{0x20768, 0x207c8}, {0x207c8, 0x207e8}} {
			start, end := span[0], span[1]
			a[0] = NativeRequesterAddress{Address: cb.CodeBase + uint32(start), Code: true}
			c.D[3] = uint32(1 - bank)
			for at := start; at < end; at += 4 {
				command, e := cb.Code.Read16(at)
				if e != nil {
					return e
				}
				parameter, e := cb.Code.Read16(at + 2)
				if e != nil {
					return e
				}
				c.Word(1, command)
				c.Word(2, parameter)
				a[3] = NativeRequesterAddress{Address: cb.CodeBase + 0x210b0, Code: true}
				offset, e := cb.Code.Read16(0x210b0 + int(int16(c.D[1])))
				if e != nil {
					return e
				}
				c.Word(0, offset)
				c.Word(0, uint16(c.D[0])>>1)
				enabled, e := m.Read8(god + 0x70 + int(int16(c.D[0])))
				if e != nil {
					return e
				}
				if int8(enabled) > 0 {
					c.Word(3, uint16(c.D[3])+1)
					if e = m.Write16(out, uint16(c.D[1])); e != nil {
						return e
					}
					if e = m.Write16(out+2, uint16(c.D[2])); e != nil {
						return e
					}
					out += 4
					a[2].Address = c.AddressBase + uint32(out)
				}
				a[0].Address = cb.CodeBase + uint32(at+4)
			}
			if e := m.Write16(god+0x94+bank*2, uint16(c.D[3])); e != nil {
				return e
			}
		}
		if e := m.Write16(god+0x26, 0); e != nil {
			return e
		}
		a[1].Address = c.AddressBase + uint32(god+314)
	}
	return nil
}

// NativeStartupConstructionFrameState is the complete$10d6a control body.
// It executes the destructive reset, CODE custom-record copy and genuine raw
// template/compiler bodies. Conquest/UI loading remain real pending children;
// the actual8D/A outputs are returned, never borrowed from typed NewWorld.
type NativeStartupConstructionFrameState struct {
	Started, Finished bool
	PC                int
	Registers         [8]uint32
	A                 [7]NativeRequesterAddress
	ChildActive       bool
	ChildRoutine      int
	ChildPhase        uint32
	LastZero          bool
	failed            error
}

func (s *NativeStartupConstructionFrameState) Advance(cb NativeStartupResetFrameCallbacks) (out NativeStartupResetFrameStep, failure error) {
	if s == nil || cb.Frame == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) {
		return out, fmt.Errorf("native startup construction backing missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		s.Started = true
		s.PC = 0x10d6a
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
		out.FlagsKnown, out.Zero, out.Negative = true, cb.Frame.D[0] == 0, false
		return out, nil
	}
	c, m := cb.Frame, cb.Memory
	child := func(routine, next int) (bool, error) {
		if cb.Call == nil {
			return false, fmt.Errorf("native startup construction child%x missing", routine)
		}
		if !s.ChildActive {
			s.ChildActive = true
			s.ChildRoutine = routine
			out.Calls = append(out.Calls, routine)
		}
		if s.ChildRoutine != routine {
			return false, fmt.Errorf("native construction child changed while pending")
		}
		result, e := cb.Call(NativeStartupResetFrameCall{Routine: routine, Frame: c, A: &s.A}, &s.ChildPhase)
		if e != nil {
			return false, e
		}
		if !result.Complete {
			out.Waiting = true
			return false, nil
		}
		s.LastZero = result.Zero
		s.ChildActive = false
		s.ChildPhase = 0
		s.PC = next
		return true, nil
	}
	for transition := 0; transition < 64; transition++ {
		switch s.PC {
		case 0x10d6a:
			if e := ResetNativeStartup10F1A(cb, &s.A); e != nil {
				return out, e
			}
			mode, e := m.Read16(0xeb44)
			if e != nil {
				return out, e
			}
			c.Word(0, mode)
			branch, e := cb.Code.Read16(0x10d7e + int(int16(c.D[0])))
			if e != nil {
				return out, e
			}
			c.Word(0, branch)
			s.PC = 0x10d7e + int(int16(c.D[0]))
		case 0x10d8a:
			if e := m.Write16(0xeb44, 4); e != nil {
				return out, e
			}
			s.PC = 0x10d92
		case 0x10d92:
			done, e := child(0x11078, 0x10d98)
			if e != nil || !done {
				return out, e
			}
		case 0x10d98:
			s.A[0] = NativeRequesterAddress{Address: cb.CodeBase + 0x20630, Code: true}
			s.A[1] = NativeRequesterAddress{Address: cb.CodeBase + 0x20536, Code: true}
			c.Word(1, 124)
			for i := 0; i < 125; i++ {
				v, e := cb.Code.Read16(0x20630 + i*2)
				if e != nil {
					return out, e
				}
				if e = cb.Code.Write16(0x20536+i*2, v); e != nil {
					return out, e
				}
				s.A[0].Address += 2
				s.A[1].Address += 2
				c.Word(1, uint16(c.D[1])-1)
			}
			seed, e := m.Read16(0xeb2a)
			if e != nil {
				return out, e
			}
			c.Word(0, seed)
			if e = cb.Code.Write16(0x205ee, uint16(c.D[0])); e != nil {
				return out, e
			}
			terrain, e := m.Read16(0xeb22)
			if e != nil {
				return out, e
			}
			if e = cb.Code.Write16(0x205ec, terrain); e != nil {
				return out, e
			}
			if e = LoadNativeStartupTemplates(cb, &s.A); e != nil {
				return out, e
			}
			s.PC = 0x10de8
		case 0x10dcc:
			world, e := m.Read16(0xeb46)
			if e != nil {
				return out, e
			}
			c.Word(0, world)
			s.PC = 0x10dd2
		case 0x10dd2:
			done, e := child(0x11044, 0x10dd8)
			if e != nil || !done {
				return out, e
			}
		case 0x10dd8:
			done, e := child(0x3cba, 0x10dde)
			if e != nil || !done {
				return out, e
			}
		case 0x10dde:
			if s.LastZero {
				c.D[0] = 0
				s.PC = 0
			} else {
				s.PC = 0x10de8
			}
		case 0x10de8:
			if e := CompileNativeStartupChoices(cb, &s.A); e != nil {
				return out, e
			}
			c.D[0] = 1
			s.PC = 0
		case 0:
			s.Finished = true
			out.Complete = true
			out.FlagsKnown, out.Zero, out.Negative = true, c.D[0] == 0, false
			return out, nil
		default:
			return out, fmt.Errorf("native startup constructor target%x unsupported", s.PC)
		}
	}
	return out, fmt.Errorf("native startup construction exceeded source transition bound")
}

// NativeStartupPowerFrameState is$11078. Resource12 is loaded by a genuine
// resumable child, then the original36-byte power slices are ORed cumulatively
// into the two mutable custom-record fields. RAM uses absolute hunk labels.
type NativeStartupPowerFrameState struct {
	Started, Finished, ChildActive bool
	Registers                      [8]uint32
	A                              [7]NativeRequesterAddress
	ChildPhase                     uint32
	failed                         error
}

func (s *NativeStartupPowerFrameState) Advance(cb NativeStartupResetFrameCallbacks) (out NativeStartupResetFrameStep, failure error) {
	if s == nil || cb.Frame == nil || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.Code) || !winMemoryValid(cb.RAM) {
		return out, fmt.Errorf("native startup power backing missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		s.Started = true
		s.Registers = cb.Frame.D
		cb.Frame.Word(0, 12)
		s.Registers = cb.Frame.D
	}
	cb.Frame.D = s.Registers
	defer func() {
		s.Registers = cb.Frame.D
		if failure != nil {
			s.failed = failure
		}
	}()
	if s.Finished {
		out.Complete = true
		return out, nil
	}
	if cb.Call == nil {
		return out, fmt.Errorf("native startup conquest resource child missing")
	}
	if !s.ChildActive {
		s.ChildActive = true
		out.Calls = append(out.Calls, 0x19cd0)
	}
	result, e := cb.Call(NativeStartupResetFrameCall{Routine: 0x19cd0, Frame: cb.Frame, A: &s.A}, &s.ChildPhase)
	if e != nil {
		return out, e
	}
	if !result.Complete {
		out.Waiting = true
		out.PC = 0x1107c
		return out, nil
	}
	s.ChildActive = false
	s.ChildPhase = 0
	c := cb.Frame
	source, e := cb.Code.Read32(0x11084)
	if e != nil {
		return out, e
	}
	profile, e := cb.Memory.Read16(0xeb42)
	if e != nil {
		return out, e
	}
	if profile != 1 {
		source += 58
	}
	s.A[0] = NativeRequesterAddress{Address: source, Absolute: true}
	s.A[1] = NativeRequesterAddress{Address: cb.CodeBase + 0x20646, Code: true}
	s.A[2] = NativeRequesterAddress{Address: cb.CodeBase + 0x20680, Code: true}
	c.D[1] = 0
	world, e := cb.Memory.Read16(0xeb46)
	if e != nil {
		return out, e
	}
	c.Word(1, world)
	if e = frameDivide(c, 1, 5); e != nil {
		return out, e
	}
	for row := 0; row < 65536; row++ {
		c.D[2] = 8
		s.A[3], s.A[4], s.A[5] = s.A[0], s.A[1], s.A[2]
		for i := 0; i < 9; i++ {
			value, e := cb.RAM.Read32(int(s.A[3].Address))
			if e != nil {
				return out, e
			}
			c.D[0] = value
			s.A[3].Address += 4
			for _, reg := range []int{4, 5} {
				at := int(int64(s.A[reg].Address) - int64(cb.CodeBase))
				old, e := cb.Code.Read32(at)
				if e != nil {
					return out, e
				}
				if e = cb.Code.Write32(at, old|c.D[0]); e != nil {
					return out, e
				}
				s.A[reg].Address += 4
			}
			c.Word(2, uint16(c.D[2])-1)
		}
		s.A[0].Address += 250
		c.Word(1, uint16(c.D[1])-1)
		if uint16(c.D[1]) == 0xffff {
			s.Finished = true
			out.Complete = true
			return out, nil
		}
	}
	return out, fmt.Errorf("native startup power loop exceeded16-bit count")
}
