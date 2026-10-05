package populous2

import "fmt"

// NativeCampaignFrameCallbacks retains physical resource addresses and the
// actual shared UI backing. Child owns each genuine operation and its phase.
type NativeCampaignFrameCallbacks struct {
	NativeFileFrameCallbacks
	RAM   FollowerCleanupMemory
	Child func(NativeStartupResetFrameCall, *uint32) (NativeCommandFrameResult, error)
}

type NativeCampaignFrameStep struct {
	Complete, Waiting          bool
	FlagsKnown, Zero, Negative bool
	PC                         int
}

// NativeCampaignRecordFrameState is $11044/$1a4f2. The source operand at
// CODE$1a502 belongs to the actual resource loader and may change while it
// waits. No campaign Level or constructor output replaces that raw record.
type NativeCampaignRecordFrameState struct {
	Started, Finished bool
	PC                int
	Registers         [8]uint32
	A, SavedA         [7]NativeRequesterAddress
	SavedLoad         [7]uint32
	World             uint16
	ChildActive       bool
	ChildPhase        uint32
	failed            error
}

func (s *NativeCampaignRecordFrameState) Advance(cb NativeCampaignFrameCallbacks) (out NativeCampaignFrameStep, failure error) {
	if s == nil || cb.Frame == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.RAM) {
		return out, fmt.Errorf("native campaign physical backing missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		s.Started = true
		s.PC = 0x11044
		s.Registers = cb.Frame.D
		s.World = uint16(cb.Frame.D[0])
	}
	cb.Frame.D = s.Registers
	defer func() {
		s.Registers = cb.Frame.D
		out.PC = s.PC
		if failure != nil {
			s.failed = failure
		}
	}()
	c := cb.Frame
	terminal := func() {
		out.Complete = true
		out.FlagsKnown = true
		out.Zero = c.D[1] == 0
		out.Negative = int32(c.D[1]) < 0
	}
	if s.Finished {
		terminal()
		return out, nil
	}
	for transitions := 0; transitions < 8; transitions++ {
		switch s.PC {
		case 0x11044:
			c.D[0] = uint32(int32(int16(c.D[0])))
			if e := frameDivide(c, 0, 5); e != nil {
				return out, e
			}
			c.D[0] = uint32(uint16(c.D[0])) * 250
			c.D[2] = c.D[0]
			copy(s.SavedLoad[:], c.D[1:])
			s.SavedA = s.A
			c.Word(0, 12)
			s.PC = 0x1a4fa
		case 0x1a4fa:
			if cb.Child == nil {
				return out, fmt.Errorf("native campaign resource19cd0 child missing")
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
			s.PC = 0x1a500
		case 0x1a500:
			source, e := cb.Code.Read32(0x1a502)
			if e != nil {
				return out, e
			}
			s.A[0] = NativeRequesterAddress{Address: source + c.D[2], Absolute: true}
			s.A[1] = NativeRequesterAddress{Address: cb.CodeBase + 0x20536, Code: true}
			c.Word(0, 249)
			for i := 0; i < 250; i++ {
				v, e := cb.RAM.Read8(int(s.A[0].Address))
				if e != nil {
					return out, e
				}
				if e = cb.Code.Write8(int(s.A[1].Address-cb.CodeBase), v); e != nil {
					return out, e
				}
				s.A[0].Address++
				s.A[1].Address++
				c.Word(0, uint16(c.D[0])-1)
			}
			copy(c.D[1:], s.SavedLoad[:])
			s.A = s.SavedA
			s.PC = 0x1105a
		case 0x1105a:
			if e := LoadNativeStartupTemplates(NativeStartupResetFrameCallbacks{Code: cb.Code, Memory: cb.Memory, RAM: cb.RAM, CodeBase: cb.CodeBase, Frame: c}, &s.A); e != nil {
				return out, e
			}
			c.Word(0, s.World)
			c.D[0] = uint32(int32(int16(c.D[0])))
			if e := frameDivide(c, 0, 5); e != nil {
				return out, e
			}
			c.Swap(0)
			c.D[0] = uint32(uint16(c.D[0])) * 0x2d7
			c.D[1] += c.D[0]
			if e := cb.Memory.Write32(0xeb28, c.D[1]); e != nil {
				return out, e
			}
			s.PC = 0x11076
			s.Finished = true
			terminal()
			return out, nil
		default:
			return out, fmt.Errorf("native campaign record PC%x unsupported", s.PC)
		}
	}
	return out, fmt.Errorf("native campaign record transition overflow")
}
