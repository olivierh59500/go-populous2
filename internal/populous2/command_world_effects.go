package populous2

import "fmt"

func (w *World) commandBasaltCreation(call NativeCommandCall) (bool, error) {
	c, m := call.Context, w.nativeCleanupMemory()
	x, y := uint8(c.D[0]), uint8(c.D[1])
	origin := uint16(y)<<8 | uint16(x)
	c.D[0] = 0
	if origin&0xc0c0 != 0 {
		return true, nil
	}
	tile, e := m.Read8(tsunamiGrid(origin) + 1)
	if e != nil {
		return false, e
	}
	if w.BasaltRules.Geometry[tile] != 0 {
		return true, nil
	}
	at, e := primitiveFreeRecord(m, 0xc800, 0xe740, 32)
	if e != nil || at == 0 {
		return true, e
	}
	s := nativeWhirlwindMemory{m: m}
	s.putByte(tsunamiGrid(origin)+1, 0xe0)
	for _, v := range []struct {
		offset int
		value  uint8
	}{{12, uint8(c.D[2])}, {6, x}, {8, y}, {7, 128}, {9, 128}, {0, 0x3a}, {22, 0x38}} {
		s.putByte(at+v.offset, v.value)
	}
	s.putWord(at+24, uint16(c.D[4]))
	s.putWord(at+26, uint16(c.D[3]))
	s.putWord(at+10, 0x5ec)
	s.putWord(at+20, uint16(w.random()%w.BasaltRules.DelayModulus+4))
	if s.err != nil {
		return false, s.err
	}
	ref := NativeRecordReference(uint16(at - 0x76c0))
	if e := w.nativeRuntimeInsert(ref); e != nil {
		return false, e
	}
	w.BasaltState.Directions[(at-0xc800)/32] = uint16(c.D[3])
	c.D[0] = 1
	return false, nil
}

// commandMove is the unfiltered $12518 common-prefix operation. Lightning's
// raw deity pointer can name another pool or a parcel/control alias; the source
// does not require an allocated actor or normalize it to a typed marker.
func (w *World) commandMove(at int, c *NativeCommandRegisterContext) error {
	m := nativeWhirlwindMemory{m: w.nativeCleanupMemory()}
	old := m.word(at+8)&0xff00 | uint16(m.byte(at+6))
	commandWord(c, 0, old)
	m.putWord(at+6, uint16(c.D[6]))
	m.putWord(at+8, uint16(c.D[7]))
	packed := m.word(at+8)&0xff00 | uint16(m.byte(at+6))
	commandWord(c, 7, packed)
	if old == packed {
		c.D[0] = 0
		return m.err
	}
	oldGrid, newGrid := tsunamiGrid(old), tsunamiGrid(packed)
	previous, next := m.word(at+4), m.word(at+2)
	if previous != 0 {
		m.putWord(cleanupRecordAddress(NativeRecordReference(previous))+2, next)
	} else {
		m.putWord(oldGrid+2, next)
	}
	if next != 0 {
		m.putWord(cleanupRecordAddress(NativeRecordReference(next))+4, previous)
	}
	m.putWord(at+2, 0)
	m.putWord(at+4, 0)
	ref := uint16(at - 0x76c0)
	c.D[6] = uint32(int32(at - 0x76c0))
	head := m.word(newGrid + 2)
	if head != 0 {
		m.putWord(at+2, head)
		m.putWord(cleanupRecordAddress(NativeRecordReference(head))+4, ref)
	}
	m.putWord(newGrid+2, ref)
	m.putByte(newGrid, m.byte(newGrid)+8)
	commandWord(c, 7, packed&0xff00|uint16(uint8(packed)*4))
	c.D[0] = 1
	return m.err
}

func (w *World) commandLightning(call NativeCommandCall) (bool, error) {
	c := call.Context
	m := nativeWhirlwindMemory{m: w.nativeCleanupMemory()}
	owner := uint8(c.D[2])
	god := heroGodAddress(owner)
	ref := m.word(god + 0x12)
	switch call.Routine {
	case 0x15de2:
		c.D[3] = uint32(uint16(int16(int8(owner)))) * 314
		commandWord(c, 3, ref)
		if ref != 0 {
			commandWord(c, 0, uint16(c.D[0])<<8|128)
			commandWord(c, 1, uint16(c.D[1])<<8|128)
			commandWord(c, 6, uint16(c.D[0]))
			commandWord(c, 7, uint16(c.D[1]))
			if m.err != nil {
				return false, m.err
			}
			return false, w.commandMove(cleanupRecordAddress(NativeRecordReference(ref)), c)
		}
		at, e := primitiveFreeRecord(m.m, 0xc800, 0xe740, 32)
		if e != nil {
			return false, e
		}
		if at == 0 {
			c.D[0] = 0
			return true, nil
		}
		ref = uint16(at - 0x76c0)
		c.D[3] = uint32(ref)
		m.putWord(god+0x12, ref)
		x, y := uint8(c.D[0]), uint8(c.D[1])
		for _, v := range []struct {
			offset int
			value  uint8
		}{{12, owner}, {6, x}, {8, y}, {7, 128}, {9, 128}, {0, 0x28}, {22, 0x16}} {
			m.putByte(at+v.offset, v.value)
		}
		m.putWord(at+24, uint16(w.LightningRules.MarkerLife))
		m.putWord(at+10, 0x6e0)
		m.putWord(at+26, 0)
		if m.err != nil {
			return false, m.err
		}
		commandWord(c, 1, m.word(tsunamiGrid(uint16(y)<<8|uint16(x))+2))
		c.D[0] = uint32(ref)
		return false, w.nativeRuntimeInsert(NativeRecordReference(ref))
	case 0x15e8a:
		if ref == 0 {
			return true, m.err
		}
		marker := cleanupRecordAddress(NativeRecordReference(ref))
		if m.word(marker+26) != 0 {
			return false, m.err
		}
		xp := m.byte(god + 0x55)
		count := uint16(xp>>5) + 1
		for i := uint16(0); i <= count; i++ {
			at, e := primitiveFreeRecord(m.m, 0xc800, 0xe740, 32)
			if e != nil {
				return false, e
			}
			if at == 0 {
				return true, nil
			}
			jitter := w.LightningRules.Jitter[w.random()%9]
			x, y := m.byte(marker+6), m.byte(marker+8)
			xx, yy := uint8(int(x)+int(jitter[0])), uint8(int(y)+int(jitter[1]))
			if int8(xx) < 0 || int8(xx) >= 64 {
				xx = x
			}
			if int8(yy) < 0 || int8(yy) >= 64 {
				yy = y
			}
			for _, v := range []struct {
				offset int
				value  uint8
			}{{6, xx}, {8, yy}, {7, 128}, {9, 128}, {12, m.byte(marker + 12)}, {22, 0x18}, {0, 0x2a}} {
				m.putByte(at+v.offset, v.value)
			}
			child := uint16(at - 0x76c0)
			m.putWord(at+26, m.word(marker+26))
			m.putWord(marker+26, child)
			m.putWord(at+28, ref)
			m.putWord(at+10, 0)
			if m.err != nil {
				return false, m.err
			}
			if e := w.nativeRuntimeInsert(NativeRecordReference(child)); e != nil {
				return false, e
			}
		}
		return false, m.err
	case 0x15f80:
		if ref == 0 {
			return true, m.err
		}
		at := cleanupRecordAddress(NativeRecordReference(ref))
		m.putWord(god+0x12, 0)
		m.putByte(at+22, 0x1a)
		m.putWord(at+10, 0x720)
		seen := map[uint16]bool{}
		for child := m.word(at + 26); child != 0; {
			if seen[child] {
				return false, fmt.Errorf("cyclic native lightning bolt chain")
			}
			seen[child] = true
			bolt := cleanupRecordAddress(NativeRecordReference(child))
			m.putByte(bolt+12, 0)
			next := m.word(bolt + 26)
			if m.err != nil {
				return false, m.err
			}
			if e := w.nativeRuntimeUnlink(NativeRecordReference(child)); e != nil {
				return false, e
			}
			child = next
		}
		return false, m.err
	}
	return false, fmt.Errorf("native lightning command routine%x unavailable", call.Routine)
}

func (w *World) commandHeroCreation(call NativeCommandCall, trace func(NativeCommandCall)) (bool, error) {
	c, m := call.Context, w.nativeCleanupMemory()
	selector := uint16(c.D[0])
	if selector > 10 || selector&1 != 0 {
		return false, fmt.Errorf("native hero selector%d unavailable", selector)
	}
	owner := uint16(int16(int8(uint8(c.D[2]))))
	c.D[2] = uint32(owner) * 314
	ref, e := m.Read16(primitiveDeityAddress(owner) + 8)
	if e != nil {
		return false, e
	}
	commandWord(c, 1, ref)
	if ref == 0 {
		c.D[0] = 0
		return true, nil
	}
	return w.commandHeroConversion(NativeRecordReference(ref), call, trace)
}

func (w *World) commandHeroConversion(reference NativeRecordReference, call NativeCommandCall, trace func(NativeCommandCall)) (bool, error) {
	c, m := call.Context, w.nativeCleanupMemory()
	selector := uint16(c.D[0])
	if selector > 10 || selector&1 != 0 {
		return false, fmt.Errorf("native direct hero selector%d unavailable", selector)
	}
	ref := uint16(reference)
	at := cleanupRecordAddress(NativeRecordReference(ref))
	actorOwner, e := m.Read8(at + 12)
	if e != nil {
		return false, e
	}
	kind, e := m.Read8(at)
	if e != nil {
		return false, e
	}
	speed, e := m.Read8(at + 18)
	if e != nil {
		return false, e
	}
	flags, e := m.Read8(at + 13)
	if e != nil {
		return false, e
	}
	id := heroIDs[selector/2]
	cb := HeroCreationCallbacks{Memory: m}
	cb.ClearLeader = func(target NativeRecordReference) error {
		commandByte(c, 2, actorOwner)
		commandExtend(c, 2)
		if flags&1 != 0 {
			x, e := m.Read8(at + 6)
			if e != nil {
				return e
			}
			y, e := m.Read8(at + 8)
			if e != nil {
				return e
			}
			commandByte(c, 1, y)
			if trace != nil {
				markerContext := *c
				markerContext.D[0] = uint32(uint16(c.D[2])) * 314
				commandByte(&markerContext, 0, x)
				trace(NativeCommandCall{Routine: 0x13fe4, Caller: call.Caller, Context: &markerContext})
			}
		}
		return w.clearNativeLeader(target)
	}
	cb.ClearFarms = func(target NativeRecordReference, tile uint8) error {
		commandWord(c, 2, uint16(tile))
		return w.clearNativeFarms(target, tile)
	}
	cb.Sound = func(cue uint16) error {
		c.D[1] = 0
		commandWord(c, 0, cue)
		if trace != nil {
			trace(NativeCommandCall{Routine: 0x184f6, Caller: call.Caller, Context: c})
		}
		return w.nativeEntryCallbacks().Sound(cue)
	}
	if e := w.HeroArt.Convert(NativeRecordReference(ref), id, cb); e != nil {
		return false, e
	}
	xp, e := m.Read8(heroGodAddress(actorOwner) + 0x52 + int(selector/2))
	if e != nil {
		return false, e
	}
	c.D[1] = uint32(xp >> 3)
	if id == Odysseus {
		c.D[1] += uint32(speed)
	}
	if kind == 4 {
		commandWord(c, 2, 15)
	}
	c.D[0] = 1
	return false, nil
}

func (w *World) commandTsunamiCreation(call NativeCommandCall) (bool, error) {
	c, m := call.Context, w.nativeCleanupMemory()
	owner, x, y := uint16(c.D[2]), uint8(c.D[0]), uint8(c.D[1])
	origin := uint16(y)<<8 | uint16(x)
	commandWord(c, 1, origin)
	c.D[5] = 0xfffffffc
	tiles := [4]uint8{}
	for i, offset := range w.TsunamiRules.Origins {
		parcel := origin + offset
		if parcel&0xc0c0 == 0 {
			v, e := m.Read8(tsunamiGrid(parcel) + 1)
			if e != nil {
				return false, e
			}
			tiles[i] = v
		}
	}
	step, e := w.TsunamiRules.Create(owner, x, y, w.tsunamiCallbacks())
	if e != nil {
		return false, e
	}
	created := 0
	for i, offset := range w.TsunamiRules.Origins {
		commandWord(c, 5, uint16(c.D[5])+4)
		parcel := origin + offset
		commandWord(c, 3, parcel)
		commandWord(c, 0, parcel&0xc0c0)
		if parcel&0xc0c0 != 0 {
			continue
		}
		commandWord(c, 4, parcel)
		commandWord(c, 3, parcel&0xff00|uint16(uint8(parcel)*4))
		commandByte(c, 0, tiles[i])
		commandWord(c, 0, uint16(c.D[0])*2)
		if w.TsunamiRules.Properties[tiles[i]]&8 == 0 {
			continue
		}
		if created >= len(step.References) && step.PoolFull {
			return false, nil
		}
		if created >= len(step.References) {
			return false, fmt.Errorf("native tsunami source allocation sequence differs")
		}
		c.D[6] = uint32(w.TsunamiRules.Speed)
		c.D[0] = uint32(uint16(step.References[created]))
		created++
	}
	commandWord(c, 5, uint16(c.D[5])+4)
	commandWord(c, 3, 0xff9d)
	return false, nil
}

func (w *World) commandArmageddon(call NativeCommandCall, trace func(NativeCommandCall)) (bool, error) {
	c, m := call.Context, w.nativeCleanupMemory()
	enabled, e := m.Read16(0xf12)
	if e != nil {
		return false, e
	}
	if enabled != 0 {
		return false, nil
	}
	cb := ArmageddonCallbacks{Memory: m}
	cb.Random = func() uint16 {
		v := uint16(w.random())
		c.D[0] = uint32(v)
		_ = commandDivide(c, 0, 4)
		commandSwap(c, 0)
		commandWord(c, 0, uint16(c.D[0])*2)
		return v
	}
	cb.Convert = func(ref NativeRecordReference, selector uint16) error {
		owner, e := m.Read8(cleanupRecordAddress(ref) + 12)
		if e != nil {
			return e
		}
		commandByte(c, 2, owner)
		inner := NativeCommandCall{Routine: 0x142fe, Caller: call.Caller, Context: c}
		if trace != nil {
			trace(inner)
		}
		_, e = w.commandHeroConversion(ref, inner, trace)
		return e
	}
	cb.Cleanup = func(ref NativeRecordReference, mode uint16) error {
		return w.commandFollowerCleanup(ref, mode, call, trace)
	}
	_, e = w.ArmageddonRules.Cast(cb)
	if e != nil {
		return false, e
	}
	c.D[0] = 1
	return false, nil
}

// commandRoad translates $16744/$1677a on the raw four-byte parcel map.
// $168e8 returns1 even for rejected terrain; the command caller therefore
// increments God+$44 and debits those casts too. Existing wall records retain
// their phased break art/stage when a road crosses their cell.
func (w *World) commandRoad(call NativeCommandCall) (bool, error) {
	c, m := call.Context, w.nativeCleanupMemory()
	r := NativeCommandRules{Code: w.NativeAI.Code}
	origin := uint16(uint8(c.D[1]))<<8 | uint16(uint8(c.D[0]))
	commandWord(c, 1, origin)
	if call.Routine == 0x16744 {
		commandWord(c, 1, origin&0xff00|uint16(uint8(origin)*4))
		tile, e := m.Read8(0xf44 + int(int16(uint16(c.D[1]))) + 1)
		if e != nil {
			return false, e
		}
		commandWord(c, 2, uint16(tile)*2)
		property, e := r.word(0x33312 + int(uint16(c.D[2])))
		if e != nil {
			return false, e
		}
		if property&0x40 != 0 {
			commandWord(c, 2, uint16(c.D[2])>>1)
			raster, e := r.byte(0x33512 + int(int16(uint16(c.D[2]))))
			if e != nil {
				return false, e
			}
			if e := m.Write8(0xf44+int(int16(uint16(c.D[1])))+1, raster); e != nil {
				return false, e
			}
		}
		return false, nil
	}
	commandWord(c, 0, origin&0xff00|uint16(uint8(origin)*4))
	commandWord(c, 4, uint16(c.D[2]))
	tile, e := m.Read8(0xf44 + int(int16(uint16(c.D[0]))) + 1)
	if e != nil {
		return false, e
	}
	commandByte(c, 2, tile)
	property, e := r.word(0x33312 + int(tile)*2)
	if e != nil {
		return false, e
	}
	commandWord(c, 3, property)
	finish := func() (bool, error) { c.D[0] = 1; return false, nil }
	if property&0x400 != 0 {
		return finish()
	}
	mask := uint16(0x25)
	if uint8(c.D[4]) == 1 {
		mask = 0x23
	}
	commandWord(c, 3, uint16(c.D[3])&mask)
	if uint16(c.D[3]) == 0 {
		commandWord(c, 3, 3)
		matched := false
		for i := 0; i < 4; i++ {
			shape, e := r.byte(0x16930 + i)
			if e != nil {
				return false, e
			}
			if tile == shape {
				paint, e := r.byte(0x16934 + i)
				if e != nil {
					return false, e
				}
				commandByte(c, 3, paint)
				matched = true
				break
			}
			commandWord(c, 3, uint16(c.D[3])-1)
		}
		if !matched {
			return finish()
		}
	} else {
		c.D[3], c.D[2] = 0, 3
		for i := 0; i < 4; i++ {
			offset, e := r.word(0x168f4 + i*2)
			if e != nil {
				return false, e
			}
			parcel := origin + offset
			commandWord(c, 4, parcel)
			commandWord(c, 0, parcel&0xc0c0)
			if parcel&0xc0c0 == 0 {
				commandWord(c, 4, parcel&0xff00|uint16(uint8(parcel)*4))
				t, e := m.Read8(0xf44 + int(int16(uint16(c.D[4]))) + 1)
				if e != nil {
					return false, e
				}
				commandByte(c, 0, t)
				commandWord(c, 0, uint16(c.D[0])*2)
				p, e := r.word(0x33312 + int(int16(uint16(c.D[0]))))
				if e != nil {
					return false, e
				}
				if p&0x40 != 0 {
					c.D[3] |= 1 << uint(uint16(c.D[2])&31)
				}
			}
			commandWord(c, 2, uint16(c.D[2])-1)
		}
		commandWord(c, 0, origin&0xff00|uint16(uint8(origin)*4))
		paint, e := r.byte(0x168fc + int(int16(uint16(c.D[3]))))
		if e != nil {
			return false, e
		}
		commandByte(c, 3, paint)
	}
	dirty, e := m.Read16(0xf2e)
	if e != nil {
		return false, e
	}
	if e := m.Write16(0xf2e, dirty+1); e != nil {
		return false, e
	}
	grid := 0xf44 + int(int16(uint16(c.D[0])))
	if e := m.Write8(grid+1, uint8(c.D[3])); e != nil {
		return false, e
	}
	commandByte(c, 3, uint8(c.D[3])-197)
	head, e := m.Read16(grid + 2)
	if e != nil {
		return false, e
	}
	commandWord(c, 0, head)
	seen := map[uint16]bool{}
	for uint16(c.D[0]) != 0 {
		ref := uint16(c.D[0])
		if seen[ref] {
			return false, fmt.Errorf("cyclic native road wall scan")
		}
		seen[ref] = true
		at := cleanupRecordAddress(NativeRecordReference(ref))
		kind, e := m.Read8(at)
		if e != nil {
			return false, e
		}
		if kind == 0x1a {
			if e := m.Write16(at+10, 0xb44); e != nil {
				return false, e
			}
			if e := m.Write8(at+1, 8); e != nil {
				return false, e
			}
			connections, e := r.byte(0x1691c + int(int16(uint16(c.D[3]))))
			if e != nil {
				return false, e
			}
			commandByte(c, 0, connections)
			commandWord(c, 0, uint16(c.D[0])&5)
			if uint16(c.D[0]) != 0 {
				if e := m.Write16(at+10, 0xb54); e != nil {
					return false, e
				}
				if e := m.Write8(at+1, 6); e != nil {
					return false, e
				}
			}
			break
		}
		next, e := m.Read16(at + 2)
		if e != nil {
			return false, e
		}
		commandWord(c, 0, next)
	}
	connections, e := r.byte(0x1691c + int(int16(uint16(c.D[3]))))
	if e != nil {
		return false, e
	}
	commandByte(c, 3, connections)
	c.D[2] = 3
	for i := 0; i < 4; i++ {
		offset, e := r.word(0x168f4 + i*2)
		if e != nil {
			return false, e
		}
		commandWord(c, 4, offset)
		if c.D[3]&(1<<uint(uint16(c.D[2])&31)) != 0 {
			parcel := origin + offset
			commandWord(c, 4, parcel)
			commandWord(c, 0, parcel&0xc0c0)
			if parcel&0xc0c0 == 0 {
				commandWord(c, 4, parcel&0xff00|uint16(uint8(parcel)*4))
				t, e := m.Read8(0xf44 + int(int16(uint16(c.D[4]))) + 1)
				if e != nil {
					return false, e
				}
				commandByte(c, 0, t)
				commandWord(c, 6, uint16(c.D[0]))
				commandWord(c, 0, uint16(c.D[0])*2)
				p, e := r.word(0x33312 + int(int16(uint16(c.D[0]))))
				if e != nil {
					return false, e
				}
				if p&0x40 != 0 {
					commandWord(c, 0, uint16(c.D[6]))
					commandByte(c, 0, uint8(c.D[0])-197)
					v, e := r.byte(0x1691c + int(int16(uint16(c.D[0]))))
					if e != nil {
						return false, e
					}
					commandByte(c, 0, v)
					bit, e := r.byte(0x168f0 + int(int16(uint16(c.D[2]))))
					if e != nil {
						return false, e
					}
					commandByte(c, 5, bit)
					c.D[0] |= 1 << uint(uint8(c.D[5])&31)
					commandWord(c, 5, 3)
					matched := false
					for j := 0; j < 4; j++ {
						shape, e := r.byte(0x16934 + j)
						if e != nil {
							return false, e
						}
						if uint8(c.D[6]) == shape {
							matched = true
							break
						}
						commandWord(c, 5, uint16(c.D[5])-1)
					}
					if !matched {
						paint, e := r.byte(0x1690c + int(int16(uint16(c.D[0]))))
						if e != nil {
							return false, e
						}
						if e := m.Write8(0xf44+int(int16(uint16(c.D[4])))+1, paint); e != nil {
							return false, e
						}
					}
				}
			}
		}
		commandWord(c, 2, uint16(c.D[2])-1)
	}
	return finish()
}

func commandSwap(c *NativeCommandRegisterContext, reg int) { c.D[reg] = c.D[reg]<<16 | c.D[reg]>>16 }
func commandDivide(c *NativeCommandRegisterContext, reg int, divisor uint16) error {
	if divisor == 0 {
		return fmt.Errorf("native command DIVU zero divisor")
	}
	d := c.D[reg]
	if d/uint32(divisor) <= 0xffff {
		c.D[reg] = d%uint32(divisor)<<16 | d/uint32(divisor)
	}
	return nil
}

// commandWeatherCreation observes the same RNG draws and insertion boundary
// as the actual creators. These source routines leave their roulette counters
// and last packed position live; Storm's MOVEM.W also sign-extends D1-D3 after
// each insertion. No extra random draw or second effect body is executed.
func (w *World) commandWeatherCreation(call NativeCommandCall) (bool, error) {
	c := call.Context
	owner, x, y := uint16(c.D[2]), uint8(c.D[0]), uint8(c.D[1])
	origin := uint16(y)<<8 | uint16(x)
	commandWord(c, 3, origin)
	meteor := call.Routine == 0x1648c
	base := w.StormRules.CountModulus
	if meteor {
		base = w.FireRainRules.BaseCount
		c.D[1] = 0
		if int16(owner) <= 2 {
			xp, e := w.nativeCleanupMemory().Read8(primitiveDeityAddress(owner) + 0x56)
			if e != nil {
				return false, e
			}
			base += uint16(xp >> 5)
		}
	}
	commandWord(c, 1, base)
	first, iteration, delay := true, false, false
	var transferError error
	random := func() uint16 {
		v := uint16(w.random())
		c.D[0] = uint32(v)
		if first {
			first = false
			transferError = commandDivide(c, 0, base)
			commandSwap(c, 0)
			commandWord(c, 1, base/4+uint16(c.D[0]))
			return v
		}
		if delay {
			delay = false
			transferError = commandDivide(c, 0, 9)
			commandSwap(c, 0)
			return v
		}
		if iteration {
			commandWord(c, 1, uint16(c.D[1])-1)
		}
		iteration = true
		if !meteor {
			commandWord(c, 5, v)
		}
		packed := origin + (v & 0x0707)
		commandWord(c, 0, packed)
		commandWord(c, 4, packed)
		commandWord(c, 0, packed&0xc0c0)
		if meteor && packed&0xc0c0 == 0 {
			delay = true
		}
		return v
	}
	if meteor {
		cb := w.fireRainCallbacks()
		cb.Random = random
		step, e := w.FireRainRules.Create(owner, x, y, cb)
		if iteration {
			commandWord(c, 1, uint16(c.D[1])-1)
		}
		if e != nil {
			return false, e
		}
		if transferError != nil {
			return false, transferError
		}
		for _, ref := range step.References {
			if loc, ok := LocateNativeRecord(ref); ok {
				w.NativeEnvironment[loc.Index] = NativeEnvironmentFireRain
			}
		}
		return !step.Admitted, nil
	}
	cb := w.stormCallbacks()
	cb.Random = random
	link := cb.Link
	cb.Link = func(ref NativeRecordReference) error {
		c.D[0] = uint32(uint16(ref))
		commandWord(c, 5, uint16(c.D[5])&12)
		for _, reg := range []int{1, 2, 3} {
			c.D[reg] = uint32(int32(int16(uint16(c.D[reg]))))
		}
		return link(ref)
	}
	step, e := w.StormRules.Create(owner, x, y, call.Caller, cb)
	if iteration {
		commandWord(c, 1, uint16(c.D[1])-1)
	}
	if e != nil {
		return false, e
	}
	if transferError != nil {
		return false, transferError
	}
	return !step.Admitted, nil
}

// commandGroundCreation follows the native data-register writes alongside the
// already proven raw creator. Its memory observer sees the actual source
// read before a repeated tile write, including retained occupancy rejection.
func (w *World) commandGroundCreation(call NativeCommandCall) (bool, error) {
	c := call.Context
	owner, x, y := uint16(c.D[2]), uint8(c.D[0]), uint8(c.D[1])
	origin := uint16(y)<<8 | uint16(x)
	font := call.Routine == 0x16938
	renew := call.Routine == 0x16a62
	base, mask := w.NativeGround.SwampCount, uint16(0x27)
	id := Swamp
	if font {
		base, mask, id = w.NativeGround.FontCount, 0x67, Baptism
	}
	if renew {
		base = w.RenewNative.CountModulus
	}
	xp := uint16(0)
	if font {
		c.D[4] = 0
	} else {
		c.D[3] = 0
	}
	xpBranch := int8(uint8(owner)) <= 2
	if font {
		xpBranch = int16(owner) <= 2
	}
	if xpBranch {
		c.D[2] = uint32(owner) * 314
		xpAt := 0x53
		if font {
			xpAt = 0x57
		}
		v, e := w.nativeCleanupMemory().Read8(primitiveDeityAddress(owner) + xpAt)
		if e != nil {
			return false, e
		}
		xp = uint16(v >> 5)
		if font {
			c.D[4] = uint32(xp)
			base += xp
		} else {
			c.D[3] = uint32(xp)
		}
	}
	commandWord(c, 1, origin)
	if font {
		commandWord(c, 4, base)
	}
	first, iteration := true, false
	grid := 0
	var transferError error
	random := func() uint16 {
		v := uint16(w.random())
		c.D[0] = uint32(v)
		if first {
			first = false
			commandWord(c, 2, v)
			if !font {
				commandWord(c, 0, base)
			}
			transferError = commandDivide(c, 2, base)
			if !font && !renew {
				commandWord(c, 2, uint16(c.D[2])+xp)
			}
			commandSwap(c, 2)
			commandWord(c, 2, uint16(c.D[2])+base/2)
			if renew {
				commandWord(c, 2, uint16(c.D[2])+xp)
			}
			if font {
				commandWord(c, 4, base/2)
			} else {
				commandWord(c, 0, base/2)
			}
			return v
		}
		if iteration {
			commandWord(c, 2, uint16(c.D[2])-1)
		}
		iteration = true
		transferError = commandDivide(c, 0, 90)
		commandSwap(c, 0)
		commandWord(c, 0, uint16(c.D[0])&0xfffe)
		offset := w.NativeGround.SwampOffsets[(v%90)/2]
		if font {
			offset = w.NativeGround.FontOffsets[(v%90)/2]
		}
		if renew {
			offset = w.RenewNative.Offsets[(v%90)/2]
		}
		packed := origin + offset
		commandWord(c, 3, packed)
		commandWord(c, 0, packed&0xc0c0)
		grid = 0
		if packed&0xc0c0 == 0 {
			commandWord(c, 3, packed&0xff00|uint16(uint8(packed)*4))
			grid = tsunamiGrid(packed)
		}
		return v
	}
	m := w.nativeCleanupMemory()
	read8 := m.Read8
	m.Read8 = func(at int) (uint8, error) {
		v, e := read8(at)
		if e == nil && iteration && grid != 0 && at == grid+1 {
			commandByte(c, 0, v)
			if renew {
				commandByte(c, 0, w.RenewNative.Raster[v])
				commandWord(c, 0, uint16(c.D[0])&15)
			} else {
				// Font deliberately preserves D0's quotient upper word and
				// the preexisting low-word high byte before its byte load.
				commandWord(c, 0, uint16(c.D[0])*2)
				commandWord(c, 0, w.NativeGround.Properties[v]&mask)
			}
		}
		return v, e
	}
	var err error
	if renew {
		_, err = w.RenewNative.Create(owner, x, y, RenewNativeCallbacks{Memory: m, Random: random})
	} else {
		cb := w.nativeGroundCallbacks(id, int(uint8(owner))-1)
		cb.Memory = m
		cb.Random = random
		cb.SourceD2Upper = uint16(c.D[2] >> 16)
		_, err = w.NativeGround.Create(id, owner, x, y, cb)
	}
	if iteration {
		commandWord(c, 2, uint16(c.D[2])-1)
	}
	if err != nil {
		return false, err
	}
	if transferError != nil {
		return false, transferError
	}
	return false, nil // These normal handlers do not branch on returned Z.
}

func (w *World) commandFollowerCleanup(ref NativeRecordReference, mode uint16, call NativeCommandCall, trace func(NativeCommandCall)) error {
	c, m := call.Context, w.nativeCleanupMemory()

	c.D[0] = uint32(mode)
	inner := NativeCommandCall{Routine: 0x124a2, Caller: call.Caller, Context: c}
	if trace != nil {
		trace(inner)
	}
	at := cleanupRecordAddress(ref)
	owner, e := m.Read8(at + 12)
	if e != nil {
		return e
	}
	flags, e := m.Read8(at + 13)
	if e != nil {
		return e
	}
	marker, e := m.Read16(heroGodAddress(owner) + 10)
	if e != nil {
		return e
	}
	traced := false
	cleanup := FollowerCleanupCallbacks{Memory: m, Insert: w.nativeRuntimeInsert, ClearFarms: w.clearNativeFarms}
	cleanup.Unlink = func(target NativeRecordReference) error {
		if !traced && flags&1 != 0 && uint16(target) == marker && trace != nil {
			x, e := m.Read8(at + 6)
			if e != nil {
				return e
			}
			y, e := m.Read8(at + 8)
			if e != nil {
				return e
			}
			markerContext := *c
			product := uint32(uint16(int16(int8(owner)))) * 314
			markerContext.D[0], markerContext.D[1] = product, product
			commandByte(&markerContext, 0, x)
			commandByte(&markerContext, 1, y)
			commandByte(&markerContext, 2, owner)
			commandExtend(&markerContext, 2)
			trace(NativeCommandCall{Routine: 0x13fe4, Caller: call.Caller, Context: &markerContext})
			traced = true
		}
		return w.nativeRuntimeUnlink(target)
	}
	step, e := CleanupFollower(ref, FollowerCleanupRegisters{D0: c.D[0], D1: c.D[1], D2: c.D[2]}, cleanup)
	c.D[0], c.D[1], c.D[2] = step.Registers.D0, step.Registers.D1, step.Registers.D2
	return e

}
