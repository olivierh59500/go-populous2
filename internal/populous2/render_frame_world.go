package populous2

import "fmt"

type NativeWorldRenderState struct {
	ProjectionX, ProjectionY uint16
	Actor                    NativeActorRenderState
}
type NativeWorldRenderCallbacks struct {
	Effects    NativeActorEffectsCallbacks
	Tiles      *NativeTileBitmapBank
	Tile       func(NativeTileChunkRequest, []byte) error
	Background []byte // Original bitmap$22, distinct from the draw buffer$1e.
	Children   NativeActorRenderChildren
	// ResolveTargets refreshes current global actor image targets after a modal.
	// The retained A6 tile bitmap remains distinct.
	ResolveTargets func(*NativeWorldRenderCallbacks) error
}

type NativeWorldRenderPlan struct {
	NativeActorEffectsPlan
	TileRequests []NativeTileChunkRequest
	Actors       []NativeRecordReference
}

func (r *NativeActorRenderRules) overlayCell(grid int, cb NativeWorldRenderCallbacks, state *NativeWorldRenderState, p *NativeWorldRenderPlan) error {
	c, m := cb.Effects.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Effects.Memory}}
	c.D[0] = c.AddressBase + uint32(grid)
	c.D[0] -= c.AddressBase + 0xf44
	c.Word(0, uint16(c.D[0])>>2)
	c.Byte(0, m.byte(0x4f44+int(int16(c.D[0]))))
	if uint16(c.D[0])&255 == 0 {
		return m.err
	}
	saved := c.D
	c.Word(0, uint16(c.D[0])&255)
	c.Word(0, uint16(c.D[0])*2)
	image, e := r.word(0x137e4 + int(int16(c.D[0])))
	if e != nil {
		return e
	}
	c.Word(0, uint16(c.D[6])<<3)
	c.Word(1, uint16(c.D[7])<<3)
	c.Word(4, uint16(c.D[1])+uint16(c.D[0]))
	c.Word(0, (uint16(c.D[0])-uint16(c.D[1]))*2)
	c.Word(1, uint16(c.D[4]))
	c.Word(0, uint16(c.D[0])+state.ProjectionX)
	c.Word(1, uint16(c.D[1])+state.ProjectionY+8)
	c.D[2] = uint32(m.byte(grid-2)&7) * 8
	c.Word(1, uint16(c.D[1])-uint16(c.D[2]))
	c.Word(2, image)
	q := NativeRenderFramePlan{}
	if e := r.image(cb.Effects.NativeRenderFrameCallbacks, &q); e != nil {
		return e
	}
	p.Sprites = append(p.Sprites, q.Sprites...)
	p.HardwarePending = p.HardwarePending || q.HardwarePending
	for reg := 1; reg < 8; reg++ {
		c.D[reg] = saved[reg]
	}
	return m.err
}

// ProjectActor is the actual$e35e fractional position/slope producer. Its
// wrapper restores all eight incoming data longs after the real actor child.
func (r *NativeActorRenderRules) projectActorCoordinates(at, grid int, cb NativeWorldRenderCallbacks, state *NativeWorldRenderState) error {
	c, m := cb.Effects.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Effects.Memory}}
	c.D[4] = uint32(m.byte(at + 7))
	c.D[5] = uint32(m.byte(at + 9))
	c.Word(2, uint16(c.D[4])-uint16(c.D[5]))
	c.Word(2, uint16(int16(c.D[2])>>4))
	c.D[0] = uint32(m.byte(grid - 3))
	tile := uint16(c.D[0])
	raster, e := r.Frames.byte(0x33512 + int(tile))
	if e != nil {
		return e
	}
	c.Byte(0, raster)
	c.Word(0, uint16(c.D[0])&15)
	c.Word(0, uint16(c.D[0])*2)
	offset, e := r.word(0xe392 + int(int16(c.D[0])))
	if e != nil {
		return e
	}
	c.Word(0, offset)
	target := 0xe392 + int(int16(c.D[0]))
	x, y := uint16(c.D[4]), uint16(c.D[5])
	carry := uint16(uint8(x))+uint16(uint8(y)) > 255
	// Execute the original small branch graph by its concrete source labels.
	for steps := 0; steps < 8; steps++ {
		switch target {
		case 0xe3b2:
			c.Byte(0, uint8(x)+uint8(y))
			if carry {
				target = 0xe3f8
			} else {
				target = 0xe3b8
			}
		case 0xe3c4:
			if int16(y) < int16(x) {
				target = 0xe3b8
			} else {
				target = 0xe3c8
			}
		case 0xe3d4:
			c.Byte(0, uint8(x)+uint8(y))
			if carry {
				target = 0xe3e0
			} else {
				target = 0xe3c8
			}
		case 0xe3dc:
			if int16(y) < int16(x) {
				target = 0xe3f8
			} else {
				target = 0xe3e0
			}
		case 0xe3e6:
			if int16(y) < int16(x) {
				target = 0xe3f2
			} else {
				c.Byte(0, uint8(x)+uint8(y))
				if carry {
					target = 0xe3c8
				} else {
					target = 0xe3e0
				}
			}
		case 0xe3ea:
			c.Byte(0, uint8(x)+uint8(y))
			if carry {
				target = 0xe3c8
			} else {
				target = 0xe3e0
			}
		case 0xe406:
			if int16(y) < int16(x) {
				target = 0xe3c8
			} else {
				target = 0xe3b8
			}
		case 0xe40c:
			if int16(y) < int16(x) {
				target = 0xe3e0
			} else {
				target = 0xe3f8
			}
		case 0xe3f2:
			c.Byte(0, uint8(x)+uint8(y))
			if carry {
				target = 0xe3b8
			} else {
				target = 0xe3f8
			}
		case 0xe3fe:
			c.Byte(0, uint8(x)+uint8(y))
			if carry {
				if int16(y) < int16(x) {
					target = 0xe3e0
				} else {
					target = 0xe3f8
				}
			} else {
				if int16(y) < int16(x) {
					target = 0xe3c8
				} else {
					target = 0xe3b8
				}
			}
		case 0xe3b8:
			c.Word(3, uint16(int16(x)>>1)+y)
			c.Word(3, uint16(int16(c.D[3])>>4))
			c.Word(3, uint16(c.D[3])-8)
			target = 0xe422
		case 0xe3c8:
			c.Word(3, uint16(int16(y)>>1)+x)
			c.Word(3, uint16(int16(c.D[3])>>4))
			c.Word(3, uint16(c.D[3])-8)
			target = 0xe422
		case 0xe3e0:
			c.Word(3, uint16(int16(x)>>5))
			target = 0xe422
		case 0xe3f8:
			c.Word(3, uint16(int16(y)>>5))
			target = 0xe422
		case 0xe412:
			c.Word(3, x+y)
			c.Word(3, uint16(int16(c.D[3])>>5))
			target = 0xe422
		case 0xe41a:
			c.Word(3, x+y)
			c.Word(3, uint16(int16(c.D[3])>>5))
			c.Word(3, uint16(c.D[3])-8)
			target = 0xe422
		case 0xe422:
			steps = 8
		default:
			return fmt.Errorf("native projection branch%#x outside original graph", target)
		}
	}
	c.Word(0, uint16(c.D[6])<<3)
	c.Word(1, uint16(c.D[7])<<3)
	c.Word(4, uint16(c.D[1])+uint16(c.D[0]))
	c.Word(0, (uint16(c.D[0])-uint16(c.D[1]))*2)
	c.Word(1, uint16(c.D[4]))
	c.Word(0, uint16(c.D[0])+uint16(c.D[2])+state.ProjectionX)
	c.Word(1, uint16(c.D[1])+uint16(c.D[3])+state.ProjectionY)
	c.D[2] = uint32(m.byte(grid-4)&7) * 8
	c.Word(1, uint16(c.D[1])-uint16(c.D[2]))
	return m.err
}

func (r *NativeActorRenderRules) ProjectActor(at, grid int, cb NativeWorldRenderCallbacks, state *NativeWorldRenderState) (NativeActorEffectsPlan, error) {
	p := NativeActorEffectsPlan{}
	saved := cb.Effects.Frame.D
	if e := r.projectActorCoordinates(at, grid, cb, state); e != nil {
		return p, e
	}
	actor := cb.Effects
	actor.GridCursorAddress = grid
	p, e := r.Actor(at, actor, &state.Actor, cb.Children)
	if e != nil {
		return p, e
	}
	cb.Effects.Frame.D = saved
	return p, nil
}

// WorldDraw is genuine$bbe0's 8x8 ordered grid/list walk. It preserves the
// original whole-register wrapper, raw headers and tail-to-head actor order.
func (r *NativeActorRenderRules) WorldDraw(cb NativeWorldRenderCallbacks, state *NativeWorldRenderState) (NativeWorldRenderPlan, error) {
	p := NativeWorldRenderPlan{NativeActorEffectsPlan: NativeActorEffectsPlan{NativeRenderFramePlan: NativeRenderFramePlan{Drawn: true}}, TileRequests: []NativeTileChunkRequest{}, Actors: []NativeRecordReference{}}
	if r == nil || state == nil || cb.Effects.Frame == nil || !winMemoryValid(cb.Effects.Memory) || cb.Tiles == nil || len(cb.Effects.Bitmap) != 32000 || len(cb.Background) != 32000 {
		return p, fmt.Errorf("native world renderer backing missing")
	}
	c, m := cb.Effects.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Effects.Memory}}
	saved := c.D
	state.ProjectionX, state.ProjectionY = 192, 72
	c.RestoreWord(0, m.word(0x5f44))
	c.RestoreWord(1, m.word(0x5f46))
	c.Word(2, uint16(c.D[0]))
	c.Word(3, uint16(c.D[1]))
	c.Word(3, (uint16(c.D[3])<<6)+uint16(c.D[2]))
	c.Word(3, uint16(c.D[3])*4)
	grid := 0xf44 + int(int16(c.D[3]))
	camX, camY := uint16(c.D[0]), uint16(c.D[1])
	for side := 0; side < 2; side++ {
		c.RestoreWord(0, camX)
		c.RestoreWord(1, camY)
		if side == 0 {
			c.Word(1, uint16(c.D[1])+8)
		} else {
			c.Word(0, uint16(c.D[0])+8)
		}
		if e := r.Frames.TerrainHeight(cb.Effects.Memory, c); e != nil {
			return p, e
		}
		c.Word(1, uint16(c.D[2])*8-1)
		if int16(c.D[1]) > 0 {
			rows := int(uint16(c.D[1]))
			delta := (64 - rows) * 40
			src, dst := 0x1028+delta, 0xb20+delta
			if side != 0 {
				src, dst = 0x1030+delta, 0xb38+delta
			}
			for plane := 0; plane < 4; plane++ {
				for row := 0; row < rows; row++ {
					a, b := src+plane*8000+row*40, dst+plane*8000+row*40
					if a < 0 || a+8 > 32000 || b < 0 || b+8 > 32000 {
						return p, fmt.Errorf("native backdrop strip outside retained bitmap")
					}
					copy(cb.Effects.Bitmap[b:b+8], cb.Background[a:a+8])
				}
			}
		}
	}
	destination := 0xa16
	c.D[7] = 0
	for row := 0; row < 8; row++ {
		c.D[6] = 0
		for column := 0; column < 8; column++ {
			header, tile := m.byte(grid), m.byte(grid+1)
			c.D[4] = 0
			c.Byte(4, header*2)
			c.Byte(4, uint8(c.D[4])&14)
			height, e := r.word(0xbe36 + int(int16(c.D[4])))
			if e != nil {
				return p, e
			}
			c.Word(4, height)
			c.D[5] = uint32(tile)
			if tile <= 15 && height != 0 {
				c.Byte(5, tile+16)
			}
			if int8(tile) < 0 {
				switch tile {
				case 168:
					c.Byte(0, m.byte(0xf43)+uint8(c.D[6])+uint8(c.D[7]))
					c.Word(0, uint16(c.D[0])&3)
					c.Word(5, uint16(c.D[5])+uint16(c.D[0]))
				case 143:
					c.Byte(0, m.byte(0xf43)+uint8(c.D[6])+uint8(c.D[7]))
					c.Word(0, uint16(c.D[0])&1)
					c.Word(5, uint16(c.D[5])+uint16(c.D[0]))
				case 220:
					c.Byte(0, m.byte(0xf43))
					c.Word(0, uint16(c.D[0])&3)
					c.D[0] = uint32(uint16(c.D[0])) * 40
					c.Word(4, uint16(c.D[4])-uint16(c.D[0]))
					c.Byte(0, m.byte(0xf43)+uint8(c.D[6])+uint8(c.D[7]))
					c.Word(0, uint16(c.D[0])&3)
					c.Word(5, uint16(c.D[5])+uint16(c.D[0]))
				}
			} else if uint16(c.D[5]) < 15 {
				c.Byte(0, m.byte(0xf43)+uint8(c.D[6])+uint8(c.D[7]))
				c.Word(0, uint16(c.D[0])&7)
				if uint16(c.D[0]) != 0 {
					c.Word(0, uint16(c.D[0])+1)
				}
				c.Word(0, uint16(c.D[0])<<4)
				c.Word(5, uint16(c.D[5])+uint16(c.D[0]))
			}
			selected := int(uint16(c.D[5]))
			if selected < 0 || selected >= len(cb.Tiles.Descriptors) {
				return p, fmt.Errorf("native tile index%d needs adjacent BLOCK descriptors", selected)
			}
			base := destination - int(int16(c.D[4]))
			for part, offset := range cb.Tiles.Descriptors[selected] {
				request := NativeTileChunkRequest{SourceOffset: offset, DestinationOffset: base + (part/2)*320 + (part%2)*2}
				if offset != 0 {
					p.TileRequests = append(p.TileRequests, request)
					if cb.Tile == nil {
						p.HardwarePending = true
					} else if e := cb.Tile(request, cb.Effects.Bitmap); e != nil {
						return p, e
					}
				}
			}
			grid += 2
			if e := r.overlayCell(grid, cb, state, &p); e != nil {
				return p, e
			}
			head := m.word(grid)
			grid += 2
			if head != 0 {
				ref := NativeRecordReference(head)
				seen := map[NativeRecordReference]bool{}
				for {
					if seen[ref] {
						return p, fmt.Errorf("native world next-link cycle")
					}
					seen[ref] = true
					next := m.word(cleanupRecordAddress(ref) + 2)
					if next == 0 {
						break
					}
					ref = NativeRecordReference(next)
				}
				seen = map[NativeRecordReference]bool{}
				for {
					if seen[ref] {
						return p, fmt.Errorf("native world previous-link cycle")
					}
					seen[ref] = true
					p.Actors = append(p.Actors, ref)
					q, e := r.ProjectActor(cleanupRecordAddress(ref), grid, cb, state)
					if e != nil {
						return p, e
					}
					p.Sprites = append(p.Sprites, q.Sprites...)
					p.Crops = append(p.Crops, q.Crops...)
					p.Pixels = append(p.Pixels, q.Pixels...)
					p.HardwarePending = p.HardwarePending || q.HardwarePending
					previous := m.word(cleanupRecordAddress(ref) + 4)
					if previous == 0 {
						break
					}
					ref = NativeRecordReference(previous)
				}
			}
			destination += 322
			c.Word(6, uint16(c.D[6])+1)
		}
		destination += 318 - 8*322
		grid += 224
		c.Word(7, uint16(c.D[7])+1)
	}
	c.D = saved
	return p, m.err
}

func (r *NativeActorRenderRules) beginWorldDraw(cb NativeWorldRenderCallbacks, state *NativeWorldRenderState, s *NativeWorldRenderContinuation) error {
	c, m := cb.Effects.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Effects.Memory}}

	state.ProjectionX, state.ProjectionY = 192, 72
	c.RestoreWord(0, m.word(0x5f44))
	c.RestoreWord(1, m.word(0x5f46))
	c.Word(2, uint16(c.D[0]))
	c.Word(3, uint16(c.D[1]))
	c.Word(3, (uint16(c.D[3])<<6)+uint16(c.D[2]))
	c.Word(3, uint16(c.D[3])*4)
	grid := 0xf44 + int(int16(c.D[3]))
	camX, camY := uint16(c.D[0]), uint16(c.D[1])
	for side := 0; side < 2; side++ {
		c.RestoreWord(0, camX)
		c.RestoreWord(1, camY)
		if side == 0 {
			c.Word(1, uint16(c.D[1])+8)
		} else {
			c.Word(0, uint16(c.D[0])+8)
		}
		if e := r.Frames.TerrainHeight(cb.Effects.Memory, c); e != nil {
			return e
		}
		c.Word(1, uint16(c.D[2])*8-1)
		if int16(c.D[1]) > 0 {
			rows := int(uint16(c.D[1]))
			delta := (64 - rows) * 40
			src, dst := 0x1028+delta, 0xb20+delta
			if side != 0 {
				src, dst = 0x1030+delta, 0xb38+delta
			}
			for plane := 0; plane < 4; plane++ {
				for row := 0; row < rows; row++ {
					a, b := src+plane*8000+row*40, dst+plane*8000+row*40
					if a < 0 || a+8 > 32000 || b < 0 || b+8 > 32000 {
						return fmt.Errorf("native backdrop strip outside retained bitmap")
					}
					copy(cb.Effects.Bitmap[b:b+8], cb.Background[a:a+8])
				}
			}
		}
	}
	s.Grid = grid
	s.Destination = 0xa16
	c.D[7] = 0
	c.D[6] = 0
	return m.err
}

func (r *NativeActorRenderRules) worldDrawCell(cb NativeWorldRenderCallbacks, state *NativeWorldRenderState, s *NativeWorldRenderContinuation, p *NativeWorldRenderPlan) error {
	c, m := cb.Effects.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Effects.Memory}}
	grid, destination := s.Grid, s.Destination

	header, tile := m.byte(grid), m.byte(grid+1)
	c.D[4] = 0
	c.Byte(4, header*2)
	c.Byte(4, uint8(c.D[4])&14)
	height, e := r.word(0xbe36 + int(int16(c.D[4])))
	if e != nil {
		return e
	}
	c.Word(4, height)
	c.D[5] = uint32(tile)
	if tile <= 15 && height != 0 {
		c.Byte(5, tile+16)
	}
	if int8(tile) < 0 {
		switch tile {
		case 168:
			c.Byte(0, m.byte(0xf43)+uint8(c.D[6])+uint8(c.D[7]))
			c.Word(0, uint16(c.D[0])&3)
			c.Word(5, uint16(c.D[5])+uint16(c.D[0]))
		case 143:
			c.Byte(0, m.byte(0xf43)+uint8(c.D[6])+uint8(c.D[7]))
			c.Word(0, uint16(c.D[0])&1)
			c.Word(5, uint16(c.D[5])+uint16(c.D[0]))
		case 220:
			c.Byte(0, m.byte(0xf43))
			c.Word(0, uint16(c.D[0])&3)
			c.D[0] = uint32(uint16(c.D[0])) * 40
			c.Word(4, uint16(c.D[4])-uint16(c.D[0]))
			c.Byte(0, m.byte(0xf43)+uint8(c.D[6])+uint8(c.D[7]))
			c.Word(0, uint16(c.D[0])&3)
			c.Word(5, uint16(c.D[5])+uint16(c.D[0]))
		}
	} else if uint16(c.D[5]) < 15 {
		c.Byte(0, m.byte(0xf43)+uint8(c.D[6])+uint8(c.D[7]))
		c.Word(0, uint16(c.D[0])&7)
		if uint16(c.D[0]) != 0 {
			c.Word(0, uint16(c.D[0])+1)
		}
		c.Word(0, uint16(c.D[0])<<4)
		c.Word(5, uint16(c.D[5])+uint16(c.D[0]))
	}
	selected := int(uint16(c.D[5]))
	if selected < 0 || selected >= len(cb.Tiles.Descriptors) {
		return fmt.Errorf("native tile index%d needs adjacent BLOCK descriptors", selected)
	}
	base := destination - int(int16(c.D[4]))
	for part, offset := range cb.Tiles.Descriptors[selected] {
		request := NativeTileChunkRequest{SourceOffset: offset, DestinationOffset: base + (part/2)*320 + (part%2)*2}
		if offset != 0 {
			p.TileRequests = append(p.TileRequests, request)
			if cb.Tile == nil {
				p.HardwarePending = true
			} else if e := cb.Tile(request, s.TileTarget); e != nil {
				return e
			}
		}
	}
	grid += 2
	if e := r.overlayCell(grid, cb, state, p); e != nil {
		return e
	}
	s.Grid = grid
	return m.err
}
