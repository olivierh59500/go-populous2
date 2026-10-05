package populous2

import (
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeOptionsFrameRules struct{ Keys NativeInputRules }

func DecodeNativeOptionsFrameRules(exe *amiga.Executable) (NativeOptionsFrameRules, error) {
	var r NativeOptionsFrameRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x34868 {
		return r, fmt.Errorf("native options requester resources missing")
	}
	var err error
	r.Keys, err = DecodeNativeInputRules(exe)
	return r, err
}

// NativeOptionsFrameState is the actual $471c requester. Its current side,
// text field, option words and reaction setting remain in shared CODE/BSS.
// It yields at the original $47ec polling edge or an unsatisfied $786 modal.
type NativeOptionsFrameState struct {
	Started, Finished bool
	PC                int
	Registers         [8]uint32
	Modal             NativeFileTextModal
	optionAddress     int
	modalEnd          int
	failed            error
}

func (s *NativeOptionsFrameState) Advance(r *NativeOptionsFrameRules, cb NativeFileFrameCallbacks) (out NativeFileFrameStep, failure error) {
	if s == nil || r == nil || cb.Frame == nil || cb.Presentation == nil || cb.Bitmap == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) {
		return out, fmt.Errorf("native options frame/backing missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		s.Started, s.PC, s.Registers = true, 0x471c, cb.Frame.D
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
	side := func() (uint16, error) { return code.Read16(0x4952) }
	draw := func() error {
		target, err := m.Read32(0x1e)
		if err != nil {
			return err
		}
		start, err := code.Read16(0xab4e)
		if err != nil {
			return err
		}
		column, err := code.Read16(0xab50)
		if err != nil {
			return err
		}
		row, err := code.Read16(0xab52)
		if err != nil {
			return err
		}
		c.Word(0, column)
		c.Word(1, row)
		if err := b.text(target, 0xab4e+int(int16(start))); err != nil {
			return err
		}
		return fileFrameSwap(b, cb.Presentation)
	}
	for transitions := 0; transitions < 128; transitions++ {
		if s.Modal.Active {
			done, err := s.Modal.advance(b, cb.Presentation, r.Keys)
			if err != nil {
				return out, err
			}
			if !done {
				out.Waiting = true
				return out, nil
			}
			s.PC = 0x491a
		}
		switch s.PC {
		case 0x471c:
			if err := code.Write16(0x4eb0, 0x0101); err != nil {
				return out, err
			}
			owner, err := m.Read16(0xeb42)
			if err != nil {
				return out, err
			}
			if err := code.Write16(0x4952, owner); err != nil {
				return out, err
			}
			s.PC = 0x472e
		case 0x472e:
			owner, err := side()
			if err != nil {
				return out, err
			}
			name := 0x9670
			if owner != 1 {
				name = 0x9675
			}
			if err := code.Write32(0x4954, cb.CodeBase+uint32(name)); err != nil {
				return out, err
			}
			parameters := make([]NativeRequesterAddress, 2)
			for i := range parameters {
				p, err := code.Read32(0x4954 + i*4)
				if err != nil {
					return out, err
				}
				parameters[i] = NativeRequesterAddress{Address: p, Code: true}
			}
			c.D[3] = 1
			if err := b.compile(cb.CodeBase+0x8c42, parameters); err != nil {
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
				return out, fmt.Errorf("native options screen copy outside real RAM")
			}
			// $f6da reads each32-byte MOVEM block before writing it. Actual
			// overlap must be observed by the next block, not snapshotted away.
			for at := 0; at < 32000; at += 32 {
				var block [32]byte
				copy(block[:], source[at:at+32])
				copy(target[at:at+32], block[:])
			}
			s.optionAddress = 0xeb2c
			if owner != 1 {
				s.optionAddress = 0xeb2e
			}
			options, err := m.Read16(s.optionAddress)
			if err != nil {
				return out, err
			}
			c.Word(2, options)
			start, err := code.Read16(0xab4e)
			if err != nil {
				return out, err
			}
			for cursor := 0xab4e + int(int16(start)); ; {
				v, err := code.Read8(cursor)
				if err != nil {
					return out, err
				}
				cursor++
				c.Byte(0, v)
				if v == 0 {
					break
				}
				if v == 'h' {
					god := 0xe8a4
					if owner != 1 {
						god = 0xe9de
					}
					reaction, err := m.Read16(god + 0x68)
					if err != nil {
						return out, err
					}
					c.Word(0, reaction)
					if int16(c.D[0]) >= 16 {
						c.Word(0, 15)
					}
					if err := code.Write8(cursor+int(int16(c.D[0])), 'g'); err != nil {
						return out, err
					}
				} else if v == 'y' || v == 'z' {
					marker := uint8('y')
					if uint16(c.D[2])&1 == 0 {
						marker = 'z'
					}
					if err := code.Write8(cursor-1, marker); err != nil {
						return out, err
					}
					c.Word(2, uint16(c.D[2])>>1)
				}
			}
			s.PC = 0x47ec
		case 0x47ec:
			if err := draw(); err != nil {
				return out, err
			}
			end, err := b.click()
			if err != nil {
				return out, err
			}
			action := uint16(c.D[0])
			mode, err := m.Read16(0xeb44)
			if err != nil {
				return out, err
			}
			if mode == 2 && int16(action) > 2 && int16(action) <= 26 {
				out.Idle = true
				return out, nil
			}
			dispatch, err := code.Read16(0x4824 + int(int16(action)))
			if err != nil {
				return out, err
			}
			c.Word(0, dispatch)
			s.PC = 0x4824 + int(int16(c.D[0]))
			if s.PC == 0x47ec {
				out.Idle = true
				return out, nil
			}
			if s.PC == 0x4906 {
				s.modalEnd = int(int64(end) - int64(cb.CodeBase))
			}
		case 0x4844:
			owner, err := side()
			if err != nil {
				return out, err
			}
			c.Word(0, 1)
			if owner == 1 {
				c.Word(0, 2)
			}
			if err := code.Write16(0x4952, uint16(c.D[0])); err != nil {
				return out, err
			}
			s.PC = 0x472e
		case 0x485e, 0x486a, 0x4876, 0x4882, 0x488e, 0x489a, 0x48a6, 0x48b2, 0x48be, 0x48ca:
			options, err := m.Read16(s.optionAddress)
			if err != nil {
				return out, err
			}
			c.Word(0, options)
			c.Word(0, uint16(c.D[0])^(1<<uint((s.PC-0x485e)/12)))
			if err := m.Write16(s.optionAddress, uint16(c.D[0])); err != nil {
				return out, err
			}
			s.PC = 0x472e
		case 0x48d6, 0x48da:
			c.D[0] = 0xffffffff
			if s.PC == 0x48da {
				c.D[0] = 1
			}
			owner, err := side()
			if err != nil {
				return out, err
			}
			god := 0xe8a4
			if owner != 1 {
				god = 0xe9de
			}
			reaction, err := m.Read16(god + 0x68)
			if err != nil {
				return out, err
			}
			c.Word(0, uint16(c.D[0])+reaction)
			if int16(c.D[0]) >= 0 && uint16(c.D[0]) != 16 {
				if err := m.Write16(god+0x68, uint16(c.D[0])); err != nil {
					return out, err
				}
			}
			s.PC = 0x472e
		case 0x4906:
			if err := s.Modal.begin(b, 0x495c, s.modalEnd); err != nil {
				return out, err
			}
		case 0x491a:
			word, err := code.Read32(0x495c)
			if err != nil {
				return out, err
			}
			if word == 0x4d555349 {
				enabled, err := m.Read16(0x3bc)
				if err != nil {
					return out, err
				}
				if err := m.Write16(0x3bc, ^enabled); err != nil {
					return out, err
				}
			}
			s.PC = 0x47ec
		case 0x4932:
			owner, err := side()
			if err != nil {
				return out, err
			}
			profile := 0x20630
			if owner != 1 {
				profile = 0x2072a
			}
			options, err := m.Read16(s.optionAddress)
			if err != nil {
				return out, err
			}
			if err := code.Write16(profile+12, options); err != nil {
				return out, err
			}
			c.D[0] = 0
			s.PC = 0x481a
		case 0x481a:
			if err := code.Write16(0x4eb0, 0); err != nil {
				return out, err
			}
			s.PC, s.Finished, out.Complete = 0x4822, true, true
			return out, nil
		default:
			return out, fmt.Errorf("native options requester source branch %#x unavailable", s.PC)
		}
	}
	return out, fmt.Errorf("native options requester did not reach its polling/modal boundary")
}
