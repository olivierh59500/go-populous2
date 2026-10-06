package engine

import "fmt"

type FireVictimMode uint8

const (
	FireVictimAlive FireVictimMode = iota
	FireVictimDying
	FireVictimTownRuin
	FireVictimBurning
)

type FireVictimDeath struct {
	Mode          FireVictimMode
	Frame, Frames int
	TownStage     uint8
}

// FireDamageState retains victims and temporary painted terrain independently
// of the moving effect controllers. Terrain edits clear the painted override;
// scorched ground is represented by the shared nature ground state.
type FireDamageState struct {
	Deaths  [FollowerCapacity]FireVictimDeath
	Painted [MapSize * MapSize]bool
	Tiles   [MapSize * MapSize]uint8
}

type worldFireHabitat struct{ world *World }

func (h worldFireHabitat) Reserve(kind EffectKind, owner uint8) int {
	id := h.world.allocateEffect(kind, owner)
	if id >= 0 && kind == EffectFireColumn {
		previous := h.world.effects.Slots[id]
		h.world.Fire.Columns[id].VX, h.world.Fire.Columns[id].VY = previous.LastVelocityX, previous.LastVelocityY
	}
	return id
}
func (h worldFireHabitat) Release(id int) {
	w := h.world
	if id >= 0 && id < EffectCapacity && w.effects.Slots[id].Kind == EffectFireColumn {
		w.effects.Slots[id].LastVelocityX, w.effects.Slots[id].LastVelocityY = w.Fire.Columns[id].VX, w.Fire.Columns[id].VY
	}
	w.releaseEffect(id)
}
func (h worldFireHabitat) Random() uint16 { return h.world.random.next() }
func (h worldFireHabitat) FireExperience(owner uint8) uint8 {
	if owner > 1 {
		return 0
	}
	return h.world.Players[owner].Experience[Fire]
}
func (h worldFireHabitat) Parcel(x, y int) FireParcel {
	if !inside(x, y) {
		return FireParcel{Water: true}
	}
	c := h.world.Cell(x, y)
	return FireParcel{Altitude: c.BaseAltitude & 7, Shape: c.Shape, Tile: c.Code, Water: c.IsWater(), ExtraRise: c.Shape&1 != 0}
}
func (h worldFireHabitat) Scorch(x, y int)      { h.world.scorchFireParcel(x, y) }
func (h worldFireHabitat) Damage(x, y int) int  { return h.world.damageFireParcel(x, y, false) }
func (h worldFireHabitat) LowerCorner(x, y int) { h.world.directFireTerrain(x, y, false) }
func (h worldFireHabitat) RaiseCorner(x, y int) { h.world.directFireTerrain(x, y, true) }
func (h worldFireHabitat) Paint(x, y int, tile uint8) {
	if inside(x, y) {
		at := x + y*MapSize
		h.world.FireDamage.Painted[at], h.world.FireDamage.Tiles[at] = true, tile
	}
}
func (h worldFireHabitat) CreateBasalt(owner uint8, x, y, direction int) {
	h.world.CreateBasalt(owner, x, y, direction, 100)
}
func (h worldFireHabitat) PushByLava(x, y, dx, dy int) { h.world.pushFireParcel(x, y, dx, dy) }

// CastFireColumn performs admission and allocation only. Cost and campaign
// availability are checked by the shared Cast entry before this method runs.
func (w *World) CastFireColumn(owner, x, y int) error {
	if owner < 0 || owner > 1 || !inside(x, y) {
		return fmt.Errorf("invalid fire-column target")
	}
	if !w.Fire.CreateColumn(uint8(owner), x, y, worldFireHabitat{w}) {
		return fmt.Errorf("fire column could not reserve an admitted effect")
	}
	return nil
}
func (w *World) CastFireRain(owner, x, y int) error {
	if owner < 0 || owner > 1 || !inside(x, y) {
		return fmt.Errorf("invalid fire-rain target")
	}
	if !w.Fire.CreateRain(uint8(owner), x, y, worldFireHabitat{w}) {
		return fmt.Errorf("fire rain exhausted the shared effect pool")
	}
	return nil
}
func (w *World) CastVolcano(owner, x, y int) error {
	if owner < 0 || owner > 1 || !inside(x, y) {
		return fmt.Errorf("invalid volcano target")
	}
	if !w.Fire.CreateVolcano(uint8(owner), x, y, worldFireHabitat{w}) {
		return fmt.Errorf("volcano exhausted the shared effect pool")
	}
	return nil
}

func (w *World) tickFireEffects() {
	for id := 0; id < EffectCapacity; id++ {
		w.tickFireEffect(id)
	}
}

// tickFireEffect lets the world's shared scheduler advance one reservation
// in ascending pool order, including newborn actors in later free slots.
func (w *World) tickFireEffect(id int) {
	h := worldFireHabitat{w}
	switch w.effects.Slots[id].Kind {
	case EffectFireColumn:
		w.Fire.TickColumn(id, h)
	case EffectFireRain:
		w.Fire.TickRain(id, h)
	case EffectVolcano:
		w.Fire.TickVolcano(id, h)
	case EffectLava:
		w.Fire.TickLava(id, w.Tick, h)
	}
}

func (w *World) FireTileCode(x, y int) (uint8, bool) {
	if !inside(x, y) {
		return 0, false
	}
	at := x + y*MapSize
	return w.FireDamage.Tiles[at], w.FireDamage.Painted[at]
}

func (w *World) ClearFireTerrain(x, y int) {
	if inside(x, y) {
		w.FireDamage.Painted[x+y*MapSize] = false
	}
}

func (w *World) directFireTerrain(x, y int, raise bool) {
	if !insideCorner(x, y) {
		return
	}
	before := w.Heights
	if raise {
		w.raiseCorner(x, y)
	} else {
		w.lowerCorner(x, y)
	}
	for cy := 0; cy < MapSize; cy++ {
		for cx := 0; cx < MapSize; cx++ {
			at := cx + cy*CornerSize
			if before[at] != w.Heights[at] || before[at+1] != w.Heights[at+1] || before[at+CornerSize] != w.Heights[at+CornerSize] || before[at+CornerSize+1] != w.Heights[at+CornerSize+1] {
				w.ClearFireTerrain(cx, cy)
				w.ClearEarthTerrain(cx, cy)
				w.ClearWaterTerrain(cx, cy)
				w.Nature.Ground[cx+cy*MapSize] = GroundParcel{}
				w.Pressure[cx+cy*MapSize] = 0
			}
		}
	}
	w.rebuildCells()
}

func (w *World) scorchFireParcel(x, y int) {
	if !inside(x, y) {
		return
	}
	c := w.Cell(x, y)
	if c.Shape == 15 && (c.Code < 220 || c.Code > 223) {
		w.ClearEarthTerrain(x, y)
		w.Water.Painted[x+y*MapSize] = false
		w.ClearFireTerrain(x, y)
		w.Nature.Ground[x+y*MapSize] = GroundParcel{Mark: GroundScorched}
	}
}

// BurnFireCell applies direct fire to this parcel. Tree spread uses its own
// entry because living heroes are immune to the neighboring tree burn.
func (w *World) BurnFireCell(x, y int) int {
	w.scorchFireParcel(x, y)
	return w.damageFireParcel(x, y, false)
}
func (w *World) SpreadTreeFireCell(x, y int) int { return w.damageFireParcel(x, y, true) }

var fireTownDeathFrames = [TownStages]int{14, 14, 14, 13, 14, 15, 13, 13, 13, 14, 8, 14, 9, 12, 14, 14, 14, 13, 13}

func (w *World) damageFireParcel(x, y int, treeSpread bool) int {
	if !inside(x, y) {
		return 0
	}
	for id := range w.Nature.Scenery {
		a := &w.Nature.Scenery[id]
		if a.Kind == SceneryTree && int(a.X) == x && int(a.Y) == y {
			a.Kind, a.Frame = SceneryBurningTree, 0
		}
	}
	hits := 0
	for id := 1; id < FollowerCapacity; id++ {
		f := &w.Followers[id]
		if f.State == Inactive || f.State == Ruin || int(f.X) != x || int(f.Y) != y {
			continue
		}
		if f.State != Town && (f.Consecrated || f.ImmuneToBurning() || treeSpread && f.IsHero()) {
			continue
		}
		death := FireVictimDeath{Mode: FireVictimDying, Frames: 9}
		if f.State == Town {
			stage := min(TownStages-1, int(f.Stage))
			death.Mode, death.TownStage, death.Frames = FireVictimTownRuin, f.Stage, fireTownDeathFrames[stage]
			w.clearBurntTownFarms(id)
		}
		f.Population, f.State, f.Frame = 0, Ruin, 0
		f.moving, f.BattleWith, f.BattleAggressor = false, 0, false
		w.FireDamage.Deaths[id] = death
		hits++
	}
	return hits
}

func (w *World) clearBurntTownFarms(id int) {
	f := w.Followers[id]
	limit := 9
	if f.Stage >= 10 {
		limit = 25
	}
	if f.Stage >= 18 {
		limit = 49
	}
	for _, d := range townFootprint[:limit] {
		x, y := int(f.X)+d[0], int(f.Y)+d[1]
		if inside(x, y) && w.Farms[x+y*MapSize] == f.Owner+1 {
			w.Farms[x+y*MapSize] = 0
			w.Nature.Ground[x+y*MapSize] = GroundParcel{Mark: GroundScorched}
		}
	}
}

// AdvanceFireDeath keeps the original allocation and occupancy until a direct
// death animation finishes. Lava burning instead damages a living population
// every pass, with a looping animation and complete cleanup on depletion.
func (w *World) AdvanceFireDeath(id int) bool {
	if id <= 0 || id >= FollowerCapacity {
		return false
	}
	d := &w.FireDamage.Deaths[id]
	if d.Mode == FireVictimAlive {
		return false
	}
	f := &w.Followers[id]
	if f.State == Inactive {
		*d = FireVictimDeath{}
		return false
	}
	if d.Mode == FireVictimBurning {
		d.Frame = (d.Frame + 1) % d.Frames
		f.Frame = uint16(d.Frame)
		population := uint32(f.Population)
		damage := population >> 4
		damage = damage&0xffff0000 | uint32(uint16(damage)+4)
		if int64(int32(population))-int64(int32(damage)) <= 0 {
			w.remove(id)
			*d = FireVictimDeath{}
		} else {
			f.Population = int(int32(population - damage))
		}
		return true
	}
	d.Frame++
	if d.Frame >= d.Frames {
		w.remove(id)
		*d = FireVictimDeath{}
	} else {
		f.Frame = uint16(d.Frame)
	}
	return true
}

func (w *World) pushFireParcel(x, y, dx, dy int) {
	for id := 1; id < FollowerCapacity; id++ {
		f := &w.Followers[id]
		if f.State == Inactive || int(f.X) != x || int(f.Y) != y || f.Consecrated || f.ImmuneToBurning() {
			continue
		}
		if f.State == Town {
			w.clearBurntTownFarms(id)
		}
		if w.FireDamage.Deaths[id].Mode != FireVictimBurning {
			frames := 21
			if f.IsHero() {
				frames = 9
			}
			w.FireDamage.Deaths[id] = FireVictimDeath{Mode: FireVictimBurning, Frames: frames}
			f.State, f.Frame, f.moving = Ruin, 0, false
		}
		f.initialisePosition()
		nx, ny := f.positionX+dx, f.positionY+dy
		if nx < 0 || ny < 0 || nx >= MapSize*256 || ny >= MapSize*256 {
			w.remove(id)
			continue
		}
		old := int(f.X) + int(f.Y)*MapSize
		if w.Occupants[old] == uint16(id) {
			w.Occupants[old] = 0
		}
		f.positionX, f.positionY = nx, ny
		f.X, f.Y = uint8(nx>>8), uint8(ny>>8)
		at := int(f.X) + int(f.Y)*MapSize
		if w.Occupants[at] == 0 {
			w.Occupants[at] = uint16(id)
		}
	}
}
