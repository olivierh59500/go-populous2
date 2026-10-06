package engine

import "fmt"

type BasaltEffect struct {
	Active             bool
	Owner              uint8
	X, Y               int
	Direction          int // North, east, south, west.
	Life, Delay, Frame int
}

// WaterEffects uses the same effect reservations as every other power.
// Deposited terrain is independent of the actor that briefly animates it.
type WaterEffects struct {
	Basalt  [EffectCapacity]BasaltEffect
	Painted [MapSize * MapSize]bool
	Tiles   [MapSize * MapSize]uint8
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
		if w.effects.Slots[id].Kind == EffectBasalt {
			w.tickBasalt(id)
		}
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
