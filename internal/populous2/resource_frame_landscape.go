package populous2

import "fmt"

type NativeLandscapeResourceFrameState struct {
	Started, Finished bool
	PC                int
	Registers         [8]uint32
	Saved             [7]uint32
	Load              NativeResourceFrameState
	failed            error
}

// Advance translates complete$1a32a, including the deliberate repeated cached
// S16 difference load, both original physical patch targets, sprite preparation
// and BLOCK's different mask-last layout. D1-D7 survive its outer MOVEM.
func (s *NativeLandscapeResourceFrameState) Advance(r *NativeResourceFrameRules, cb NativeResourceFrameCallbacks) (out NativeResourceFrameStep, failure error) {
	if s == nil || r == nil || cb.Frame == nil || !winMemoryValid(cb.RAM) {
		return out, fmt.Errorf("native landscape resource frame missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		s.Started, s.PC, s.Registers = true, 0x1a32a, cb.Frame.D
		copy(s.Saved[:], cb.Frame.D[1:])
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
	c, m := cb.Frame, cb.RAM
	cs, bs := int(cb.CodeBase), int(c.AddressBase)
	load := func() (bool, error) {
		step, err := s.Load.Advance(r, cb)
		out.IO, out.Calls = step.IO, append(out.Calls, step.Calls...)
		if err != nil {
			return false, err
		}
		if !step.Complete {
			out.Waiting = true
			return false, nil
		}
		s.Load = NativeResourceFrameState{}
		return true, nil
	}
	for transitions := 0; transitions < 32; transitions++ {
		switch s.PC {
		case 0x1a32a:
			land, err := m.Read16(bs + 0xeb22)
			if err != nil {
				return out, err
			}
			c.Word(0, land+4)
			s.PC = 0x1a338
		case 0x1a338:
			done, err := load()
			if err != nil || !done {
				return out, err
			}
			if uint16(c.D[0]) == 2 {
				s.PC = 0x1a3c2
			} else {
				s.PC = 0x1a346
			}
		case 0x1a346, 0x1a35e:
			next := 0x1a358
			bit := uint16(23)
			if s.PC == 0x1a35e {
				bit, next = 24, 0x1a370
			}
			c.Word(0, bit)
			flags, err := m.Read32(bs + 0x3ac)
			if err != nil {
				return out, err
			}
			c.D[1] = flags &^ (1 << uint(bit))
			if err := m.Write32(bs+0x3ac, c.D[1]); err != nil {
				return out, err
			}
			s.PC = next
		case 0x1a358:
			done, err := load()
			if err != nil || !done {
				return out, err
			}
			s.PC = 0x1a35e
		case 0x1a370:
			done, err := load()
			if err != nil || !done {
				return out, err
			}
			land, err := m.Read16(bs + 0xeb22)
			if err != nil {
				return out, err
			}
			c.Word(0, land+15)
			s.PC = 0x1a380
		case 0x1a380:
			done, err := load()
			if err != nil || !done {
				return out, err
			}
			target, err := m.Read32(cs + 0x1a388)
			if err != nil {
				return out, err
			}
			table, err := m.Read32(cs + 0x1a524)
			if err != nil {
				return out, err
			}
			source, err := m.Read32(cs + 0x1a538)
			if err != nil {
				return out, err
			}
			if err := ApplyNativeResourceFrameDifference(m, table, source, target); err != nil {
				return out, err
			}
			s.PC = 0x1a392
		case 0x1a392:
			done, err := load()
			if err != nil || !done {
				return out, err
			}
			land, err := m.Read16(bs + 0xeb22)
			if err != nil {
				return out, err
			}
			c.Word(0, land+19)
			s.PC = 0x1a3a2
		case 0x1a3a2:
			done, err := load()
			if err != nil || !done {
				return out, err
			}
			target, err := m.Read32(cs + 0x1a3aa)
			if err != nil {
				return out, err
			}
			table, err := m.Read32(cs + 0x1a524)
			if err != nil {
				return out, err
			}
			source, err := m.Read32(cs + 0x1a538)
			if err != nil {
				return out, err
			}
			if err := ApplyNativeResourceFrameDifference(m, table, source, target); err != nil {
				return out, err
			}
			if err := prepareNativeResourceFramePlanes(m, cb.CodeBase+0x21632, uint32(bs+0x3be), c); err != nil {
				return out, err
			}
			s.PC = 0x1a3c2
		case 0x1a3c2:
			land, err := m.Read16(bs + 0xeb22)
			if err != nil {
				return out, err
			}
			c.Word(0, land)
			s.PC = 0x1a3cc
		case 0x1a3cc:
			done, err := load()
			if err != nil || !done {
				return out, err
			}
			if uint16(c.D[0]) != 2 {
				start, err := m.Read32(cs + 0x1a3dc)
				if err != nil {
					return out, err
				}
				end, err := m.Read32(cs + 0x1a3e2)
				if err != nil {
					return out, err
				}
				if err := prepareNativeResourceFrameBlocks(m, start, end, uint32(bs+0x3be), c); err != nil {
					return out, err
				}
			}
			copy(c.D[1:], s.Saved[:])
			s.PC, s.Finished, out.Complete = 0x1a43c, true, true
			return out, nil
		default:
			return out, fmt.Errorf("native landscape resource source branch %#x missing", s.PC)
		}
	}
	return out, fmt.Errorf("native landscape load did not yield or finish")
}

// ApplyNativeResourceFrameDifference executes$1a51e's raw ten-byte records.
// The two bases are real relocated source operands, not rebased asset slices.
func ApplyNativeResourceFrameDifference(m FollowerCleanupMemory, table, source, target uint32) error {
	for {
		offset, err := m.Read32(int(table))
		if err != nil {
			return err
		}
		if offset == 0xffffffff {
			return nil
		}
		destination, err := m.Read32(int(table) + 4)
		if err != nil {
			return err
		}
		count, err := m.Read16(int(table) + 8)
		if err != nil {
			return err
		}
		for i := 0; i <= int(count); i++ {
			value, err := m.Read8(int(source+offset) + i)
			if err != nil {
				return err
			}
			if err := m.Write8(int(target+destination)+i, value); err != nil {
				return err
			}
		}
		table += 10
	}
}

// PrepareNativeResourceFrameBlocks is$1a3da/$1a3f0: four color planes first,
// complemented mask last, with the actual80-byte chunk and BSS scratch writes.
func PrepareNativeResourceFrameBlocks(m FollowerCleanupMemory, start, end, scratch uint32) error {
	return prepareNativeResourceFrameBlocks(m, start, end, scratch, nil)
}

func prepareNativeResourceFrameBlocks(m FollowerCleanupMemory, start, end, scratch uint32, c *NativeFrameRegisterContext) error {
	lookup := start
	for {
		offset, err := m.Read16(int(lookup))
		if err != nil {
			return err
		}
		lookup += 2
		if c != nil {
			c.Word(0, offset)
		}
		if offset != 0 {
			start += uint32(int32(int16(offset)))
			break
		}
	}
	for {
		if c != nil {
			c.D[1] = 7
		}
		for row := 0; row < 8; row++ {
			mask, err := m.Read16(int(start) + row*10)
			if err != nil {
				return err
			}
			if err := m.Write16(int(scratch)+64+row*2, ^mask); err != nil {
				return err
			}
			if c != nil {
				c.Word(0, ^mask)
			}
			for plane := 0; plane < 4; plane++ {
				value, err := m.Read16(int(start) + row*10 + 2 + plane*2)
				if err != nil {
					return err
				}
				if err := m.Write16(int(scratch)+plane*16+row*2, value); err != nil {
					return err
				}
			}
			if c != nil {
				c.Word(1, uint16(c.D[1])-1)
			}
		}
		if c != nil {
			c.D[1] = 7
		}
		for i := 0; i < 40; i++ {
			value, err := m.Read16(int(scratch) + i*2)
			if err != nil {
				return err
			}
			if err := m.Write16(int(start)+i*2, value); err != nil {
				return err
			}
			if c != nil && i%5 == 4 {
				c.Word(1, uint16(c.D[1])-1)
			}
		}
		start += 80
		if int32(end) <= int32(start) {
			return nil
		}
	}
}
