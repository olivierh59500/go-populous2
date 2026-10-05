package populous2

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// NativeNumericModal retains $4bba/$4c14's live input cursor and the saved
// data-register context. Its display wait is resumed by a real input VBlank;
// it neither polls invented key values nor completes a modal in one frame.
type NativeNumericModal struct {
	Active, Waiting            bool
	Field, Caret, DisplayStart int
	Capacity                   uint16
	Column, Row                uint16
	Registers                  [8]uint32
	BufferToggle               uint32 // Original mutable CODE $77a; caller supplies current value.
	Pending                    NativeRequester
	Tail                       byte
	FinishAfterPaint           uint8 // 1: Enter, 2: actual requester click.
}

type NativeModalInputStep struct {
	Draws        []NativeModalInputDraw
	Audio        []uint16
	Finished     bool
	ReturnAction uint16
}

type NativeModalInputDraw struct {
	Requester NativeRequester
	Buffer    uint32 // Actual pre-swap BSS $1e, not the later visible pointer.
}

// NativePaintingModalOrigin recovers the actual post-$4dac A0 offset from
// retained click words. Its output is the byte after the last scanned glyph,
// not an arbitrary numeric field start or a guessed zero pointer.
func NativePaintingModalOrigin(p *NativePaintingState, m FollowerCleanupMemory) (int, error) {
	if p == nil || m.Read16 == nil {
		return 0, fmt.Errorf("native modal click backing missing")
	}
	x, err := m.Read16(0x134)
	if err != nil {
		return 0, err
	}
	y, err := m.Read16(0x136)
	if err != nil {
		return 0, err
	}
	w := func(a int) uint16 { return binary.BigEndian.Uint16(p.Scratch[a:]) }
	dx, dy := uint16((x>>3)-w(2)), uint16(y-w(4))
	if int16(dx) < 0 || int16(uint16(dx-w(6))) >= 0 || int16(dy) < 0 || int16(uint16(dy-w(8))) > 0 {
		return 0, fmt.Errorf("native modal origin is outside actual requester click")
	}
	end := int(int16(w(0))) + int(int16(uint16((dy/8)*(w(6)+1)))) + int(int16(dx)) + 1
	if end < 0 || end >= len(p.Scratch) {
		return 0, fmt.Errorf("native modal click alias outside retained scratch")
	}
	return end, nil
}

// BeginNativeNumericModal follows $4bba..$4c2e. clickEnd is the real A0
// continuation from the requester scan, and registers are the actual caller
// values. Pointer subtraction cancels the CODE address origin exactly.
func BeginNativeNumericModal(p *NativePaintingState, field, clickEnd int, registers [8]uint32, bufferToggle uint32) (NativeNumericModal, error) {
	var s NativeNumericModal
	if p == nil || field < 0 || field >= len(p.Fields) || clickEnd < 0 || clickEnd >= len(p.Scratch) {
		return s, fmt.Errorf("native numeric modal origin missing")
	}
	v := clickEnd
	for v >= 0 && p.Scratch[v] != 'v' {
		v--
	}
	if v < 0 {
		return s, fmt.Errorf("native modal field marker outside retained workspace")
	}
	start := int(int16(binary.BigEndian.Uint16(p.Scratch[:])))
	width := binary.BigEndian.Uint16(p.Scratch[6:])
	divisor := uint16(width + 1)
	if divisor == 0 {
		return s, fmt.Errorf("native modal requester division by zero")
	}
	registers[0] = uint32(v - start)
	registers[0] = hudWord(registers[0], uint16(registers[0])+1)
	registers[2] = hudWord(registers[2], divisor)
	quotient, remainder := registers[0]/uint32(divisor), registers[0]%uint32(divisor)
	if quotient > 0xffff {
		return s, fmt.Errorf("native modal pointer division overflow")
	}
	registers[0] = remainder<<16 | quotient
	registers[2] = hudWord(registers[2], uint16(registers[0]))
	registers[0] = registers[0]<<16 | registers[0]>>16
	registers[0] = hudWord(registers[0], uint16(registers[0])+binary.BigEndian.Uint16(p.Scratch[2:]))
	registers[1] = hudWord(registers[1], uint16(registers[0]))
	registers[2] = uint32(uint16(registers[2])) * 8
	registers[2] = hudWord(registers[2], uint16(registers[2])+binary.BigEndian.Uint16(p.Scratch[4:]))
	end := v + 1
	for end < len(p.Scratch) && p.Scratch[end] != 'w' {
		end++
	}
	if end >= len(p.Scratch) {
		return s, fmt.Errorf("native modal field terminator outside retained workspace")
	}
	capacity := uint16(end - v - 1)
	if capacity == 0 {
		return s, fmt.Errorf("native zero-width modal exceeds bounded retained memory")
	}
	registers[3] = 0xffff0000 | uint32(capacity)
	length := bytes.IndexByte(p.Fields[field][:], 0)
	if length < 0 {
		return s, fmt.Errorf("native modal input has no retained terminator")
	}
	registers[4] = 0xffff0000 | uint32(length)
	if int16(registers[4]) >= int16(capacity) {
		registers[4] = hudWord(registers[4], capacity-1)
	}
	s = NativeNumericModal{Active: true, Field: field, Caret: length, DisplayStart: v + 1, Capacity: capacity, Column: uint16(registers[1]), Row: uint16(registers[2]), Registers: registers, BufferToggle: bufferToggle}
	return s, nil
}

func (s *NativeNumericModal) preparePainting(p *NativePaintingState, input *NativeInputState) error {
	if s.Caret < 0 || s.Caret >= len(p.Fields[s.Field]) {
		return fmt.Errorf("native modal cursor outside retained numeric buffer")
	}
	start := s.Caret - int(int16(s.Registers[4]))
	if start < 0 || s.DisplayStart < 0 || s.DisplayStart+int(s.Capacity) >= len(p.Scratch) {
		return fmt.Errorf("native modal viewport outside retained fields")
	}
	text := make([]byte, int(s.Capacity))
	for i := range text {
		ch := byte('k')
		if start >= len(p.Fields[s.Field]) {
			return fmt.Errorf("native modal text read outside retained numeric buffer")
		}
		if p.Fields[s.Field][start] != 0 {
			ch = p.Fields[s.Field][start]
			start++
		}
		if uint16(i) == uint16(s.Registers[5]) && input.Low[0xf]&0x10 != 0 {
			ch = 'm'
		}
		text[i] = ch
		p.Scratch[s.DisplayStart+i] = ch
	}
	tail := s.DisplayStart + int(s.Capacity)
	s.Tail = p.Scratch[tail]
	p.Scratch[tail] = 0
	s.Pending = NativeRequester{Column: int(s.Column), Row: int(s.Row), Width: int(s.Capacity), Height: 8, Text: text}
	s.Waiting = true
	return nil
}

// Advance follows the native loop until its next unsatisfied $786 wait or
// real return. Already pending redraws complete before the next key is read.
// Each completed redraw performs $72e's exact pointer swap and wait reset.
func (s *NativeNumericModal) Advance(r *NativePresentationInputRules, p *NativePaintingState, input *NativeInputState, keyRules NativeInputRules) (NativeModalInputStep, error) {
	var out NativeModalInputStep
	if s == nil || p == nil || input == nil || r == nil || s.Field < 0 || s.Field >= len(p.Fields) {
		return out, fmt.Errorf("native modal live backing missing")
	}
	if !s.Active {
		out.Finished = true
		out.ReturnAction = uint16(s.Registers[0])
		return out, nil
	}
	for transitions := 0; transitions < 8; transitions++ {
		if s.Waiting {
			if input.word(0xa) == 0 {
				return out, nil
			}
			out.Draws = append(out.Draws, NativeModalInputDraw{Requester: s.Pending, Buffer: input.long(0x1e)})
			front, back := input.long(0x1a), input.long(0x1e)
			input.setLong(0x1a, back)
			input.setLong(0x1e, front)
			s.BufferToggle ^= 4
			input.setWord(0xa, 0)
			p.Scratch[s.DisplayStart+int(s.Capacity)] = s.Tail
			s.Waiting = false
			if s.FinishAfterPaint != 0 {
				if s.FinishAfterPaint == 1 {
					s.Registers[0] = 0
				}
				s.Active = false
				out.Finished = true
				out.ReturnAction = uint16(s.Registers[0])
				return out, nil
			}
		}
		if input.word(0x140) != 0 || input.word(0x142) != 0 {
			action, err := r.clickPainting(p, input.Memory(FollowerCleanupMemory{}))
			if err != nil {
				return out, err
			}
			input.setWord(0x140, 0) // $4d9c always clears it, including outside clicks.
			s.Registers[0] = uint32(action)
			if action != 0 {
				out.Audio = append(out.Audio, 0x1d6)
				s.Registers[5] = 0xffffffff
				s.FinishAfterPaint = 2
				if err := s.preparePainting(p, input); err != nil {
					return out, err
				}
				continue
			}
		}
		field := &p.Fields[s.Field]
		decrement := func() {
			s.Registers[4] = hudWord(s.Registers[4], uint16(s.Registers[4])-1)
			if int16(s.Registers[4]) < 0 {
				s.Registers[4] = 0
			}
		}
		if input.Low[0x93] != 0 {
			input.Low[0x93] = 0
			if s.Caret != 0 {
				s.Caret--
				decrement()
			}
		} else if input.Low[0x95] != 0 {
			input.Low[0x95] = 0
			if s.Caret < 0 || s.Caret >= len(field) {
				return out, fmt.Errorf("native modal cursor outside retained numeric buffer")
			}
			if field[s.Caret] != 0 {
				s.Caret++
				s.Registers[4] = hudWord(s.Registers[4], uint16(s.Registers[4])+1)
				if uint16(s.Registers[4]) == s.Capacity {
					s.Registers[4] = hudWord(s.Registers[4], uint16(s.Registers[4])-1)
				}
			}
		} else {
			ch, err := keyRules.Character(input, &s.Registers)
			if err != nil {
				return out, err
			}
			if ch == 13 {
				s.Registers[5] = 0xffffffff
				s.FinishAfterPaint = 1
				if err := s.preparePainting(p, input); err != nil {
					return out, err
				}
				continue
			}
			if ch == 8 {
				if s.Caret != 0 {
					s.Caret--
					end := bytes.IndexByte(field[:], 0)
					if end < 0 || s.Caret > end {
						return out, fmt.Errorf("native modal backspace alias outside retained numeric buffer")
					}
					copy(field[s.Caret:end], field[s.Caret+1:end+1])
					decrement()
				}
			} else if ch != 0 {
				end := bytes.IndexByte(field[:], 0)
				if end < 0 || end+1 >= len(field) || s.Caret > end {
					return out, fmt.Errorf("native modal insertion alias outside retained numeric buffer")
				}
				if field[end+1] != 0xff {
					copy(field[s.Caret+1:end+2], field[s.Caret:end+1])
					field[s.Caret] = ch
					s.Caret++
					s.Registers[4] = hudWord(s.Registers[4], uint16(s.Registers[4])+1)
					if uint16(s.Registers[4]) == s.Capacity {
						s.Registers[4] = hudWord(s.Registers[4], uint16(s.Registers[4])-1)
					}
				}
			}
		}
		s.Registers[5] = hudWord(s.Registers[5], uint16(s.Registers[4]))
		if err := s.preparePainting(p, input); err != nil {
			return out, err
		}
	}
	return out, fmt.Errorf("native modal exceeded bounded wait/return transitions")
}
