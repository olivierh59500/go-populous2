package populous2

import "fmt"

// NativeStartupMenuFrameState is the original $3b64 main menu. Workspace,
// input and real screen RAM remain caller owned across source polling edges.
type NativeStartupMenuFrameState struct {
	Started, Finished bool
	PC                int
	Registers         [8]uint32
	A                 [7]NativeRequesterAddress
	ChildActive       bool
	ChildRoutine      int
	ChildPhase        uint32
	loadResult        uint16
	failed            error
}

func (s *NativeStartupMenuFrameState) Advance(cb NativeCampaignFrameCallbacks) (out NativeCampaignFrameStep, failure error) {
	if s == nil || cb.Frame == nil || cb.Presentation == nil || cb.Bitmap == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.RAM) {
		return out, fmt.Errorf("native startup menu backing missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		s.Started = true
		s.PC = 0x3b64
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
	c, m, code := cb.Frame, cb.Memory, cb.Code
	b := nativeRequesterFrameBacking{Code: code, Memory: m, CodeBase: cb.CodeBase, Frame: c, Bitmap: cb.Bitmap, Sound: cb.Sound, ReadAbsolute: cb.ReadAbsolute}
	ca := func(at int) NativeRequesterAddress {
		return NativeRequesterAddress{Address: cb.CodeBase + uint32(at), Code: true}
	}
	child := func(routine, next int) (bool, error) {
		if cb.Child == nil {
			return false, fmt.Errorf("native startup menu child%x has no actual body", routine)
		}
		if !s.ChildActive {
			s.ChildActive = true
			s.ChildRoutine = routine
		}
		if s.ChildRoutine != routine {
			return false, fmt.Errorf("native startup menu child changed during a wait")
		}
		result, err := cb.Child(NativeStartupResetFrameCall{Routine: routine, Frame: c, A: &s.A}, &s.ChildPhase)
		if err != nil {
			return false, err
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
	terminal := func() {
		out.Complete = true
		out.FlagsKnown = true
		out.Zero = c.D[0] == 0
		out.Negative = int32(c.D[0]) < 0
	}
	if s.Finished {
		terminal()
		return out, nil
	}
	for transitions := 0; transitions < 64; transitions++ {
		switch s.PC {
		case 0x3b64, 0x3b76:
			targetAt, next := 0x1e, 0x3b76
			if s.PC == 0x3b76 {
				targetAt, next = 0x1a, 0x3b88
			}
			target, err := m.Read32(targetAt)
			if err != nil {
				return out, err
			}
			s.A[0], s.A[1] = ca(0x34828), NativeRequesterAddress{Address: target, Chip: true}
			if err = copyNativeStartupMenuBackground(cb, &s.A); err != nil {
				return out, err
			}
			s.PC = next
		case 0x3b88:
			s.A[1], s.A[2] = ca(0x8854), NativeRequesterAddress{Address: 0, Absolute: true}
			c.D[3] = 1
			if err := campaignRequesterCompile(b, &s.A, s.A[1].Address, 0); err != nil {
				return out, err
			}
			if err := code.Write16(0xab56, 40); err != nil {
				return out, err
			}
			start, err := code.Read16(0xab4e)
			if err != nil {
				return out, err
			}
			s.A[1] = ca(0xab4e + int(int16(start)) + 99)
			if err = code.Write8(int(s.A[1].Address-cb.CodeBase), 0); err != nil {
				return out, err
			}
			if err = m.Write16(0x140, 0); err != nil {
				return out, err
			}
			s.PC = 0x3bb8
		case 0x3bb8:
			target, err := m.Read32(0x1e)
			if err != nil {
				return out, err
			}
			start, err := code.Read16(0xab4e)
			if err != nil {
				return out, err
			}
			s.A[0], s.A[1] = NativeRequesterAddress{Address: target, Chip: true}, ca(0xab4e+int(int16(start)))
			x, err := code.Read16(0xab50)
			if err != nil {
				return out, err
			}
			y, err := code.Read16(0xab52)
			if err != nil {
				return out, err
			}
			c.Word(0, x)
			c.Word(1, y)
			if err = campaignRequesterText(b, &s.A, target, 0xab4e+int(int16(start))); err != nil {
				return out, err
			}
			// $072e saves/restores its only modified data/address registers
			// D0/A0. Its real screen/Copper/ready state is not a no-op.
			if err = fileFrameSwap(b, cb.Presentation); err != nil {
				return out, err
			}
			gate, err := m.Read16(0x3b0)
			if err != nil {
				return out, err
			}
			if gate == 0 {
				s.A[2], s.A[3] = ca(0x3361a), ca(0x3c528)
				s.PC = 0x3bd2
			} else {
				s.PC = 0x3bd8
			}
		case 0x3bd2:
			done, err := child(0x102e4, 0x3bd8)
			if err != nil || !done {
				return out, err
			}
		case 0x3bd8:
			if _, err := campaignRequesterClick(b, &s.A); err != nil {
				return out, err
			}
			branch, err := code.Read16(0x3c22 + int(int16(c.D[0])))
			if err != nil {
				return out, err
			}
			c.Word(0, branch)
			s.PC = 0x3c22 + int(int16(branch))
			if s.PC == 0x3bb8 {
				out.Waiting = true
				return out, nil
			}
		case 0x3be6, 0x3c32:
			s.A[2], s.A[3] = ca(0x3c528), ca(0x3361a)
			next := 0x3bf8
			if s.PC == 0x3c32 {
				next = 0x3c44
			}
			done, err := child(0x102e4, next)
			if err != nil || !done {
				return out, err
			}
		case 0x3bf8, 0x3c0e:
			target, err := m.Read32(0x1e)
			if err != nil {
				return out, err
			}
			s.A[0] = NativeRequesterAddress{Address: target, Chip: true}
			c.Word(0, 0x1f3f)
			bitmap, err := cb.Bitmap(target)
			if err != nil {
				return out, err
			}
			if len(bitmap) != 32000 {
				return out, fmt.Errorf("native startup clear outside actual bitmap")
			}
			for i := 0; i < 8000; i++ {
				clear(bitmap[i*4 : i*4+4])
				s.A[0].Address += 4
				c.Word(0, uint16(c.D[0])-1)
			}
			if s.PC == 0x3bf8 {
				if err = fileFrameSwap(b, cb.Presentation); err != nil {
					return out, err
				}
				s.PC = 0x3c0e
			} else {
				c.D[0] = 0
				s.Finished = true
				terminal()
				return out, nil
			}
		case 0x3c44:
			done, err := child(0xb740, 0x3b64)
			if err != nil || !done {
				return out, err
			}
		case 0x3c4e, 0x3c58, 0x3ca0, 0x3cac:
			mode := uint16(2)
			switch s.PC {
			case 0x3c58:
				mode = 4
			case 0x3ca0:
				mode = 8
			case 0x3cac:
				mode = 10
			}
			if err := m.Write16(0xeb44, mode); err != nil {
				return out, err
			}
			s.PC = 0x3be6
		case 0x3c62:
			if err := code.Write16(0x3f90, 1); err != nil {
				return out, err
			}
			if err := code.Write16(0x4468, 0); err != nil {
				return out, err
			}
			s.PC = 0x3c72
		case 0x3c72:
			done, err := child(0x3f92, 0x3c78)
			if err != nil || !done {
				return out, err
			}
		case 0x3c78:
			s.loadResult = uint16(c.D[0])
			s.A[2], s.A[3] = ca(0x3c528), ca(0x3361a)
			s.PC = 0x3c86
		case 0x3c86:
			done, err := child(0x102e4, 0x3c8c)
			if err != nil || !done {
				return out, err
			}
		case 0x3c8c:
			if s.loadResult == 0 {
				s.PC = 0x3b64
			} else {
				c.D[0] = 1
				s.Finished = true
				terminal()
				return out, nil
			}
		case 0x3c96:
			if err := m.Write16(0x3aa, 1); err != nil {
				return out, err
			}
			c.D[0] = 0xffffffff
			s.PC = 0x3ca0 // Original fall-through into mode8, then cleanup returns0.
		default:
			return out, fmt.Errorf("native startup menu PC%x has no translated source", s.PC)
		}
	}
	return out, fmt.Errorf("native startup menu exceeded source transition bound")
}

// copyNativeStartupMenuBackground is actual $f6da: 500 pairs of eight-LONG
// MOVEM blocks. D0-D7/A3 are restored; A2 retains the last copied bitmap LONG.
func copyNativeStartupMenuBackground(cb NativeCampaignFrameCallbacks, a *[7]NativeRequesterAddress) error {
	for block := 0; block < 1000; block++ {
		var words [8]uint32
		for i := range words {
			v, err := cb.RAM.Read32(int(a[0].Address) + i*4)
			if err != nil {
				return err
			}
			words[i] = v
		}
		for i, v := range words {
			if err := cb.RAM.Write32(int(a[1].Address)+i*4, v); err != nil {
				return err
			}
		}
		a[2] = NativeRequesterAddress{Address: words[6], Absolute: true}
		a[0].Address += 32
		a[1].Address += 32
	}
	return nil
}
