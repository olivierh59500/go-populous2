package populous2

import "fmt"

// NativeRuntimeFileOverwriteState retains the real $341e requester and its
// address outputs. The enclosing $19b54 caller separately saves only A0.
type NativeRuntimeFileOverwriteState struct {
	Started, Finished bool
	Registers         [8]uint32
	A                 [7]NativeRequesterAddress
	failed            error
}

func (s *NativeRuntimeFileOverwriteState) Advance(cb NativeFileFrameCallbacks, a *[7]NativeRequesterAddress) (out NativeFileFrameStep, failure error) {
	if s == nil || a == nil || cb.Frame == nil || cb.Presentation == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) || cb.Bitmap == nil {
		return out, fmt.Errorf("native file overwrite full-register backing missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	defer func() {
		s.Registers, s.A = cb.Frame.D, *a
		if failure != nil {
			s.failed = failure
		}
	}()
	b := nativeRequesterFrameBacking{Code: cb.Code, Memory: cb.Memory, CodeBase: cb.CodeBase, Frame: cb.Frame, Bitmap: cb.Bitmap, Sound: cb.Sound}
	if !s.Started {
		s.Started = true
		a[1] = NativeRequesterAddress{Address: cb.CodeBase + 0x930c, Code: true}
		a[2] = NativeRequesterAddress{Address: cb.CodeBase + 0x3466, Code: true}
		cb.Frame.D[3] = 1
		if e := campaignRequesterCompile(b, a, a[1].Address, a[2].Address); e != nil {
			return out, e
		}
		if e := campaignRequesterCopy(b, a); e != nil {
			return out, e
		}
		s.Registers, s.A = cb.Frame.D, *a
	}
	cb.Frame.D, *a = s.Registers, s.A
	if s.Finished {
		out.Complete = true
		return out, nil
	}
	target, e := cb.Memory.Read32(0x1e)
	if e != nil {
		return out, e
	}
	start, e := cb.Code.Read16(0xab4e)
	if e != nil {
		return out, e
	}
	column, e := cb.Code.Read16(0xab50)
	if e != nil {
		return out, e
	}
	row, e := cb.Code.Read16(0xab52)
	if e != nil {
		return out, e
	}
	cb.Frame.Word(0, column)
	cb.Frame.Word(1, row)
	if e = campaignRequesterText(b, a, target, 0xab4e+int(int16(start))); e != nil {
		return out, e
	}
	if e = fileFrameSwap(b, cb.Presentation); e != nil {
		return out, e
	}
	if _, e = campaignRequesterClick(b, a); e != nil {
		return out, e
	}
	offset, e := cb.Code.Read16(0x345c + int(int16(cb.Frame.D[0])))
	if e != nil {
		return out, e
	}
	cb.Frame.Word(0, offset)
	out.PC = 0x345c + int(int16(offset))
	switch out.PC {
	case 0x3444:
		out.Idle = true
	case 0x3458:
		cb.Frame.D[0] = 0
		s.Finished, out.Complete = true, true
	case 0x3462:
		cb.Frame.D[0] = 1
		s.Finished, out.Complete = true, true
	default:
		return out, fmt.Errorf("native file overwrite branch%x unavailable", out.PC)
	}
	return out, nil
}
