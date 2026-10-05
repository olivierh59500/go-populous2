package populous2

import "fmt"

// NativeStartupWorldFrameStep describes a source routine that has no host
// wait. Negative is meaningful for $cdca's actual terminal CCR, including the
// signed word increment of BSS$dd2 on its successful path.
type NativeStartupWorldFrameStep struct {
	Complete, Negative                              bool
	RandomDraws, Raised, Trees, Boulders, Followers int
}

type startupWorldFrame struct {
	cb      NativeStartupResetFrameCallbacks
	c       *NativeFrameRegisterContext
	a       *[7]NativeRequesterAddress
	m, code nativeTownFrameMemory
	step    NativeStartupWorldFrameStep
	work    int
}

// RunNativeStartupWorldFrame executes the raw world-creation children of
// $10ad8. It uses mutable CODE tables and original four-byte parcels; it does
// not construct a typed World or infer the surviving register values.
func RunNativeStartupWorldFrame(routine int, cb NativeStartupResetFrameCallbacks, a *[7]NativeRequesterAddress) (NativeStartupWorldFrameStep, error) {
	if cb.Frame == nil || a == nil || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.Code) {
		return NativeStartupWorldFrameStep{}, fmt.Errorf("native startup world backing/frame missing")
	}
	s := startupWorldFrame{cb: cb, c: cb.Frame, a: a, m: nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}, code: nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Code}}}
	var err error
	switch routine {
	case 0xcd22:
		err = s.hills()
	case 0xcd76:
		err = s.hill()
	case 0xcdca:
		s.step.Negative, err = s.raise()
	case 0xd9d8:
		err = s.scenery(true)
	case 0xda0a:
		err = s.cluster(true)
	case 0xdbd4:
		err = s.scenery(false)
	case 0xdc04:
		err = s.cluster(false)
	case 0xdc6c:
		err = s.boulder()
	case 0x10b38:
		err = s.people()
	case 0x10c7e:
		err = s.place()
	case 0x10cbe:
		err = s.allocate()
	case 0x14048:
		err = s.marker()
	case 0x13fb6:
		s.leader()
	case 0x125a0:
		s.insert()
	default:
		err = fmt.Errorf("unsupported native startup world routine %x", routine)
	}
	if err == nil {
		err = s.m.err
	}
	if err == nil {
		err = s.code.err
	}
	s.step.Complete = err == nil
	return s.step, err
}

func (s *startupWorldFrame) bss(reg, at int) {
	(*s.a)[reg] = NativeRequesterAddress{Address: s.c.AddressBase + uint32(at)}
}
func (s *startupWorldFrame) cod(reg, at int) {
	(*s.a)[reg] = NativeRequesterAddress{Address: s.cb.CodeBase + uint32(at), Code: true}
}
func (s *startupWorldFrame) at(reg int) int { return int((*s.a)[reg].Address - s.c.AddressBase) }
func (s *startupWorldFrame) limit() error {
	s.work++
	if s.work > 2000000 {
		return fmt.Errorf("native startup world work exceeded bounded source window")
	}
	return nil
}
func (s *startupWorldFrame) random() {
	seed := s.m.long(0xeb28)
	if seed == 0 {
		seed = 0xbc614e
	}
	seed *= 0xbb40e62d
	s.m.putLong(0xeb28, seed)
	s.c.D[0] = seed >> 8 & 0x7fff
	s.step.RandomDraws++
}
func (s *startupWorldFrame) divide(reg int, divisor uint16) error {
	if divisor == 0 {
		return fmt.Errorf("native startup DIVU by zero")
	}
	v := s.c.D[reg]
	if v/uint32(divisor) <= 0xffff {
		s.c.D[reg] = v%uint32(divisor)<<16 | v/uint32(divisor)
	}
	return nil // DIVU overflow leaves the original destination unchanged.
}
func (s *startupWorldFrame) swap(reg int) { s.c.D[reg] = s.c.D[reg]<<16 | s.c.D[reg]>>16 }

func (s *startupWorldFrame) hills() error {
	c := s.c
	s.m.putLong(0xeb24, s.m.long(0xeb28))
	s.bss(0, 0xf44)
	c.D[1] = 0
	c.Word(0, 0xfff)
	for i := 0; i < 4096; i++ {
		s.m.putLong(s.at(0), 0)
		(*s.a)[0].Address += 4
		c.Word(0, uint16(c.D[0])-1)
	}
	s.cod(0, 0x2072a)
	for {
		if err := s.limit(); err != nil {
			return err
		}
		at := int((*s.a)[0].Address - s.cb.CodeBase)
		for i := 0; i < 4; i++ {
			c.D[i+2] = uint32(int32(int16(s.code.word(at + i*2))))
		}
		(*s.a)[0].Address += 8
		if uint16(c.D[2]) == 0xff9d {
			return nil
		}
		s.random()
		c.D[6], c.D[1] = c.D[0], c.D[0]
		if err := s.divide(1, uint16(c.D[2])); err != nil {
			return err
		}
		s.swap(1)
		c.Word(1, uint16(c.D[1])+uint16(c.D[3]))
		s.random()
		c.D[7] = c.D[0]
		if err := s.divide(0, uint16(c.D[4])); err != nil {
			return err
		}
		s.swap(0)
		c.Word(0, uint16(c.D[0])+uint16(c.D[5]))
		if err := s.hill(); err != nil {
			return err
		}
	}
}
func (s *startupWorldFrame) hill() error {
	d, a := s.c.D, *s.a
	defer func() { s.c.D, *s.a = d, a }()
	c := s.c
	for {
		if err := s.limit(); err != nil {
			return err
		}
		for _, v := range [][2]int{{6, 0}, {7, 1}} {
			c.D[v[0]] = uint32(uint16(c.D[v[0]])) * 0x24a1
			c.Word(v[0], uint16(c.D[v[0]])+0x24df)
			c.D[v[0]] &^= 1 << 15
			c.D[5] = uint32(int32(int16(c.D[v[0]])))
			if err := s.divide(5, 7); err != nil {
				return err
			}
			s.swap(5)
			c.Word(5, uint16(c.D[5])-3)
			c.Word(v[1], uint16(c.D[v[1]])+uint16(c.D[5]))
		}
		d6, d7 := c.D[6], c.D[7]
		negative, err := s.raise()
		c.D[6], c.D[7] = d6, d7
		if err != nil || negative || int16(c.D[4]) >= 8 {
			return err
		}
	}
}
func (s *startupWorldFrame) height() int {
	c := s.c
	x, y := int(int16(c.D[0])), int(int16(c.D[1]))
	if x < 0 || y < 0 || x > 64 || y > 64 {
		c.D[2] = 0xffffffff
		return -1
	}
	bit := uint8(0)
	if x == 64 {
		x--
		bit = 1
		if y == 64 {
			y--
			bit = 2
		}
	} else if y == 64 {
		y--
		bit = 3
	}
	at := 0xf44 + (x+y*64)*4
	c.D[2] = uint32(s.m.byte(at) & 7)
	if s.code.byte(0x33512+int(s.m.byte(at+1)))&(1<<bit) != 0 {
		c.Word(2, uint16(c.D[2])+1)
	}
	return int(uint16(c.D[2]))
}

// raise is genuine $cdca, not $d81e's later redraw wrapper. The successful
// CCR comes from the native dirty-counter increment, not from D4's height.
func (s *startupWorldFrame) raise() (bool, error) {
	if err := s.limit(); err != nil {
		return false, err
	}
	c := s.c
	d, a0 := [4]uint32{c.D[0], c.D[1], c.D[2], c.D[3]}, (*s.a)[0]
	defer func() { copy(c.D[:4], d[:]); (*s.a)[0] = a0 }()
	s.m.putWord(0xf2e, s.m.word(0xf2e)+1)
	s.cod(6, 0x33512)
	h := s.height()
	if h < 0 || h == 8 {
		c.D[4] = 0xffffffff
		return true, s.m.err
	}
	c.Word(2, uint16(c.D[2])+1)
	c.Word(3, uint16(c.D[2]))
	for _, delta := range [][2]int{{0, -1}, {1, 0}, {0, 1}, {0, 1}, {-1, 0}, {-1, 0}, {0, -1}, {0, -1}} {
		c.Word(0, uint16(c.D[0])+uint16(delta[0]))
		c.Word(1, uint16(c.D[1])+uint16(delta[1]))
		if s.height() >= 0 {
			c.Word(2, uint16(c.D[2])-uint16(c.D[3])+1)
			if int16(c.D[2]) < 0 {
				if _, err := s.raise(); err != nil {
					return false, err
				}
			}
		}
	}
	s.bss(0, 0xf44)
	c.Word(2, uint16(c.D[1])<<6)
	c.Word(2, uint16(c.D[2])+uint16(c.D[0]))
	c.Word(2, uint16(c.D[2])*4)
	(*s.a)[0].Address += uint32(int32(int16(c.D[2])))
	for i, bit := range []uint8{4, 8, 2, 1} {
		if i == 1 || i == 3 {
			c.Word(0, uint16(c.D[0])+1)
			(*s.a)[0].Address += 4
		}
		if i == 2 {
			c.Word(0, uint16(c.D[0])-1)
			c.Word(1, uint16(c.D[1])+1)
			(*s.a)[0].Address += 252
		}
		x, y := int16(c.D[0]), int16(c.D[1])
		if x < 0 || y < 0 || x >= 64 || y >= 64 {
			continue
		}
		if int16(c.D[6]) >= y {
			c.Word(6, uint16(y))
		}
		if int16(c.D[7]) <= y {
			c.Word(7, uint16(y))
		}
		s.swap(6)
		s.swap(7)
		if int16(c.D[6]) >= x {
			c.Word(6, uint16(x))
		}
		if int16(c.D[7]) <= x {
			c.Word(7, uint16(x))
		}
		s.swap(6)
		s.swap(7)
		c.D[5] = uint32(s.code.byte(0x33512 + int(s.m.byte(s.at(0)+1))))
		if uint8(c.D[5])&bit == 0 {
			c.Byte(5, uint8(c.D[5])+bit)
		} else {
			s.m.putByte(s.at(0), s.m.byte(s.at(0))+1)
			c.Word(5, uint16(c.D[5])&(0xf0|uint16(bit)))
		}
		s.m.putByte(s.at(0)+1, uint8(c.D[5]))
		s.m.putByte(s.at(0), s.m.byte(s.at(0))&7)
	}
	c.Word(4, uint16(c.D[3]))
	count := s.m.word(0xdd2) + 1
	s.m.putWord(0xdd2, count)
	s.step.Raised++
	return int16(count) < 0, s.m.err
}

func (s *startupWorldFrame) scenery(tree bool) error {
	c := s.c
	s.random()
	c.Word(1, uint16(c.D[0]))
	c.D[1] = uint32(int32(int16(c.D[1])))
	count := 0x20f40
	if !tree {
		count = 0xddd0
	}
	c.Word(0, s.code.word(count))
	if err := s.divide(1, uint16(c.D[0])); err != nil {
		return err
	}
	s.swap(1)
	c.Word(0, uint16(c.D[0])>>1)
	c.Word(1, uint16(c.D[1])+uint16(c.D[0]))
	for {
		if err := s.limit(); err != nil {
			return err
		}
		s.random()
		c.D[3] = c.D[0]
		c.Word(3, uint16(c.D[3])&0x3f3f)
		if tree {
			c.D[0] = 0
		}
		if err := s.cluster(tree); err != nil {
			return err
		}
		c.Word(1, uint16(c.D[1])-1)
		if uint16(c.D[1]) == 0xffff {
			return nil
		}
	}
}
func (s *startupWorldFrame) cluster(tree bool) error {
	c := s.c
	d, a := c.D, *s.a
	defer func() { result := c.D[0]; c.D, *s.a = d, a; c.D[0] = result }()
	c.D[6] = 0
	s.random()
	c.D[4] = c.D[0]
	if err := s.divide(4, 8); err != nil {
		return err
	}
	s.swap(4)
	c.Word(4, uint16(c.D[4])&0xfe)
	c.D[5] = c.D[0]
	divisor, offsets := 0xddce, 0xddda
	if tree {
		divisor, offsets = 0x20f3e, 0x20f4a
	}
	if err := s.divide(5, s.code.word(divisor)); err != nil {
		return err
	}
	s.swap(5)
	if tree && uint16(c.D[2]) != 0 && int16(c.D[2]) <= 2 {
		s.bss(1, 0xe76a)
		c.D[2] = uint32(uint16(c.D[2])) * 314
		(*s.a)[1].Address += uint32(int32(int16(c.D[2])))
		c.D[2] = uint32(s.m.byte(s.at(1) + 0x53))
		c.Word(2, uint16(c.D[2])>>4)
		c.Word(5, uint16(c.D[5])+uint16(c.D[2]))
	}
	s.bss(2, 0xf44)
	s.cod(4, 0x33312)
	s.cod(1, offsets)
	if tree {
		s.cod(5, 0x20f42)
	}
	for {
		if err := s.limit(); err != nil {
			return err
		}
		s.random()
		if err := s.divide(0, 90); err != nil {
			return err
		}
		s.swap(0)
		c.Word(0, uint16(c.D[0])&0xfe)
		c.Word(2, s.code.word(offsets+int(int16(c.D[0]))))
		c.Word(2, uint16(c.D[2])+uint16(c.D[3]))
		c.Word(0, uint16(c.D[2])&0xc0c0)
		if uint16(c.D[0]) == 0 {
			c.Byte(2, uint8(c.D[2])*4)
			if tree {
				full, err := s.placeScenery(true)
				if err != nil {
					return err
				}
				if full {
					break
				}
			} else if err := s.boulder(); err != nil {
				return err
			}
		}
		c.Word(5, uint16(c.D[5])-1)
		if uint16(c.D[5]) == 0xffff {
			break
		}
	}
	if tree {
		c.D[0] = c.D[6]
	}
	return nil
}
func (s *startupWorldFrame) boulder() error {
	a4 := (*s.a)[4]
	defer func() { (*s.a)[4] = a4 }()
	s.cod(4, 0x33312)
	s.cod(5, 0xddd2)
	s.bss(2, 0xf44)
	_, err := s.placeScenery(false)
	return err
}
func (s *startupWorldFrame) placeScenery(tree bool) (bool, error) {
	c := s.c
	grid := s.at(2) + int(int16(c.D[2]))
	c.Byte(0, s.m.byte(grid+1))
	if uint8(c.D[0]) == 0 {
		return false, nil
	}
	c.Word(0, uint16(int16(int8(c.D[0]))))
	c.Word(0, uint16(c.D[0])*2)
	c.Word(0, s.code.word(0x33312+int(int16(c.D[0]))))
	c.Word(0, uint16(c.D[0])&0x40)
	if uint16(c.D[0]) != 0 || s.m.word(grid+2) != 0 {
		return false, nil
	}
	s.bss(3, 0x6bd0)
	for s.m.byte(s.at(3)+12) != 0 {
		(*s.a)[3].Address += 14
		if s.at(3) >= 0x76c0 {
			return true, nil
		}
	}
	at := s.at(3)
	kind, age, art := uint8(0x18), 0xddcd, 0xddd2
	if tree {
		kind, age, art = 0x16, 0x20f3d, 0x20f42
	}
	s.m.putByte(at+12, 3)
	s.m.putByte(at, kind)
	s.m.putByte(at+1, s.code.byte(age))
	s.m.putWord(at+10, s.code.word(art+int(int16(c.D[4]))))
	s.random()
	if err := s.divide(0, 90); err != nil {
		return false, err
	}
	s.swap(0)
	if uint16(c.D[0]) == 0 {
		c.Word(0, uint16(c.D[0])&0xfe)
		s.m.putWord(at+10, s.code.word(art+int(int16(c.D[0]))))
	}
	c.Byte(2, uint8(c.D[2])>>2)
	s.m.putByte(at+6, uint8(c.D[2]))
	s.m.putByte(at+7, 128)
	c.Byte(2, 128)
	s.m.putWord(at+8, uint16(c.D[2]))
	c.Word(6, uint16(c.D[6])+1)
	d, a := c.D, *s.a
	s.insert()
	result := c.D[0]
	c.D, *s.a = d, a
	c.D[0] = result
	if tree {
		s.step.Trees++
	} else {
		s.step.Boulders++
	}
	return false, s.m.err
}

// insert executes $125a0, whose A3 changes to the record-base label when a
// preexisting head must receive its previous link. Callers save it exactly
// where the original MOVEM or explicit stack word does so.
func (s *startupWorldFrame) insert() {
	c := s.c
	at := s.at(3)
	s.m.putLong(at+2, 0)
	c.Word(0, s.m.word(at+8))
	c.Byte(0, s.m.byte(at+6))
	c.Byte(0, uint8(c.D[0])*4)
	s.bss(2, 0xf44+int(int16(c.D[0])))
	c.D[0] = uint32(at - 0x76c0)
	c.Word(1, s.m.word(s.at(2)+2))
	if uint16(c.D[1]) != 0 {
		s.m.putWord(at+2, uint16(c.D[1]))
		s.bss(3, 0x76c0)
		s.m.putWord(0x76c0+int(int16(c.D[1]))+4, uint16(c.D[0]))
	}
	s.m.putWord(s.at(2)+2, uint16(c.D[0]))
}
func (s *startupWorldFrame) marker() error {
	d, a := s.c.D, *s.a
	defer func() { s.c.D, *s.a = d, a }()
	c := s.c
	c.Word(2, uint16(int16(int8(c.D[2]))))
	c.Word(7, uint16(c.D[2]))
	c.D[2] = uint32(uint16(c.D[2])) * 314
	s.bss(1, 0xe76a+int(int16(c.D[2])))
	c.Word(2, uint16(c.D[7]))
	c.D[2] = uint32(uint16(c.D[2])) * 14
	s.bss(0, 0xe740+int(int16(c.D[2])))
	c.D[3] = uint32(s.at(0) - 0x76c0)
	s.m.putWord(s.at(1)+10, uint16(c.D[3]))
	s.m.putByte(s.at(0)+6, uint8(c.D[0]))
	s.m.putByte(s.at(0)+8, uint8(c.D[1]))
	s.m.putByte(s.at(0)+7, 128)
	s.m.putByte(s.at(0)+9, 128)
	s.m.putByte(s.at(0), 0x14)
	s.m.putByte(s.at(0)+12, uint8(c.D[7]))
	c.Word(7, uint16(c.D[7])*2)
	s.m.putWord(s.at(0)+10, s.code.word(0x140a8+int(int16(c.D[7]))))
	(*s.a)[3] = (*s.a)[0]
	s.insert()
	return s.m.err
}
func (s *startupWorldFrame) leader() {
	d0, a1 := s.c.D[0], (*s.a)[1]
	defer func() { s.c.D[0], (*s.a)[1] = d0, a1 }()
	c := s.c
	c.Byte(0, s.m.byte(s.at(3)+12))
	c.Word(0, uint16(int16(int8(c.D[0]))))
	c.D[0] = uint32(uint16(c.D[0])) * 314
	s.bss(1, 0xe76a+int(int16(c.D[0])))
	c.D[0] = uint32(s.at(3) - 0x76c0)
	s.m.putWord(s.at(1)+8, uint16(c.D[0]))
	s.m.putByte(s.at(3)+13, s.m.byte(s.at(3)+13)|1)
}
func (s *startupWorldFrame) allocate() error {
	d, a := s.c.D, *s.a
	defer func() { a3 := (*s.a)[3]; s.c.D, *s.a = d, a; (*s.a)[3] = a3 }()
	c := s.c
	s.bss(3, 0x76f4)
	for s.m.byte(s.at(3)+12) != 0 {
		(*s.a)[3].Address += 52
		if s.at(3) >= 0xc800 {
			c.D[0] = 0
			return s.m.err
		}
	}
	c.D[1] = c.D[2]
	c.D[1] = uint32(uint16(c.D[1])) * 314
	s.bss(1, 0xe76a+int(int16(c.D[1])))
	at := s.at(3)
	s.m.putByte(at, 2)
	s.m.putByte(at+12, uint8(c.D[2]))
	c.D[1] = uint32(at - 0x76c0)
	if err := s.divide(1, 52); err != nil {
		return err
	}
	c.Word(1, uint16(c.D[1])&14)
	s.m.putWord(at+50, uint16(c.D[1]))
	c.Byte(1, uint8(c.D[0]))
	c.Byte(1, uint8(c.D[1])>>2)
	s.m.putByte(at+6, uint8(c.D[1]))
	s.m.putByte(at+7, 128)
	c.Word(0, uint16(c.D[0])&0xff00)
	c.Word(0, uint16(c.D[0])+128)
	s.m.putWord(at+10, 0)
	s.m.putByte(at+13, 0)
	s.m.putWord(at+8, uint16(c.D[0]))
	s.m.putLong(at+14, 0)
	s.m.putByte(at+24, 2)
	s.m.putByte(at+18, s.m.byte(s.at(1)+0x5f))
	s.m.putByte(at+25, s.m.byte(s.at(1)+0x61))
	s.m.putWord(at+28, s.m.word(s.at(1)+0x5c))
	s.m.putByte(at+22, 2)
	s.m.putByte(at+23, 2)
	a3 := (*s.a)[3]
	s.insert()
	(*s.a)[3] = a3
	s.step.Followers++
	return s.m.err
}
func (s *startupWorldFrame) place() error {
	c := s.c
	c.D[0] = uint32(s.at(0) - 0xf44)
	if err := s.allocate(); err != nil {
		return err
	}
	if uint16(c.D[1]) != 0 {
		return nil
	}
	if uint16(c.D[2]) == s.m.word(0xeb42) {
		c.Byte(0, s.m.byte(s.at(3)+6)-4)
		s.m.putByte(0x5f45, uint8(c.D[0]))
		c.Byte(0, s.m.byte(s.at(3)+8)-4)
		s.m.putByte(0x5f47, uint8(c.D[0]))
	}
	if int16(s.m.word(s.at(1)+0x6e)) < 0 {
		s.leader()
	}
	return s.m.err
}
func (s *startupWorldFrame) people() error {
	c := s.c
	for side := 0; side < 2; side++ {
		start, finish, stride := 0xf44, 0x4f44, 4
		if side == 1 {
			start, finish, stride = 0x4f44, 0xf44, -4
		}
		s.bss(0, start)
		s.bss(1, 0xe8a4+side*314)
		if side == 0 {
			s.cod(2, 0x33312)
		}
		god := s.at(1)
		c.Word(0, 32)
		c.Word(1, 32)
		if int16(s.m.word(god+0x6e)) >= 0 {
			c.Byte(0, s.m.byte(god+0x6e))
			c.Byte(1, s.m.byte(god+0x6f))
		}
		c.Word(2, uint16(side+1))
		if err := s.marker(); err != nil {
			return err
		}
		s.m.putWord(god+0x16, s.m.word(god+0x64))
		s.m.putWord(god+2, s.m.word(god+0x62))
		s.m.putWord(0xeb2c+side*2, s.m.word(god+0x66))
		if s.m.word(god+0x5a) == 0 {
			continue
		}
		c.D[1] = 0
		for pass := 0; pass < 2; pass++ {
			if pass == 1 {
				s.bss(0, start)
			}
			for {
				(*s.a)[0].Address += uint32(int32(stride))
				at := s.at(0)
				if side == 0 && at >= finish || side == 1 && at < finish {
					break
				}
				if err := s.limit(); err != nil {
					return err
				}
				if pass == 0 {
					c.D[0] = uint32(s.m.byte(at + 1))
					c.Word(0, uint16(c.D[0])*2)
					if s.code.byte(0x33313+int(int16(c.D[0])))&1 == 0 {
						continue
					}
				} else if s.m.byte(at+1) == 0 {
					continue
				}
				if err := s.place(); err != nil {
					return err
				}
				c.Word(1, uint16(c.D[1])+1)
				if uint16(c.D[1]) == s.m.word(god+0x5a) {
					break
				}
			}
			if uint16(c.D[1]) == s.m.word(god+0x5a) {
				break
			}
		}
	}
	return s.m.err
}
