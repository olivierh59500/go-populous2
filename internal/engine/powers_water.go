package engine

import "fmt"

type BasaltEffect struct {
	Active             bool
	Owner              uint8
	X, Y               int
	Direction          int // North, east, south, west.
	Life, Delay, Frame int
}

type WhirlpoolEffect struct {
	Active             bool
	Owner              uint8
	X, Y               int
	Life, Delay, Frame int
}

type TidalEffect struct {
	Active, Newborn  bool
	Owner            uint8
	X, Y             int // 1/256-tile position.
	Direction, Frame int
}

// WaterEffects uses the same effect reservations as every other power.
// Deposited terrain is independent of the actor that briefly animates it.
type WaterEffects struct {
	Basalt     [EffectCapacity]BasaltEffect
	Whirlpools [EffectCapacity]WhirlpoolEffect
	Waves      [EffectCapacity]TidalEffect
	Painted    [MapSize * MapSize]bool
	Tiles      [MapSize * MapSize]uint8
}

func (s *WaterEffects) TileCode(x, y int) (uint8, bool) {
	if s == nil || !inside(x, y) {
		return 0, false
	}
	at := x + y*MapSize
	return s.Tiles[at], s.Painted[at]
}

func (w *World) ClearWaterTerrain(x, y int) {
	if !inside(x, y) {
		return
	}
	at := x + y*MapSize
	// Basalt survives landscape sculpting after its controller has departed.
	if w.Water.Tiles[at] >= 224 && w.Water.Tiles[at] <= 243 {
		return
	}
	w.Water.Painted[at] = false
}

// basaltEligible is a named description of the original shape-raster zero
// class. Raised flat ground, farms, flowers, lava and prior basalt reject it.
func basaltEligible(c Cell) bool {
	code := c.Code
	return code == 0 || code >= 16 && code <= 142 && code%16 != 15 || code >= 152 && code <= 167 || code >= 246
}

// CreateBasalt admits terrain before reserving or drawing randomness. A
// propagating child inherits the parent's remaining life without adding XP.
func (w *World) CreateBasalt(owner uint8, x, y, direction, life int) bool {
	if owner > 2 || !inside(x, y) || direction < 0 || direction > 3 || !basaltEligible(w.Cell(x, y)) {
		return false
	}
	id := w.allocateEffect(EffectBasalt, owner)
	if id < 0 {
		return false
	}
	w.Water.Painted[x+y*MapSize], w.Water.Tiles[x+y*MapSize] = true, 224
	w.Water.Basalt[id] = BasaltEffect{Active: true, Owner: owner, X: x, Y: y, Direction: direction, Life: life, Delay: int(w.random.next()%18) + 4}
	return true
}

func (w *World) CastBasalt(owner, x, y, direction int) error {
	if owner < 0 || owner > 1 || !inside(x, y) || direction < 0 || direction > 3 {
		return fmt.Errorf("invalid basalt target")
	}
	if !w.CreateBasalt(uint8(owner), x, y, direction, 100+int(w.Players[owner].Experience[Water])) {
		return fmt.Errorf("basalt cannot be placed here")
	}
	return nil
}

func (w *World) tickWaterEffects() {
	for id := range w.Water.Basalt {
		w.tickWaterEffect(id)
	}
}

func (w *World) tickWaterEffect(id int) {
	switch w.effects.Slots[id].Kind {
	case EffectBasalt:
		w.tickBasalt(id)
	case EffectWhirlpool:
		w.tickWhirlpool(id)
	}
}

// The child is allocated while its parent is still reserved. Ascending slot
// iteration lets a child in a later slot advance on the same simulation pass.
func (w *World) tickBasalt(id int) {
	e := &w.Water.Basalt[id]
	if !e.Active {
		return
	}
	finish := func() { e.Active = false; w.releaseEffect(id) }
	previousLife := e.Life
	e.Life = int(int16(uint16(e.Life) - 1))
	if previousLife <= 1 {
		finish()
		return
	}
	e.Frame = (e.Frame + 1) % 4
	previousDelay := e.Delay
	e.Delay = int(int16(uint16(e.Delay) - 1))
	if previousDelay > 1 {
		return
	}
	d := [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}[e.Direction]
	w.CreateBasalt(e.Owner, e.X+d[0], e.Y+d[1], e.Direction, e.Life)
	finish()
}

var whirlpoolFootprint = [4][2]int{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
var whirlpoolCorners = [24][2]int{
	{0, 0}, {1, 0}, {0, 1}, {1, 1}, {1, 0}, {2, 0}, {1, 1}, {2, 1},
	{1, 1}, {2, 1}, {1, 2}, {2, 2}, {0, 1}, {1, 1}, {0, 2}, {1, 2},
	{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1},
}

func waterParcel(code uint8) bool { return code == 0 || code == 16 || code >= 152 && code <= 167 }
func (w *World) paintWater(x, y int, code uint8) {
	if !inside(x, y) {
		return
	}
	at := x + y*MapSize
	w.Water.Painted[at], w.Water.Tiles[at] = true, code
	// All ground-effect creators replace the active parcel's tile artwork.
	w.Nature.Ground[at] = GroundParcel{}
	w.FireDamage.Painted[at] = false
}

func (w *World) CastWhirlpool(owner, x, y int) error {
	if owner < 0 || owner > 1 || !inside(x, y) {
		return fmt.Errorf("invalid whirlpool target")
	}
	for _, d := range whirlpoolFootprint {
		if !inside(x+d[0], y+d[1]) || w.Cell(x+d[0], y+d[1]).Code != 0 {
			return fmt.Errorf("whirlpool needs four untouched water parcels")
		}
	}
	id := w.allocateEffect(EffectWhirlpool, uint8(owner))
	if id < 0 {
		return fmt.Errorf("whirlpool exhausted effect reservations")
	}
	w.Water.Whirlpools[id] = WhirlpoolEffect{Active: true, Owner: uint8(owner), X: x, Y: y, Life: 300 + int(w.Players[owner].Experience[Water]), Delay: 16}
	w.paintWhirlpool(&w.Water.Whirlpools[id])
	return nil
}

func (w *World) paintWhirlpool(e *WhirlpoolEffect) {
	for quadrant, d := range whirlpoolFootprint {
		x, y := e.X+d[0], e.Y+d[1]
		if inside(x, y) && waterParcel(w.Cell(x, y).Code) {
			w.paintWater(x, y, uint8(152+e.Frame*4+quadrant))
		}
	}
}

// Whirlpool erases only its own previous frame before expiry or movement.
// When it reaches coastline it lowers four valid vertices of the first
// nonwater quadrant, then leaves later quadrants for a subsequent update.
func (w *World) tickWhirlpool(id int) {
	e := &w.Water.Whirlpools[id]
	if !e.Active {
		return
	}
	for quadrant, d := range whirlpoolFootprint {
		x, y := e.X+d[0], e.Y+d[1]
		if inside(x, y) && w.Cell(x, y).Code == uint8(152+e.Frame*4+quadrant) {
			w.paintWater(x, y, 0)
		}
	}
	e.Frame = (e.Frame + 1) % 4
	previousLife := e.Life
	e.Life = int(int16(uint16(e.Life) - 1))
	if previousLife <= 1 {
		e.Active = false
		w.releaseEffect(id)
		return
	}
	previousDelay := e.Delay
	e.Delay = int(int16(uint16(e.Delay) - 1))
	if previousDelay <= 1 {
		e.Delay = 16
		d := fireNeighbors[(w.random.next()&14)/2]
		x, y, valid := offsetFireParcel(e.X, e.Y, d[0], d[1])
		// Basalt is the named permanent barrier; no raw memory alias is read.
		if valid && w.Cell(x, y).Code != 224 {
			e.X, e.Y = x, y
		}
	}
	for quadrant, d := range whirlpoolFootprint {
		x, y := e.X+d[0], e.Y+d[1]
		if !inside(x, y) {
			continue
		}
		if waterParcel(w.Cell(x, y).Code) {
			w.paintWater(x, y, uint8(152+e.Frame*4+quadrant))
			continue
		}
		lowered := 0
		for at := quadrant * 4; at < len(whirlpoolCorners) && lowered < 4; at++ {
			c := whirlpoolCorners[at]
			cx, cy := e.X+c[0], e.Y+c[1]
			if !inside(cx, cy) {
				continue
			}
			w.directFireTerrain(cx, cy, false)
			lowered++
		}
		if waterParcel(w.Cell(x, y).Code) {
			w.paintWater(x, y, uint8(152+e.Frame*4+quadrant))
		}
		return
	}
}
