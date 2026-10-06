package populous2

import "fmt"

func (w *World) commandBatholith(call NativeCommandCall) (bool, error) {
	c := call.Context
	m := nativeWhirlwindMemory{m: w.nativeCleanupMemory()}
	rules := NativeCommandRules{Code: w.NativeAI.Code}
	commandWord(c, 3, uint16(c.D[0]))
	c.D[0] = uint32(w.random())
	commandByte(c, 0, (uint8(c.D[0])&7)-4)
	commandByte(c, 3, uint8(c.D[3])+uint8(c.D[0]))
	if int8(uint8(c.D[3])) < 0 || int8(uint8(c.D[3])) >= 64 {
		c.D[0] = 0
		return true, nil
	}
	commandWord(c, 0, uint16(c.D[0])>>8)
	commandByte(c, 0, (uint8(c.D[0])&7)-4)
	commandByte(c, 1, uint8(c.D[1])+uint8(c.D[0]))
	if int8(uint8(c.D[1])) < 0 || int8(uint8(c.D[1])) >= 64 {
		c.D[0] = 0
		return true, nil
	}
	c.D[0] = uint32(w.random())
	rangeWord, e := rules.word(0xde34)
	if e != nil {
		return false, e
	}
	commandWord(c, 4, rangeWord)
	if e := commandDivide(c, 0, rangeWord); e != nil {
		return false, e
	}
	commandSwap(c, 0)
	commandWord(c, 4, uint16(c.D[4])>>1)
	commandWord(c, 0, uint16(c.D[0])-uint16(c.D[4]))
	if int16(uint16(c.D[0])) >= 0 {
		commandByte(c, 0, uint8(c.D[3]))
		commandExtend(c, 0)
		commandExtend(c, 1)
		saved0, saved1 := uint16(c.D[0]), uint16(c.D[1])
		if _, e := w.commandDirectTerrain(call, true); e != nil {
			return false, e
		}
		c.D[0], c.D[1] = uint32(int32(int16(saved0))), uint32(int32(int16(saved1)))
	} else {
		commandByte(c, 2, uint8(c.D[1]))
		commandWord(c, 2, uint16(c.D[2])<<8)
		commandByte(c, 2, uint8(c.D[2])+uint8(c.D[3]))
		commandByte(c, 2, uint8(c.D[2])*4)
		c.D[0] = uint32(w.random())
		c.D[4] = c.D[0]
		if e := commandDivide(c, 4, 8); e != nil {
			return false, e
		}
		commandSwap(c, 4)
		commandWord(c, 4, uint16(c.D[4])&0xfe)
		grid := 0xf44 + int(int16(uint16(c.D[2])))
		tile := m.byte(grid + 1)
		commandByte(c, 0, tile)
		if tile != 0 {
			commandExtend(c, 0)
			commandWord(c, 0, uint16(c.D[0])*2)
			property, e := rules.word(0x33312 + int(int16(uint16(c.D[0]))))
			if e != nil {
				return false, e
			}
			commandWord(c, 0, property&0x40)
			if uint16(c.D[0]) == 0 && m.word(grid+2) == 0 {
				at, e := primitiveFreeRecord(m.m, 0x6bd0, 0x76c0, 14)
				if e != nil {
					return false, e
				}
				if at != 0 {
					age, e := rules.byte(0xddcd)
					if e != nil {
						return false, e
					}
					art, e := rules.word(0xddd2 + int(int16(uint16(c.D[4]))))
					if e != nil {
						return false, e
					}
					m.putByte(at+12, 3)
					m.putByte(at, 0x18)
					m.putByte(at+1, age)
					m.putWord(at+10, art)
					c.D[0] = uint32(w.random())
					if e := commandDivide(c, 0, 90); e != nil {
						return false, e
					}
					commandSwap(c, 0)
					if uint16(c.D[0]) == 0 {
						commandWord(c, 0, uint16(c.D[0])&0xfe)
						art, e := rules.word(0xddd2 + int(int16(uint16(c.D[0]))))
						if e != nil {
							return false, e
						}
						m.putWord(at+10, art)
					}
					commandByte(c, 2, uint8(c.D[2])>>2)
					m.putByte(at+6, uint8(c.D[2]))
					m.putByte(at+7, 128)
					commandByte(c, 2, 128)
					m.putWord(at+8, uint16(c.D[2]))
					commandWord(c, 6, uint16(c.D[6])+1)
					if m.err != nil {
						return false, m.err
					}
					if e := w.nativeRuntimeInsert(NativeRecordReference(uint16(at - 0x76c0))); e != nil {
						return false, e
					}
				}
			}
		}
	}
	c.D[0] = 1
	return false, m.err
}

func (w *World) commandEarthquakeCreation(call NativeCommandCall) (bool, error) {
	c := call.Context
	m := nativeWhirlwindMemory{m: w.nativeCleanupMemory()}
	commandExtend(c, 0)
	commandExtend(c, 1)
	x, y := int(int16(uint16(c.D[0]))), int(int16(uint16(c.D[1])))
	if !inside(x, y) {
		return false, nil
	}
	at, e := primitiveFreeRecord(m.m, 0xc800, 0xe740, 32)
	if e != nil || at == 0 {
		return false, e
	}
	for _, v := range []struct {
		offset int
		value  uint8
	}{{12, uint8(c.D[2])}, {6, uint8(x)}, {8, uint8(y)}, {7, 0}, {9, 0}, {22, 0x22}} {
		m.putByte(at+v.offset, v.value)
	}
	commandExtend(c, 3)
	m.putByte(at+26, uint8(c.D[3]))
	commandWord(c, 3, uint16(c.D[3])>>1)
	m.putWord(at+10, uint16(c.D[3]))
	m.putWord(at+20, w.EarthquakeRules.InitialTimer)
	m.putByte(at+18, w.EarthquakeRules.Speed)
	m.putWord(at+24, uint16(c.D[4]))
	direction := c.D[3]
	grid := 0xf44 + (x+y*64)*4
	commandWord(c, 4, uint16(y)<<8|uint16(uint8(x)*4))
	tile := m.byte(grid + 1)
	c.D[3] = uint32(tile)
	height := 0
	if tile >= 0xac && tile <= 0xc4 {
		c.D[0] = 0
	} else {
		commandByte(c, 3, w.EarthquakeRules.Raster[tile])
		commandWord(c, 3, uint16(c.D[3])&15)
		if uint16(c.D[3]) == 15 {
			m.putByte(grid+1, m.byte(at+11)+0xac)
			c.D[0] = 0
		} else {
			commandByte(c, 5, m.byte(grid))
			commandWord(c, 5, uint16(c.D[5])&7)
			c.D[0] = uint32(tile)
			if w.EarthquakeRules.Raster[tile]&1 != 0 {
				commandWord(c, 5, uint16(c.D[5])+1)
			}
			height = int(uint16(c.D[5]))
			if height == 0 {
				m.putByte(at+12, 0)
				c.D[0] = 0xffffffff
			} else {
				if height > 1 {
					c.D[0], c.D[1] = uint32(x), uint32(y)
					saved5 := c.D[5]
					if m.err != nil {
						return false, m.err
					}
					if _, e := w.commandDirectTerrain(call, false); e != nil {
						return false, e
					}
					c.D[5] = saved5
				}
				commandWord(c, 0, uint16(c.D[5]))
			}
		}
	}
	c.D[3] = direction
	if c.D[0] == 0 {
		commandByte(c, 3, uint8(c.D[3])+0xac)
		c.D[0] = uint32(m.byte(grid + 1))
		m.putWord(0xf2e, m.word(0xf2e)+1)
		m.putByte(grid+1, uint8(c.D[3]))
	}
	if m.byte(at+12) != 0 {
		w.NativeEnvironment[(at-0xc800)/32] = NativeEnvironmentQuake
	}
	return false, m.err
}

func (w *World) commandSculpt(call NativeCommandCall) (bool, error) {
	c, m := call.Context, w.nativeCleanupMemory()
	rules := NativeCommandRules{Code: w.NativeAI.Code}
	raise := call.Routine == 0xd80c
	if !raise {
		owner, x, y := uint8(c.D[2]), uint8(c.D[0]), uint8(c.D[1])
		scenarioAt, mask := 0xeb2c, uint8(0x3f)
		if owner != 1 {
			scenarioAt, mask = 0xeb2e, 0x2f
		}
		scenario, e := m.Read16(scenarioAt)
		if e != nil {
			return false, e
		}
		commandWord(c, 4, scenario)
		if scenario&0x80 == 0 && int8(x) < 64 && int8(y) < 64 {
			word := uint16(y) << 6
			word = word&0xff00 | uint16(uint8(word)+x)
			word <<= 2
			head, e := m.Read16(0xf44 + int(int16(word)) + 2)
			if e != nil {
				return false, e
			}
			commandWord(c, 4, head)
			seen := map[uint16]bool{}
			for head != 0 {
				if seen[head] {
					return false, fmt.Errorf("cyclic native right-click town scan")
				}
				seen[head] = true
				at := cleanupRecordAddress(NativeRecordReference(head))
				kind, e := m.Read8(at)
				if e != nil {
					return false, e
				}
				targetOwner, e := m.Read8(at + 12)
				if e != nil {
					return false, e
				}
				if kind == 4 && owner == targetOwner {
					flags, e := m.Read8(at + 13)
					if e != nil {
						return false, e
					}
					if e := m.Write8(at+13, flags|4); e != nil {
						return false, e
					}
					c.D[0] = 1
					return false, nil
				}
				head, e = m.Read16(at + 2)
				if e != nil {
					return false, e
				}
				commandWord(c, 4, head)
			}
		}
		commandByte(c, 3, mask)
		commandWord(c, 4, scenario)
		if scenario&0x10 != 0 {
			c.D[0] = 0
			return true, nil
		}
		if scenario&4 == 0 {
			commandWord(c, 3, 0)
		}
	}
	mask := uint16(c.D[3])
	rules.Code[0xd80a], rules.Code[0xd80b] = uint8(mask>>8), uint8(mask)
	allowed, e := rules.PlanTerrain(raise, mask, c, m)
	if e != nil {
		return false, e
	}
	if allowed {
		if _, e := rules.DirectTerrainWithPoints(raise, c, m, w.nativeTerrainPoint); e != nil {
			return false, e
		}
		w.hydrateCommandTerrainHeights()
	}
	if !raise {
		c.D[0] = 1
	}
	return false, nil
}

// PlanTerrain is $d464/$d47c. Its17-by17 scratch is actual mutable CODE at
// $d6bc, and its recursive order differs from the later map edit. It checks
// all four parcel heads for raw kind$1a and the exact selected farm byte.
// The saved coordinate words are sign-extended by the original MOVEM.W.
func (r *NativeCommandRules) PlanTerrain(raise bool, mask uint16, c *NativeCommandRegisterContext, memory FollowerCleanupMemory) (bool, error) {
	if r == nil || c == nil || !winMemoryValid(memory) || len(r.Code) < 0xd80c {
		return false, fmt.Errorf("native terrain planner missing")
	}
	m := nativeWhirlwindMemory{m: memory}
	x, y := int(int16(uint16(c.D[0]))), int(int16(uint16(c.D[1])))
	height := func(xx, yy int) int {
		if xx < 0 || yy < 0 || xx > 64 || yy > 64 {
			return -1
		}
		bit := uint8(0)
		if xx == 64 {
			xx--
			bit = 1
		}
		if yy == 64 {
			yy--
			if bit == 1 {
				bit = 2
			} else {
				bit = 3
			}
		}
		at := 0xf44 + (xx+yy*64)*4
		tile := m.byte(at + 1)
		raster, e := r.byte(0x33512 + int(tile))
		if e != nil && m.err == nil {
			m.err = e
		}
		return int(m.byte(at)&7) + int(raster>>bit&1)
	}
	for yy := 0; yy < 17; yy++ {
		for xx := 0; xx < 17; xx++ {
			v := height(x-8+xx, y-8+yy)
			r.Code[0xd6bc+yy*17+xx] = uint8(v)
			c.D[2] = uint32(int32(v))
		}
	}
	c.D[6], c.D[7] = 0xffff, 0xffff
	checks := func(xx, yy int) (bool, error) {
		commandWord(c, 7, uint16((yy*64+xx)*4))
		for _, delta := range [4][2]int{{-1, -1}, {0, -1}, {-1, 0}, {0, 0}} {
			px, py := xx+delta[0], yy+delta[1]
			if !inside(px, py) {
				continue
			}
			at := 0xf44 + (px+py*64)*4
			commandWord(c, 5, mask)
			if mask != 0 && m.byte(at+1) == uint8(mask) {
				c.D[4] = 0xffffffff
				return false, m.err
			}
			head := m.word(at + 2)
			commandWord(c, 5, head)
			seen := map[uint16]bool{}
			for head != 0 {
				if seen[head] {
					return false, fmt.Errorf("cyclic native terrain wall scan")
				}
				seen[head] = true
				actor := cleanupRecordAddress(NativeRecordReference(head))
				if m.byte(actor) == 0x1a {
					c.D[4] = 0xffffffff
					return false, m.err
				}
				head = m.word(actor + 2)
				commandWord(c, 5, head)
			}
		}
		c.D[4] = 0
		return true, m.err
	}
	work := 0
	var plan func(int, int, int) (bool, error)
	plan = func(index, xx, yy int) (bool, error) {
		work++
		if work > 65536 {
			return false, fmt.Errorf("native terrain planner work exceeds bounded window")
		}
		at := 0xd6bc + index
		if at < 0 || at >= len(r.Code) {
			return false, fmt.Errorf("native terrain scratch CODE address%x unavailable", at)
		}
		before := c.D[2]
		defer func() { c.D[2] = before }()
		value := r.Code[at]
		commandByte(c, 2, value)
		if int8(value) < 0 {
			c.D[4] = 0
			return true, nil
		}
		if raise {
			value++
		} else {
			value--
			if int8(value) < 0 {
				c.D[4] = 0
				return true, nil
			}
		}
		commandByte(c, 2, value)
		r.Code[at] = value
		ok, e := checks(xx, yy)
		if e != nil || !ok {
			return ok, e
		}
		for _, delta := range [8][3]int{{-1, -1, 0}, {-18, -1, -1}, {-17, 0, -1}, {-16, 1, -1}, {1, 1, 0}, {18, 1, 1}, {17, 0, 1}, {16, -1, 1}} {
			neighborAt := at + delta[0]
			if neighborAt < 0 || neighborAt >= len(r.Code) {
				return false, fmt.Errorf("native terrain neighbor CODE%x unavailable", neighborAt)
			}
			neighbor := r.Code[neighborAt]
			commandByte(c, 3, neighbor)
			if int8(neighbor) < 0 {
				continue
			}
			difference := uint8(neighbor - value)
			if raise {
				difference = 0 - difference
			}
			commandByte(c, 3, difference)
			if int8(difference) > 1 {
				ok, e := plan(index+delta[0], xx+delta[1], yy+delta[2])
				if e != nil || !ok {
					return ok, e
				}
			}
		}
		c.D[4] = 0
		return true, m.err
	}
	ok, e := plan(144, x, y)
	c.D[0], c.D[1] = uint32(int32(int16(uint16(x)))), uint32(int32(int16(uint16(y))))
	return ok, e
}

type NativeCommandTerrainStep struct {
	Calls, ChangedVertices, Queued int
	MinX, MinY, MaxX, MaxY         int
}

// DirectTerrain is the original $d7f0/$d81e primitive, including raw parcel
// reconstruction, per-call dirty counts, vertex count and $d838 redraw queue.
// The recursive order and byte operations belong to CODE, not a height clamp
// or an inherited sculpting action. Incoming D0-D3 survive the point helper;
// the redraw suffix subsequently leaves its actual data-register outputs.
func (r *NativeCommandRules) DirectTerrain(raise bool, c *NativeCommandRegisterContext, memory FollowerCleanupMemory) (NativeCommandTerrainStep, error) {
	return r.DirectTerrainWithPoints(raise, c, memory, nil)
}

// DirectTerrainWithPoints exposes the real $d8b4/$e196 bitmap call. A caller
// supplies its actual BSS$22 target; the callback owns D0-D2 pixel-helper
// outputs before the source redraw loop advances D4/D5.
func (r *NativeCommandRules) DirectTerrainWithPoints(raise bool, c *NativeCommandRegisterContext, memory FollowerCleanupMemory, point func(*NativeCommandRegisterContext) error) (NativeCommandTerrainStep, error) {
	step := NativeCommandTerrainStep{MinX: 256, MinY: 256, MaxX: -1, MaxY: -1}
	if c == nil || !winMemoryValid(memory) {
		return step, fmt.Errorf("native terrain memory/context missing")
	}
	m := nativeWhirlwindMemory{m: memory}
	m.putWord(0xdd2, 0)
	c.D[6], c.D[7] = 0x01000100, 0
	raster := func(tile uint8) uint8 {
		v, e := r.byte(0x33512 + int(tile))
		if e != nil && m.err == nil {
			m.err = e
		}
		return v
	}
	height := func(x, y int) int {
		if x < 0 || y < 0 || x > 64 || y > 64 {
			return -1
		}
		bit := uint8(0)
		if x == 64 {
			x--
			bit = 1
		}
		if y == 64 {
			y--
			if bit == 1 {
				bit = 2
			} else {
				bit = 3
			}
		}
		at := 0xf44 + (x+y*64)*4
		return int(m.byte(at)&7) + int(raster(m.byte(at+1))>>bit&1)
	}
	var edit func(int, int) error
	edit = func(x, y int) error {
		step.Calls++
		if step.Calls > 65536 {
			return fmt.Errorf("native terrain recursive work exceeds bounded window")
		}
		m.putWord(0xf2e, m.word(0xf2e)+1)
		h := height(x, y)
		if h < 0 || !raise && h == 0 || raise && h == 8 {
			c.D[4] = 0xffffffff
			return m.err
		}
		wanted := h - 1
		if raise {
			wanted = h + 1
		}
		for _, delta := range [8][2]int{{0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}} {
			xx, yy := x+delta[0], y+delta[1]
			neighbor := height(xx, yy)
			if neighbor >= 0 && (!raise && neighbor-wanted > 1 || raise && wanted-neighbor > 1) {
				if e := edit(xx, yy); e != nil {
					return e
				}
			}
		}
		for _, corner := range [4][3]int{{-1, -1, 4}, {0, -1, 8}, {-1, 0, 2}, {0, 0, 1}} {
			xx, yy, bit := x+corner[0], y+corner[1], uint8(corner[2])
			if !inside(xx, yy) {
				continue
			}
			step.MinX = min(step.MinX, xx)
			step.MinY = min(step.MinY, yy)
			step.MaxX = max(step.MaxX, xx)
			step.MaxY = max(step.MaxY, yy)
			at := 0xf44 + (xx+yy*64)*4
			header := m.byte(at)
			shape := raster(m.byte(at + 1))
			c.D[5] = uint32(shape)
			if raise {
				if shape&bit == 0 {
					shape += bit
					commandByte(c, 5, shape)
				} else {
					header++
					shape &= 0xf0 | bit
					commandWord(c, 5, uint16(shape))
				}
			} else {
				decrement := false
				if shape&bit != 0 {
					shape -= bit
					commandByte(c, 5, shape)
					commandByte(c, 4, shape&15)
					if shape&15 == 0 {
						commandByte(c, 4, header&7)
						if header&7 != 0 {
							shape |= 15
							commandByte(c, 5, shape)
							decrement = true
						}
					}
				} else {
					shape &= ^bit
					commandWord(c, 5, uint16(shape))
					decrement = true
				}
				if decrement {
					header--
				}
			}
			m.putByte(at, header&7)
			m.putByte(at+1, shape)
		}
		commandWord(c, 4, uint16(wanted))
		step.ChangedVertices++
		m.putWord(0xdd2, m.word(0xdd2)+1)
		return m.err
	}
	if e := edit(int(int16(uint16(c.D[0]))), int(int16(uint16(c.D[1])))); e != nil {
		return step, e
	}
	if step.MinY == 256 {
		return step, m.err
	}
	c.D[6] = uint32(step.MinY)<<16 | uint32(step.MinX)
	c.D[7] = uint32(step.MaxX)<<16 | uint32(step.MaxY)
	commandWord(c, 5, uint16(step.MinY))
	c.D[1] = uint32(4 * (63 - (step.MaxX - step.MinX)))
	for y := step.MinY; y <= step.MaxY; y++ {
		commandWord(c, 4, uint16(step.MinX))
		for x := step.MinX; x <= step.MaxX; x++ {
			tile := m.byte(0xf45 + (x+y*64)*4)
			color, e := r.byte(0x33744 + int(tile))
			if e != nil {
				return step, e
			}
			c.D[2] = uint32(color)
			cursor := m.word(0xeb6e)
			at := 0xeb70 + int(int16(cursor))
			if at < 0x11280 {
				commandWord(c, 3, uint16(x*64+y))
				word := uint16(color)
				word = word>>4 | word<<12
				commandWord(c, 3, uint16(c.D[3])+word)
				m.putWord(at, uint16(c.D[3]))
				m.putWord(0xeb6e, m.word(0xeb6e)+2)
				step.Queued++
			}
			pixelX, pixelY := uint16(68+x-y), uint16((x+y)/2+4)
			c.D[0] = uint32(pixelX)
			commandWord(c, 1, pixelY)
			if point != nil {
				if m.err != nil {
					return step, m.err
				}
				if e := point(c); e != nil {
					return step, e
				}
			} else {
				commandWord(c, 0, (uint16(c.D[0])&7)^7)
				commandWord(c, 1, pixelX>>3)
				commandWord(c, 2, (uint16(c.D[2])&15)<<4)
			}
			commandWord(c, 4, uint16(x+1))
		}
		commandWord(c, 5, uint16(y+1))
	}
	return step, m.err
}

func (w *World) commandDirectTerrain(call NativeCommandCall, raise bool) (bool, error) {
	rules := NativeCommandRules{Code: w.NativeAI.Code}
	_, e := rules.DirectTerrainWithPoints(raise, call.Context, w.nativeCleanupMemory(), w.nativeTerrainPoint)
	if e != nil {
		return false, e
	}
	w.hydrateCommandTerrainHeights()
	return false, nil
}

func (w *World) hydrateCommandTerrainHeights() {
	for y := 0; y <= 64; y++ {
		for x := 0; x <= 64; x++ {
			cx, cy, bit := x, y, uint8(0)
			if cx == 64 {
				cx--
				bit = 1
			}
			if cy == 64 {
				cy--
				if bit == 1 {
					bit = 2
				} else {
					bit = 3
				}
			}
			cell := w.Occupancy.Grid.Cells[cx+cy*64]
			w.Core.Alt[x+y*65] = int(cell.Header&7) + int(w.NativeAI.Raster[cell.Tile]>>bit&1)
		}
	}
}
