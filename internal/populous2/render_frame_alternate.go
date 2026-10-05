package populous2

import "fmt"

type NativeAlternateRenderState struct {
	Scratch [5]uint16
	World   NativeWorldRenderState
}

// AlternateClear is$ c826's native planar pattern fill. It does not clear
// caller registers; the final DBF leaves MOVEQ7's D0 low wordFFFF.
func (r *NativeRenderFrameRules) AlternateClear(cb NativeRenderFrameCallbacks) error {
	if r == nil || cb.Frame == nil || len(cb.Bitmap) != 32000 {
		return fmt.Errorf("native alternate clear backing missing")
	}
	for i := 8000; i < 32000; i++ {
		cb.Bitmap[i] = 0
	}
	for row := 0; row < 200; row++ {
		pattern, e := r.word(0xc816 + (row&7)*2)
		if e != nil {
			return e
		}
		for word := 0; word < 20; word++ {
			at := row*40 + word*2
			cb.Bitmap[at], cb.Bitmap[at+1] = byte(pattern>>8), byte(pattern)
		}
	}
	cb.Frame.D[0] = 0x0000ffff
	return nil
}

// AlternateDraw is the original variable-grid$ c204 traversal. Mutable
// CODEC132..C13A and projection anchors remain state, not guessed viewport
// rectangles. Source clip dispatch selects full/left/right half tile pairs.
func (r *NativeActorRenderRules) AlternateDraw(cb NativeWorldRenderCallbacks, state *NativeAlternateRenderState) (NativeWorldRenderPlan, error) {
	p := NativeWorldRenderPlan{NativeActorEffectsPlan: NativeActorEffectsPlan{NativeRenderFramePlan: NativeRenderFramePlan{Drawn: true}}, Actors: []NativeRecordReference{}, TileRequests: []NativeTileChunkRequest{}}
	if r == nil || state == nil || cb.Effects.Frame == nil || !winMemoryValid(cb.Effects.Memory) || cb.Tiles == nil || len(cb.Effects.Bitmap) != 32000 {
		return p, fmt.Errorf("native alternate draw backing missing")
	}
	c, m := cb.Effects.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Effects.Memory}}
	saved := c.D
	view := uint16(c.D[0])
	c.Word(1, view<<3)
	params := [4]uint16{}
	for i := range params {
		v, e := r.word(0xc13c + int(int16(c.D[1])) + i*2)
		if e != nil {
			return p, e
		}
		params[i] = v
	}
	c.RestoreWord(2, params[0])
	c.RestoreWord(3, params[1])
	c.RestoreWord(5, params[2])
	c.RestoreWord(6, params[3])
	c.Word(1, view)
	state.Scratch = [5]uint16{0, view, view, uint16(c.D[2]), uint16(c.D[3])}
	state.World.ProjectionX, state.World.ProjectionY = 160, uint16(c.D[6])
	projectionX, projectionY := state.World.ProjectionX, state.World.ProjectionY
	c.Word(2, view>>1)
	c.Word(2, uint16(c.D[2])-4)
	c.RestoreWord(0, m.word(0x5f44))
	c.RestoreWord(1, m.word(0x5f46))
	c.Word(0, uint16(c.D[0])-uint16(c.D[2]))
	if int16(c.D[0]) < 0 {
		c.Word(3, uint16(c.D[0]))
		state.Scratch[0] -= uint16(c.D[3])
		c.Word(3, -uint16(c.D[3]))
		state.Scratch[1] -= uint16(c.D[3])
		c.Word(3, uint16(c.D[3])*2)
		state.Scratch[3] += uint16(c.D[3])
		c.Word(5, uint16(c.D[5])+uint16(c.D[3]))
		c.Word(3, uint16(c.D[3])*2)
		state.Scratch[4] += uint16(c.D[3])
		c.Word(3, uint16(c.D[3])*2)
		state.World.ProjectionY += uint16(c.D[3])
		c.Word(3, uint16(c.D[3])*2)
		state.World.ProjectionX += uint16(c.D[3])
		c.Word(3, uint16(c.D[3])*4)
		c.Word(5, uint16(c.D[5])+uint16(c.D[3]))
		state.Scratch[3] += uint16(c.D[3])
		c.Word(3, uint16(c.D[3])*4)
		c.Word(5, uint16(c.D[5])+uint16(c.D[3]))
		state.Scratch[3] += uint16(c.D[3])
		c.D[0] = 0
	}
	c.Word(3, uint16(c.D[0])+state.Scratch[1])
	c.Word(3, uint16(c.D[3])-64)
	if int16(c.D[3]) > 0 {
		state.Scratch[1] -= uint16(c.D[3])
		c.Word(3, uint16(c.D[3])*2)
		state.Scratch[3] += uint16(c.D[3])
		c.Word(3, uint16(c.D[3])*2)
		state.Scratch[4] += uint16(c.D[3])
		c.Word(3, uint16(c.D[3])<<4)
		state.Scratch[3] += uint16(c.D[3])
		c.Word(3, uint16(c.D[3])*4)
		state.Scratch[3] += uint16(c.D[3])
	}
	c.Word(1, uint16(c.D[1])-uint16(c.D[2]))
	if int16(c.D[1]) < 0 {
		c.Word(3, uint16(c.D[1]))
		state.Scratch[0] += uint16(c.D[3])
		c.Word(3, -uint16(c.D[3]))
		state.Scratch[2] -= uint16(c.D[3])
		c.Word(3, uint16(c.D[3])*2)
		c.Word(5, uint16(c.D[5])-uint16(c.D[3]))
		c.Word(3, uint16(c.D[3])*4)
		state.World.ProjectionY += uint16(c.D[3])
		c.Word(3, uint16(c.D[3])*2)
		state.World.ProjectionX -= uint16(c.D[3])
		c.Word(3, uint16(c.D[3])*4)
		c.Word(5, uint16(c.D[5])+uint16(c.D[3]))
		c.Word(3, uint16(c.D[3])*4)
		c.Word(5, uint16(c.D[5])+uint16(c.D[3]))
		c.D[1] = 0
	}
	c.Word(2, uint16(c.D[0]))
	c.Word(3, (uint16(c.D[1])<<6)+uint16(c.D[2]))
	c.Word(3, uint16(c.D[3])*4)
	grid := 0xf44 + int(int16(c.D[3]))
	destination := int(int16(c.D[5]))
	c.D[7] = 0
	if state.Scratch[1] == 0 || state.Scratch[2] == 0 || state.Scratch[1] > 64 || state.Scratch[2] > 64 {
		return p, fmt.Errorf("native alternate view dimensions need original bounded input")
	}
	for row := 0; row < int(state.Scratch[2]); row++ {
		if grid >= 0x4f44 {
			break
		}
		c.D[6] = 0
		for column := 0; column < int(state.Scratch[1]); column++ {
			header, tile := m.byte(grid), m.byte(grid+1)
			c.D[4] = 0
			c.Byte(4, header*2)
			c.Byte(4, uint8(c.D[4])&14)
			height, e := r.word(0xc36e + int(int16(c.D[4])))
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
			if selected >= len(cb.Tiles.Descriptors) {
				return p, fmt.Errorf("native alternate tile descriptor alias unsupported")
			}
			c.Word(0, uint16(c.D[6])-uint16(c.D[7])+state.Scratch[0])
			c.Word(0, uint16(c.D[0])*2)
			v, e := r.word(0xc47e + int(int16(c.D[0])))
			if e != nil {
				return p, e
			}
			c.Word(0, v)
			target := 0xc47e + int(int16(c.D[0]))
			skip := target == 0xc4ae
			if !skip {
				if target != 0xc4b8 && target != 0xc4ca && target != 0xc4dc {
					return p, fmt.Errorf("native alternate tile clip branch%#x unsupported", target)
				}
				base := destination - int(int16(c.D[4]))
				for part, offset := range cb.Tiles.Descriptors[selected] {
					pair := part / 2
					half := part % 2
					at := base + pair*320 + half*2
					if target == 0xc4b8 && half != 0 {
						continue
					}
					if target == 0xc4ca && half != 1 {
						continue
					}
					checkOffset := base + pair*320
					if target == 0xc4ca {
						checkOffset += 2
					}
					if checkOffset < 0 || checkOffset >= 8000 {
						continue
					}
					if offset != 0 {
						request := NativeTileChunkRequest{SourceOffset: offset, DestinationOffset: at}
						p.TileRequests = append(p.TileRequests, request)
						if cb.Tile == nil {
							p.HardwarePending = true
						} else if e := cb.Tile(request, cb.Effects.Bitmap); e != nil {
							return p, e
						}
					}
				}
			}
			grid += 2
			if !skip {
				if e := r.overlayCell(grid, cb, &state.World, &p); e != nil {
					return p, e
				}
				head := m.word(grid)
				grid += 2
				if head != 0 {
					ref := NativeRecordReference(head)
					seen := map[NativeRecordReference]bool{}
					for {
						if seen[ref] {
							return p, fmt.Errorf("native alternate next-link cycle")
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
							return p, fmt.Errorf("native alternate previous-link cycle")
						}
						seen[ref] = true
						p.Actors = append(p.Actors, ref)
						q, e := r.ProjectActor(cleanupRecordAddress(ref), grid, cb, &state.World)
						if e != nil {
							return p, e
						}
						p.Sprites = append(p.Sprites, q.Sprites...)
						p.Crops = append(p.Crops, q.Crops...)
						p.Pixels = append(p.Pixels, q.Pixels...)
						previous := m.word(cleanupRecordAddress(ref) + 4)
						if previous == 0 {
							break
						}
						ref = NativeRecordReference(previous)
					}
				}
			} else {
				grid += 2
			}
			destination += 322
			c.Word(6, uint16(c.D[6])+1)
		}
		destination += int(int16(state.Scratch[3]))
		grid += int(int16(state.Scratch[4]))
		c.Word(7, uint16(c.D[7])+1)
	}
	state.World.ProjectionX, state.World.ProjectionY = projectionX, projectionY
	c.D = saved
	return p, m.err
}
