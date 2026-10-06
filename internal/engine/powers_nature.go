package engine

import (
	"errors"
	"fmt"
)

var ErrNatureNoChange = errors.New("nature cast created no scenery")

const SceneryCapacity = 200

type SceneryKind uint8

const (
	SceneryNone SceneryKind = iota
	SceneryTree
	SceneryBoulder
	SceneryBurningTree
)

type SceneryActor struct {
	Kind    SceneryKind
	X, Y    uint8
	Age     int8
	Variant uint8
	Frame   uint16
}

type GroundMark uint8

const (
	GroundNone GroundMark = iota
	GroundFlowers
	GroundSwamp
	GroundFungusFresh
	GroundFungusYoung
	GroundFungusGrowing
	GroundFungusMature
	GroundFungusOld
	GroundFungusDying
	GroundFungusDead
	GroundRestored
	GroundScorched
	GroundBaptism
)

type GroundParcel struct {
	Mark  GroundMark
	Owner uint8
}

type FungusController struct {
	Active, Collecting                 bool
	Owner                              uint8
	Wait, Period                       int
	MinX, MinY, MaxX, MaxY             int
	AgeMinX, AgeMinY, AgeMaxX, AgeMaxY int
}

type NatureDeath uint8

const (
	NatureAlive NatureDeath = iota
	NatureSwampDeath
	NatureFungusDeath
)

// NatureState contains landscape decoration, ground effects and their timed
// controllers. A fungus controller uses the world's shared effect-slot budget.
type NatureState struct {
	Ground        [MapSize * MapSize]GroundParcel
	Scenery       [SceneryCapacity]SceneryActor
	Fungi         [EffectCapacity]FungusController
	PendingFungus [2]uint16
	Deaths        [FollowerCapacity]NatureDeath
	DeathFrames   [FollowerCapacity]uint16
}

// The sampling footprint is intentionally asymmetric. These parcels are
// selected randomly; a cast does not fill a disc or every parcel in a radius.
var natureOffsets = [45][2]int{
	{0, -4}, {-1, -3}, {0, -3}, {1, -3},
	{-3, -2}, {-2, -2}, {-1, -2}, {0, -2}, {1, -2}, {2, -2}, {3, -2},
	{-4, -1}, {-3, -1}, {-2, -1}, {-1, -1}, {0, -1}, {1, -1}, {2, -1}, {3, -1}, {4, -1},
	{-4, 0}, {-3, 0}, {-2, 0}, {-1, 0}, {0, 0},
	{-4, 1}, {-3, 1}, {-2, 1}, {-1, 1}, {0, 1}, {1, 1}, {2, 1}, {3, 1}, {4, 1},
	{-3, 2}, {-2, 2}, {-1, 2}, {0, 2}, {1, 2}, {2, 2}, {3, 2},
	{-1, 3}, {0, 3}, {1, 3}, {0, 4},
}

func (n *NatureState) TileCode(x, y int) (uint8, bool) {
	if n == nil || !inside(x, y) {
		return 0, false
	}
	switch n.Ground[x+y*MapSize].Mark {
	case GroundFlowers:
		return 245, true
	case GroundSwamp:
		return 168, true
	case GroundRestored:
		return 15, true
	case GroundScorched:
		return 95, true
	case GroundBaptism:
		return 143, true
	case GroundFungusFresh, GroundFungusYoung, GroundFungusGrowing, GroundFungusMature, GroundFungusOld, GroundFungusDying, GroundFungusDead:
		return 145 + uint8(n.Ground[x+y*MapSize].Mark-GroundFungusFresh), true
	default:
		return 0, false
	}
}

func (n *NatureState) sceneryAt(x, y int) int {
	for id, actor := range n.Scenery {
		if actor.Kind != SceneryNone && int(actor.X) == x && int(actor.Y) == y {
			return id
		}
	}
	return -1
}

// Trees can be walked through; boulders block admission to their parcel.
func (n *NatureState) BlocksWalking(x, y int) bool {
	if n == nil {
		return false
	}
	id := n.sceneryAt(x, y)
	return id >= 0 && n.Scenery[id].Kind == SceneryBoulder
}

// NatureTownAllowed checks support land independently of its geometric height.
func (w *World) NatureTownAllowed(owner, x, y int) bool {
	if owner < 0 || owner > 1 || !inside(x, y) || w.Nature.sceneryAt(x, y) >= 0 {
		return false
	}
	mark := w.Nature.Ground[x+y*MapSize].Mark
	return mark == GroundNone || mark == GroundFlowers || mark == GroundRestored
}

func (w *World) natureSample(x, y int) (int, int, bool) {
	d := natureOffsets[(int(w.random.next())%90)/2]
	x, y = x+d[0], y+d[1]
	return x, y, inside(x, y)
}

func (w *World) validNatureTarget(owner, x, y int) error {
	if owner < 0 || owner > 1 || !inside(x, y) {
		return fmt.Errorf("invalid nature target")
	}
	return nil
}

func fertileNatureCode(code uint8) bool {
	return code == 15 || code == 31 || code == 47 || code == 63 || code == 95 || code == 145 || code == 151 || code == 245
}

func flowersNatureCode(code uint8) bool {
	return code >= 15 && code <= 127 && code%16 == 15 || code >= 143 && code <= 151 || code >= 168 && code <= 196 || code >= 201 && code <= 216 || code >= 220 && code <= 223 || code == 245
}

func (w *World) paintNature(at int, p GroundParcel) {
	w.ClearEarthTerrain(at%MapSize, at/MapSize)
	w.Nature.Ground[at] = p
	w.Water.Painted[at] = false
	w.FireDamage.Painted[at] = false
}

// CastTrees plants empty sampled parcels. A full scenery pool ends the cast
// immediately, and rejected parcels do not consume the rare-variant draw.
func (w *World) CastTrees(owner, x, y int) error {
	if err := w.validNatureTarget(owner, x, y); err != nil {
		return err
	}
	return w.plantTrees(x, y, int(w.Players[owner].Experience[Plants]>>4))
}

// CastNeutralTrees uses the same scenery creator without a faction ledger.
func (w *World) CastNeutralTrees(x, y int) error {
	if !inside(x, y) {
		return fmt.Errorf("invalid neutral forest target")
	}
	return w.plantTrees(x, y, 0)
}

func (w *World) plantTrees(x, y, experienceAttempts int) error {
	bits := int(w.random.next())
	variant := uint8((bits % 8) / 2)
	count := bits%14 + experienceAttempts
	planted := 0
	for attempt := 0; attempt <= count; attempt++ {
		nx, ny, ok := w.natureSample(x, y)
		if !ok {
			continue
		}
		at := nx + ny*MapSize
		if w.Tiles[at].IsWater() || w.Occupants[at] != 0 || w.Nature.sceneryAt(nx, ny) >= 0 {
			continue
		}
		// Swamp does not reject trees: the original game lets decoration
		// coexist with its animated ground. Decomposed fungus rejects trees.
		if mark := w.Nature.Ground[at].Mark; mark == GroundFungusDead {
			continue
		}
		free := -1
		for id, actor := range w.Nature.Scenery {
			if actor.Kind == SceneryNone {
				free = id
				break
			}
		}
		if free < 0 {
			if planted == 0 {
				return ErrNatureNoChange
			}
			return nil
		}
		selected := variant
		if w.random.next()%90 == 0 {
			selected = 0
		}
		w.Nature.Scenery[free] = SceneryActor{Kind: SceneryTree, X: uint8(nx), Y: uint8(ny), Age: 24, Variant: selected}
		w.syncSceneryActor(free)
		planted++
	}
	if planted == 0 {
		return ErrNatureNoChange
	}
	return nil
}

// CastFlowers restores sampled flat parcels, including damaged ground, while
// leaving ownership, followers and occupancy unchanged.
func (w *World) CastFlowers(owner, x, y int) error {
	if err := w.validNatureTarget(owner, x, y); err != nil {
		return err
	}
	count := int(w.random.next()%17) + 8 + int(w.Players[owner].Experience[Plants]>>5)
	for attempt := 0; attempt <= count; attempt++ {
		nx, ny, ok := w.natureSample(x, y)
		if !ok || !flowersNatureCode(w.Cell(nx, ny).Code) {
			continue
		}
		w.paintNature(nx+ny*MapSize, GroundParcel{Mark: GroundFlowers, Owner: uint8(owner)})
	}
	return nil
}

// CastSwamp consumes the same sampled attempts at every experience tier.
// Occupied parcels, water and sloped ground reject planting.
func (w *World) CastSwamp(owner, x, y int) error {
	if err := w.validNatureTarget(owner, x, y); err != nil {
		return err
	}
	count := int(w.random.next()%17) + 8
	for attempt := 0; attempt <= count; attempt++ {
		nx, ny, ok := w.natureSample(x, y)
		if !ok {
			continue
		}
		at := nx + ny*MapSize
		if !fertileNatureCode(w.Cell(nx, ny).Code) || w.Occupants[at] != 0 || w.Nature.sceneryAt(nx, ny) >= 0 {
			continue
		}
		w.paintNature(at, GroundParcel{Mark: GroundSwamp, Owner: uint8(owner)})
	}
	return nil
}

// CastFungus plants first, then reserves or extends a collecting controller.
// Planting still succeeds when the shared effect pool is exhausted.
func (w *World) CastFungus(owner, x, y int) error {
	if err := w.validNatureTarget(owner, x, y); err != nil {
		return err
	}
	at := x + y*MapSize
	if !fertileNatureCode(w.Cell(x, y).Code) {
		return nil
	}
	w.paintNature(at, GroundParcel{Mark: GroundFungusFresh, Owner: uint8(owner)})
	if pending := w.Nature.PendingFungus[owner]; pending > 0 {
		id := int(pending) - 1
		f := &w.Nature.Fungi[id]
		if f.Active && f.Collecting {
			// A later seed expands the positive side by two parcels; a seed
			// at/before the minimum instead shifts the rectangle's origin.
			if x <= f.MinX {
				f.MinX = x
			} else {
				f.MaxX = max(f.MaxX, x+2)
			}
			if y < f.MinY {
				f.MinY = y
			} else {
				f.MaxY = max(f.MaxY, y+2)
			}
			f.AgeMinX, f.AgeMinY, f.AgeMaxX, f.AgeMaxY = f.MinX, f.MinY, f.MaxX, f.MaxY
			return nil
		}
	}
	id := w.allocateEffect(EffectFungus, uint8(owner))
	if id < 0 {
		return nil
	}
	period := 10 - int(w.Players[owner].Experience[Plants]>>5)
	w.Nature.Fungi[id] = FungusController{Active: true, Collecting: true, Owner: uint8(owner), Wait: 100, Period: period, MinX: x - 1, MinY: y - 1, MaxX: x + 1, MaxY: y + 1, AgeMinX: x - 1, AgeMinY: y - 1, AgeMaxX: x + 1, AgeMaxY: y + 1}
	w.Nature.PendingFungus[owner] = uint16(id + 1)
	return nil
}

func (w *World) tickNature() {
	w.tickNatureScenery()
	for id := range w.Nature.Fungi {
		w.tickNatureEffect(id)
	}
}

func (w *World) tickNatureScenery() {
	for id := range w.Nature.Scenery {
		a := &w.Nature.Scenery[id]
		if a.Kind == SceneryNone {
			continue
		}
		if a.Kind == SceneryBurningTree {
			if a.Age != 0 {
				magnitude := int(a.Age)
				if magnitude < 0 {
					magnitude = -magnitude
				}
				magnitude++
				if magnitude == 24 {
					*a = SceneryActor{}
					w.syncSceneryActor(id)
					continue
				}
				a.Age = -int8(magnitude)
				a.Frame++
				continue
			}
			a.Frame++
			if a.Frame >= 4 {
				a.Age = -1
				a.Frame = 0
				for _, d := range [...][2]int{{0, -1}, {0, 1}, {1, 0}, {-1, 0}} {
					x, y := int(a.X)+d[0], int(a.Y)+d[1]
					if inside(x, y) {
						w.SpreadTreeFireCell(x, y)
					}
				}
			}
			continue
		}
		a.Frame++
		hazard := w.Tiles[int(a.X)+int(a.Y)*MapSize].IsWater()
		if a.Age < 0 {
			a.Age--
			if a.Age == -24 {
				*a = SceneryActor{}
				w.syncSceneryActor(id)
				continue
			}
			if !hazard {
				a.Age = -a.Age
			}
		} else {
			if a.Age > 0 && w.Tick&3 == 0 {
				a.Age--
			}
			if hazard {
				a.Age = -(a.Age + 1)
			}
		}
	}
}

func (w *World) tickNatureEffect(id int) {
	f := &w.Nature.Fungi[id]
	if !f.Active || w.effects.Slots[id].Kind != EffectFungus {
		return
	}
	if f.Collecting {
		f.Wait--
		if f.Wait > 0 {
			return
		}
		f.Collecting = false
		w.Nature.PendingFungus[f.Owner] = 0
		f.Wait = f.Period
	}
	f.Wait--
	if f.Wait >= 0 {
		if f.Wait%(f.Period/3) == 0 {
			w.ageFungus(f)
		}
	} else {
		f.Wait = f.Period
		w.generateFungus(id, f)
	}
}

func (w *World) ageFungus(f *FungusController) {
	for y := max(0, f.AgeMinY); y <= min(MapSize-1, f.AgeMaxY); y++ {
		for x := max(0, f.AgeMinX); x <= min(MapSize-1, f.AgeMaxX); x++ {
			p := &w.Nature.Ground[x+y*MapSize]
			switch p.Mark {
			case GroundFungusFresh, GroundFungusYoung, GroundFungusGrowing, GroundFungusMature, GroundFungusDying:
				p.Mark++
			case GroundFungusDead:
				p.Mark = GroundRestored
			}
		}
	}
}

func (w *World) liveFungus(x, y int) bool {
	if !inside(x, y) {
		return false
	}
	mark := w.Nature.Ground[x+y*MapSize].Mark
	return mark >= GroundFungusYoung && mark <= GroundFungusDying
}

// Generations are ordered writes, rather than a buffered cellular automaton.
// Fresh births do not count as mature neighbours later in the same scan.
func (w *World) generateFungus(id int, f *FungusController) {
	minX, minY, maxX, maxY := MapSize, MapSize, -1, -1
	for y := max(0, f.MinY); y <= min(MapSize-1, f.MaxY); y++ {
		for x := max(0, f.MinX); x <= min(MapSize-1, f.MaxX); x++ {
			at := x + y*MapSize
			if !w.Tiles[at].IsFlat() {
				continue
			}
			mark := w.Nature.Ground[at].Mark
			eligible := mark == GroundNone || mark == GroundFlowers || mark == GroundRestored || mark == GroundScorched || mark >= GroundFungusFresh && mark <= GroundFungusDead
			if !eligible {
				continue
			}
			neighbors := 0
			for _, d := range directions {
				if w.liveFungus(x+d[0], y+d[1]) {
					neighbors++
				}
			}
			if mark >= GroundFungusFresh && mark <= GroundFungusDying {
				if neighbors != 2 && neighbors != 3 {
					w.Nature.Ground[at].Mark = GroundFungusDying
					continue
				}
			} else {
				if neighbors != 3 {
					continue
				}
				w.Nature.Ground[at] = GroundParcel{Mark: GroundFungusFresh, Owner: f.Owner}
			}
			minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
		}
	}
	if maxX < 0 {
		f.Active = false
		w.releaseEffect(id)
		return
	}
	oldMinX, oldMinY, oldDX, oldDY := f.MinX, f.MinY, f.MaxX-f.MinX, f.MaxY-f.MinY
	f.MinX, f.MinY = max(0, minX-1), max(0, minY-1)
	f.MaxX, f.MaxY = min(MapSize-1, maxX+1), min(MapSize-1, maxY+1)
	f.AgeMinX, f.AgeMinY = min(oldMinX, f.MinX), min(oldMinY, f.MinY)
	f.AgeMaxX, f.AgeMaxY = f.AgeMinX+max(oldDX, f.MaxX-f.MinX), f.AgeMinY+max(oldDY, f.MaxY-f.MinY)
}

// EnterNatureHazard retains an affected follower until its death sequence has
// completed. Newly planted fungus is harmless; mature fungus and swamp kill.
func (w *World) EnterNatureHazard(id int) bool {
	if id <= 0 || id >= len(w.Followers) {
		return false
	}
	f := &w.Followers[id]
	if f.State == Inactive {
		w.Nature.Deaths[id] = NatureAlive
		return false
	}
	death := w.Nature.Deaths[id]
	if death == NatureAlive {
		mark := w.Nature.Ground[int(f.X)+int(f.Y)*MapSize].Mark
		switch {
		case mark == GroundSwamp:
			if f.ImmuneToSwamp() {
				return false
			}
			death = NatureSwampDeath
			if w.Level.Players[f.Owner].Scenario.ShallowSwamps {
				w.Nature.Ground[int(f.X)+int(f.Y)*MapSize] = GroundParcel{Mark: GroundRestored}
			}
		case mark >= GroundFungusYoung && mark <= GroundFungusDying:
			if f.ImmuneToFungus() {
				return false
			}
			death = NatureFungusDeath
		default:
			return false
		}
		w.Nature.Deaths[id] = death
		w.Nature.DeathFrames[id] = 0
		f.State = Ruin
		return true
	}
	w.Nature.DeathFrames[id]++
	hazard := HazardFungus
	if death == NatureSwampDeath {
		hazard = HazardSwamp
	}
	if int(w.Nature.DeathFrames[id]) >= f.HazardDeathFrames(hazard) {
		w.remove(id)
		w.Nature.Deaths[id] = NatureAlive
		w.Nature.DeathFrames[id] = 0
	}
	return true
}
