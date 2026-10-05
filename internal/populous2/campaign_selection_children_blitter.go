package populous2

import "fmt"

// NativeCampaignBlitterState is the actual retained custom-chip register
// window. BFAC deliberately inherits controls and masks from earlier draws.
type NativeCampaignBlitterState struct{ Words [128]uint16 }

func (s *NativeCampaignBlitterState) word(at uint32) uint16 { return s.Words[(at-0xdff000)/2] }
func (s *NativeCampaignBlitterState) long(at uint32) uint32 {
	return uint32(s.word(at))<<16 | uint32(s.word(at+2))
}
func (s *NativeCampaignBlitterState) Write16(at uint32, v uint16, ram FollowerCleanupMemory) error {
	if s == nil || at < 0xdff000 || at > 0xdff0fe || at&1 != 0 {
		return fmt.Errorf("native campaign hardware word%x unavailable", at)
	}
	s.Words[(at-0xdff000)/2] = v
	if at == 0xdff058 {
		return s.blit(v, ram)
	}
	return nil
}
func (s *NativeCampaignBlitterState) Write32(at, v uint32, ram FollowerCleanupMemory) error {
	if s == nil || at < 0xdff000 || at > 0xdff0fc || at&1 != 0 {
		return fmt.Errorf("native campaign hardware long%x unavailable", at)
	}
	if e := s.Write16(at, uint16(v>>16), ram); e != nil {
		return e
	}
	return s.Write16(at+2, uint16(v), ram)
}
func (s *NativeCampaignBlitterState) blit(size uint16, ram FollowerCleanupMemory) error {
	con0, con1 := s.word(0xdff040), s.word(0xdff042)
	if con1&0xfff != 0 {
		return fmt.Errorf("native campaign DMA control1%x requires its actual body", con1)
	}
	width, height := int(size&63), int(size>>6)
	if width == 0 {
		width = 64
	}
	if height == 0 {
		height = 1024
	}
	if con0&0x100 == 0 {
		return fmt.Errorf("native campaign DMA destination disabled")
	}
	ptr := [4]uint32{s.long(0xdff050), s.long(0xdff04c), s.long(0xdff048), s.long(0xdff054)}
	mods := [4]int16{int16(s.word(0xdff064)), int16(s.word(0xdff062)), int16(s.word(0xdff060)), int16(s.word(0xdff066))}
	first, last := s.word(0xdff044), s.word(0xdff046)
	ashift, bshift := uint(con0>>12), uint(con1>>12)
	previousA, previousB := uint16(0), uint16(0)
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			values := [3]uint16{s.word(0xdff074), s.word(0xdff072), s.word(0xdff070)}
			for i := 0; i < 3; i++ {
				if con0&(0x800>>uint(i)) != 0 {
					v, e := ram.Read16(int(ptr[i]))
					if e != nil {
						return e
					}
					values[i] = v
				}
			}
			av, bv, cv := values[0], values[1], values[2]
			if col == 0 {
				av &= first
			}
			if col == width-1 {
				av &= last
			}
			shiftA := uint16((uint32(previousA)<<16 | uint32(av)) >> ashift)
			shiftB := uint16((uint32(previousB)<<16 | uint32(bv)) >> bshift)
			previousA, previousB = av, bv
			out := uint16(0)
			for term := 0; term < 8; term++ {
				if con0&(1<<uint(term)) == 0 {
					continue
				}
				aa, bb, cc := shiftA, shiftB, cv
				if term&4 == 0 {
					aa = ^aa
				}
				if term&2 == 0 {
					bb = ^bb
				}
				if term&1 == 0 {
					cc = ^cc
				}
				out |= aa & bb & cc
			}
			if e := ram.Write16(int(ptr[3]), out); e != nil {
				return e
			}
			for i := range ptr {
				if con0&(0x800>>uint(i)) != 0 {
					ptr[i] += 2
				}
			}
		}
		for i := range ptr {
			if con0&(0x800>>uint(i)) != 0 {
				ptr[i] = uint32(int64(ptr[i]) + int64(mods[i]))
			}
		}
	}
	for i, at := range []uint32{0xdff050, 0xdff04c, 0xdff048, 0xdff054} {
		if con0&(0x800>>uint(i)) != 0 {
			s.Words[(at-0xdff000)/2] = uint16(ptr[i] >> 16)
			s.Words[(at+2-0xdff000)/2] = uint16(ptr[i])
		}
	}
	return nil
}
