package populous2

import "fmt"

// NativeRuntimeProgressionState is the original B244 award controller. Its
// register state, actual resources and VBlank gates survive suspended children.
// Pixel/preview/deity calls remain genuine operations supplied by the host.
type NativeRuntimeProgressionState struct {
	Started, Finished bool
	PC                int
	Registers         [8]uint32
	A                 [7]NativeRequesterAddress
	Resource          *NativeResourceHostFrameState
	Ending            *NativeRuntimeEndingState
	Palette           *NativeFramePaletteState
	ChildRoutine      int
	ChildPhase        uint32
	savedD            [8]uint32
	savedA            [7]NativeRequesterAddress
	savedBack         uint32
	failed            error
}

// AdvanceChild is the full-address ABI used by381E's B244 continuation.
// Incoming address registers are captured once, then the caller receives the
// exact retained outputs after each suspension or actual return.
func (s *NativeRuntimeProgressionState) AdvanceChild(h *NativeRuntimeHost, call NativeStartupResetFrameCall, phase *uint32, cb NativeRuntimeProgressionCallbacks) (NativeCommandFrameResult, error) {
	if s == nil || call.Routine != 0xb244 || call.Frame == nil || call.A == nil || phase == nil {
		return NativeCommandFrameResult{}, fmt.Errorf("native progression child context missing")
	}
	if *phase == 0 {
		*s = NativeRuntimeProgressionState{A: *call.A}
		*phase = 1
	}
	step, err := s.Advance(h, call.Frame, cb)
	*call.A = s.A
	return NativeCommandFrameResult{Complete: step.Complete}, err
}

func (s *NativeRuntimeProgressionState) Advance(h *NativeRuntimeHost, frame *NativeFrameRegisterContext, supplied NativeRuntimeProgressionCallbacks) (out NativeFileFrameStep, failure error) {
	if s == nil || h == nil || h.Memory == nil || frame == nil || frame.AddressBase != h.Memory.BSSBase {
		return out, fmt.Errorf("native progression runtime/context missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		s.Started = true
		s.PC = 0xb244
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
	c, m, code := frame, h.Memory.BSS, h.Memory.Code
	cb := supplied.NativeFileFrameCallbacks
	cb.Frame, cb.Memory, cb.Code, cb.CodeBase = c, m, code, h.Memory.CodeBase
	cb.Presentation, cb.Bitmap = h.Session.Presentation, h.Bitmap
	cb.ReadAbsolute = func(at uint32) (byte, error) { return h.Memory.RAM.Read8(int(at)) }
	backing := nativeFileBrowserBacking(cb)
	body := NativeStartupResetFrameCallbacks{Frame: c, Memory: m, Code: code, RAM: h.Memory.RAM, CodeBase: h.Memory.CodeBase}
	addr := func(at int) NativeRequesterAddress {
		return NativeRequesterAddress{Address: h.Memory.CodeBase + uint32(at), Code: true}
	}
	child := func(routine, next int) (bool, error) {
		if supplied.Child == nil {
			return false, fmt.Errorf("native progression child%x missing", routine)
		}
		if s.ChildRoutine == 0 {
			s.ChildRoutine = routine
		}
		if s.ChildRoutine != routine {
			return false, fmt.Errorf("native progression child changed while pending")
		}
		result, e := supplied.Child(NativeStartupResetFrameCall{Routine: routine, Frame: c, A: &s.A}, &s.ChildPhase)
		if e != nil || !result.Complete {
			out.Waiting = !result.Complete
			return false, e
		}
		s.ChildRoutine, s.ChildPhase, s.PC = 0, 0, next
		return true, nil
	}
	wait := func(clear bool) (bool, error) {
		v, e := m.Read16(0xa)
		if e != nil {
			return false, e
		}
		if v == 0 {
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
	screen := func(reg, at int) error {
		v, e := m.Read32(at)
		if e == nil {
			s.A[reg] = NativeRequesterAddress{Address: v, Chip: true}
		}
		return e
	}
	context := func(load bool, operand int) error {
		at, e := code.Read32(operand)
		if e != nil {
			return e
		}
		if load {
			v, e := h.Memory.RAM.Read32(int(at))
			if e != nil {
				return e
			}
			c.D[0] = v
			for i, reg := range []int{4, 5} {
				v, e = h.Memory.RAM.Read32(int(at) + 4 + i*4)
				if e != nil {
					return e
				}
				s.A[reg] = NativeRequesterAddress{Address: v, Absolute: true}
			}
			return nil
		}
		for i, v := range []uint32{c.D[0], s.A[4].Address, s.A[5].Address} {
			if e = h.Memory.RAM.Write32(int(at)+i*4, v); e != nil {
				return e
			}
		}
		return nil
	}
	load := func(id uint16, next int) (bool, error) {
		if s.Resource == nil {
			c.Word(0, id)
			s.Resource = &NativeResourceHostFrameState{}
		}
		resource, e := h.ResourceCallbacks(c, NativeErrorFrameCallbacks{NativeFileFrameCallbacks: cb, RAM: h.Memory.RAM})
		if e != nil {
			return false, e
		}
		resource.ResourceCallerA = s.A
		step, e := s.Resource.Advance(&h.ResourceRules, resource)
		if e != nil || !step.Complete {
			out.Waiting = !step.Complete
			return false, e
		}
		s.Resource = nil
		s.PC = next
		return true, nil
	}
	// These exact MOVEM masks surround the two-screen pixel children.
	savePixels := func() { s.savedD = c.D; s.savedA = s.A }
	restorePixels := func() {
		for _, reg := range []int{0, 1, 2, 6, 7} {
			c.D[reg] = s.savedD[reg]
		}
		s.A[3], s.A[6] = s.savedA[3], s.savedA[6]
	}
	campaign := NativeCampaignFrameCallbacks{NativeFileFrameCallbacks: cb, RAM: h.Memory.RAM}
	render, e := DecodeNativeRenderFrameRules(h.Bundle.Executable)
	if e != nil {
		return out, e
	}
	for work := 0; work < 512; work++ {
		switch s.PC {
		case 0xb244:
			world, e := m.Read16(0xeb46)
			if e != nil {
				return out, e
			}
			if int16(world) >= 1000 {
				s.Ending = &NativeRuntimeEndingState{A: s.A}
				s.PC = 0xb24e
			} else {
				s.PC = 0xb25e
			}
		case 0xb24e:
			step, e := s.Ending.Advance(h, c, supplied)
			s.A = s.Ending.A
			if e != nil || !step.Complete {
				out.Waiting = !step.Complete
				return out, e
			}
			s.Ending = nil
			if e = m.Write16(0xeb46, 0); e != nil {
				return out, e
			}
			s.PC = 0xb67c
		case 0xb25e:
			done, e := load(9, 0xb268)
			if e != nil || !done {
				return out, e
			}
		case 0xb268:
			done, e := load(8, 0xb272)
			if e != nil || !done {
				return out, e
			}
		case 0xb272:
			at, e := code.Read32(0xb274)
			if e != nil {
				return out, e
			}
			s.A[4] = NativeRequesterAddress{Address: at, Absolute: true}
			c.D[0] = nativeAnimationInit
			if e = RunNativeProgressionAnimation(body, &s.A); e != nil {
				return out, e
			}
			if e = context(false, 0xb288); e != nil {
				return out, e
			}
			if e = fileFrameSwap(backing, cb.Presentation); e != nil {
				return out, e
			}
			s.PC = 0xb292
		case 0xb292:
			ready, e := wait(false)
			if e != nil || !ready {
				return out, e
			}
			if e = context(true, 0xb29c); e != nil {
				return out, e
			}
			if e = RunNativeProgressionAnimation(body, &s.A); e != nil {
				return out, e
			}
			if e = context(false, 0xb2aa); e != nil {
				return out, e
			}
			c.D[1] = 1000
			if e = frameDivide(c, 1, 31); e != nil {
				return out, e
			}
			v, e := m.Read16(0xeb46)
			if e != nil {
				return out, e
			}
			c.RestoreWord(0, v)
			if e = frameDivide(c, 0, uint16(c.D[1])); e != nil {
				return out, e
			}
			c.D[0] = uint32(uint16(c.D[0])) * 14
			s.A[3] = addr(0x96e4)
			s.A[3].Address = uint32(int64(s.A[3].Address) + int64(int16(c.D[0])))
			s.A[2] = addr(0xb71c)
			s.savedA = s.A
			if e = screen(0, 0x1e); e != nil {
				return out, e
			}
			s.PC = 0xb2de
		case 0xb2de:
			done, e := child(0xbaee, 0xb2e4)
			if e != nil || !done {
				return out, e
			}
		case 0xb2e4:
			s.A[2], s.A[3] = s.savedA[2], s.savedA[3]
			if e = screen(0, 0x1a); e != nil {
				return out, e
			}
			s.PC = 0xb2ee
		case 0xb2ee:
			done, e := child(0xbaee, 0xb2f4)
			if e != nil || !done {
				return out, e
			}
		case 0xb2f4:
			v, e := m.Read16(0xeb42)
			if e != nil {
				return out, e
			}
			c.Word(0, v)
			s.A[3] = NativeRequesterAddress{Address: h.Memory.BSSBase + 0xe76a, Absolute: true}
			c.D[0] = uint32(uint16(c.D[0])) * 314
			s.A[3].Address = uint32(int64(s.A[3].Address) + int64(int16(c.D[0])))
			c.D[0] = 0
			v, e = m.Read16(0xdd0)
			if e != nil {
				return out, e
			}
			c.Word(0, v)
			if e = frameDivide(c, 0, 13007); e != nil {
				return out, e
			}
			if int16(c.D[0]) > 5 {
				c.Word(0, 5)
			}
			if e = code.Write16(0xb6fe, uint16(c.D[0])); e != nil {
				return out, e
			}
			s.A[3].Address += 0x51
			s.A[2] = addr(0xb72e)
			s.savedA = s.A
			if e = screen(0, 0x1e); e != nil {
				return out, e
			}
			s.PC = 0xb336
		case 0xb336:
			done, e := child(0xbaee, 0xb33c)
			if e != nil || !done {
				return out, e
			}
		case 0xb33c:
			s.A[2], s.A[3] = s.savedA[2], s.savedA[3]
			if e = screen(0, 0x1a); e != nil {
				return out, e
			}
			s.PC = 0xb346
		case 0xb346:
			done, e := child(0xbaee, 0xb34c)
			if e != nil || !done {
				return out, e
			}
		case 0xb34c:
			c.D[0] = 0
			v, e := m.Read16(0xeb46)
			if e != nil {
				return out, e
			}
			c.Word(0, v)
			if e = frameDivide(c, 0, 150); e != nil {
				return out, e
			}
			c.Word(0, uint16(c.D[0])-1)
			if int16(c.D[0]) <= 0 {
				s.PC = 0xb394
				continue
			}
			s.savedD[0] = c.D[0]
			s.PC = 0xb360
		case 0xb360:
			done, e := load(10, 0xb36a)
			if e != nil || !done {
				return out, e
			}
		case 0xb36a:
			c.D[0] = uint32(uint16(s.savedD[0])) * 0x480
			at, e := code.Read32(0xb372)
			if e != nil {
				return out, e
			}
			s.A[1] = NativeRequesterAddress{Address: uint32(int64(at) + int64(int16(c.D[0]))), Absolute: true}
			s.savedA[1] = s.A[1]
			if e = screen(0, 0x1e); e != nil {
				return out, e
			}
			s.PC = 0xb382
		case 0xb382:
			done, e := child(0xb6b6, 0xb386)
			if e != nil || !done {
				return out, e
			}
		case 0xb386:
			s.A[1] = s.savedA[1]
			if e = screen(0, 0x1a); e != nil {
				return out, e
			}
			s.PC = 0xb390
		case 0xb390:
			done, e := child(0xb6b6, 0xb394)
			if e != nil || !done {
				return out, e
			}
		case 0xb394:
			done, e := load(11, 0xb39e)
			if e != nil || !done {
				return out, e
			}
		case 0xb39e:
			ready, e := wait(false)
			if e != nil || !ready {
				return out, e
			}
			if e = fileFrameSwap(backing, cb.Presentation); e != nil {
				return out, e
			}
			for _, at := range []int{0xb704, 0xb706, 0xb702} {
				if e = code.Write16(at, 0); e != nil {
					return out, e
				}
			}
			if e = m.Write32(0x140, 0); e != nil {
				return out, e
			}
			if e = code.Write16(0xb6f8, 0xffff); e != nil {
				return out, e
			}
			s.A[6] = s.A[3]
			s.PC = 0xb3cc
		case 0xb3cc:
			if int32(s.A[6].Address) > int32(s.A[3].Address) {
				s.PC = 0xb43a
				continue
			}
			v, e := m.Read32(0xeb24)
			if e != nil {
				return out, e
			}
			if e = m.Write32(0xeb28, v); e != nil {
				return out, e
			}
			s.PC = 0xb3da
		case 0xb3da:
			done, e := child(0xcd22, 0xb3e0)
			if e != nil || !done {
				return out, e
			}
		case 0xb3e0:
			s.A[1] = addr(0x33612)
			if e = h.Memory.RAM.Write16(int(s.A[1].Address), 16); e != nil {
				return out, e
			}
			s.A[1].Address += 2
			if e = h.Memory.RAM.Write16(int(s.A[1].Address), 123); e != nil {
				return out, e
			}
			s.A[1].Address += 2
			if e = h.Memory.RAM.Write32(int(s.A[1].Address), h.Memory.CodeBase+0xe0fe); e != nil {
				return out, e
			}
			if e = screen(0, 0x1e); e != nil {
				return out, e
			}
			s.PC = 0xb3fa
		case 0xb3fa:
			done, e := child(0xd8cc, 0xb400)
			if e != nil || !done {
				return out, e
			}
		case 0xb400:
			if e = screen(0, 0x1a); e != nil {
				return out, e
			}
			s.PC = 0xb406
		case 0xb406:
			done, e := child(0xd8cc, 0xb40c)
			if e != nil || !done {
				return out, e
			}
		case 0xb40c:
			s.A[1] = addr(0x33612)
			if e = h.Memory.RAM.Write16(int(s.A[1].Address), 4); e != nil {
				return out, e
			}
			s.A[1].Address += 2
			if e = h.Memory.RAM.Write16(int(s.A[1].Address), 4); e != nil {
				return out, e
			}
			s.A[1].Address += 2
			if e = h.Memory.RAM.Write32(int(s.A[1].Address), h.Memory.CodeBase+0xe196); e != nil {
				return out, e
			}
			s.A[3] = NativeRequesterAddress{Address: h.Memory.BSSBase + 0xeb70, Absolute: true}
			c.D[7] = 0
			v, e := m.Read16(0xeb6e)
			if e != nil {
				return out, e
			}
			c.Word(7, v)
			if v == 0 {
				s.PC = 0xb530
				continue
			}
			s.A[6] = NativeRequesterAddress{Address: s.A[3].Address + c.D[7], Absolute: true}
			if e = frameDivide(c, 7, 2000); e != nil {
				return out, e
			}
			s.PC = 0xb43a
		case 0xb43a, 0xb446, 0xb452, 0xb45e:
			ready, e := wait(true)
			if e != nil || !ready {
				return out, e
			}
			s.PC += 12
		case 0xb46a:
			ready, e := wait(false)
			if e != nil || !ready {
				return out, e
			}
			v, e := code.Read16(0xb6f8)
			if e != nil {
				return out, e
			}
			c.Word(2, v)
			if int16(v) < 0 {
				s.PC = 0xb4a8
				continue
			}
			v, e = code.Read16(0xb6fa)
			if e != nil {
				return out, e
			}
			c.Word(0, v)
			v, e = code.Read16(0xb6fc)
			if e != nil {
				return out, e
			}
			c.Word(1, v)
			savePixels()
			if e = screen(0, 0x1e); e != nil {
				return out, e
			}
			s.PC = 0xb48e
		case 0xb48e:
			done, e := child(0xe0fe, 0xb494)
			if e != nil || !done {
				return out, e
			}
		case 0xb494:
			restorePixels()
			if e = screen(0, 0x1a); e != nil {
				return out, e
			}
			s.PC = 0xb49e
		case 0xb49e:
			done, e := child(0xe0fe, 0xb4a4)
			if e != nil || !done {
				return out, e
			}
		case 0xb4a4:
			restorePixels()
			s.PC = 0xb4a8
		case 0xb4a8:
			c.D[6] = c.D[7]
			if e = code.Write16(0xb6f8, 0xffff); e != nil {
				return out, e
			}
			s.PC = 0xb4b2
		case 0xb4b2:
			v, e := h.Memory.RAM.Read16(int(s.A[3].Address))
			if e != nil {
				return out, e
			}
			s.A[3].Address += 2
			c.Word(5, v)
			if v == 0 {
				s.PC = 0xb3cc
				continue
			}
			c.Word(2, v)
			c.Word(2, v<<4|v>>12)
			c.Word(4, v>>6)
			s.PC = 0xb4c0
		case 0xb4c0:
			ready, e := wait(false)
			if e != nil || !ready {
				return out, e
			}
			c.Word(4, uint16(c.D[4])&63)
			c.Word(5, uint16(c.D[5])&63)
			c.Word(2, uint16(c.D[2])&15)
			c.D[0] = 64
			c.Word(0, uint16(c.D[0])+uint16(c.D[4])-uint16(c.D[5])+16)
			c.Word(1, (uint16(c.D[4])+uint16(c.D[5]))>>1)
			c.Word(1, uint16(c.D[1])+123)
			if e = screen(0, 0x1e); e != nil {
				return out, e
			}
			if uint16(c.D[2]) != 0 {
				s.PC = 0xb50e
				continue
			}
			if e = code.Write16(0xb6fa, uint16(c.D[0])); e != nil {
				return out, e
			}
			if e = code.Write16(0xb6fc, uint16(c.D[1])); e != nil {
				return out, e
			}
			s.PC = 0xb4fc
		case 0xb4fc:
			done, e := child(0xe11a, 0xb502)
			if e != nil || !done {
				return out, e
			}
		case 0xb502:
			if e = code.Write16(0xb6f8, uint16(c.D[2])); e != nil {
				return out, e
			}
			c.Word(2, 5)
			c.Word(6, 0)
			s.PC = 0xb50e
		case 0xb50e:
			savePixels()
			s.PC = 0xb512
		case 0xb512:
			done, e := child(0xe0fe, 0xb518)
			if e != nil || !done {
				return out, e
			}
		case 0xb518:
			restorePixels()
			if e = screen(0, 0x1a); e != nil {
				return out, e
			}
			s.PC = 0xb522
		case 0xb522:
			done, e := child(0xe0fe, 0xb528)
			if e != nil || !done {
				return out, e
			}
		case 0xb528:
			restorePixels()
			c.Word(6, uint16(c.D[6])-1)
			if uint16(c.D[6]) != 0xffff {
				s.PC = 0xb4b2
			} else {
				s.PC = 0xb530
			}
		case 0xb530:
			v, e := code.Read16(0xb704)
			if e != nil {
				return out, e
			}
			c.Word(2, v+12)
			if int16(c.D[2]) < 96 {
				s.PC = 0xb5e4
				continue
			}
			c.Word(2, 0)
			v, e = code.Read16(0xb702)
			if e != nil {
				return out, e
			}
			if v != 0 {
				s.PC = 0xb56a
				continue
			}
			v, e = code.Read16(0xb6fe)
			if e != nil {
				return out, e
			}
			if int16(v) <= 0 {
				s.PC = 0xb5e4
				continue
			}
			if e = code.Write16(0xb6fe, v-1); e != nil {
				return out, e
			}
			if e = code.Write16(0xb702, 96); e != nil {
				return out, e
			}
			s.PC = 0xb5e4
		case 0xb56a:
			c.Word(0, 0x15e)
			if supplied.Audio.DirectCue == nil {
				return out, fmt.Errorf("native progression direct cue missing")
			}
			if e = supplied.Audio.DirectCue(0x15e, c); e != nil {
				return out, e
			}
			if e = code.Write16(0xb702, 0); e != nil {
				return out, e
			}
			v, e := code.Read16(0xb706)
			if e != nil {
				return out, e
			}
			c.Word(0, v)
			if e = code.Write16(0xb706, v+4); e != nil {
				return out, e
			}
			s.A[0] = addr(0xb708)
			at := int(int64(s.A[0].Address) + int64(int16(c.D[0])))
			x, e := h.Memory.RAM.Read16(at)
			if e != nil {
				return out, e
			}
			y, e := h.Memory.RAM.Read16(at + 2)
			if e != nil {
				return out, e
			}
			c.RestoreWord(0, x)
			c.RestoreWord(1, y)
			c.Word(2, 0x7cc)
			s.savedD = c.D
			s.savedA = s.A
			s.PC = 0xb59c
		case 0xb59c:
			if e = s.drawAwardImage(c, &render, campaign, supplied); e != nil {
				return out, e
			}
			s.restoreAward(c)
			s.savedBack, e = m.Read32(0x1e)
			if e != nil {
				return out, e
			}
			front, e := m.Read32(0x1a)
			if e != nil {
				return out, e
			}
			if e = m.Write32(0x1e, front); e != nil {
				return out, e
			}
			s.PC = 0xb5c6
		case 0xb5c6:
			if e = s.drawAwardImage(c, &render, campaign, supplied); e != nil {
				return out, e
			}
			s.restoreAward(c)
			if e = m.Write32(0x1e, s.savedBack); e != nil {
				return out, e
			}
			c.Word(2, 0)
			s.PC = 0xb5e4
		case 0xb5e4:
			v, e := code.Read16(0x186de)
			if e != nil {
				return out, e
			}
			if e = code.Write16(0x186de, v+1); e != nil {
				return out, e
			}
			if e = code.Write16(0xb704, uint16(c.D[2])); e != nil {
				return out, e
			}
			v, e = code.Read16(0xb702)
			if e != nil {
				return out, e
			}
			c.Word(2, uint16(c.D[2])+v)
			c.Word(0, 148)
			c.Word(1, 52)
			s.A[0] = addr(0x213e6)
			s.A[0].Address = uint32(int64(s.A[0].Address) + int64(int16(c.D[2])))
			at := int(s.A[0].Address)
			source, e := h.Memory.RAM.Read32(at)
			if e != nil {
				return out, e
			}
			s.A[1] = NativeRequesterAddress{Address: source, Absolute: true}
			height, e := h.Memory.RAM.Read16(at + 6)
			if e != nil {
				return out, e
			}
			c.Word(2, height)
			routine, e := h.Memory.RAM.Read32(at + 8)
			if e != nil {
				return out, e
			}
			s.A[2] = NativeRequesterAddress{Address: routine, Code: true}
			if e = screen(0, 0x1e); e != nil {
				return out, e
			}
			s.savedD = c.D
			s.savedA = s.A
			s.PC = 0xb61e
		case 0xb61e:
			if e = s.drawAnimatedAward(c, &render, campaign, supplied); e != nil {
				return out, e
			}
			s.restoreAnimatedAward(c)
			s.PC = 0xb630
		case 0xb630:
			if supplied.Ownership == nil {
				return out, fmt.Errorf("native progression ownership missing")
			}
			if e = supplied.Ownership(false, c, &s.A); e != nil {
				return out, e
			}
			if e = screen(0, 0x1a); e != nil {
				return out, e
			}
			if e = campaignChildSprite(&render, campaign, &s.A, s.A[2].Address); e != nil {
				return out, e
			}
			if e = supplied.Ownership(true, c, &s.A); e != nil {
				return out, e
			}
			s.restoreAnimatedAward(c)
			s.PC = 0xb648
		case 0xb648:
			if supplied.Audio.Command == nil {
				return out, fmt.Errorf("native progression audio scheduler missing")
			}
			if e = TickNativeCampaignAudio(campaign, &s.A, supplied.Audio.Command); e != nil {
				return out, e
			}
			if e = context(true, 0xb652); e != nil {
				return out, e
			}
			saved7, saved3 := c.D[7], s.A[3]
			if e = RunNativeProgressionAnimation(body, &s.A); e != nil {
				return out, e
			}
			c.D[7], s.A[3] = saved7, saved3
			if e = context(false, 0xb668); e != nil {
				return out, e
			}
			if e = fileFrameSwap(backing, cb.Presentation); e != nil {
				return out, e
			}
			clicked, e := m.Read16(0x140)
			if e != nil {
				return out, e
			}
			if clicked == 0 {
				s.PC = 0xb3cc
				queued, err := m.Read16(0xeb6e)
				if err != nil {
					return out, err
				}
				// With an empty redraw queue the original loop has no786
				// gate. Yield at its actual swap so the host can deliver
				// the next IRQ without repeating completed drawing work.
				if queued == 0 {
					out.Waiting = true
					return out, nil
				}
				continue
			}
			s.PC = 0xb67c
		case 0xb67c:
			audio, e := DecodeNativeAudioControlFrameRules(h.Bundle.Executable)
			if e != nil {
				return out, e
			}
			if _, e = audio.Run(0x1842e, NativeAudioControlFrameCallbacks{Memory: m, Frame: c, CodeBase: h.Memory.CodeBase, Command: supplied.Audio.Command, MusicCommand: supplied.Audio.MusicCommand}); e != nil {
				return out, e
			}
			s.A[2], s.A[3] = addr(0x33844), addr(0x3361a)
			s.PC = 0xb68e
		case 0xb68e:
			if s.Palette == nil {
				bank := func(at int) (NativeFramePaletteBank, error) {
					p := NativeFramePaletteBank{Address: h.Memory.CodeBase + uint32(at)}
					for i := range p.Words {
						v, e := code.Read16(at + i*2)
						if e != nil {
							return p, e
						}
						p.Words[i] = v
					}
					return p, nil
				}
				from, e := bank(0x33844)
				if e != nil {
					return out, e
				}
				to, e := bank(0x3361a)
				if e != nil {
					return out, e
				}
				s.Palette = NewNativeFramePaletteState(from, to, h.Memory.CodeBase)
			}
			done, e := s.Palette.Advance(cb.Presentation, c, m)
			offset := uint32(0x34)
			if done {
				offset = 0x74
			}
			s.A[0] = NativeRequesterAddress{Address: cb.Presentation.ChipBase + offset, Chip: true}
			s.A[1] = NativeRequesterAddress{Address: cb.Presentation.ChipBase + 0x200 + offset, Chip: true}
			if e != nil || !done {
				out.Waiting = !done
				return out, e
			}
			s.Palette = nil
			c.D[0] = 0
			v, e := m.Read16(0xeb42)
			if e != nil {
				return out, e
			}
			c.Word(0, v)
			s.A[1] = NativeRequesterAddress{Address: h.Memory.BSSBase + 0xe76a, Absolute: true}
			c.D[0] = uint32(uint16(c.D[0])) * 314
			s.A[1].Address = uint32(int64(s.A[1].Address) + int64(int16(c.D[0])))
			v, e = h.Memory.RAM.Read16(int(s.A[1].Address) + 0x58)
			if e != nil {
				return out, e
			}
			if v == 0 {
				s.PC = 0xb6b4
			} else {
				s.PC = 0xb6ae
			}
		case 0xb6ae:
			done, e := child(0xb740, 0xb6b4)
			if e != nil || !done {
				return out, e
			}
		case 0xb6b4:
			s.Finished, out.Complete = true, true
			s.PC = 0
			return out, nil
		default:
			return out, fmt.Errorf("native progression branch%x unsupported", s.PC)
		}
	}
	out.Waiting = true
	return out, nil
}

func (s *NativeRuntimeProgressionState) restoreAward(c *NativeFrameRegisterContext) {
	for _, reg := range []int{0, 1, 2, 7} {
		c.D[reg] = s.savedD[reg]
	}
	s.A = s.savedA
}

func (s *NativeRuntimeProgressionState) drawAwardImage(c *NativeFrameRegisterContext, r *NativeRenderFrameRules, cb NativeCampaignFrameCallbacks, supplied NativeRuntimeProgressionCallbacks) error {
	if supplied.Ownership == nil {
		return fmt.Errorf("native progression ownership missing")
	}
	if e := supplied.Ownership(false, c, &s.A); e != nil {
		return e
	}
	if e := DrawNativeCampaignImage(r, cb, &s.A); e != nil {
		return e
	}
	return supplied.Ownership(true, c, &s.A)
}
func (s *NativeRuntimeProgressionState) restoreAnimatedAward(c *NativeFrameRegisterContext) {
	c.D = s.savedD
	for i := 1; i < 7; i++ {
		s.A[i] = s.savedA[i]
	}
}
func (s *NativeRuntimeProgressionState) drawAnimatedAward(c *NativeFrameRegisterContext, r *NativeRenderFrameRules, cb NativeCampaignFrameCallbacks, supplied NativeRuntimeProgressionCallbacks) error {
	if supplied.Ownership == nil {
		return fmt.Errorf("native progression ownership missing")
	}
	if e := supplied.Ownership(false, c, &s.A); e != nil {
		return e
	}
	if e := campaignChildSprite(r, cb, &s.A, s.A[2].Address); e != nil {
		return e
	}
	return supplied.Ownership(true, c, &s.A)
}
