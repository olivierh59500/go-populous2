package populous2

import "fmt"

// resultDecimal is original10024; D0-D3 survive its MOVEM, and the physical
// field pointer advances past the terminating byte.1001A performs EXT.L first.
func resultDecimal(code FollowerCleanupMemory, c *NativeFrameRegisterContext, target int, signedWord bool) (int, error) {
	if signedWord {
		c.RestoreWord(0, uint16(c.D[0]))
	}
	value := c.D[0]
	if value == 0 {
		if err := code.Write8(target, '0'); err != nil {
			return target, err
		}
		target++
	} else {
		started := false
		for divisorAt := 0x1006a; ; divisorAt += 4 {
			divisor, err := code.Read32(divisorAt)
			if err != nil {
				return target, err
			}
			if divisor == 0 {
				break
			}
			digit := byte('/')
			for {
				digit++
				value -= divisor
				if int32(value) < 0 {
					break
				}
			}
			value += divisor
			if started || digit != '0' {
				started = true
				if err := code.Write8(target, digit); err != nil {
					return target, err
				}
				target++
			}
		}
	}
	if err := code.Write8(target, 0); err != nil {
		return target, err
	}
	return target + 1, nil
}

func resultDivideSigned(c *NativeFrameRegisterContext, reg int, divisor int32) {
	value := int32(c.D[reg])
	q, r := value/divisor, value%divisor
	if q >= -32768 && q <= 32767 {
		c.D[reg] = uint32(uint16(r))<<16 | uint32(uint16(q))
	}
}

func (s *NativeRuntimeResultState) prepareRequester(h *NativeRuntimeHost, c *NativeFrameRegisterContext) error {
	code, m := h.Memory.Code, h.Memory.BSS
	profile, err := m.Read16(0xeb42)
	if err != nil {
		return err
	}
	local, other := 0xe8a4, 0xe9de
	if profile != 1 {
		local, other = other, local
	}
	s.A[1] = NativeRequesterAddress{Address: h.Memory.CodeBase + 0x3ad4, Code: true}
	s.A[3], s.A[4] = NativeRequesterAddress{Address: h.Memory.BSSBase + uint32(local), Absolute: true}, NativeRequesterAddress{Address: h.Memory.BSSBase + uint32(other), Absolute: true}
	c.D[3], c.D[4] = 0, 0
	eliminated, err := code.Read16(0x3b62)
	if err != nil {
		return err
	}
	for i, at := range []int{local, other} {
		metric, err := m.Read16(at + 0x44)
		if err != nil {
			return err
		}
		c.Word(0, metric)
		if i == 0 {
			c.D[5] = 0
			c.Word(5, metric)
		}
		resultDivideSigned(c, 0, 4)
		if eliminated == uint16(i+1) {
			c.Word(0, uint16(c.D[0])+50)
		}
		if int16(c.D[0]) <= 0 {
			c.D[0] = 1
		} else if int16(c.D[0]) > 99 {
			c.D[0] = 99
		}
		if err := m.Write16(at+0x44, uint16(c.D[0])); err != nil {
			return err
		}
	}
	parameter := 0x3ad4
	format := func(signed bool) (int, error) {
		pointer, err := code.Read32(parameter)
		if err != nil {
			return 0, err
		}
		parameter += 4
		s.A[1].Address = h.Memory.CodeBase + uint32(parameter)
		at := int(int64(pointer) - int64(h.Memory.CodeBase))
		end, err := resultDecimal(code, c, at, signed)
		s.A[0] = NativeRequesterAddress{Address: h.Memory.CodeBase + uint32(end), Code: true}
		return end, err
	}
	c.D[0], err = m.Read32(0xf40)
	if err != nil {
		return err
	}
	if err := frameDivide(c, 0, 50); err != nil {
		return err
	}
	c.RestoreWord(0, uint16(c.D[0]))
	end, err := format(false)
	if err != nil {
		return err
	}
	s.A[0].Address--
	s.A[2] = NativeRequesterAddress{Address: h.Memory.CodeBase + 0xa96c, Code: true}
	for i := 0; i < 4096; i++ {
		v, err := code.Read8(0xa96c + i)
		if err != nil {
			return err
		}
		if err := code.Write8(end-1+i, v); err != nil {
			return err
		}
		s.A[0].Address++
		s.A[2].Address++
		if v == 0 {
			break
		}
		if i == 4095 {
			return fmt.Errorf("native day suffix unterminated")
		}
	}
	for _, at := range []int{local + 0x3c, other + 0x3c, local + 0x40, other + 0x40} {
		c.D[0], err = m.Read32(at)
		if err != nil {
			return err
		}
		if at == local+0x3c {
			c.D[2] = c.D[0]
		}
		if _, err := format(false); err != nil {
			return err
		}
	}
	localWins, err := m.Read16(local + 0x48)
	if err != nil {
		return err
	}
	c.Word(0, localWins)
	c.Word(1, localWins)
	if _, err := format(true); err != nil {
		return err
	}
	otherWins, err := m.Read16(other + 0x48)
	if err != nil {
		return err
	}
	c.Word(0, otherWins)
	c.Word(1, uint16(c.D[1])-otherWins)
	if int16(c.D[1]) <= 0 {
		c.Word(1, 1)
	}
	if _, err := format(true); err != nil {
		return err
	}
	for i, at := range []int{local + 0x46, other + 0x46} {
		value, err := m.Read16(at)
		if err != nil {
			return err
		}
		c.Word(0, value)
		if i == 0 {
			c.Word(4, value)
		}
		if _, err := format(true); err != nil {
			return err
		}
	}
	c.D[2] = 5000
	clock, err := m.Read32(0xf40)
	if err != nil {
		return err
	}
	c.D[2] += clock
	c.D[0], c.D[1] = 0, 0
	c.Word(0, localWins-otherWins)
	c.D[1], err = m.Read32(local + 0x48)
	if err != nil {
		return err
	}
	c.D[0] *= c.D[1]
	c.D[1], err = m.Read32(other + 0x48)
	if err != nil {
		return err
	}
	c.Word(0, uint16(c.D[0])+1)
	if err := frameDivide(c, 0, uint16(c.D[1])); err != nil {
		return err
	}
	c.Word(3, uint16(c.D[0]))
	c.D[3] &= 0xffff
	uses, err := m.Read16(local + 0x138)
	if err != nil {
		return err
	}
	c.Word(4, uses)
	c.D[4] = uint32(uint16(c.D[4])) * 150
	copy(s.Saved[:], c.D[2:6])
	c.Word(2, uint16(c.D[2])+uint16(c.D[3]))
	c.Word(2, uint16(c.D[2])+uint16(c.D[4]))
	s.Score = uint16(c.D[2])
	if err := m.Write16(0xdd0, s.Score); err != nil {
		return err
	}
	c.D[0] = uint32(uint16(c.D[2]))
	if _, err := format(false); err != nil {
		return err
	}
	s.A[1], s.A[2] = NativeRequesterAddress{Address: h.Memory.CodeBase + 0x7db2, Code: true}, NativeRequesterAddress{Address: h.Memory.CodeBase + 0x3ad0, Code: true}
	c.D[3] = 1
	b := nativeFileBrowserBacking(NativeFileFrameCallbacks{Code: code, Memory: m, CodeBase: h.Memory.CodeBase, Frame: c, Bitmap: h.Bitmap, ReadAbsolute: func(at uint32) (byte, error) { return h.Memory.RAM.Read8(int(at)) }})
	return nativeFileBrowserCompile(b, &s.A, h.Memory.CodeBase+0x7db2, h.Memory.CodeBase+0x3ad0)
}

func (s *NativeRuntimeResultState) progress(h *NativeRuntimeHost, c *NativeFrameRegisterContext) error {
	m, code := h.Memory.BSS, h.Memory.Code
	profile, err := m.Read16(0xeb42)
	if err != nil {
		return err
	}
	c.Word(0, profile)
	c.D[0] = uint32(uint16(c.D[0])) * 314
	deity := 0xe76a + int(int16(c.D[0]))
	s.A[1] = NativeRequesterAddress{Address: h.Memory.BSSBase + uint32(deity), Absolute: true}
	c.D[0] = 0
	score, err := m.Read16(0xdd0)
	if err != nil {
		return err
	}
	c.Word(0, score)
	if err := frameDivide(c, 0, 13007); err != nil {
		return err
	}
	mode, err := m.Read16(0xeb44)
	if err != nil {
		return err
	}
	if mode != 2 {
		s.PC = 0x3ab0
		return nil
	}
	if int16(c.D[0]) > 5 {
		c.Word(0, 5)
	}
	bolts, err := m.Read16(deity + 0x58)
	if err != nil {
		return err
	}
	if err := m.Write16(deity+0x58, bolts+uint16(c.D[0])); err != nil {
		return err
	}
	c.D[2] = uint32(score)
	if err := frameDivide(c, 2, 6000); err != nil {
		return err
	}
	c.Word(2, uint16(c.D[2])+1)
	if int16(c.D[2]) > 6 {
		c.Word(2, 6)
	}
	c.Word(0, profile)
	eliminated, err := code.Read16(0x3b62)
	if err != nil {
		return err
	}
	world, err := m.Read16(0xeb46)
	if err != nil {
		return err
	}
	if uint16(c.D[0]) == eliminated {
		if world != 999 {
			if err := m.Write16(0xeb46, world+1); err != nil {
				return err
			}
		}
		s.PC = 0x3ab0
		return nil
	}
	c.Word(2, uint16(c.D[2])+world)
	if int16(c.D[2]) > 999 && world != 999 {
		c.Word(2, 999)
	}
	if err := m.Write16(0xeb46, uint16(c.D[2])); err != nil {
		return err
	}
	s.PC = 0x3aa8
	return nil
}
