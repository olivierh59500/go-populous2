package populous2

import "fmt"

// NativeDOSOverwriteState runs the actual $341e two-button requester. The
// saved DOS caller owns A0's restoration; this child leaves its real full-D
// drawing/click outputs and MOVEQ 0/1 return, rather than a synthetic answer.
type NativeDOSOverwriteState struct {
	Started, Finished bool
	Registers         [8]uint32
	failed            error
}

func (s *NativeDOSOverwriteState) Advance(cb NativeFileFrameCallbacks) (out NativeFileFrameStep, failure error) {
	if s == nil || cb.Frame == nil || cb.Presentation == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) || cb.Bitmap == nil {
		return out, fmt.Errorf("native DOS overwrite backing missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	defer func() {
		s.Registers = cb.Frame.D
		if failure != nil {
			s.failed = failure
		}
	}()
	if !s.Started {
		s.Started = true
		b := nativeRequesterFrameBacking{Code: cb.Code, Memory: cb.Memory, CodeBase: cb.CodeBase, Frame: cb.Frame, Bitmap: cb.Bitmap, Sound: cb.Sound}
		cb.Frame.D[3] = 1
		name, e := cb.Code.Read32(0x3466)
		if e != nil {
			return out, e
		}
		if e := b.compile(cb.CodeBase+0x930c, []NativeRequesterAddress{{Address: name, Code: true}}); e != nil {
			return out, e
		}
		front, e := cb.Memory.Read32(0x1a)
		if e != nil {
			return out, e
		}
		back, e := cb.Memory.Read32(0x1e)
		if e != nil {
			return out, e
		}
		source, e := cb.Bitmap(front)
		if e != nil {
			return out, e
		}
		target, e := cb.Bitmap(back)
		if e != nil {
			return out, e
		}
		if len(source) < 32000 || len(target) < 32000 {
			return out, fmt.Errorf("native DOS overwrite screen RAM missing")
		}
		for at := 0; at < 32000; at += 32 {
			var block [32]byte
			copy(block[:], source[at:at+32])
			copy(target[at:at+32], block[:])
		}
		s.Registers = cb.Frame.D
	}
	cb.Frame.D = s.Registers
	if s.Finished {
		out.Complete = true
		return out, nil
	}
	b := nativeRequesterFrameBacking{Code: cb.Code, Memory: cb.Memory, CodeBase: cb.CodeBase, Frame: cb.Frame, Bitmap: cb.Bitmap, Sound: cb.Sound}
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
	if e := b.text(target, 0xab4e+int(int16(start))); e != nil {
		return out, e
	}
	if e := fileFrameSwap(b, cb.Presentation); e != nil {
		return out, e
	}
	if _, e := b.click(); e != nil {
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
		s.Finished = true
		out.Complete = true
	case 0x3462:
		cb.Frame.D[0] = 1
		s.Finished = true
		out.Complete = true
	default:
		return out, fmt.Errorf("native overwrite requester dispatch%x unavailable", out.PC)
	}
	return out, nil
}
