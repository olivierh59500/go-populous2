package populous2

import "fmt"

type NativeStartupDeityFrameState struct {
	Started, Finished bool
	PC                int
	Registers         [8]uint32
	A                 [7]NativeRequesterAddress
	ChildActive       bool
	ChildRoutine      int
	ChildPhase        uint32
	Modal             NativeRuntimeFileBrowserModal
	ClickEnd          uint32
	modalReturn       int
	keys              NativeInputRules
	failed            error
}

// Advance is the retained original B740 deity editor, including the actual
// 4BBA name/password modal. Physical resource operations are genuine child
// ports and retain their complete register/address context.
func (s *NativeStartupDeityFrameState) Advance(r *NativeRenderFrameRules, cb NativeCampaignFrameCallbacks) (out NativeCampaignFrameStep, failure error) {
	if s == nil || r == nil || cb.Frame == nil || cb.Presentation == nil || cb.Bitmap == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.RAM) {
		return out, fmt.Errorf("native deity editor backing missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		for i := range s.keys.Keys {
			value, err := cb.Code.Read8(0x100f4 + i)
			if err != nil {
				return out, err
			}
			s.keys.Keys[i] = value
		}
		s.Started = true
		s.PC = 0xb740
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
	b := nativeRequesterFrameBacking{Code: nativeOffsetMemory(cb.RAM, cb.CodeBase), Memory: m, CodeBase: cb.CodeBase, Frame: c, Bitmap: cb.Bitmap, Sound: cb.Sound, ReadAbsolute: cb.ReadAbsolute}
	ca := func(at int) NativeRequesterAddress {
		return NativeRequesterAddress{Address: cb.CodeBase + uint32(at), Code: true}
	}
	child := func(routine, next int) (bool, error) {
		if cb.Child == nil {
			return false, fmt.Errorf("native deity editor child%x has no actual body", routine)
		}
		if !s.ChildActive {
			s.ChildActive = true
			s.ChildRoutine = routine
		}
		if s.ChildRoutine != routine {
			return false, fmt.Errorf("native deity child changed during wait")
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
	god := func(reg int) (int, error) {
		profile, err := m.Read16(0xeb42)
		if err != nil {
			return 0, err
		}
		c.Word(reg, profile)
		c.D[reg] = uint32(uint16(c.D[reg])) * 314
		return 0xe76a + int(int16(c.D[reg])), nil
	}
	primitive := func(routine int) error {
		return RunNativeDeityPasswordFrame(routine, NativeStartupResetFrameCallbacks{Code: code, Memory: m, RAM: cb.RAM, CodeBase: cb.CodeBase, Frame: c}, &s.A)
	}
	if s.Finished {
		out.Complete = true
		out.FlagsKnown, out.Zero = true, true
		return out, nil
	}
	for transitions := 0; transitions < 128; transitions++ {
		if s.Modal.Active {
			done, err := s.Modal.advance(b, cb.Presentation, s.keys, &s.A)
			if err != nil {
				return out, err
			}
			if !done {
				out.Waiting = true
				return out, nil
			}
			if s.Modal.Finish == 2 {
				// 4C5E restores the selected pointer into A0; a following
				// field action must use this click, not the modal's first one.
				s.ClickEnd = s.A[0].Address
			}
			s.PC = s.modalReturn
		}
		switch s.PC {
		case 0xb740:
			c.Word(0, 1)
			s.PC = 0xb744
		case 0xb744:
			done, err := child(0x111ae, 0xb74a)
			if err != nil || !done {
				return out, err
			}
		case 0xb74a:
			c.Word(0, 8)
			s.PC = 0xb74e
		case 0xb74e:
			done, err := child(0x19cd0, 0xb754)
			if err != nil || !done {
				return out, err
			}
		case 0xb754:
			if err := code.Write32(0xba58, cb.CodeBase+0x96be); err != nil {
				return out, err
			}
			at, err := god(2)
			if err != nil {
				return out, err
			}
			s.A[1] = NativeRequesterAddress{Address: c.AddressBase + uint32(at)}
			bolts, err := m.Read16(at + 0x58)
			if err != nil {
				return out, err
			}
			c.Word(0, bolts)
			if bolts != 0 {
				if int16(c.D[0]) > 8 {
					c.Word(0, 8)
				}
				c.Word(0, uint16(c.D[0])-1)
				c.Word(0, uint16(c.D[0])*2)
				c.Word(0, -uint16(c.D[0])+14)
				c.D[0] = uint32(int32(int16(c.D[0])))
				c.D[0] += cb.CodeBase + 0x96c6
				if err = code.Write32(0xba58, c.D[0]); err != nil {
					return out, err
				}
			}
			s.A[0] = ca(0xbaa0)
			x, err := m.Read8(at + 0x4e)
			if err != nil {
				return out, err
			}
			c.Byte(0, x)
			c.Byte(0, uint8(c.D[0])<<4)
			x, err = m.Read8(at + 0x4f)
			if err != nil {
				return out, err
			}
			c.Byte(0, uint8(c.D[0])|x)
			if err = code.Write8(0xbaa0, uint8(c.D[0])); err != nil {
				return out, err
			}
			s.A[0].Address++
			x, err = m.Read8(at + 0x50)
			if err != nil {
				return out, err
			}
			c.Byte(0, x)
			c.Byte(0, uint8(c.D[0])<<4)
			x, err = m.Read8(at + 0x59)
			if err != nil {
				return out, err
			}
			c.Byte(1, x)
			c.Byte(0, uint8(c.D[0])|uint8(c.D[1]))
			if err = code.Write8(0xbaa1, uint8(c.D[0])); err != nil {
				return out, err
			}
			s.A[0].Address++
			c.D[1] = 5
			s.A[2] = NativeRequesterAddress{Address: c.AddressBase + uint32(at+0x52)}
			for i := 0; i < 6; i++ {
				x, err = cb.RAM.Read8(int(s.A[2].Address))
				if err != nil {
					return out, err
				}
				if err = code.Write8(0xbaa2+i, x); err != nil {
					return out, err
				}
				s.A[0].Address++
				s.A[2].Address++
				c.Word(1, uint16(c.D[1])-1)
			}
			s.A[0], s.A[1] = ca(0xbaa0), ca(0xba60)
			if err = primitive(0x1047c); err != nil {
				return out, err
			}
			s.A[1], s.A[2] = ca(0x760e), ca(0xba54)
			c.D[3] = 1
			if err = campaignRequesterCompile(b, &s.A, s.A[1].Address, s.A[2].Address); err != nil {
				return out, err
			}
			if err = m.Write16(0x140, 0); err != nil {
				return out, err
			}
			s.PC = 0xb7ec
		case 0xb7ec:
			ready, err := m.Read16(0xa)
			if err != nil {
				return out, err
			}
			if ready == 0 {
				out.Waiting = true
				return out, nil
			}
			target, err := m.Read32(0x1e)
			if err != nil {
				return out, err
			}
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
			s.A[0], s.A[1] = NativeRequesterAddress{Address: target, Chip: true}, ca(0xab4e+int(int16(start)))
			if err = campaignRequesterText(b, &s.A, target, 0xab4e+int(int16(start))); err != nil {
				return out, err
			}
			s.A[2] = ca(0xba8c)
			at, err := god(3)
			if err != nil {
				return out, err
			}
			s.A[3] = NativeRequesterAddress{Address: c.AddressBase + uint32(at+0x51)}
			if err = DrawNativeCampaignPortrait(r, cb, &s.A); err != nil {
				return out, err
			}
			if err = DrawNativeDeityExperienceFrame(NativeStartupResetFrameCallbacks{Code: code, Memory: m, RAM: cb.RAM, CodeBase: cb.CodeBase, Frame: c}, &s.A); err != nil {
				return out, err
			}
			if err = fileFrameSwap(b, cb.Presentation); err != nil {
				return out, err
			}
			gate, err := m.Read16(0x3b0)
			if err != nil {
				return out, err
			}
			if gate == 0 {
				s.A[2], s.A[3] = ca(0x3361a), ca(0x33844)
				s.PC = 0xb858
			} else {
				s.PC = 0xb85e
			}
		case 0xb858:
			done, err := child(0x102e4, 0xb85e)
			if err != nil || !done {
				return out, err
			}
		case 0xb85e:
			end, err := campaignRequesterClick(b, &s.A)
			if err != nil {
				return out, err
			}
			s.ClickEnd = end
			s.PC = 0xb864
		case 0xb864:
			c.Word(1, uint16(c.D[0]))
			offset, err := code.Read16(0xb882 + int(int16(c.D[0])))
			if err != nil {
				return out, err
			}
			c.Word(0, offset)
			s.PC = 0xb882 + int(int16(offset))
			if s.PC == 0xb7ec {
				out.Waiting = true
				return out, nil
			}
		case 0xb86e, 0xba50:
			s.A[2], s.A[3] = ca(0x33844), ca(0x3361a)
			s.PC = 0xb87a
		case 0xb87a:
			done, err := child(0x102e4, 0xb880)
			if err != nil || !done {
				return out, err
			}
		case 0xb880:
			s.Finished = true
			out.Complete = true
			// The final 102E4 CMP.W #17,D7 is equal. Its MOVEM
			// restores and B740's RTS leave that condition unchanged.
			out.FlagsKnown, out.Zero = true, true
			return out, nil
		case 0xb8c6:
			s.A[1] = NativeRequesterAddress{Address: c.AddressBase + 0xeb30}
			s.PC = 0xb8cc
		case 0xb8cc:
			field := int(int64(s.A[1].Address) - int64(cb.CodeBase))
			if err := s.Modal.begin(b, field, int(int64(s.ClickEnd)-int64(cb.CodeBase)), &s.A); err != nil {
				return out, err
			}
			s.modalReturn = 0xb864
		case 0xb8d6, 0xb8dc, 0xb8e2, 0xb8e8, 0xb8ee, 0xb8f4:
			c.Word(0, uint16((s.PC-0xb8d6)/6))
			s.PC = 0xb8f8
		case 0xb8f8:
			at, err := god(2)
			if err != nil {
				return out, err
			}
			s.A[3] = NativeRequesterAddress{Address: c.AddressBase + uint32(at)}
			bolts, err := m.Read16(at + 0x58)
			if err != nil {
				return out, err
			}
			c.Word(2, bolts)
			if bolts == 0 {
				s.PC = 0xb7ec
				continue
			}
			xp, err := m.Read8(at + 0x52 + int(int16(c.D[0])))
			if err != nil {
				return out, err
			}
			c.Byte(1, xp)
			if xp == 255 {
				s.PC = 0xb7ec
				continue
			}
			c.Byte(1, xp+1)
			c.Word(2, uint16(c.D[2])-1)
			if err = m.Write8(at+0x52+int(int16(c.D[0])), uint8(c.D[1])); err != nil {
				return out, err
			}
			if err = m.Write16(at+0x58, uint16(c.D[2])); err != nil {
				return out, err
			}
			s.PC = 0xb754
		case 0xb932, 0xb936, 0xb93a, 0xb93e, 0xb942, 0xb946:
			forward := s.PC >= 0xb93e
			base := 0xb932
			if forward {
				base = 0xb93e
			}
			c.D[1] = uint32((s.PC - base) / 4)
			at, err := god(2)
			if err != nil {
				return out, err
			}
			s.A[3] = NativeRequesterAddress{Address: c.AddressBase + uint32(at)}
			part := at + 0x4e + int(int16(c.D[1]))
			value, err := m.Read8(part)
			if err != nil {
				return out, err
			}
			c.Byte(2, value)
			if forward {
				c.Byte(2, value+1)
				if int8(c.D[2]) >= 8 {
					c.Byte(2, 0)
				}
			} else {
				c.Byte(2, value-1)
				if int8(c.D[2]) < 0 {
					c.Byte(2, 7)
				}
			}
			if err = m.Write8(part, uint8(c.D[2])); err != nil {
				return out, err
			}
			s.PC = 0xb754
		case 0xb99e:
			c.D[1] = 20
			s.A[1], s.A[2] = ca(0xba60), ca(0xba76)
			for i := 20; i >= 0; i-- {
				x, err := code.Read8(0xba60 + i)
				if err != nil {
					return out, err
				}
				if err = code.Write8(0xba76+i, x); err != nil {
					return out, err
				}
				c.Word(1, uint16(c.D[1])-1)
			}
			if err := code.Write8(0xba60, 0); err != nil {
				return out, err
			}
			s.PC = 0xb9b8
		case 0xb9b8:
			if err := s.Modal.begin(b, 0xba60, int(int64(s.ClickEnd)-int64(cb.CodeBase)), &s.A); err != nil {
				return out, err
			}
			s.modalReturn = 0xb9be
		case 0xb9be:
			s.A[0], s.A[1] = ca(0xba60), ca(0xbaa0)
			if err := primitive(0x10564); err != nil {
				return out, err
			}
			if c.D[0] == 0xffffffff {
				s.PC = 0xb9d4
				continue
			}
			s.PC = 0xb9f0
		case 0xb9d4:
			c.D[1] = 20
			s.A[1], s.A[2] = ca(0xba76), ca(0xba60)
			for i := 20; i >= 0; i-- {
				x, err := code.Read8(0xba76 + i)
				if err != nil {
					return out, err
				}
				if err = code.Write8(0xba60+i, x); err != nil {
					return out, err
				}
				c.Word(1, uint16(c.D[1])-1)
			}
			s.PC = 0xb754
		case 0xb9f0:
			at, err := god(2)
			if err != nil {
				return out, err
			}
			s.A[1] = NativeRequesterAddress{Address: c.AddressBase + uint32(at)}
			s.A[0] = ca(0xbaa0)
			for part := 0; part < 2; part++ {
				value, err := code.Read8(0xbaa0 + part)
				if err != nil {
					return out, err
				}
				s.A[0].Address++
				c.Byte(0, value)
				c.Byte(1, value)
				c.Byte(1, uint8(c.D[1])&0x88)
				if uint8(c.D[1]) != 0 {
					s.PC = 0xb9d4
					break
				}
				c.Byte(1, value)
				c.Byte(0, value&0xf0)
				c.Byte(1, uint8(c.D[1])^uint8(c.D[0]))
				c.Byte(0, uint8(c.D[0])>>4)
				first, second := at+0x4e, at+0x4f
				if part == 1 {
					first, second = at+0x50, at+0x59
				}
				if err = m.Write8(first, uint8(c.D[0])); err != nil {
					return out, err
				}
				if err = m.Write8(second, uint8(c.D[1])); err != nil {
					return out, err
				}
			}
			if s.PC == 0xb9d4 {
				continue
			}
			s.A[2] = NativeRequesterAddress{Address: c.AddressBase + uint32(at+0x52)}
			c.D[1] = 5
			for i := 0; i < 6; i++ {
				value, err := code.Read8(0xbaa2 + i)
				if err != nil {
					return out, err
				}
				if err = m.Write8(at+0x52+i, value); err != nil {
					return out, err
				}
				s.A[0].Address++
				s.A[2].Address++
				c.Word(1, uint16(c.D[1])-1)
			}
			s.PC = 0xb754
		default:
			return out, fmt.Errorf("native deity editor PC%x unsupported", s.PC)
		}
	}
	return out, fmt.Errorf("native deity editor exceeded source transition bound")
}
