package populous2

import (
	"encoding/binary"
	"fmt"
	"math/bits"

	"go-populous2/internal/amiga"
)

// NativeHUDRegisters retains the three data registers consumed by following
// follower/AI/wall routines. It does not pretend to emulate unrelated registers.
type NativeHUDRegisters struct{ D4, D5, D7 uint32 }
type NativeHUDPixel struct {
	X, Y  int16
	Color uint16
}
type NativeHUDSprite struct {
	Sprite                  int
	X, Y, HalfWidth, Height int16
}
type NativeHUDPopulation struct {
	Side     uint8
	XByte, Y int16
	Rotation uint16
	Variants [4][3][5]uint8 // Native preserve mask then four palette-plane patterns.
}
type NativeHUDPlan struct {
	Drawn          bool
	Pixels         []NativeHUDPixel
	Sprites        []NativeHUDSprite
	Population     []NativeHUDPopulation
	PopulationSeed uint16 // D2 retained from indicator blit or the later line.
}
type NativeHUDRules struct {
	Mana          ManaRules
	Costs         [36]uint16
	Bars          [36][3]int16
	Indicator     []NativeHUDSprite
	Population    [2][74][2]uint16
	PopulationInk [2][4][4][3][5]uint8
}

func DecodeNativeHUDRules(exe *amiga.Executable) (NativeHUDRules, error) {
	var r NativeHUDRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33c68 {
		return r, fmt.Errorf("native HUD tables missing")
	}
	var err error
	r.Mana, err = DecodeManaRules(exe)
	if err != nil {
		return r, err
	}
	code := exe.Hunks[0].Data
	for i := range r.Costs {
		r.Costs[i] = binary.BigEndian.Uint16(code[0x21238+i*2:])
		for j := range r.Bars[i] {
			r.Bars[i][j] = int16(binary.BigEndian.Uint16(code[0x2115e+i*6+j*2:]))
		}
	}
	image := binary.BigEndian.Uint16(code[0x23d1a+0x195c:])
	layers, err := decodeImageLayers(code, image)
	if err != nil {
		return r, err
	}
	for _, l := range layers {
		a := 0x21626 + l.Sprite*12
		half, height := int16(binary.BigEndian.Uint16(code[a+4:])), int16(binary.BigEndian.Uint16(code[a+6:]))
		if half != 8 && half != 16 || height <= 0 {
			return r, fmt.Errorf("native HUD indicator descriptor unsupported")
		}
		r.Indicator = append(r.Indicator, NativeHUDSprite{Sprite: l.Sprite, X: int16(l.X), Y: int16(l.Y), HalfWidth: half, Height: height})
	}
	for side := range r.Population {
		for i := range r.Population[side] {
			a := 0x219e + side*0x128 + i*4
			r.Population[side][i] = [2]uint16{binary.BigEndian.Uint16(code[a:]), binary.BigEndian.Uint16(code[a+2:])}
		}
		for bit := range 4 {
			for phase := range 4 {
				for row := range 3 {
					for plane := range 5 {
						r.PopulationInk[side][bit][phase][row][plane] = code[0x33a88+side*240+bit*60+phase*15+row*5+plane]
					}
				}
			}
		}
	}
	return r, nil
}

func hudWord(v uint32, w uint16) uint32 { return v&0xffff0000 | uint32(w) }
func hudDivide(v uint32, d uint16) (uint32, error) {
	if d == 0 {
		return v, fmt.Errorf("native HUD DIVU by zero")
	}
	q := v / uint32(d)
	if q > 65535 {
		return v, nil
	}
	return v%uint32(d)<<16 | q, nil
}

// Continue is the bounded original $1f5e HUD body. redraw is the actual caller
// gate, not an invented dirty heuristic: false reads nothing and preserves the
// register continuation. Main view8 calls this body at$eb6; alternate view
// rendering and selected-follower presentation remain separate caller stages.
// The plan retains original 320x200 palette-index geometry for later UI binding.
func (r *NativeHUDRules) Continue(m FollowerCleanupMemory, c *NativeHUDRegisters, redraw bool) (NativeHUDPlan, error) {
	plan := NativeHUDPlan{Pixels: []NativeHUDPixel{}, Sprites: []NativeHUDSprite{}, Population: []NativeHUDPopulation{}}
	if c == nil {
		return plan, fmt.Errorf("native HUD register continuation missing")
	}
	if !redraw {
		return plan, nil
	}
	if r == nil || !winMemoryValid(m) {
		return plan, fmt.Errorf("native HUD raw memory missing")
	}
	side, err := m.Read16(0xeb42)
	if err != nil {
		return plan, err
	}
	god := 0xe76a + int(int16(uint16(uint32(side)*314)))
	mana, err := m.Read32(god)
	if err != nil {
		return plan, err
	}
	var xp [6]uint8
	for i := range xp {
		xp[i], err = m.Read8(god + 0x52 + i)
		if err != nil {
			return plan, err
		}
	}
	cost := func(slot uint16) (uint16, error) {
		if slot >= 36 {
			return 0, fmt.Errorf("native HUD cost table outside decoded powers")
		}
		return uint16(r.Mana.Cost(SpellID(slot), int(r.Costs[slot]), xp)), nil
	}
	plan.Drawn = true
	c.D7 = 0
	for slot, b := range r.Bars {
		price, e := cost(uint16(slot))
		if e != nil {
			return plan, e
		}
		c.D5 = hudWord(c.D5, price)
		flag, e := m.Read8(god + 0x70 + slot)
		if e != nil {
			return plan, e
		}
		if int8(flag) > 0 {
			c.D4 = mana >> 2
			c.D4, e = hudDivide(c.D4, price)
			if e != nil {
				return plan, e
			}
			count := int16(uint16(c.D4))
			if count > 0 {
				if count > 4 {
					count = 4
					c.D4 = hudWord(c.D4, 4)
				}
				c.D4 = hudWord(c.D4, uint16(count-1))
				for i := int16(0); i < count; i++ {
					plan.Pixels = append(plan.Pixels, NativeHUDPixel{b[0], b[1] - i, uint16(b[2])})
				}
				c.D4 = hudWord(c.D4, 0xffff)
			}
		}
		c.D7 = hudWord(c.D7, uint16(c.D7)+1)
	}
	element, err := m.Read16(0xf3a)
	if err != nil {
		return plan, err
	}
	c.D4 = uint32(element) * 6
	c.D4 = hudWord(c.D4, uint16(c.D4)>>1)
	c.D5 = 0
	selected, available := uint16(0), uint32(0)
	for range 6 {
		slot := uint16(c.D4)
		price, e := cost(slot)
		if e != nil {
			return plan, e
		}
		flag, e := m.Read8(god + 0x70 + int(int16(slot)))
		if e != nil {
			return plan, e
		}
		if int8(flag) > 0 && int32(uint32(price)*4) < int32(mana) {
			available = uint32(price) * 4
			selected = uint16(c.D5)
		} else if int8(flag) > 0 {
			break
		}
		c.D4 = hudWord(c.D4, uint16(c.D4)+1)
		c.D5 = hudWord(c.D5, uint16(c.D5)+1)
	}
	x, y := int16(0), int16(0)
	if available != 0 {
		progress, e := hudDivide(mana>>2, uint16(available>>2))
		if e != nil {
			return plan, e
		}
		fraction := uint16(progress)
		if fraction > 7 {
			fraction = 7
		}
		x = int16(selected*16 + fraction*2)
		y = int16(selected*8 + fraction)
	}
	x += 27
	y += 166
	for _, s := range r.Indicator {
		s.X += x - s.HalfWidth
		s.Y += y - s.Height
		plan.Sprites = append(plan.Sprites, s)
		// $ee32's actual descriptor dispatch loads the source plane stride
		// into D7.W before clipping. It preserves D4/D5 for these decoded
		// indicator descriptors; stride follows data, not a final constant.
		c.D7 = hudWord(c.D7, uint16(s.Height*s.HalfWidth/4))
		visible := int(s.Height)
		if s.Y < 0 {
			visible += int(s.Y)
		} else if int(s.Y)+visible > 200 {
			visible = 200 - int(s.Y)
		}
		if visible > 0 && s.Y < 200 && s.X > -16 && s.X < 320 {
			words := uint16(s.HalfWidth/8 + 1)
			if s.X < 0 || s.X >= 304 {
				words--
			}
			plan.PopulationSeed = uint16(visible)<<6 | words
		} else {
			plan.PopulationSeed = uint16(s.Height)
		}
	}
	units, e := hudDivide(mana, 5000)
	if e != nil {
		return plan, e
	}
	span := int16(uint16(units))
	if span > 0 {
		plan.PopulationSeed = 0x80 // $e196 leaves palette8 shifted by four.
		if span > 35 {
			span = 35
		}
		// $20a0's endpoints are (25,164) and (25+2n,164+n).
		c.D7 = uint32(uint16(span*2))<<16 | uint32(uint16(span))
		c.D5 = uint32(1 << 15)
		c.D4 = hudWord(c.D4, 25)
		for i := int16(0); i < span*2; i++ {
			plan.Pixels = append(plan.Pixels, NativeHUDPixel{25 + i, 164 + i/2, 8})
		}
		c.D4 = hudWord(c.D4, uint16(25+span*2))
		c.D7 = uint32(uint16(span)) << 16
	}
	mx, err := m.Read16(0x138)
	if err != nil {
		return plan, err
	}
	my, err := m.Read16(0x13a)
	if err != nil {
		return plan, err
	}
	clock, err := m.Read16(0xf42)
	if err != nil {
		return plan, err
	}
	c.D4 = hudWord(c.D4, mx+my+clock)
	for owner := uint8(1); owner <= 2; owner++ {
		population, e := m.Read32(0xe76a + int(owner)*314 + 4)
		if e != nil {
			return plan, e
		}
		if population == 0 {
			continue
		}
		q, e := hudDivide(population, 2048)
		if e != nil {
			return plan, e
		}
		count := int16(uint16(q))
		if count > 73 {
			count = 73
		}
		if count <= 0 {
			continue
		}
		for i := int16(0); i <= count; i++ {
			c.D4 = hudWord(c.D4, bits.RotateLeft16(uint16(c.D4), 1))
			point := r.Population[owner-1][i]
			if point[0]&1 != 0 {
				return plan, fmt.Errorf("native HUD glyph bit index causes odd word access")
			}
			plan.Population = append(plan.Population, NativeHUDPopulation{Side: owner, XByte: int16(point[0]>>3) + 31, Y: int16(point[1] / 40), Rotation: uint16(c.D4), Variants: r.PopulationInk[owner-1][(point[0]&7)/2]})
		}
	}
	return plan, nil
}

// PaintSoftware applies original palette-index pixels and population stencils
// to a caller-supplied planar320x200 bitmap copy. The population variation
// depends on its previous plane byte, so an invented empty background is not
// part of the API. Indicator sprite layers stay separate in the plan; their
// hardware blit does not overlap the right-hand population stencil region.
func (plan NativeHUDPlan) PaintSoftware(bitmap [32000]uint8) ([32000]uint8, error) {
	if !plan.Drawn {
		return bitmap, nil
	}
	for _, p := range plan.Pixels {
		if p.X < 0 || p.X >= 320 || p.Y < 0 || p.Y >= 200 {
			continue
		}
		at := int(p.Y)*40 + int(p.X)/8
		bit := uint8(1 << uint(7-int(p.X)&7))
		for plane := range 4 {
			if p.Color&(1<<plane) != 0 {
				bitmap[at+plane*8000] |= bit
			} else {
				bitmap[at+plane*8000] &= ^bit
			}
		}
	}
	ink := uint8(plan.PopulationSeed)
	for _, stamp := range plan.Population {
		phase := ((uint16(ink) + stamp.Rotation) & 6) / 2
		for row := range 3 {
			at := (int(stamp.Y)+row)*40 + int(stamp.XByte)
			if at < 0 || at >= 8000 {
				return bitmap, fmt.Errorf("native HUD population stencil outside bitmap")
			}
			pattern := stamp.Variants[phase][row]
			for plane := range 4 {
				a := at + plane*8000
				bitmap[a] = bitmap[a]&pattern[0] | pattern[plane+1]
				ink = bitmap[a]
			}
		}
	}
	return bitmap, nil
}
