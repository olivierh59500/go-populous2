package populous2

import "fmt"

// NativeRuntimeResultCallbacks supplies original result children. Audio,
// ownership and reset/deity/ending callbacks run inside the same already-owned
// runtime scope; asynchronous child completion is explicit.
type NativeRuntimeResultCallbacks struct {
	Audio     NativeRuntimeAudioOperations
	Sound     func(uint16, *NativeFrameRegisterContext) error
	Ownership func(bool, *NativeFrameRegisterContext) error
	Child     func(NativeStartupResetFrameCall, *uint32) (NativeCommandFrameResult, error)
}

// NativeRuntimeResultState retains 381E's source continuation at real VBlank,
// click, palette and child boundaries. It never reruns the follower pass.
// Incoming address registers are optional at the host boundary; source-assigned
// addresses used by this routine remain explicit physical labels.
type NativeRuntimeResultState struct {
	Started, Complete bool
	PC                int
	Registers         [8]uint32
	A                 [7]NativeRequesterAddress
	Saved             [4]uint32 // Original 39BA saves the complete D2-D5 LONG registers.
	ChildRoutine      int
	ChildPhase        uint32
	Score             uint16
	Eliminated        uint16
	palette           *NativeFramePaletteState
	failed            error
}

type NativeRuntimeResultStep struct {
	Complete, Waiting bool
	PC                int
	ChildRoutine      int
}

func (s *NativeRuntimeResultState) Begin(identity uint16, frame *NativeFrameRegisterContext) error {
	if s == nil || frame == nil || s.Started {
		return fmt.Errorf("native result already active/context missing")
	}
	s.Started, s.PC, s.Eliminated = true, 0x381e, identity
	s.Registers = frame.D
	s.Registers[0] = hudWord(s.Registers[0], identity)
	return nil
}

func (s *NativeRuntimeResultState) Advance(h *NativeRuntimeHost, frame *NativeFrameRegisterContext, cb NativeRuntimeResultCallbacks) (out NativeRuntimeResultStep, failure error) {
	if s == nil || !s.Started || h == nil || h.Memory == nil || frame == nil || frame.AddressBase != h.Memory.BSSBase {
		return out, fmt.Errorf("native result state/runtime/context missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	frame.D = s.Registers
	defer func() {
		s.Registers = frame.D
		out.PC = s.PC
		if failure != nil {
			s.failed = failure
		}
	}()
	if s.Complete {
		out.Complete = true
		return out, nil
	}
	c, m, code := frame, h.Memory.BSS, h.Memory.Code
	address := func(at int) NativeRequesterAddress {
		return NativeRequesterAddress{Address: h.Memory.CodeBase + uint32(at), Code: true}
	}
	ownership := func(owned bool) error {
		if cb.Ownership == nil {
			return fmt.Errorf("native result E28/E4C host child missing")
		}
		saved := c.D
		err := cb.Ownership(owned, c)
		c.D = saved
		return err
	}
	child := func(routine, next int) (bool, error) {
		if cb.Child == nil {
			return false, fmt.Errorf("native result source child%x missing", routine)
		}
		if s.ChildRoutine == 0 {
			s.ChildRoutine = routine
		}
		if s.ChildRoutine != routine {
			return false, fmt.Errorf("native result child changed while suspended")
		}
		step, err := cb.Child(NativeStartupResetFrameCall{Routine: routine, Frame: c, A: &s.A}, &s.ChildPhase)
		if err != nil || !step.Complete {
			out.Waiting = !step.Complete
			out.ChildRoutine = routine
			return false, err
		}
		s.ChildRoutine, s.ChildPhase, s.PC = 0, 0, next
		return true, nil
	}
	for transitions := 0; transitions < 64; transitions++ {
		switch s.PC {
		case 0x381e:
			if err := ownership(true); err != nil {
				return out, err
			}
			if err := code.Write16(0x3b62, uint16(c.D[0])); err != nil {
				return out, err
			}
			rules, err := DecodeNativeAudioControlFrameRules(h.Bundle.Executable)
			if err != nil {
				return out, err
			}
			_, err = rules.Run(0x1842e, NativeAudioControlFrameCallbacks{Memory: m, Frame: c, CodeBase: h.Memory.CodeBase, Command: cb.Audio.Command, MusicCommand: cb.Audio.MusicCommand})
			if err != nil {
				return out, err
			}
			if err := code.Write16(0xa2a, 0); err != nil {
				return out, err
			}
			if err := code.Write32(0x3ad0, h.Memory.CodeBase+0x9675); err != nil {
				return out, err
			}
			c.Word(0, 0x500)
			identity, err := code.Read16(0x3b62)
			if err != nil {
				return out, err
			}
			if identity != 1 {
				c.Word(0, 0x168)
				if err := code.Write32(0x3ad0, h.Memory.CodeBase+0x9670); err != nil {
					return out, err
				}
			}
			if cb.Sound == nil {
				return out, fmt.Errorf("native result direct184F6 sound missing")
			}
			if err := cb.Sound(uint16(c.D[0]), c); err != nil {
				return out, err
			}
			c.Word(0, 100)
			s.PC = 0x3868
		case 0x3868:
			ready, err := m.Read16(0xa)
			if err != nil {
				return out, err
			}
			if ready == 0 {
				out.Waiting = true
				return out, nil
			}
			if err := m.Write16(0xa, 0); err != nil {
				return out, err
			}
			c.Word(0, uint16(c.D[0])-1)
			if uint16(c.D[0]) != 0xffff {
				out.Waiting = true
				return out, nil
			}
			s.PC = 0x3878
		case 0x3878:
			if err := s.prepareRequester(h, c); err != nil {
				return out, err
			}
			s.PC = 0x39e8
		case 0x39e8:
			b := nativeRequesterFrameBacking{Code: code, Memory: m, CodeBase: h.Memory.CodeBase, Frame: c, Bitmap: h.Bitmap}
			if err := campaignRequesterCopy(b, &s.A); err != nil {
				return out, err
			}
			s.PC = 0x39fa
		case 0x39fa:
			back, err := m.Read32(0x1e)
			if err != nil {
				return out, err
			}
			b := nativeRequesterFrameBacking{Code: code, Memory: m, CodeBase: h.Memory.CodeBase, Frame: c, Bitmap: h.Bitmap, Sound: cb.Sound, ReadAbsolute: func(at uint32) (byte, error) { return h.Memory.RAM.Read8(int(at)) }}
			start, err := code.Read16(0xab4e)
			if err != nil {
				return out, err
			}
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
			if err := campaignRequesterText(b, &s.A, back, 0xab4e+int(int16(start))); err != nil {
				return out, err
			}
			if err := fileFrameSwap(b, h.Session.Presentation); err != nil {
				return out, err
			}
			s.PC = 0x3a00
		case 0x3a00:
			b := nativeRequesterFrameBacking{Code: code, Memory: m, CodeBase: h.Memory.CodeBase, Frame: c, Bitmap: h.Bitmap, Sound: cb.Sound}
			if _, err := campaignRequesterClick(b, &s.A); err != nil {
				return out, err
			}
			branch, err := code.Read16(0x3a0e + int(int16(c.D[0])))
			if err != nil {
				return out, err
			}
			c.Word(0, branch)
			s.PC = 0x3a0e + int(int16(branch))
			if s.PC == 0x39fa {
				out.Waiting = true
				return out, nil
			}
		case 0x3a12:
			copy(c.D[2:6], s.Saved[:])
			if err := s.progress(h, c); err != nil {
				return out, err
			}
		case 0x3aa8:
			ok, err := child(0xb244, 0x3ac2)
			if err != nil || !ok {
				return out, err
			}
		case 0x3ab0:
			if s.palette == nil {
				bank := func(at int) (NativeFramePaletteBank, error) {
					p := NativeFramePaletteBank{Address: h.Memory.CodeBase + uint32(at)}
					for i := range p.Words {
						v, err := code.Read16(at + i*2)
						if err != nil {
							return p, err
						}
						p.Words[i] = v
					}
					return p, nil
				}
				from, err := bank(0x33844)
				if err != nil {
					return out, err
				}
				to, err := bank(0x3361a)
				if err != nil {
					return out, err
				}
				s.A[2], s.A[3] = address(0x33844), address(0x3361a)
				s.palette = NewNativeFramePaletteState(from, to, h.Memory.CodeBase)
			}
			complete, err := s.palette.Advance(h.Session.Presentation, c, m)
			if err == nil {
				offset := uint32(0x34)
				if complete {
					offset = 0x74
				}
				s.A[0] = NativeRequesterAddress{Address: h.Session.Presentation.ChipBase + offset, Chip: true}
				s.A[1] = NativeRequesterAddress{Address: h.Session.Presentation.ChipBase + 0x200 + offset, Chip: true}
			}
			if err != nil || !complete {
				out.Waiting = !complete
				return out, err
			}
			s.palette = nil
			s.PC = 0x3ac2
		case 0x3ac2:
			ok, err := child(0x10a8c, 0x3ac8)
			if err != nil || !ok {
				return out, err
			}
		case 0x3ac8:
			if err := ownership(false); err != nil {
				return out, err
			}
			s.PC = 0
			s.Complete, out.Complete = true, true
			return out, nil
		default:
			return out, fmt.Errorf("native result continuation%x unsupported", s.PC)
		}
	}
	return out, fmt.Errorf("native result transition bound exceeded")
}
