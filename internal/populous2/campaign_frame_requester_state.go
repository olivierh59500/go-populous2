package populous2

import (
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeCampaignSelectionFrameRules struct {
	Render NativeRenderFrameRules
	Keys   NativeInputRules
}

func DecodeNativeCampaignSelectionFrameRules(exe *amiga.Executable) (NativeCampaignSelectionFrameRules, error) {
	var r NativeCampaignSelectionFrameRules
	var e error
	r.Render, e = DecodeNativeRenderFrameRules(exe)
	if e == nil {
		r.Keys, e = DecodeNativeInputRules(exe)
	}
	return r, e
}

type NativeCampaignSelectionFrameState struct {
	Started, Finished bool
	PC                int
	Registers         [8]uint32
	A                 [7]NativeRequesterAddress
	ChildActive       bool
	ChildRoutine      int
	ChildPhase        uint32
	Modal             NativeFileTextModal
	ModalA            [7]NativeRequesterAddress
	Record            NativeCampaignRecordFrameState
	ClickEnd          uint32
	failed            error
}

// Advance is the complete $3cba control program. Input waits retain the live
// CODE field, workspace, actual screen pointers and source register context.
func (s *NativeCampaignSelectionFrameState) Advance(r *NativeCampaignSelectionFrameRules, cb NativeCampaignFrameCallbacks) (out NativeCampaignFrameStep, failure error) {
	if s == nil || r == nil || cb.Frame == nil || cb.Presentation == nil || cb.Bitmap == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.RAM) {
		return out, fmt.Errorf("native campaign selection backing missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		s.Started = true
		s.PC = 0x3cba
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
			return false, fmt.Errorf("native campaign child%x missing", routine)
		}
		if !s.ChildActive {
			s.ChildActive = true
			s.ChildRoutine = routine
		}
		if s.ChildRoutine != routine {
			return false, fmt.Errorf("native campaign child changed during wait")
		}
		result, e := cb.Child(NativeStartupResetFrameCall{Routine: routine, Frame: c, A: &s.A}, &s.ChildPhase)
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
	if s.Finished {
		out.Complete = true
		out.FlagsKnown = true
		out.Zero = c.D[0] == 0
		out.Negative = int32(c.D[0]) < 0
		return out, nil
	}
	for steps := 0; steps < 64; steps++ {
		switch s.PC {
		case 0x3cba:
			c.Word(0, 1)
			s.PC = 0x3cbe
		case 0x3cbe:
			done, e := child(0x111ae, 0x3cc4)
			if e != nil || !done {
				return out, e
			}
		case 0x3cc4:
			profile, e := m.Read16(0xeb42)
			if e != nil {
				return out, e
			}
			c.Word(0, profile)
			c.D[0] = uint32(uint16(c.D[0])) * 314
			s.A[6] = NativeRequesterAddress{Address: uint32(int64(c.AddressBase+0xe76a) + int64(int16(c.D[0]))), Absolute: true}
			world, e := m.Read16(0xeb46)
			if e != nil {
				return out, e
			}
			c.Word(0, world)
			s.A[0] = ca(0x3f56)
			if e = code.Write32(0x3f3a, s.A[0].Address); e != nil {
				return out, e
			}
			if e = campaignWorldCode(cb, &s.A, s.A[0].Address); e != nil {
				return out, e
			}
			c.Word(0, world)
			c.D[0] = uint32(int32(int16(c.D[0])))
			s.A[0] = ca(0x3f4e)
			if e = code.Write32(0x3f3e, s.A[0].Address); e != nil {
				return out, e
			}
			decimal := ""
			if int32(c.D[0]) >= 0 {
				decimal = fmt.Sprintf("%d", int32(c.D[0]))
			}
			for _, v := range []byte(decimal) {
				if e = code.Write8(int(s.A[0].Address-cb.CodeBase), v); e != nil {
					return out, e
				}
				s.A[0].Address++
			}
			if e = code.Write8(int(s.A[0].Address-cb.CodeBase), 0); e != nil {
				return out, e
			}
			s.A[0].Address++
			land, e := m.Read16(0xeb22)
			if e != nil {
				return out, e
			}
			c.Word(0, land)
			c.D[0] = uint32(uint16(c.D[0]))*14 + cb.CodeBase + 0xa825
			if e = code.Write32(0x3f42, c.D[0]); e != nil {
				return out, e
			}
			c.D[1] = 1000
			if e = frameDivide(c, 1, 31); e != nil {
				return out, e
			}
			c.Word(0, world)
			c.D[0] = uint32(int32(int16(c.D[0])))
			if e = frameDivide(c, 0, uint16(c.D[1])); e != nil {
				return out, e
			}
			c.D[0] = uint32(uint16(c.D[0]))*14 + cb.CodeBase + 0x96d6
			if e = code.Write32(0x3f46, c.D[0]); e != nil {
				return out, e
			}
			s.A[1], s.A[2] = ca(0x809c), ca(0x3f3a)
			c.D[3] = 1
			if e = campaignRequesterCompile(b, &s.A, s.A[1].Address, s.A[2].Address); e != nil {
				return out, e
			}
			if e = campaignRequesterCopy(b, &s.A); e != nil {
				return out, e
			}
			start, e := code.Read16(0xab4e)
			if e != nil {
				return out, e
			}
			s.A[1] = ca(0xab4e + int(int16(start)))
			c.D[1] = 0
			bits, e := cb.RAM.Read16(int(s.A[6].Address) + 0x66)
			if e != nil {
				return out, e
			}
			c.Word(0, bits)
			for {
				v, e := code.Read8(int(s.A[1].Address - cb.CodeBase))
				if e != nil {
					return out, e
				}
				s.A[1].Address++
				c.Byte(1, v)
				if v == 0 {
					break
				}
				if v == 'y' || v == 'z' {
					replacement := byte('z')
					if uint16(c.D[0])&1 != 0 {
						replacement = 'y'
					}
					if e = code.Write8(int(s.A[1].Address-cb.CodeBase)-1, replacement); e != nil {
						return out, e
					}
					c.Word(0, uint16(c.D[0])>>1)
				}
			}
			s.PC = 0x3d9a
		case 0x3d9a:
			ready, e := m.Read16(0xa)
			if e != nil {
				return out, e
			}
			if ready == 0 {
				out.Waiting = true
				return out, nil
			}
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
			s.A[0] = NativeRequesterAddress{Address: target, Chip: true}
			s.A[1] = ca(0xab4e + int(int16(start)))
			if e = campaignRequesterText(b, &s.A, target, 0xab4e+int(int16(start))); e != nil {
				return out, e
			}
			if e = DrawNativeCampaignIcons(&r.Render, cb, &s.A); e != nil {
				return out, e
			}
			if e = fileFrameSwap(b, cb.Presentation); e != nil {
				return out, e
			}
			flag, e := m.Read16(0x3b0)
			if e != nil {
				return out, e
			}
			if flag == 0 {
				s.A[2], s.A[3] = ca(0x3361a), ca(0x33844)
				s.PC = 0x3de4
			} else {
				s.PC = 0x3dea
			}
		case 0x3de4:
			done, e := child(0x102e4, 0x3dea)
			if e != nil || !done {
				return out, e
			}
		case 0x3dea:
			pressed, e := m.Read16(0x140)
			if e != nil {
				return out, e
			}
			if pressed != 0 {
				x, e := m.Read16(0x134)
				if e != nil {
					return out, e
				}
				y, e := m.Read16(0x136)
				if e != nil {
					return out, e
				}
				c.RestoreWord(0, x)
				c.RestoreWord(1, y)
				c.Word(0, uint16(c.D[0])-126)
				c.Word(1, uint16(c.D[1])+10)
				c.Word(0, uint16(int16(c.D[0])>>1))
				c.Word(2, uint16(c.D[0]))
				c.Word(0, uint16(c.D[0])+uint16(c.D[1]))
				c.Word(1, uint16(c.D[1])-uint16(c.D[2]))
				c.Word(0, uint16(int16(c.D[0])>>4))
				c.Word(1, uint16(int16(c.D[1])>>4))
				if int16(c.D[1]) >= 0 {
					c.Word(0, uint16(c.D[0])-5)
					c.Word(0, uint16(c.D[0])+uint16(c.D[1]))
					if int16(c.D[0]) >= 0 && int16(c.D[0]) < 5 && int16(c.D[1]) < 6 {
						c.Word(1, -uint16(c.D[1])+5)
						c.D[1] = uint32(uint16(c.D[1])) * 6
						c.Word(1, uint16(c.D[1])+uint16(c.D[0]))
						c.Word(1, uint16(c.D[1])*2)
						if e = m.Write16(0x140, 0); e != nil {
							return out, e
						}
						s.PC = 0x3e38
						continue
					}
				}
			}
			end, e := campaignRequesterClick(b, &s.A)
			if e != nil {
				return out, e
			}
			s.ClickEnd = end
			offset, e := code.Read16(0x3e52 + int(int16(c.D[0])))
			if e != nil {
				return out, e
			}
			c.Word(0, offset)
			s.PC = 0x3e52 + int(int16(offset))
			if s.PC == 0x3d9a {
				out.Waiting = true
				return out, nil
			}
		case 0x3e38:
			done, e := child(0x517a, 0x3cc4)
			if e != nil || !done {
				return out, e
			}
		case 0x3e5c:
			s.A[1] = ca(0x3f66)
			if e := code.Write8(0x3f66, 0); e != nil {
				return out, e
			}
			if e := s.Modal.begin(b, 0x3f66, int(s.ClickEnd-cb.CodeBase)); e != nil {
				return out, e
			}
			s.ModalA = s.A
			s.PC = 0x3e64
		case 0x3e64:
			done, e := s.advanceModal(b, cb.Presentation, r.Keys)
			if e != nil {
				return out, e
			}
			if !done {
				s.A[0] = NativeRequesterAddress{Address: s.Modal.Target, Chip: true}
				s.A[1] = ca(s.Modal.Display)
				s.A[2] = ca(s.Modal.Caret)
				cursor := s.Modal.Caret - int(int16(s.Modal.Saved[4]))
				for i := 0; i < int(s.Modal.Capacity); i++ {
					v, e := code.Read8(cursor)
					if e != nil {
						return out, e
					}
					if v != 0 {
						cursor++
					}
				}
				s.A[3] = ca(cursor)
				s.A[4], s.A[5], s.A[6] = s.ModalA[4], s.ModalA[5], s.ModalA[6]
				out.Waiting = true
				return out, nil
			}
			s.A[0] = ca(s.Modal.Display)
			s.A[1] = ca(s.Modal.Field)
			s.A[2] = ca(s.Modal.Caret)
			s.A[3], s.A[4], s.A[5], s.A[6] = s.ModalA[3], s.ModalA[4], s.ModalA[5], s.ModalA[6]
			s.PC = 0x3e6a
		case 0x3e6a:
			s.A[0] = ca(0x3f66)
			c.D[0] = 0
			first, e := code.Read8(0x3f66)
			if e != nil {
				return out, e
			}
			if first != 0 {
				for {
					if e := campaignCompareWorldCode(cb, &s.A); e != nil {
						return out, e
					}
					if c.D[1] == 0 {
						break
					}
					c.Word(0, uint16(c.D[0])+1)
					if uint16(c.D[0]) == 1000 {
						s.A[1], s.A[2] = ca(0x91d0), ca(0xa91e)
						s.PC = 0x3ea6
						break
					}
				}
			}
			if s.PC == 0x3ea6 {
				continue
			}
			if e = m.Write16(0xeb46, uint16(c.D[0])); e != nil {
				return out, e
			}
			s.Record = NativeCampaignRecordFrameState{A: s.A}
			s.PC = 0x3e90
		case 0x3e90:
			step, e := s.Record.Advance(cb)
			s.A = s.Record.A
			if e != nil {
				return out, e
			}
			if !step.Complete {
				out.Waiting = true
				return out, nil
			}
			s.PC = 0x3cc4
		case 0x3ea6:
			done, e := child(0x33b2, 0x3cc4)
			if e != nil || !done {
				return out, e
			}
		case 0x3eb0:
			done, e := child(0xaf82, 0x3cc4)
			if e != nil || !done {
				return out, e
			}
		case 0x3eba:
			s.A[2], s.A[3] = ca(0x33844), ca(0x3361a)
			s.PC = 0x3ec6
		case 0x3ed0:
			s.A[2], s.A[3] = ca(0x33844), ca(0x3361a)
			s.PC = 0x3edc
		case 0x3ec6, 0x3edc:
			value := uint32(1)
			if s.PC == 0x3edc {
				value = 0
			}
			done, e := child(0x102e4, 0x3ecc)
			if e != nil || !done {
				return out, e
			}
			c.D[0] = value
			s.Finished = true
			out.Complete = true
			out.FlagsKnown = true
			out.Zero = value == 0
			return out, nil
		default:
			return out, fmt.Errorf("native campaign selection PC%x unsupported", s.PC)
		}
	}
	out.Waiting = true
	return out, nil
}

func campaignCompareWorldCode(cb NativeCampaignFrameCallbacks, a *[7]NativeRequesterAddress) error {
	c := cb.Frame
	saved := c.D[0]
	a[1] = a[0]
	a[0] = NativeRequesterAddress{Address: cb.CodeBase + 0x103ba, Code: true}
	if e := campaignWorldCode(cb, a, a[0].Address); e != nil {
		return e
	}
	a[0] = a[1]
	a[2] = NativeRequesterAddress{Address: cb.CodeBase + 0x103ba, Code: true}
	c.D[1] = 7
	for {
		v, e := cb.Code.Read8(int(a[1].Address - cb.CodeBase))
		if e != nil {
			return e
		}
		a[1].Address++
		c.Byte(2, v)
		if v == 0 {
			other, e := cb.Code.Read8(int(a[2].Address - cb.CodeBase))
			if e != nil {
				return e
			}
			a[2].Address++
			c.D[0] = saved
			c.D[1] = 1
			if other == 0 {
				c.D[1] = 0
			}
			return nil
		}
		other, e := cb.Code.Read8(int(a[2].Address - cb.CodeBase))
		if e != nil {
			return e
		}
		a[2].Address++
		if v != other {
			c.D[0] = saved
			c.D[1] = 1
			return nil
		}
		c.Word(2, uint16(c.D[2])-1)
		if uint16(c.D[2]) == 0xffff {
			c.D[0] = saved
			c.D[1] = 0
			return nil
		}
	}
}
