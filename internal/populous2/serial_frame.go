package populous2

import (
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeSerialFrameRules struct{ Keys NativeInputRules }

func DecodeNativeSerialFrameRules(exe *amiga.Executable) (NativeSerialFrameRules, error) {
	var r NativeSerialFrameRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x34868 {
		return r, fmt.Errorf("native serial requester resources missing")
	}
	var err error
	r.Keys, err = DecodeNativeInputRules(exe)
	return r, err
}

// NativeSerialFrameCall retains genuine raw address arguments. StackBytes
// describes only the source's temporary WORD$0d0d at $4aa4, without inventing
// an absolute caller stack address when the host has not established one.
type NativeSerialFrameCall struct {
	NativeFileFrameCall
	StackBytes []byte
}

// Negative is the original CCR.N return, independent of the restored D0.
// In particular $bbe's MOVEM restores D0 after its failed-read MOVE.W$ffff.
type NativeSerialFrameChildResult struct{ Complete, Zero, Negative bool }

type NativeSerialFrameCallbacks struct {
	NativeFileFrameCallbacks
	Transport          func(NativeSerialFrameCall, *uint32) (NativeSerialFrameChildResult, error)
	CallerStackPointer uint32 // Optional actual A7 on entry to $4984.
}

type NativeSerialFrameState struct {
	Started, Finished bool
	PC                int
	Registers         [8]uint32
	A                 [7]NativeRequesterAddress
	Modal             NativeFileTextModal
	ChildActive       bool
	ChildRoutine      int
	ChildPhase        uint32
	childCall         NativeSerialFrameCall
	modalEnd          int
	sendAddress       uint32
	failed            error
}

// Advance is the original $4984 serial/two-player requester. Hardware and
// connection calls remain actual asynchronous boundaries. Neither an absent
// transport nor a suspended write is interpreted as a successful operation.
func (s *NativeSerialFrameState) Advance(r *NativeSerialFrameRules, cb NativeSerialFrameCallbacks) (out NativeFileFrameStep, failure error) {
	if s == nil || r == nil || cb.Frame == nil || cb.Presentation == nil || cb.Bitmap == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) {
		return out, fmt.Errorf("native serial requester backing missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		s.Started, s.PC, s.Registers = true, 0x4984, cb.Frame.D
	}
	cb.Frame.D = s.Registers
	defer func() {
		s.Registers, out.PC = cb.Frame.D, s.PC
		if failure != nil {
			s.failed = failure
		}
	}()
	if s.Finished {
		out.Complete = true
		return out, nil
	}
	c, code, m := cb.Frame, cb.Code, cb.Memory
	b := nativeRequesterFrameBacking{Code: code, Memory: m, CodeBase: cb.CodeBase, Frame: c, Bitmap: cb.Bitmap, Sound: cb.Sound, ReadAbsolute: cb.ReadAbsolute}
	caddr := func(at int) NativeRequesterAddress {
		return NativeRequesterAddress{Address: cb.CodeBase + uint32(at), Code: true}
	}
	child := func(routine int, stack []byte) (NativeSerialFrameChildResult, error) {
		if cb.Transport == nil {
			return NativeSerialFrameChildResult{}, fmt.Errorf("native serial child %#x callback missing", routine)
		}
		if !s.ChildActive {
			s.ChildActive, s.ChildRoutine = true, routine
			args := uint8(0)
			if routine == 0xbbe || routine == 0xc2a {
				args = 1
			}
			s.childCall = NativeSerialFrameCall{NativeFileFrameCall: NativeFileFrameCall{Routine: routine, Arguments: args, A: s.A, Frame: c}, StackBytes: stack}
			out.Calls = append(out.Calls, routine)
		}
		if s.ChildRoutine != routine {
			return NativeSerialFrameChildResult{}, fmt.Errorf("native serial child changed across suspension")
		}
		s.childCall.Frame = c
		result, err := cb.Transport(s.childCall, &s.ChildPhase)
		if err != nil {
			return result, err
		}
		if !result.Complete {
			out.Waiting = true
			return result, nil
		}
		s.ChildActive, s.ChildPhase = false, 0
		return result, nil
	}
	for transitions := 0; transitions < 256; transitions++ {
		if s.Modal.Active {
			done, err := s.Modal.advance(b, cb.Presentation, r.Keys)
			if err != nil {
				return out, err
			}
			if !done {
				out.Waiting = true
				return out, nil
			}
			s.PC = 0x4a7c
		}
		switch s.PC {
		case 0x4984:
			baud, err := m.Read16(0x15a)
			if err != nil {
				return out, err
			}
			c.Word(0, baud)
			c.D[1] = 0
			index := 0
			for ; index < 5; index++ {
				value, err := code.Read16(0x4b06 + index*8 + 6)
				if err != nil {
					return out, err
				}
				if value == uint16(c.D[0]) {
					break
				}
				c.Word(1, uint16(c.D[1])+8)
			}
			if index == 5 {
				if err := m.Write16(0x15a, 300); err != nil {
					return out, err
				}
				continue
			}
			s.A[1] = caddr(0x4b06 + index*8)
			if err := code.Write32(0x4afa, s.A[1].Address); err != nil {
				return out, err
			}
			if err := code.Write16(0x4b3e, uint16(c.D[1])); err != nil {
				return out, err
			}
			s.PC = 0x49be
		case 0x49be:
			result, err := child(0xab4, nil)
			if err != nil || !result.Complete {
				return out, err
			}
			s.PC = 0x49c4
		case 0x49c4:
			result, err := child(0xae2, nil)
			if err != nil || !result.Complete {
				return out, err
			}
			if result.Zero {
				s.PC = 0x4a28
			} else {
				s.A[0] = caddr(0x4b40)
				c.Word(0, 1)
				s.PC = 0x49d6
			}
		case 0x49d6:
			result, err := child(0xbbe, nil)
			if err != nil || !result.Complete {
				return out, err
			}
			if result.Negative {
				s.PC = 0x4a28
				continue
			}
			v, err := code.Read8(0x4b40)
			if err != nil {
				return out, err
			}
			c.Byte(0, v)
			if int8(v) < 32 || int8(v) >= 0x7a {
				c.Byte(0, 32)
			}
			if int8(c.D[0]) >= 0x61 {
				c.Byte(0, uint8(c.D[0])-32)
			}
			cursor := 0x4b92
			c.D[1] = 0xffffffff
			for {
				c.Word(1, uint16(c.D[1])+1)
				v, err := code.Read8(cursor)
				if err != nil {
					return out, err
				}
				cursor++
				if v == 0 {
					break
				}
			}
			if int16(c.D[1]) >= 34 {
				cursor = 0x4b92
				for {
					v, err := code.Read8(cursor + 1)
					if err != nil {
						return out, err
					}
					if err := code.Write8(cursor, v); err != nil {
						return out, err
					}
					cursor++
					if v == 0 {
						break
					}
				}
			}
			cursor--
			if err := code.Write8(cursor, uint8(c.D[0])); err != nil {
				return out, err
			}
			if err := code.Write8(cursor+1, 0); err != nil {
				return out, err
			}
			s.PC = 0x4a28
		case 0x4a28:
			params := make([]NativeRequesterAddress, 3)
			for i := range params {
				address, err := code.Read32(0x4afa + i*4)
				if err != nil {
					return out, err
				}
				params[i] = NativeRequesterAddress{Address: address, Code: true}
			}
			c.D[3] = 1
			if err := b.compile(cb.CodeBase+0x901e, params); err != nil {
				return out, err
			}
			front, err := m.Read32(0x1a)
			if err != nil {
				return out, err
			}
			back, err := m.Read32(0x1e)
			if err != nil {
				return out, err
			}
			source, err := cb.Bitmap(front)
			if err != nil {
				return out, err
			}
			target, err := cb.Bitmap(back)
			if err != nil {
				return out, err
			}
			if len(source) < 32000 || len(target) < 32000 {
				return out, fmt.Errorf("native serial copy outside real screen RAM")
			}
			for at := 0; at < 32000; at += 32 {
				var block [32]byte
				copy(block[:], source[at:at+32])
				copy(target[at:at+32], block[:])
			}
			s.PC = 0x4a4e
		case 0x4a4e:
			target, err := m.Read32(0x1e)
			if err != nil {
				return out, err
			}
			start, err := code.Read16(0xab4e)
			if err != nil {
				return out, err
			}
			column, err := code.Read16(0xab50)
			if err != nil {
				return out, err
			}
			row, err := code.Read16(0xab52)
			if err != nil {
				return out, err
			}
			c.Word(0, column)
			c.Word(1, row)
			if err := b.text(target, 0xab4e+int(int16(start))); err != nil {
				return out, err
			}
			if err := fileFrameSwap(b, cb.Presentation); err != nil {
				return out, err
			}
			end, err := b.click()
			if err != nil {
				return out, err
			}
			action := uint16(c.D[0])
			dispatch, err := code.Read16(0x4a64 + int(int16(action)))
			if err != nil {
				return out, err
			}
			c.Word(0, dispatch)
			s.PC = 0x4a64 + int(int16(c.D[0]))
			if s.PC == 0x4984 {
				out.Idle = true
				return out, nil
			}
			if s.PC == 0x4a70 {
				s.modalEnd = int(int64(end) - int64(cb.CodeBase))
			}
		case 0x4a70:
			if err := s.Modal.begin(b, 0x4b42, s.modalEnd); err != nil {
				return out, err
			}
		case 0x4a7c:
			s.sendAddress = cb.CodeBase + 0x4b42
			s.A[0] = caddr(0x4b42)
			v, err := code.Read8(0x4b42)
			if err != nil {
				return out, err
			}
			if v == 0 {
				s.PC = 0x4984
				continue
			}
			c.D[0] = 1
			s.PC = 0x4a88
		case 0x4a88:
			v, err := code.Read8(int(int64(s.sendAddress) - int64(cb.CodeBase)))
			if err != nil {
				return out, err
			}
			if v == 0 {
				s.PC = 0x4a9e
			} else {
				s.PC = 0x4a8c
			}
		case 0x4a8c, 0x4a9e:
			ready, err := m.Read16(0xa)
			if err != nil {
				return out, err
			}
			if ready == 0 {
				out.Waiting = true
				return out, nil
			}
			if s.PC == 0x4a8c {
				s.A[0] = NativeRequesterAddress{Address: s.sendAddress, Code: true}
				s.PC = 0x4a92
			} else {
				s.A[0] = NativeRequesterAddress{Absolute: true}
				if cb.CallerStackPointer != 0 {
					s.A[0].Address = cb.CallerStackPointer - 2
				}
				c.Word(0, 1)
				s.PC = 0x4aae
			}
		case 0x4a92:
			result, err := child(0xc2a, nil)
			if err != nil || !result.Complete {
				return out, err
			}
			s.sendAddress += uint32(int32(int16(c.D[0])))
			s.A[0] = NativeRequesterAddress{Address: s.sendAddress, Code: true}
			s.PC = 0x4a88
		case 0x4aae:
			result, err := child(0xc2a, []byte{13, 13})
			if err != nil || !result.Complete {
				return out, err
			}
			s.PC = 0x4984
		case 0x4abc, 0x4ace:
			index, err := code.Read16(0x4b3e)
			if err != nil {
				return out, err
			}
			c.Word(0, index)
			if s.PC == 0x4abc {
				if int16(index) < 32 {
					c.Word(0, index+8)
				}
			} else if index != 0 {
				c.Word(0, index-8)
			}
			baud, err := code.Read16(0x4b06 + int(int16(c.D[0])) + 6)
			if err != nil {
				return out, err
			}
			if err := m.Write16(0x15a, baud); err != nil {
				return out, err
			}
			s.PC = 0x4984
		case 0x4aec:
			result, err := child(0x17eec, nil)
			if err != nil || !result.Complete {
				return out, err
			}
			s.PC = 0x4a62
		case 0x4af6:
			s.PC = 0x4a62
		case 0x4a62:
			s.Finished, out.Complete = true, true
			return out, nil
		default:
			return out, fmt.Errorf("native serial requester branch %#x unavailable", s.PC)
		}
	}
	return out, fmt.Errorf("native serial requester did not reach a poll/child boundary")
}
