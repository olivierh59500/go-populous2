package populous2

import "fmt"

type NativeRuntimeProgressionCallbacks struct {
	NativeFileFrameCallbacks
	Audio     NativeRuntimeAudioOperations
	Ownership func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error
	Child     func(NativeStartupResetFrameCall, *uint32) (NativeCommandFrameResult, error)
}

// NativeRuntimeEndingState is the genuine $b142 loop. The shared END resource,
// saved HUNK5 decoder context, scroll words and two real screens remain live
// throughout its four VBlank gates and click termination.
type NativeRuntimeEndingState struct {
	Started, Finished bool
	PC                int
	Registers         [8]uint32
	A                 [7]NativeRequesterAddress
	Resource          *NativeResourceHostFrameState
	failed            error
}

func (s *NativeRuntimeEndingState) Advance(h *NativeRuntimeHost, frame *NativeFrameRegisterContext, supplied NativeRuntimeProgressionCallbacks) (out NativeFileFrameStep, failure error) {
	if s == nil || h == nil || h.Memory == nil || h.Session == nil || frame == nil || frame.AddressBase != h.Memory.BSSBase {
		return out, fmt.Errorf("native ending runtime/context missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		s.Started = true
		s.PC = 0xb142
		s.Registers = frame.D
	}
	frame.D = s.Registers
	defer func() {
		s.Registers = frame.D
		out.PC = s.PC
		if failure != nil {
			s.failed = failure
		}
	}()
	if s.Finished {
		out.Complete = true
		return out, nil
	}
	cb := supplied.NativeFileFrameCallbacks
	cb.Frame, cb.Code, cb.Memory, cb.CodeBase = frame, h.Memory.Code, h.Memory.BSS, h.Memory.CodeBase
	cb.Presentation, cb.Bitmap = h.Session.Presentation, h.Bitmap
	cb.ReadAbsolute = func(address uint32) (byte, error) { return h.Memory.RAM.Read8(int(address)) }
	body := NativeStartupResetFrameCallbacks{Frame: frame, Code: cb.Code, Memory: cb.Memory, RAM: h.Memory.RAM, CodeBase: cb.CodeBase}
	c, m, code := frame, h.Memory.BSS, h.Memory.Code
	savedControl := func(load bool, operand int) error {
		address, e := code.Read32(operand)
		if e != nil {
			return e
		}
		if load {
			v, e := h.Memory.RAM.Read32(int(address))
			if e != nil {
				return e
			}
			c.D[0] = v
			for i, reg := range []int{4, 5} {
				v, e = h.Memory.RAM.Read32(int(address) + 4 + i*4)
				if e != nil {
					return e
				}
				s.A[reg] = NativeRequesterAddress{Address: v, Absolute: true}
			}
		} else {
			for i, v := range []uint32{c.D[0], s.A[4].Address, s.A[5].Address} {
				if e = h.Memory.RAM.Write32(int(address)+i*4, v); e != nil {
					return e
				}
			}
		}
		return nil
	}
	wait := func(clear bool) (bool, error) {
		ready, e := m.Read16(0xa)
		if e != nil {
			return false, e
		}
		if ready == 0 {
			out.Waiting = true
			return false, nil
		}
		if clear {
			if e = m.Write16(0xa, 0); e != nil {
				return false, e
			}
		}
		return true, nil
	}
	for work := 0; work < 64; work++ {
		switch s.PC {
		case 0xb142:
			c.Word(0, 25)
			s.Resource = &NativeResourceHostFrameState{}
			s.PC = 0xb146
		case 0xb146:
			resource, e := h.ResourceCallbacks(c, NativeErrorFrameCallbacks{NativeFileFrameCallbacks: cb, RAM: h.Memory.RAM})
			if e != nil {
				return out, e
			}
			resource.ResourceCallerA = s.A
			step, e := s.Resource.Advance(&h.ResourceRules, resource)
			if e != nil {
				return out, e
			}
			if !step.Complete {
				out.Waiting = true
				return out, nil
			}
			s.Resource = nil
			ptr, e := code.Read32(0xb14e)
			if e != nil {
				return out, e
			}
			s.A[4] = NativeRequesterAddress{Address: ptr, Absolute: true}
			c.D[0] = nativeAnimationInit
			if e = RunNativeProgressionAnimation(body, &s.A); e != nil {
				return out, e
			}
			if e = savedControl(false, 0xb162); e != nil {
				return out, e
			}
			if e = fileFrameSwap(nativeFileBrowserBacking(cb), cb.Presentation); e != nil {
				return out, e
			}
			s.PC = 0xb16c
		case 0xb16c:
			done, e := wait(false)
			if e != nil || !done {
				return out, e
			}
			if e = savedControl(true, 0xb176); e != nil {
				return out, e
			}
			if e = RunNativeProgressionAnimation(body, &s.A); e != nil {
				return out, e
			}
			if e = savedControl(false, 0xb184); e != nil {
				return out, e
			}
			if e = code.Write16(0xb242, 0); e != nil {
				return out, e
			}
			if e = m.Write16(0x140, 0); e != nil {
				return out, e
			}
			s.PC = 0xb194
		case 0xb194, 0xb1a0, 0xb1ac:
			done, e := wait(true)
			if e != nil || !done {
				return out, e
			}
			s.PC += 12
		case 0xb1b8:
			done, e := wait(false)
			if e != nil || !done {
				return out, e
			}
			offset, e := code.Read16(0xb242)
			if e != nil {
				return out, e
			}
			s.A[1] = NativeRequesterAddress{Address: uint32(int64(cb.CodeBase+0xaa24) + int64(int16(offset))), Code: true}
			phase, e := code.Read16(0xb240)
			if e != nil {
				return out, e
			}
			phase = ^phase
			if e = code.Write16(0xb240, phase); e != nil {
				return out, e
			}
			if phase != 0 {
				offset++
				if e = code.Write16(0xb242, offset); e != nil {
					return out, e
				}
			}
			value, e := h.Memory.RAM.Read8(int(s.A[1].Address))
			if e != nil {
				return out, e
			}
			if value == 0 {
				if e = code.Write16(0xb242, 0); e != nil {
					return out, e
				}
				s.A[1] = NativeRequesterAddress{Address: cb.CodeBase + 0xaa24, Code: true}
			}
			c.D[0] = 0
			c.Word(1, 192)
			saved0, saved1, savedA1 := c.D[0], c.D[1], s.A[1]
			for screen, at := range []int{0x1e, 0x1a} {
				if screen == 1 {
					c.D[0], c.D[1], s.A[1] = saved0, saved1, savedA1
				}
				target, e := m.Read32(at)
				if e != nil {
					return out, e
				}
				text := int(int64(s.A[1].Address) - int64(cb.CodeBase))
				if e = campaignRequesterText(nativeFileBrowserBacking(cb), &s.A, target, text); e != nil {
					return out, e
				}
			}
			if e = savedControl(true, 0xb214); e != nil {
				return out, e
			}
			if e = RunNativeProgressionAnimation(body, &s.A); e != nil {
				return out, e
			}
			if e = savedControl(false, 0xb222); e != nil {
				return out, e
			}
			if e = fileFrameSwap(nativeFileBrowserBacking(cb), cb.Presentation); e != nil {
				return out, e
			}
			click, e := m.Read16(0x140)
			if e != nil {
				return out, e
			}
			if click == 0 {
				s.PC = 0xb194
			} else {
				if e = m.Write16(0xeb46, 999); e != nil {
					return out, e
				}
				s.Finished, out.Complete, s.PC = true, true, 0xb23e
				return out, nil
			}
		default:
			return out, fmt.Errorf("native ending source branch $%x unavailable", s.PC)
		}
	}
	return out, fmt.Errorf("native ending did not reach source wait")
}
