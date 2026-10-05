package populous2

import "fmt"

type NativeCampaignHelpFrameCallbacks struct {
	NativeCampaignFrameCallbacks
	Blitter      *NativeCampaignBlitterState
	AudioCommand func(control, data uint16, input uint32) (uint32, error)
	AudioControl NativeAudioControlFrameCallbacks
}

type NativeCampaignHelpFrameState struct {
	Started, Finished bool
	PC                int
	Registers         [8]uint32
	A                 [7]NativeRequesterAddress
	ChildActive       bool
	ChildPhase        uint32
	failed            error
}

// Advance executes complete $517a. Its LAND resource, raw help bytes, all36
// previews, shared audio descriptors and real swap/input gates retain source
// order. Admission failure still executes the actual $1842e stop body.
func (s *NativeCampaignHelpFrameState) Advance(r *NativeRenderFrameRules, audio *NativeAudioControlFrameRules, cb NativeCampaignHelpFrameCallbacks) (out NativeCampaignFrameStep, failure error) {
	if s == nil || r == nil || audio == nil || cb.Frame == nil || cb.Presentation == nil || cb.Bitmap == nil || cb.Blitter == nil || cb.AudioCommand == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.RAM) {
		return out, fmt.Errorf("native campaign help backing/audio missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		s.Started = true
		s.PC = 0x517a
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
	c, m, code := cb.Frame, cb.Memory, cb.Code
	b := nativeRequesterFrameBacking{Code: code, Memory: m, CodeBase: cb.CodeBase, Frame: c, Bitmap: cb.Bitmap, Sound: cb.Sound, ReadAbsolute: cb.ReadAbsolute}
	ca := func(at int) NativeRequesterAddress {
		return NativeRequesterAddress{Address: cb.CodeBase + uint32(at), Code: true}
	}
	for transitions := 0; transitions < 32; transitions++ {
		switch s.PC {
		case 0x517a:
			s.A[1] = ca(0x21102)
			icon, e := code.Read16(0x21102 + int(int16(c.D[1])))
			if e != nil {
				return out, e
			}
			if icon == 0 {
				s.PC = 0x52a0
				continue
			}
			profile, e := m.Read16(0xeb42)
			if e != nil {
				return out, e
			}
			c.Word(0, profile)
			c.D[0] = uint32(uint16(c.D[0])) * 314
			s.A[1] = NativeRequesterAddress{Address: uint32(int64(c.AddressBase+0xe76a) + int64(int16(c.D[0])))}
			c.Word(0, uint16(c.D[1]))
			c.Word(0, uint16(c.D[0])>>1)
			flag, e := cb.RAM.Read8(int(s.A[1].Address) + 0x70 + int(int16(c.D[0])))
			if e != nil {
				return out, e
			}
			if int8(flag) <= 0 {
				s.PC = 0x52a0
				continue
			}
			if e = code.Write16(0x552a, 0xffff); e != nil {
				return out, e
			}
			if e = code.Write16(0x5528, uint16(c.D[1])); e != nil {
				return out, e
			}
			s.PC = 0x51b4
		case 0x51b4:
			if cb.Child == nil {
				return out, fmt.Errorf("native help actual LAND resource child missing")
			}
			s.ChildActive = true
			result, e := cb.Child(NativeStartupResetFrameCall{Routine: 0x1a32a, Frame: c, A: &s.A}, &s.ChildPhase)
			if e != nil {
				return out, e
			}
			if !result.Complete {
				out.Waiting = true
				return out, nil
			}
			s.ChildActive = false
			s.ChildPhase = 0
			s.PC = 0x51ba
		case 0x51ba:
			s.A[1] = ca(0x5530)
			s.A[2] = NativeRequesterAddress{Address: 0, Absolute: true}
			c.D[3] = 1
			if e := campaignRequesterCompile(b, &s.A, s.A[1].Address, 0); e != nil {
				return out, e
			}
			if e := campaignRequesterCopy(b, &s.A); e != nil {
				return out, e
			}
			s.PC = 0x51dc
		case 0x51dc:
			ready, e := m.Read16(0xa)
			if e != nil {
				return out, e
			}
			if ready == 0 {
				out.Waiting = true
				return out, nil
			}
			s.PC = 0x51e2
		case 0x51e2:
			target, e := m.Read32(0x1e)
			if e != nil {
				return out, e
			}
			start, e := code.Read16(0xab4e)
			if e != nil {
				return out, e
			}
			x, e := code.Read16(0xab50)
			if e != nil {
				return out, e
			}
			y, e := code.Read16(0xab52)
			if e != nil {
				return out, e
			}
			c.Word(0, x)
			c.Word(1, y)
			if e = campaignRequesterText(b, &s.A, target, 0xab4e+int(int16(start))); e != nil {
				return out, e
			}
			c.Word(0, x+2)
			c.Word(0, uint16(c.D[0])<<3)
			c.Word(1, y+32)
			offset, e := code.Read16(0x5528)
			if e != nil {
				return out, e
			}
			s.A[4] = ca(0x21102 + int(int16(offset)))
			descriptorOffset, e := cb.RAM.Read16(int(s.A[4].Address))
			if e != nil {
				return out, e
			}
			s.A[4].Address += 2
			descriptor := uint32(int64(cb.CodeBase+0x214b2) + int64(int16(descriptorOffset)))
			s.A[2] = NativeRequesterAddress{Address: descriptor, Code: true}
			s.A[0] = NativeRequesterAddress{Address: target, Chip: true}
			source, e := cb.RAM.Read32(int(descriptor))
			if e != nil {
				return out, e
			}
			s.A[1] = NativeRequesterAddress{Address: source, Absolute: true}
			height, e := cb.RAM.Read16(int(descriptor) + 6)
			if e != nil {
				return out, e
			}
			c.Word(2, height)
			routine, e := cb.RAM.Read32(int(descriptor) + 8)
			if e != nil {
				return out, e
			}
			s.A[2] = NativeRequesterAddress{Address: routine, Code: true}
			if e = campaignChildSprite(r, cb.NativeCampaignFrameCallbacks, &s.A, routine, cb.Blitter); e != nil {
				return out, e
			}
			s.A[1] = ca(0x58ba + int(int16(offset))*2)
			text, e := cb.RAM.Read32(int(s.A[1].Address))
			if e != nil {
				return out, e
			}
			s.A[1] = NativeRequesterAddress{Address: text, Code: true}
			c.Word(0, x+2)
			c.Word(1, y+64)
			if e = campaignRequesterText(b, &s.A, target, int(text-cb.CodeBase)); e != nil {
				return out, e
			}
			if e = AdvanceNativeCampaignPreview(r, cb.NativeCampaignFrameCallbacks, &s.A, cb.Blitter); e != nil {
				return out, e
			}
			if e = TickNativeCampaignAudio(cb.NativeCampaignFrameCallbacks, &s.A, cb.AudioCommand); e != nil {
				return out, e
			}
			if e = fileFrameSwap(b, cb.Presentation); e != nil {
				return out, e
			}
			if _, e = campaignRequesterClick(b, &s.A); e != nil {
				return out, e
			}
			branch, e := code.Read16(0x52a8 + int(int16(c.D[0])))
			if e != nil {
				return out, e
			}
			c.Word(0, branch)
			s.PC = 0x52a8 + int(int16(branch))
			if s.PC == 0x51dc {
				out.Waiting = true
				return out, nil
			}
		case 0x52a0:
			control := cb.AudioControl
			control.Memory = m
			control.Frame = c
			control.CodeBase = cb.CodeBase
			step, e := audio.Run(0x1842e, control)
			if e != nil {
				return out, e
			}
			if !step.Complete {
				return out, fmt.Errorf("native help stop operation did not complete")
			}
			if step.A0Assigned {
				s.A[0] = step.A0
			}
			s.Finished = true
			s.PC = 0x52a6
			out.Complete = true
			return out, nil
		default:
			return out, fmt.Errorf("native help PC%x unsupported", s.PC)
		}
	}
	out.Waiting = true
	return out, nil
}
