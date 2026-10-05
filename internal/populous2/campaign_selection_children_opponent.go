package populous2

import "fmt"

type NativeCampaignOpponentFrameState struct {
	Started, Finished bool
	PC                int
	Registers         [8]uint32
	A                 [7]NativeRequesterAddress
	ChildActive       bool
	ChildPhase        uint32
	failed            error
}

// Advance executes $af82 through its original resource, double VBlank wait,
// biography/portrait drawing and swap/click loop. Caller register/address
// values survive genuine pending resource operations without prefix replay.
func (s *NativeCampaignOpponentFrameState) Advance(r *NativeRenderFrameRules, cb NativeCampaignFrameCallbacks) (out NativeCampaignFrameStep, failure error) {
	if s == nil || r == nil || cb.Frame == nil || cb.Presentation == nil || cb.Bitmap == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.RAM) {
		return out, fmt.Errorf("native opponent physical backing missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		s.Started = true
		s.PC = 0xaf82
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
		out.FlagsKnown = true
		out.Negative = true
		return out, nil
	}
	c, m, code := cb.Frame, cb.Memory, cb.Code
	b := nativeRequesterFrameBacking{Code: code, Memory: m, CodeBase: cb.CodeBase, Frame: c, Bitmap: cb.Bitmap, Sound: cb.Sound, ReadAbsolute: cb.ReadAbsolute}
	ca := func(at int) NativeRequesterAddress {
		return NativeRequesterAddress{Address: cb.CodeBase + uint32(at), Code: true}
	}
	stage := func(stride uint32) error {
		c.D[1] = 1000
		if e := frameDivide(c, 1, 31); e != nil {
			return e
		}
		world, e := m.Read16(0xeb46)
		if e != nil {
			return e
		}
		c.Word(0, world)
		c.D[0] = uint32(int32(int16(c.D[0])))
		if e = frameDivide(c, 0, uint16(c.D[1])); e != nil {
			return e
		}
		c.D[0] = uint32(uint16(c.D[0])) * stride
		return nil
	}
	for transitions := 0; transitions < 32; transitions++ {
		switch s.PC {
		case 0xaf82:
			c.Word(0, 8)
			s.PC = 0xaf86
		case 0xaf86:
			if cb.Child == nil {
				return out, fmt.Errorf("native opponent resource19cd0 missing")
			}
			s.ChildActive = true
			r, e := cb.Child(NativeStartupResetFrameCall{Routine: 0x19cd0, Frame: c, A: &s.A}, &s.ChildPhase)
			if e != nil {
				return out, e
			}
			if !r.Complete {
				out.Waiting = true
				return out, nil
			}
			s.ChildActive = false
			s.ChildPhase = 0
			s.PC = 0xaf8c
		case 0xaf8c:
			if e := stage(14); e != nil {
				return out, e
			}
			c.D[0] += cb.CodeBase + 0x96d6
			if e := code.Write32(0xb108, c.D[0]); e != nil {
				return out, e
			}
			if e := stage(23); e != nil {
				return out, e
			}
			c.D[0] += cb.CodeBase + 0x9896
			if e := code.Write32(0xb10c, c.D[0]); e != nil {
				return out, e
			}
			profile, e := m.Read16(0xeb42)
			if e != nil {
				return out, e
			}
			god := 0xe9de
			if profile != 1 {
				god = 0xe8a4
			}
			s.A[6] = NativeRequesterAddress{Address: c.AddressBase + uint32(god)}
			c.D[0] = 0
			reaction, e := m.Read16(god + 0x68)
			if e != nil {
				return out, e
			}
			c.Word(0, reaction)
			if int16(c.D[0]) < 0 {
				c.D[0] = cb.CodeBase + 0xa8b3
			} else {
				c.D[0] = uint32(uint16(c.D[0]))*5 + cb.CodeBase + 0xa877
				if int32(c.D[0]) >= int32(cb.CodeBase+0xa8bd) {
					c.D[0] = cb.CodeBase + 0xa8b3
				}
			}
			if e = code.Write32(0xb110, c.D[0]); e != nil {
				return out, e
			}
			c.D[0] = 0
			aggression, e := m.Read16(god + 0x6a)
			if e != nil {
				return out, e
			}
			c.Word(0, aggression)
			if e = frameDivide(c, 0, 5); e != nil {
				return out, e
			}
			c.D[0] = uint32(uint16(c.D[0]))*12 + cb.CodeBase + 0xa8be
			if int32(c.D[0]) >= int32(cb.CodeBase+0xa91e) {
				c.D[0] = cb.CodeBase + 0xa912
			}
			if e = code.Write32(0xb114, c.D[0]); e != nil {
				return out, e
			}
			if e = stage(5); e != nil {
				return out, e
			}
			s.A[0] = ca(0x9b76)
			for uint16(c.D[0]) != 0 {
				for {
					v, e := code.Read8(int(s.A[0].Address - cb.CodeBase))
					if e != nil {
						return out, e
					}
					s.A[0].Address++
					if v == 0 {
						break
					}
				}
				c.Word(0, uint16(c.D[0])-1)
			}
			s.A[1] = ca(0xb118)
			c.D[0] = 4
			for i := 0; i < 5; i++ {
				if e = code.Write32(int(s.A[1].Address-cb.CodeBase), s.A[0].Address); e != nil {
					return out, e
				}
				s.A[1].Address += 4
				for {
					v, e := code.Read8(int(s.A[0].Address - cb.CodeBase))
					if e != nil {
						return out, e
					}
					s.A[0].Address++
					if v == 0 {
						break
					}
				}
				c.Word(0, uint16(c.D[0])-1)
			}
			s.A[1], s.A[2] = ca(0x7208), ca(0xb108)
			c.D[3] = 1
			if e = campaignRequesterCompile(b, &s.A, s.A[1].Address, s.A[2].Address); e != nil {
				return out, e
			}
			s.PC = 0xb08c
		case 0xb08c:
			ready, e := m.Read16(0xa)
			if e != nil {
				return out, e
			}
			if ready == 0 {
				out.Waiting = true
				return out, nil
			}
			if e = m.Write16(0xa, 0); e != nil {
				return out, e
			}
			s.PC = 0xb098
		case 0xb098:
			ready, e := m.Read16(0xa)
			if e != nil {
				return out, e
			}
			if ready == 0 {
				out.Waiting = true
				return out, nil
			}
			s.PC = 0xb09e
		case 0xb09e:
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
			if e = stage(14); e != nil {
				return out, e
			}
			s.A[3] = NativeRequesterAddress{Address: uint32(int64(cb.CodeBase+0x96e4) + int64(int16(c.D[0]))), Code: true}
			s.A[2] = ca(0xb12c)
			if e = DrawNativeCampaignPortrait(r, cb, &s.A); e != nil {
				return out, e
			}
			if e = fileFrameSwap(b, cb.Presentation); e != nil {
				return out, e
			}
			if _, e = campaignRequesterClick(b, &s.A); e != nil {
				return out, e
			}
			branch, e := code.Read16(0xb104 + int(int16(c.D[0])))
			if e != nil {
				return out, e
			}
			c.Word(0, branch)
			s.PC = 0xb104 + int(int16(branch))
			if s.PC == 0xb08c {
				out.Waiting = true
				return out, nil
			}
		case 0xb102:
			s.Finished = true
			out.Complete = true
			out.FlagsKnown = true
			out.Negative = true
			return out, nil
		default:
			return out, fmt.Errorf("native opponent PC%x unsupported", s.PC)
		}
	}
	out.Waiting = true
	return out, nil
}
