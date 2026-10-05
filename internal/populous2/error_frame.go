package populous2

import "fmt"

type NativeErrorFrameCallbacks struct {
	NativeFileFrameCallbacks
	RAM FollowerCleanupMemory // Genuine physical template/parameter/string addresses.
}

// NativeErrorFrameState retains complete$33b2 (or its$339e wrapper). Address
// arguments are actual physical pointers. The wrapper overwrites A1/A2 before
// the inner MOVEM saves them; it does not restore their pre-wrapper values.
type NativeErrorFrameState struct {
	Routine           int
	A, SavedA         [7]NativeRequesterAddress
	Started, Finished bool
	PC                int
	Registers, Saved  [8]uint32
	Flag              uint16
	palette           *NativeFramePaletteState
	failed            error
}

func (s *NativeErrorFrameState) Advance(cb NativeErrorFrameCallbacks) (out NativeFileFrameStep, failure error) {
	if s == nil || cb.Frame == nil || cb.Presentation == nil || cb.Bitmap == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.RAM) {
		return out, fmt.Errorf("native error requester physical backing missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		s.Started = true
		if s.Routine == 0 {
			s.Routine = 0x33b2
		}
		if s.Routine != 0x33b2 && s.Routine != 0x339e {
			return out, fmt.Errorf("native error requester entry %#x invalid", s.Routine)
		}
		if s.Routine == 0x339e {
			s.A[1] = NativeRequesterAddress{Address: cb.CodeBase + 0x91d0, Code: true}
			s.A[2] = NativeRequesterAddress{Address: cb.CodeBase + 0xa950, Code: true}
		}
		s.Registers, s.Saved, s.SavedA = cb.Frame.D, cb.Frame.D, s.A
		s.PC = 0x33b6
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
	// $4eb6 can read a real physical template outside CODE. Retain its
	// relative numeric interface while directing every read to true RAM.
	physicalCode := code
	physicalCode.Read8 = func(at int) (uint8, error) { return cb.RAM.Read8(int(int64(cb.CodeBase) + int64(at))) }
	physicalCode.Read16 = func(at int) (uint16, error) { return cb.RAM.Read16(int(int64(cb.CodeBase) + int64(at))) }
	physicalCode.Read32 = func(at int) (uint32, error) { return cb.RAM.Read32(int(int64(cb.CodeBase) + int64(at))) }
	b := nativeRequesterFrameBacking{Code: physicalCode, Memory: m, CodeBase: cb.CodeBase, Frame: c, Bitmap: cb.Bitmap, Sound: cb.Sound, ReadAbsolute: func(address uint32) (uint8, error) { return cb.RAM.Read8(int(address)) }}
	copyScreen := func() error {
		front, err := m.Read32(0x1a)
		if err != nil {
			return err
		}
		back, err := m.Read32(0x1e)
		if err != nil {
			return err
		}
		source, err := cb.Bitmap(front)
		if err != nil {
			return err
		}
		target, err := cb.Bitmap(back)
		if err != nil {
			return err
		}
		if len(source) < 32000 || len(target) < 32000 {
			return fmt.Errorf("native error copy outside actual screen RAM")
		}
		for at := 0; at < 32000; at += 32 {
			var block [32]byte
			copy(block[:], source[at:at+32])
			copy(target[at:at+32], block[:])
		}
		return nil
	}
	palette := func(source, target int) (bool, error) {
		if s.palette == nil {
			bank := func(at int) (NativeFramePaletteBank, error) {
				p := NativeFramePaletteBank{Address: cb.CodeBase + uint32(at)}
				for i := range p.Words {
					v, err := code.Read16(at + i*2)
					if err != nil {
						return p, err
					}
					p.Words[i] = v
				}
				return p, nil
			}
			a, err := bank(source)
			if err != nil {
				return false, err
			}
			d, err := bank(target)
			if err != nil {
				return false, err
			}
			s.A[2], s.A[3] = NativeRequesterAddress{Address: a.Address, Code: true}, NativeRequesterAddress{Address: d.Address, Code: true}
			s.palette = NewNativeFramePaletteState(a, d, cb.CodeBase)
		}
		done, err := s.palette.Advance(cb.Presentation, c, m)
		if done {
			s.palette = nil
		}
		return done, err
	}
	for transitions := 0; transitions < 16; transitions++ {
		switch s.PC {
		case 0x33b6:
			// Determine the live pointer slots used by the actual definition.
			// A zero v-field does not advance the source parameter pointer.
			parameters := []NativeRequesterAddress{}
			parameter := s.A[2].Address
			for at := s.A[1].Address + 4; ; at++ {
				v, err := cb.RAM.Read8(int(at))
				if err != nil {
					return out, err
				}
				if v == 0 {
					break
				}
				if int8(v) < 0 {
					v -= 0x25
				}
				if v == '{' || v == 'v' {
					p, err := cb.RAM.Read32(int(parameter))
					if err != nil {
						return out, err
					}
					parameters = append(parameters, NativeRequesterAddress{Address: p, Absolute: true})
					if v == '{' || p != 0 {
						parameter += 4
					}
				}
			}
			c.D[3] = 1
			if err := b.compile(s.A[1].Address, parameters); err != nil {
				return out, err
			}
			if err := copyScreen(); err != nil {
				return out, err
			}
			flag, err := m.Read16(0x3b0)
			if err != nil {
				return out, err
			}
			s.Flag = flag
			s.PC = 0x33d6
		case 0x33d6:
			if s.Flag == 0 {
				done, err := palette(0x3361a, 0x33844)
				if err != nil {
					return out, err
				}
				if !done {
					out.Waiting = true
					return out, nil
				}
			}
			s.PC = 0x33ea
		case 0x33ea:
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
			if _, err := b.click(); err != nil {
				return out, err
			}
			dispatch, err := code.Read16(0x341a + int(int16(c.D[0])))
			if err != nil {
				return out, err
			}
			c.Word(0, dispatch)
			s.PC = 0x341a + int(int16(c.D[0]))
			if s.PC == 0x33ea {
				out.Idle = true
				return out, nil
			}
		case 0x33fe:
			if s.Flag == 0 {
				s.PC = 0x3402
			} else {
				s.PC = 0x3414
			}
		case 0x3402:
			done, err := palette(0x33844, 0x3361a)
			if err != nil {
				return out, err
			}
			if !done {
				out.Waiting = true
				return out, nil
			}
			s.PC = 0x3414
		case 0x3414:
			c.D, s.A = s.Saved, s.SavedA
			s.Finished, out.Complete, s.PC = true, true, 0x3418
			return out, nil
		default:
			return out, fmt.Errorf("native error requester source branch %#x unavailable", s.PC)
		}
	}
	return out, fmt.Errorf("native error requester did not reach its poll/wait")
}
