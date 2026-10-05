package populous2

import (
	"encoding/binary"
	"fmt"
	"math/bits"

	"go-populous2/internal/amiga"
)

// NativeRenderFrameRules retains original rendering tables. The actual image
// counter bank belongs to the caller and is shared with the audio consumer.
type NativeRenderFrameRules struct {
	code     []byte
	Commands NativeCommandRules
	Images   NativeEditorCursorRules
}

type NativeRenderFrameCallbacks struct {
	Memory FollowerCleanupMemory
	Frame  *NativeFrameRegisterContext
	Image  *NativeImageRenderState
	Input  *NativeInputState // Mutable CODE$ a2a cursor selector, separate from BSS.
	Bitmap []byte            // The actual four contiguous8000-byte planes, never an invented background.
	// Sprite is the original hardware boundary. Nil returns the request in
	// the plan and marks HardwarePending; it does not claim a completed blit.
	Sprite func(NativePresentationSprite, []byte) error
}

type NativeRenderFramePlan struct {
	Drawn, HardwarePending bool
	Pixels                 []NativeHUDPixel
	Sprites                []NativePresentationSprite
}

func DecodeNativeRenderFrameRules(exe *amiga.Executable) (NativeRenderFrameRules, error) {
	var r NativeRenderFrameRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33c68 {
		return r, fmt.Errorf("native render frame CODE tables missing")
	}
	r.code = exe.Hunks[0].Data
	var err error
	r.Commands, err = DecodeNativeCommandRules(exe)
	if err != nil {
		return r, err
	}
	r.Images, err = DecodeNativeEditorCursorRules(exe)
	return r, err
}

func (r *NativeRenderFrameRules) word(a int) (uint16, error) {
	if r == nil || a < 0 || a&1 != 0 || a+2 > len(r.code) {
		return 0, fmt.Errorf("native rendering word outside/unaligned CODE %#x", a)
	}
	return binary.BigEndian.Uint16(r.code[a:]), nil
}

// renderFramePixel executes $e17a/$e196. Checked drawing returns without any
// assignment outside the screen; the unchecked bar child uses actual valid
// table coordinates. D0/D1/D2 retain their native bit/column/color outputs.
func renderFramePixel(cb NativeRenderFrameCallbacks, p *NativeRenderFramePlan, checked bool) error {
	c := cb.Frame
	x, y := int16(c.D[0]), int16(c.D[1])
	if checked && (x < 0 || x >= 320 || y < 0 || y >= 200) {
		return nil
	}
	if x < 0 || x >= 320 || y < 0 || y >= 200 || len(cb.Bitmap) != 32000 {
		return fmt.Errorf("native rendering pixel outside supplied bitmap")
	}
	color := uint16(c.D[2]) & 15
	p.Pixels = append(p.Pixels, NativeHUDPixel{X: x, Y: y, Color: color})
	point, err := PlanNativeMapPoint(c)
	if err != nil {
		return err
	}
	return point.Paint(cb.Bitmap)
}

// TerrainHeight is $d2b4, including all four boundary-corner reads at64.
// Only D2 survives; the source saves/restores D0/D1/D3 as complete longs.
func (r *NativeRenderFrameRules) TerrainHeight(m FollowerCleanupMemory, c *NativeFrameRegisterContext) error {
	if r == nil || c == nil || m.Read8 == nil {
		return fmt.Errorf("native terrain height backing/frame missing")
	}
	x, y := int16(c.D[0]), int16(c.D[1])
	if x < 0 || y < 0 || x > 64 || y > 64 {
		c.D[2] = 0xffffffff
		return nil
	}
	corner := uint8(0)
	if x == 64 {
		x--
		corner = 1
		if y == 64 {
			y--
			corner = 2
		}
	} else if y == 64 {
		y--
		corner = 3
	}
	grid := 0xf44 + (int(x)+int(y)*64)*4
	header, err := m.Read8(grid)
	if err != nil {
		return err
	}
	c.D[2] = uint32(header & 7)
	tile, err := m.Read8(grid + 1)
	if err != nil {
		return err
	}
	if 0x33512+int(tile) >= len(r.code) {
		return fmt.Errorf("native corner raster unavailable")
	}
	if r.code[0x33512+int(tile)]&(1<<corner) != 0 {
		c.Word(2, uint16(c.D[2])+1)
	}
	return nil
}

// TerrainAdmission is the real $d91a cursor/terrain gate, not the spell mana
// admission routine. Original deity rule bits are byte$4b bits0 and1.
func (r *NativeRenderFrameRules) TerrainAdmission(m FollowerCleanupMemory, c *NativeFrameRegisterContext) error {
	if r == nil || c == nil || !winMemoryValid(m) {
		return fmt.Errorf("native terrain admission backing/frame missing")
	}
	saved := [3]uint32{c.D[0], c.D[1], c.D[2]}
	defer func() { copy(c.D[:3], saved[:]) }()
	free, err := m.Read16(0xf0e)
	if err != nil {
		return err
	}
	if free != 0 {
		c.D[3] = 0xffffffff
		return nil
	}
	c.D[3] = uint32(uint16(c.D[3])) * 314
	rules, err := m.Read8(0xe76a + int(int16(c.D[3])) + 0x4b)
	if err != nil {
		return err
	}
	if rules&1 != 0 {
		c.D[3] = 0xffffffff
	} else if rules&2 == 0 {
		c.D[3] = 9
	} else {
		if err := r.TerrainHeight(m, c); err != nil {
			return err
		}
		c.Word(3, uint16(c.D[2]))
	}
	return nil
}

// descriptor executes an actual direct $f0ee/$f3a0 call. Unlike $ee32 it
// receives already-positioned top-left coordinates and assigns no image cue.
func (r *NativeRenderFrameRules) descriptor(a int, cb NativeRenderFrameCallbacks, p *NativeRenderFramePlan) error {
	if a < 0 || a+12 > len(r.code) || cb.Frame == nil {
		return fmt.Errorf("native direct rendering descriptor missing")
	}
	c := cb.Frame
	height := binary.BigEndian.Uint16(r.code[a+6:])
	routine := binary.BigEndian.Uint32(r.code[a+8:])
	if routine != 0xf0ee && routine != 0xf3a0 {
		return fmt.Errorf("native direct sprite needs original procedure %#x", routine)
	}
	c.Word(2, height)
	sprite := NativePresentationSprite{Sprite: (a - 0x21626) / 12, X: int16(c.D[0]), Y: int16(c.D[1]), HalfWidth: int16(binary.BigEndian.Uint16(r.code[a+4:])), Height: int16(height), Routine: routine}
	d0, d1, table := c.D[0], c.D[1], 0
	x, y, visible := int16(d0), int16(d1), true
	wide := routine == 0xf3a0
	if y < 0 {
		removed := uint16(-uint16(y))
		d1 = hudWord(d1, removed)
		if int16(height) <= int16(removed) {
			visible = false
		} else {
			factor := uint16(2)
			if wide {
				factor = 4
			}
			d1 = hudWord(d1, removed*factor)
		}
	} else if y >= 200 {
		visible = false
	} else {
		d1 = hudWord(d1, uint16(y)+height-200)
	}
	if visible {
		if x < 0 {
			limit := int16(-16)
			if wide {
				limit = -32
			}
			if x <= limit {
				visible = false
			} else {
				table = 0xf152
			}
		} else {
			d0 = hudWord(d0, uint16(x)&15)
			column := (uint16(x) & 0xfff0) >> 3
			if !wide {
				if column >= 40 {
					visible = false
				} else if column == 38 {
					table = 0xf216
				} else {
					table = 0xf2da
				}
			} else {
				if column > 38 {
					visible = false
				} else if column > 34 {
					table = 0xf216
				} else {
					table = 0xf2da
				}
			}
		}
	}
	r.Images.blitterRegisters(routine, uint16(c.D[0]), uint16(c.D[1]), &c.D)
	if visible {
		a := table + int((uint16(x)&15)*8)
		if a < 0 || a+8 > len(r.code) {
			return fmt.Errorf("native sprite hardware control outside CODE")
		}
		d0, d1 = binary.BigEndian.Uint32(r.code[a:]), binary.BigEndian.Uint32(r.code[a+4:])
	}
	// EE32 normally restores these two longs. A direct descriptor call
	// leaves the actual BLTCON/mask table words visible to its parent.
	c.D[0], c.D[1] = d0, d1
	return renderFrameSprites(cb, p, []NativePresentationSprite{sprite})
}

// MapCursor translates $f12..$1062, including real command-record bytes,
// raw adjacent terrain/raster reads and $e17a slope-outline pixels.
func (r *NativeRenderFrameRules) MapCursor(cb NativeRenderFrameCallbacks) (NativeRenderFramePlan, error) {
	p := NativeRenderFramePlan{Pixels: []NativeHUDPixel{}, Sprites: []NativePresentationSprite{}}
	if r == nil || cb.Frame == nil || !winMemoryValid(cb.Memory) || len(cb.Bitmap) != 32000 {
		return p, fmt.Errorf("native map cursor frame/backing missing")
	}
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	c.RestoreWord(0, m.word(0x5f48))
	c.RestoreWord(1, m.word(0x5f4a))
	c.Word(0, uint16(c.D[0])&0x1f0)
	m.putWord(0xdd4, uint16(c.D[1]))
	command := m.word(0xeb18)
	if uint16(c.D[1]) == 0 {
		if command == 30 {
			pointer := m.long(0xeb6a)
			m.putByte(int(int64(pointer)-int64(c.AddressBase))+1, 32)
		}
		return p, m.err
	}
	p.Drawn = true
	if command == 28 {
		pointer := m.long(0xeb6a)
		at := int(int64(pointer) - int64(c.AddressBase))
		m.putByte(at+1, m.byte(0xeb19))
		m.putByte(at+2, m.byte(0x5f4d))
		m.putByte(at+3, m.byte(0x5f4f))
	}
	c.Word(0, uint16(c.D[0])-3)
	c.Word(1, uint16(c.D[1])+1)
	if err := r.descriptor(0x219da, cb, &p); err != nil {
		return p, err
	}
	c.RestoreWord(0, m.word(0x5f48))
	c.RestoreWord(1, m.word(0x5f4a))
	c.Word(0, uint16(c.D[0])&0x1f0)
	c.Word(1, uint16(c.D[1])+3)
	x, y := uint16(c.D[0]), uint16(c.D[1])
	c.Word(2, m.word(0x5f4e)<<8)
	c.Byte(2, m.byte(0x5f4d))
	c.Byte(2, uint8(c.D[2])*4)
	grid := 0xf44 + int(int16(c.D[2]))
	c.Byte(2, m.byte(grid+1))
	c.Word(2, uint16(c.D[2])&255)
	c.Byte(2, r.code[0x33512+int(uint16(c.D[2]))])
	c.Word(2, uint16(c.D[2])&15)
	c.Word(2, uint16(c.D[2])<<4)
	shape := 0x33194 + int(int16(c.D[2]))
	c.D[3] = 0x0c080400
	c.Word(0, m.word(0x5f4e)-m.word(0x5f46))
	if int16(c.D[0]) >= 8 {
		c.D[3] = 0x400
	}
	c.Word(0, m.word(0x5f4c)-m.word(0x5f44))
	if int16(c.D[0]) >= 8 {
		if c.D[3] == 0x400 {
			c.D[3] = 0
		} else {
			c.D[3] = 0xc00
		}
	}
	for {
		c.RestoreWord(0, x)
		c.RestoreWord(1, y)
		c.Byte(2, uint8(c.D[3]))
		c.ExtendWord(2)
		c.D[3] >>= 8
		dx, err := r.word(shape + int(int16(c.D[2])))
		if err != nil {
			return p, err
		}
		dy, err := r.word(shape + int(int16(c.D[2])) + 2)
		if err != nil {
			return p, err
		}
		c.Word(0, uint16(c.D[0])+dx)
		c.Word(1, uint16(c.D[1])+dy)
		if int16(c.D[0]) >= 320 {
			c.Word(0, 319)
		}
		c.Word(2, 5)
		if err := renderFramePixel(cb, &p, true); err != nil {
			return p, err
		}
		if c.D[3] == 0 {
			break
		}
	}
	return p, m.err
}

// CameraMarker is $1062..$10b0. Alternate views perform no assignments.
func (r *NativeRenderFrameRules) CameraMarker(cb NativeRenderFrameCallbacks) (NativeRenderFramePlan, error) {
	p := NativeRenderFramePlan{Pixels: []NativeHUDPixel{}, Sprites: []NativePresentationSprite{}}
	if r == nil || cb.Frame == nil || !winMemoryValid(cb.Memory) {
		return p, fmt.Errorf("native camera rendering frame/backing missing")
	}
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	if m.word(0xf0c) != 8 {
		return p, m.err
	}
	p.Drawn = true
	c.Word(1, m.word(0x5f46))
	c.D[1] += 3
	c.D[0] = 64
	c.Word(0, uint16(c.D[0])-uint16(c.D[1]))
	c.Word(2, m.word(0x5f44))
	c.D[2] += 3
	c.Word(0, uint16(c.D[0])+uint16(c.D[2]))
	c.Word(1, uint16(c.D[1])+uint16(c.D[2]))
	c.Word(1, uint16(c.D[1])>>1)
	c.Word(0, uint16(c.D[0])+4)
	c.Word(1, uint16(c.D[1])+4)
	if err := r.descriptor(0x219da, cb, &p); err != nil {
		return p, err
	}
	return p, m.err
}

// Cursor is the complete $1be8..$1d9a pointer selector. Its editor image uses
// the shared EE32 state; the normal terrain gate is $d91a, not spell pricing.
func (r *NativeRenderFrameRules) Cursor(cb NativeRenderFrameCallbacks) (NativeRenderFramePlan, error) {
	p := NativeRenderFramePlan{Pixels: []NativeHUDPixel{}, Sprites: []NativePresentationSprite{}}
	if r == nil || cb.Frame == nil || cb.Input == nil || cb.Image == nil || !winMemoryValid(cb.Memory) {
		return p, fmt.Errorf("native pointer rendering input/frame backing missing")
	}
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	sprites, err := r.Images.Preview(cb.Memory, cb.Image, &c.D)
	if err != nil {
		return p, err
	}
	if err := renderFrameSprites(cb, &p, sprites); err != nil {
		return p, err
	}
	c.Word(0, cb.Input.Mouse.Image)
	c.Word(1, m.word(0xeb18))
	branch, err := r.word(0x1c28 + int(int16(c.D[1])))
	if err != nil {
		return p, err
	}
	c.Word(1, branch)
	target := 0x1c28 + int(int16(c.D[1]))
	switch target {
	case 0x1c7a:
		c.Word(0, 0)
		if m.word(0xdd4) != 0 {
			c.RestoreWord(0, m.word(0x5f4c))
			c.RestoreWord(1, m.word(0x5f4e))
			c.Word(3, m.word(0xeb42))
			if err := r.TerrainAdmission(cb.Memory, c); err != nil {
				return p, err
			}
			c.Word(0, 0)
			if uint16(c.D[3]) != 9 && int16(c.D[3]) <= 0 {
				c.Word(0, 0x360)
			}
		}
	case 0x1d08, 0x1d5e, 0x1d74:
		base := uint16(0x750)
		if target == 0x1d5e {
			base = 0xea0
		} else if target == 0x1d74 {
			base = 0x10e0
		}
		c.Word(0, m.word(0xf42))
		c.Word(0, uint16(c.D[0])&24)
		c.D[0] = uint32(uint16(c.D[0])) * 18
		c.Word(0, uint16(c.D[0])+base)
	case 0x1d92:
		// NOP retains the incoming mutable pointer selector.
	default:
		opcode, err := r.word(target)
		if err != nil {
			return p, err
		}
		if opcode != 0x303c {
			return p, fmt.Errorf("native pointer branch %#x needs its original body", target)
		}
		image, err := r.word(target + 2)
		if err != nil {
			return p, err
		}
		c.Word(0, image)
	}
	cb.Input.Mouse.Image = uint16(c.D[0])
	return p, m.err
}

func renderFrameSprites(cb NativeRenderFrameCallbacks, p *NativeRenderFramePlan, sprites []NativePresentationSprite) error {
	for _, sprite := range sprites {
		p.Sprites = append(p.Sprites, sprite)
		if cb.Sprite == nil {
			p.HardwarePending = true
		} else if err := cb.Sprite(sprite, cb.Bitmap); err != nil {
			return err
		}
	}
	return nil
}

// HUD is the complete $1f5e register/software-bitmap body. redraw is an
// explicit caller gate; false preserves every incoming long and reads nothing.
// Hardware sprites use the same image state and source-order callback.
func (r *NativeRenderFrameRules) HUD(cb NativeRenderFrameCallbacks, redraw bool) (NativeRenderFramePlan, error) {
	p := NativeRenderFramePlan{Pixels: []NativeHUDPixel{}, Sprites: []NativePresentationSprite{}}
	if cb.Frame == nil {
		return p, fmt.Errorf("native rendering frame missing")
	}
	if !redraw {
		return p, nil
	}
	if r == nil || !winMemoryValid(cb.Memory) || cb.Image == nil || len(cb.Bitmap) != 32000 {
		return p, fmt.Errorf("native HUD raw backing/image/bitmap missing")
	}
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	p.Drawn = true
	c.Word(0, m.word(0xeb42))
	c.D[0] = uint32(uint16(c.D[0])) * 314
	god := 0xe76a + int(int16(c.D[0]))
	c.D[3] = m.long(god) >> 2
	c.D[7] = 0
	price := func() error {
		context := c.CommandContext()
		err := r.Commands.Cost(&context, cb.Memory)
		c.SetCommandContext(context)
		return err
	}
	for pointer, slot := 0x2115e, 0; ; pointer, slot = pointer+6, slot+1 {
		v, err := r.word(pointer)
		if err != nil {
			return p, err
		}
		c.Word(2, v)
		if int16(v) < 0 {
			break
		}
		c.Word(0, uint16(c.D[7]))
		c.Word(1, m.word(0xeb42))
		if err := price(); err != nil {
			return p, err
		}
		c.Word(5, uint16(c.D[0]))
		c.Word(0, uint16(c.D[2]))
		y, err := r.word(pointer + 2)
		if err != nil {
			return p, err
		}
		color, err := r.word(pointer + 4)
		if err != nil {
			return p, err
		}
		c.Word(1, y)
		c.Word(2, color)
		if int8(m.byte(god+0x70+slot)) > 0 {
			c.D[4] = c.D[3]
			if err := frameDivide(c, 4, uint16(c.D[5])); err != nil {
				return p, err
			}
			if int16(c.D[4]) > 0 {
				if int16(c.D[4]) > 4 {
					c.Word(4, 4)
				}
				c.Word(4, uint16(c.D[4])-1)
				for {
					saved := c.D
					if err := renderFramePixel(cb, &p, false); err != nil {
						return p, err
					}
					// The original MOVEM restores D0-D4/D7, while this
					// pixel child assigns no other data registers.
					c.D = saved
					c.Word(1, uint16(c.D[1])-1)
					c.Word(4, uint16(c.D[4])-1)
					if uint16(c.D[4]) == 0xffff {
						break
					}
				}
			}
		}
		c.Word(7, uint16(c.D[7])+1)
	}
	c.Word(4, m.word(0xf3a))
	c.D[4] = uint32(uint16(c.D[4])) * 6
	c.Word(0, m.word(0xeb42))
	c.D[0] = uint32(uint16(c.D[0])) * 314
	god = 0xe76a + int(int16(c.D[0]))
	c.D[6], c.D[5], c.D[3] = 0, 0, 0
	c.Word(4, uint16(c.D[4])>>1)
	for {
		c.Word(1, m.word(0xeb42))
		c.Word(0, uint16(c.D[4]))
		if err := price(); err != nil {
			return p, err
		}
		c.D[2] = c.D[0]
		if int8(m.byte(god+0x70+int(int16(c.D[4])))) > 0 {
			c.D[2] *= 4
			if int32(c.D[2]) >= int32(m.long(god)) {
				break
			}
			c.D[3] = c.D[2]
			c.Word(6, uint16(c.D[5]))
		}
		c.Word(4, uint16(c.D[4])+1)
		c.Word(5, uint16(c.D[5])+1)
		if uint16(c.D[5]) == 6 {
			break
		}
	}
	if c.D[3] == 0 {
		c.D[0], c.D[6] = 0, 0
	} else {
		c.Word(0, uint16(c.D[6]))
		c.Word(0, uint16(c.D[0])<<4)
		c.Word(6, uint16(c.D[6])<<3)
		c.D[2] = m.long(god) >> 2
		c.D[3] >>= 2
		if err := frameDivide(c, 2, uint16(c.D[3])); err != nil {
			return p, err
		}
		if uint16(c.D[2]) > 7 {
			c.Word(2, 7)
		}
		c.Word(1, uint16(c.D[6])+uint16(c.D[2]))
		c.Word(0, uint16(c.D[0])+uint16(c.D[2])*2)
	}
	c.Word(0, uint16(c.D[0])+27)
	c.Word(1, uint16(c.D[1])+166)
	c.Word(2, 0x195c)
	sprites, err := r.Images.DrawImage(cb.Image, &c.D)
	if err != nil {
		return p, err
	}
	if err := renderFrameSprites(cb, &p, sprites); err != nil {
		return p, err
	}
	c.D[0] = m.long(god)
	if err := frameDivide(c, 0, 5000); err != nil {
		return p, err
	}
	if int16(c.D[0]) > 0 {
		if int16(c.D[0]) > 35 {
			c.Word(0, 35)
		}
		span := uint16(c.D[0])
		// These are $e056's actual x-major line assignments for its
		// source endpoints (25,164)..(25+2n,164+n), not a fitted D result.
		c.D[6] = 0x001900a4
		c.D[7] = c.D[6]
		c.Word(7, uint16(c.D[7])+span)
		c.Swap(7)
		c.Word(7, uint16(c.D[7])+span*2)
		c.Swap(7)
		c.Word(3, 8)
		c.Word(3, uint16(c.D[3])&15)
		c.D[7] -= c.D[6]
		c.Swap(6)
		c.Swap(7)
		c.D[5] = 0x8000
		c.Word(4, uint16(c.D[6]))
		c.Word(6, 0)
		for {
			c.Word(0, uint16(c.D[4]))
			c.Swap(6)
			c.Word(1, uint16(c.D[6]))
			c.Word(2, uint16(c.D[3]))
			if err := renderFramePixel(cb, &p, true); err != nil {
				return p, err
			}
			c.Swap(6)
			c.D[6] += c.D[5]
			c.Word(4, uint16(c.D[4])+1)
			c.Word(7, uint16(c.D[7])-1)
			if uint16(c.D[7]) == 0 {
				break
			}
		}
	}
	c.Word(0, m.word(0x138))
	c.Word(0, uint16(c.D[0])+m.word(0x13a)+m.word(0xf42))
	c.Word(4, uint16(c.D[0]))
	for side := 0; side < 2; side++ {
		c.D[3] = m.long(0xe8a4 + side*314 + 4)
		if c.D[3] == 0 {
			continue
		}
		if err := frameDivide(c, 3, 2048); err != nil {
			return p, err
		}
		if int16(c.D[3]) > 73 {
			c.Word(3, 73)
		}
		if int16(c.D[3]) <= 0 {
			continue
		}
		pointer := 0x219e + side*0x128
		for {
			c.Word(4, bits.RotateLeft16(uint16(c.D[4]), 1))
			point, err := r.word(pointer)
			if err != nil {
				return p, err
			}
			row, err := r.word(pointer + 2)
			if err != nil {
				return p, err
			}
			pointer += 4
			c.Word(0, point)
			c.Word(1, uint16(c.D[0])>>3)
			c.Word(1, uint16(c.D[1])+row)
			at := 31 + int(int16(c.D[1]))
			c.D[1] = 2
			c.Word(0, uint16(c.D[0])&7)
			variant, err := r.word(0x218e + int(int16(c.D[0])))
			if err != nil {
				return p, err
			}
			c.Word(0, uint16(c.D[2])+uint16(c.D[4]))
			c.Word(0, uint16(c.D[0])&6)
			phase, err := r.word(0x2196 + int(int16(c.D[0])))
			if err != nil {
				return p, err
			}
			source := 0x33a88 + side*240 + int(int16(variant)) + int(int16(phase))
			for {
				if source < 0 || source+5 > len(r.code) || at < 0 || at >= 8000 {
					return p, fmt.Errorf("native population stencil outside backing")
				}
				c.Byte(0, r.code[source])
				source++
				for plane := range 4 {
					address := at + plane*8000
					c.Byte(2, cb.Bitmap[address])
					c.Byte(2, uint8(c.D[2])&uint8(c.D[0]))
					c.Byte(2, uint8(c.D[2])|r.code[source])
					source++
					cb.Bitmap[address] = uint8(c.D[2])
				}
				at += 40
				c.Word(1, uint16(c.D[1])-1)
				if uint16(c.D[1]) == 0xffff {
					break
				}
			}
			c.Word(3, uint16(c.D[3])-1)
			if uint16(c.D[3]) == 0xffff {
				break
			}
		}
	}
	return p, m.err
}
