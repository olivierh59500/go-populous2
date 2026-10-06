package engine

// Cell describes geometry independently of graphics-bank selection. Corners
// run clockwise from northwest; raised flat land uses the top of its tile.
type Cell struct {
	Corners      [4]uint8
	BaseAltitude uint8
	Shape, Code  uint8
}

func (c Cell) IsWater() bool { return c.Corners == [4]uint8{} }
func (c Cell) IsFlat() bool {
	return c.Corners[0] > 0 && c.Corners == [4]uint8{c.Corners[0], c.Corners[0], c.Corners[0], c.Corners[0]}
}
func (c Cell) TileIndex(tick, viewX, viewY int) int {
	code := int(c.Code)
	if code == 143 {
		return code + ((tick + viewX + viewY) & 1)
	}
	if code == 168 {
		return code + ((tick + viewX + viewY) & 3)
	}
	if code <= 15 && c.BaseAltitude > 0 {
		code += 16
	}
	if code < 15 {
		phase := (tick + viewX + viewY) & 7
		if phase != 0 {
			phase++
		}
		code += phase * 16
	}
	return code
}

func (c Cell) Surface() [4][2]int {
	points := [4][2]int{{16, 8}, {32, 16}, {16, 24}, {0, 16}}
	for i := range points {
		points[i][1] -= (int(c.Corners[i]) - int(c.BaseAltitude)) * 8
	}
	return points
}

// Contains supports terrain clicks without a rectangular hitbox on sloped land.
func (c Cell) Contains(x, y int) bool {
	positive, negative := false, false
	points := c.Surface()
	for i, a := range points {
		b := points[(i+1)%4]
		cross := (b[0]-a[0])*(y-a[1]) - (b[1]-a[1])*(x-a[0])
		positive = positive || cross > 0
		negative = negative || cross < 0
	}
	return !(positive && negative)
}

// The four random walks start in distinct overlapping map quadrants.
var initialHillBounds = [4][4]int{{29, 6, 29, 6}, {23, 35, 23, 35}, {29, 6, 23, 35}, {23, 35, 29, 6}}
var directions = [8][2]int{{0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}}

type randomState uint32

func (r *randomState) next() uint16 {
	if *r == 0 {
		*r = 0xbc614e
	}
	*r *= 0xbb40e62d
	return uint16(*r>>8) & 0x7fff
}
func walkRandom(seed uint16) uint16 { return uint16(uint32(seed)*0x24a1+0x24df) & 0x7fff }

func (w *World) generate(seed uint32) {
	w.random = randomState(seed)
	for _, hill := range initialHillBounds {
		xs, ys := w.random.next(), w.random.next()
		y, x := int(xs)%hill[0]+hill[1], int(ys)%hill[2]+hill[3]
		for step := 0; step < 65536; step++ {
			xs, ys = walkRandom(xs), walkRandom(ys)
			x += int(xs)%7 - 3
			y += int(ys)%7 - 3
			if !insideCorner(x, y) {
				break
			}
			if w.raiseCorner(x, y) >= 8 {
				break
			}
		}
	}
	w.rebuildCells()
}

func insideCorner(x, y int) bool { return x >= 0 && y >= 0 && x < CornerSize && y < CornerSize }
func inside(x, y int) bool       { return x >= 0 && y >= 0 && x < MapSize && y < MapSize }
func (w *World) raiseCorner(x, y int) uint8 {
	if !insideCorner(x, y) {
		return 0
	}
	at := x + y*CornerSize
	if w.Heights[at] >= 8 {
		return w.Heights[at]
	}
	w.Heights[at]++
	for _, d := range directions {
		nx, ny := x+d[0], y+d[1]
		if insideCorner(nx, ny) && int(w.Heights[at])-int(w.Heights[nx+ny*CornerSize]) > 1 {
			w.raiseCorner(nx, ny)
		}
	}
	return w.Heights[at]
}
func (w *World) lowerCorner(x, y int) uint8 {
	if !insideCorner(x, y) {
		return 0
	}
	at := x + y*CornerSize
	if w.Heights[at] == 0 {
		return 0
	}
	w.Heights[at]--
	for _, d := range directions {
		nx, ny := x+d[0], y+d[1]
		if insideCorner(nx, ny) && int(w.Heights[nx+ny*CornerSize])-int(w.Heights[at]) > 1 {
			w.lowerCorner(nx, ny)
		}
	}
	return w.Heights[at]
}
func (w *World) rebuildCells() {
	for y := 0; y < MapSize; y++ {
		for x := 0; x < MapSize; x++ {
			at := x + y*CornerSize
			c := Cell{Corners: [4]uint8{w.Heights[at], w.Heights[at+1], w.Heights[at+CornerSize+1], w.Heights[at+CornerSize]}}
			c.BaseAltitude = min(c.Corners[0], c.Corners[1], c.Corners[2], c.Corners[3])
			for i, h := range c.Corners {
				if h > c.BaseAltitude {
					c.Shape |= 1 << i
				}
			}
			if c.Shape == 0 && c.BaseAltitude > 0 {
				c.BaseAltitude--
				c.Shape = 15
			}
			c.Code = c.Shape
			tile := x + y*MapSize
			if w.Farms[tile] > 0 && c.IsFlat() {
				c.Code = 47 + 16*(w.Farms[tile]-1)
			}
			w.Tiles[tile] = c
		}
	}
}
func (w *World) Cell(x, y int) Cell {
	if !inside(x, y) {
		return Cell{}
	}
	return w.Tiles[x+y*MapSize]
}

// RaiseAt and LowerAt admit an order at the base mana cost. Every corner
// changed by slope propagation is charged; an exhausted ledger clamps to zero.
// Saturated or invalid requests do not consume mana.
func (w *World) RaiseAt(player, x, y int) bool { return w.changeHeight(player, x, y, true) }
func (w *World) LowerAt(player, x, y int) bool { return w.changeHeight(player, x, y, false) }
func (w *World) changeHeight(player, x, y int, raise bool) bool {
	if player < 0 || player > 1 || !insideCorner(x, y) || w.Players[player].Mana < w.PowerCost(player, RaiseLower) {
		return false
	}
	h := w.Heights[x+y*CornerSize]
	if raise && h >= 8 || !raise && h == 0 {
		return false
	}
	before := w.Heights
	if raise {
		w.raiseCorner(x, y)
	} else {
		w.lowerCorner(x, y)
	}
	changes := 0
	for i, height := range w.Heights {
		if height != before[i] {
			changes++
		}
	}
	w.Players[player].Mana = max(0, w.Players[player].Mana-w.PowerCost(player, RaiseLower)*changes)
	w.rebuildCells()
	return true
}
