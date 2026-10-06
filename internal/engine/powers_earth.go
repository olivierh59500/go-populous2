package engine

import "fmt"

const WallCapacity = 200

type RoadParcel struct {
	Active      bool
	Code, Owner uint8
}
type WallActor struct {
	Active, Broken bool
	Owner          uint8
	X, Y           int
	Connections    uint8
	Variant        uint8
	Frame          uint8
	Gate           bool
	GateVertical   bool
	Next           uint16
}
type EarthState struct {
	Roads     [MapSize * MapSize]RoadParcel
	Walls     [WallCapacity]WallActor
	WallHeads [2]uint16
	Quakes    [EffectCapacity]QuakeEffect
	Cracks    [MapSize * MapSize]CrackParcel
}

type QuakePhase uint8

const (
	QuakeGrowing QuakePhase = iota
	QuakeWaiting
	QuakeFading
)

type QuakeEffect struct {
	Active                                   bool
	Owner                                    uint8
	X, Y, Direction, Descriptor, Life, Delay int
	Phase                                    QuakePhase
}
type CrackParcel struct {
	Active     bool
	Descriptor uint8
}

var roadOffsets = [4][2]int{{-1, 0}, {0, 1}, {1, 0}, {0, -1}}
var wallOffsets = [4][2]int{{0, -1}, {1, 0}, {-1, 0}, {0, 1}}
var wallVariants = [16]uint8{0, 2, 4, 0, 4, 0, 4, 0, 2, 2, 0, 0, 0, 0, 0, 0}

func (s *EarthState) TileCode(x, y int) (uint8, bool) {
	if s == nil || !inside(x, y) {
		return 0, false
	}
	p := s.Roads[x+y*MapSize]
	if crack := s.Cracks[x+y*MapSize]; crack.Active {
		return 172 + crack.Descriptor, true
	}
	return p.Code, p.Active
}
func (w *World) ClearEarthTerrain(x, y int) {
	if inside(x, y) {
		w.Earth.Roads[x+y*MapSize] = RoadParcel{}
		w.Earth.Cracks[x+y*MapSize] = CrackParcel{}
	}
}
func roadCode(code uint8) bool { return code >= 197 && code <= 216 }
func roadConnections(code uint8) uint8 {
	if code >= 201 && code <= 216 {
		return code - 201
	}
	if code == 197 || code == 200 {
		return 5
	}
	if code == 198 || code == 199 {
		return 10
	}
	return 0
}
func flatRoadEligible(owner int, code uint8) bool {
	return code == 15 || code == 31 || code == 95 || code == 145 || code == 151 || code == 245 || owner == 0 && code == 47 || owner == 1 && code == 63
}

func (w *World) paintRoad(owner, x, y int, code uint8) {
	at := x + y*MapSize
	w.Earth.Roads[at] = RoadParcel{Active: true, Code: code, Owner: uint8(owner)}
	w.Nature.Ground[at] = GroundParcel{}
	w.Water.Painted[at] = false
	w.FireDamage.Painted[at] = false
}

// CastRoad uses four connection bits and distinct artwork for the four
// supported ramps. Joining a neighbour changes its connections, not its owner.
func (w *World) CastRoad(owner, x, y int) error {
	if owner < 0 || owner > 1 || !inside(x, y) {
		return fmt.Errorf("invalid road target")
	}
	code := w.Cell(x, y).Code
	if code >= 220 && code <= 223 {
		return fmt.Errorf("road cannot be built on burning lava")
	}
	var tile uint8
	if flatRoadEligible(owner, code) {
		connections := uint8(0)
		for i, d := range roadOffsets {
			if inside(x+d[0], y+d[1]) && roadCode(w.Cell(x+d[0], y+d[1]).Code) {
				connections |= 1 << uint(3-i)
			}
		}
		tile = 201 + connections
	} else {
		for i, shape := range [4]uint8{3, 6, 9, 12} {
			if code == shape {
				tile = 197 + uint8(i)
				break
			}
		}
		if tile == 0 {
			return fmt.Errorf("road needs eligible land or a supported ramp")
		}
	}
	w.paintRoad(owner, x, y, tile)
	connections := roadConnections(tile)
	for i, d := range roadOffsets {
		if connections&(1<<uint(3-i)) == 0 {
			continue
		}
		nx, ny := x+d[0], y+d[1]
		if !inside(nx, ny) {
			continue
		}
		neighbor := w.Cell(nx, ny).Code
		if neighbor < 201 || neighbor > 216 {
			continue
		}
		owner := w.Earth.Roads[nx+ny*MapSize].Owner
		mask := roadConnections(neighbor) | 1<<uint([4]int{1, 0, 3, 2}[i])
		w.paintRoad(int(owner), nx, ny, 201+mask)
	}
	return nil
}

func (w *World) RemoveRoad(x, y int) bool {
	if !inside(x, y) || !roadCode(w.Cell(x, y).Code) {
		return false
	}
	w.Earth.Roads[x+y*MapSize] = RoadParcel{}
	return true
}

func (w *World) WallAt(x, y int) int {
	for id, a := range w.Earth.Walls {
		if a.Active && !a.Broken && a.X == x && a.Y == y {
			return id
		}
	}
	return -1
}
func (w *World) WallBlocksLightning(x, y int) bool { return w.WallAt(x, y) >= 0 }

func wallEligible(owner int, code uint8) bool {
	return code == 15 || code == 31 || code == 145 || code == 151 || code == 245 || roadCode(code) || owner == 0 && code == 47 || owner == 1 && code == 63
}

// CastWall updates the deity's construction head before checking adjacency.
// This retains the observable disconnected-placement rejection, while links
// use bounded actor IDs rather than pointers into a binary memory layout.
func (w *World) CastWall(owner, x, y int) error {
	if owner < 0 || owner > 1 || !inside(x, y) {
		return fmt.Errorf("invalid wall target")
	}
	code := w.Cell(x, y).Code
	if !wallEligible(owner, code) || w.WallAt(x, y) >= 0 {
		return fmt.Errorf("wall cannot be placed here")
	}
	id := -1
	for index, a := range w.Earth.Walls {
		if !a.Active {
			id = index
			break
		}
	}
	if id < 0 {
		return fmt.Errorf("wall pool is full")
	}
	next := w.Earth.WallHeads[owner]
	w.Earth.WallHeads[owner] = uint16(id + 1)
	w.Earth.Walls[id].Next = next
	connections := uint8(0)
	neighbors := [4]int{-1, -1, -1, -1}
	for i, d := range wallOffsets {
		connections <<= 1
		neighbors[i] = w.WallAt(x+d[0], y+d[1])
		if neighbors[i] >= 0 {
			connections |= 1
		}
	}
	if connections == 0 && next != 0 {
		return fmt.Errorf("new walls must join an existing wall")
	}
	variant := wallVariants[connections]
	w.Earth.Walls[id] = WallActor{Active: true, Owner: uint8(owner), X: x, Y: y, Connections: connections, Variant: variant, Next: next}
	w.syncWallActor(id)
	for _, neighbor := range neighbors {
		if neighbor < 0 || variant == 0 {
			continue
		}
		a := &w.Earth.Walls[neighbor]
		if a.Variant == 2 && variant != 2 || a.Variant == 4 && variant != 4 {
			a.Variant = 8
			a.Connections = 15
			a.Frame = 0
		}
	}
	if roadCode(code) {
		a := &w.Earth.Walls[id]
		a.Gate = true
		a.Variant = 8
		if roadConnections(code)&5 != 0 {
			a.Variant = 6
			a.GateVertical = true
		}
	}
	return nil
}

// tickWalls holds completed construction on its final artwork frame.
func (w *World) tickWalls() {
	for id := range w.Earth.Walls {
		a := &w.Earth.Walls[id]
		if !a.Active {
			continue
		}
		code := w.Cell(a.X, a.Y).Code
		if !wallEligible(int(a.Owner), code) && code != 95 && code != 63 && code != 47 {
			if w.Earth.WallHeads[a.Owner] == uint16(id+1) {
				w.Earth.WallHeads[a.Owner] = a.Next
			}
			a.Active = false
			w.syncWallActor(id)
			continue
		}
		limit := uint8(2)
		if a.Broken {
			limit = [5]uint8{1, 1, 1, 13, 12}[a.Variant/2]
		}
		if a.Frame < limit {
			a.Frame++
		}
	}
}

type WallCrossing uint8

const (
	WallBlocked WallCrossing = iota
	WallPass
	WallClimb
	WallBreak
)

// Ordinary movement uses the crossing prepass's strict break comparison and
// inclusive climb threshold. The deity's experience belongs to the walker.
func (w *World) DecideWallCrossing(followerID, wallID int) WallCrossing {
	if followerID <= 0 || followerID >= FollowerCapacity || wallID < 0 || wallID >= WallCapacity {
		return WallBlocked
	}
	f, a := w.Followers[followerID], w.Earth.Walls[wallID]
	if !a.Active || a.Broken || f.Owner == a.Owner {
		return WallPass
	}
	bonus := int(w.Players[f.Owner].Experience[Earth]) * 128
	if f.Population > 20000+bonus {
		return WallBreak
	}
	if f.Population >= 3000+bonus {
		return WallClimb
	}
	return WallBlocked
}

func (w *World) BreakWall(id int) bool {
	if id < 0 || id >= WallCapacity {
		return false
	}
	a := &w.Earth.Walls[id]
	if !a.Active || a.Broken {
		return false
	}
	a.Broken = true
	a.Frame = 0
	return true
}

func (w *World) tickEarthEffect(id int) {
	if w.effects.Slots[id].Kind == EffectEarthquake {
		w.tickEarthquake(id)
	}
}

// CastBatholith samples one point from the same random word for both axes.
// Its raise and boulder branches are independent of the follower's commands.
func (w *World) CastBatholith(owner, x, y int) error {
	if owner < 0 || owner > 1 {
		return fmt.Errorf("invalid batholith owner")
	}
	return w.createBatholith(owner, x, y)
}

func (w *World) createBatholith(owner, x, y int) error {
	if owner < 0 || owner > 2 || !inside(x, y) {
		return fmt.Errorf("invalid batholith target")
	}
	bits := w.random.next()
	x += int(bits&7) - 4
	if !inside(x, y) {
		return fmt.Errorf("batholith sample lies outside the world")
	}
	y += int(bits>>8&7) - 4
	if !inside(x, y) {
		return fmt.Errorf("batholith sample lies outside the world")
	}
	if int(w.random.next()%9)-4 >= 0 {
		w.directEarthTerrain(x, y, true)
		return nil
	}
	variant := uint8(w.random.next() % 8 / 2)
	code := w.Cell(x, y).Code
	if code == 0 || roadCode(code) || w.Occupants[x+y*MapSize] != 0 || w.Nature.sceneryAt(x, y) >= 0 {
		return nil
	}
	id := -1
	for index, a := range w.Nature.Scenery {
		if a.Kind == SceneryNone {
			id = index
			break
		}
	}
	if id < 0 {
		return nil
	}
	if w.random.next()%90 == 0 {
		variant = 0
	}
	w.Nature.Scenery[id] = SceneryActor{Kind: SceneryBoulder, X: uint8(x), Y: uint8(y), Age: 24, Variant: variant}
	w.syncSceneryActor(id)
	return nil
}

var quakeOffsets = [16][2]int{{0, -1}, {0, 1}, {0, -1}, {0, 1}, {-1, 0}, {1, 0}, {-1, 0}, {1, 0}, {1, 0}, {0, 1}, {0, -1}, {-1, 0}, {1, 0}, {0, -1}, {-1, 0}, {0, 1}}
var quakeBranches = [16][6]uint8{
	{0, 0, 2, 2, 8, 14}, {1, 1, 3, 3, 11, 12}, {2, 2, 0, 0, 8, 14}, {3, 3, 1, 1, 11, 12},
	{4, 4, 6, 6, 9, 13}, {5, 5, 7, 7, 10, 15}, {6, 6, 4, 4, 9, 13}, {7, 7, 5, 5, 10, 15},
	{5, 5, 7, 7, 10, 15}, {1, 1, 3, 3, 11, 12}, {0, 0, 2, 2, 8, 14}, {4, 4, 6, 6, 9, 13},
	{5, 5, 7, 7, 10, 15}, {0, 0, 2, 2, 8, 14}, {4, 4, 6, 6, 9, 13}, {1, 1, 3, 3, 11, 12},
}

func (w *World) paintCrack(x, y, descriptor int) {
	at := x + y*MapSize
	w.Earth.Roads[at] = RoadParcel{}
	w.Earth.Cracks[at] = CrackParcel{Active: true, Descriptor: uint8(descriptor)}
	w.Nature.Ground[at] = GroundParcel{}
	w.Water.Painted[at] = false
	w.FireDamage.Painted[at] = false
}

// surfaceQuake reports the pre-lowering height. Flat ground receives crack
// art immediately; a higher slope lowers once and grows again on a later pass.
func (w *World) surfaceQuake(e *QuakeEffect) int {
	c := w.Cell(e.X, e.Y)
	if c.Code >= 172 && c.Code <= 196 {
		return 0
	}
	if flowersNatureCode(c.Code) {
		w.paintCrack(e.X, e.Y, e.Descriptor)
		return 0
	}
	height := int(c.BaseAltitude&7) + int(c.Shape&1)
	if height == 0 {
		e.Active = false
		return -1
	}
	if height > 1 {
		w.directEarthTerrain(e.X, e.Y, false)
	}
	return height
}

// Direct natural terrain changes clear parcel decorations whose corner
// geometry changed. Permanent basalt retains its updated surface shape.
func (w *World) directEarthTerrain(x, y int, raise bool) {
	before := w.Heights
	w.directFireTerrain(x, y, raise)
	for cy := 0; cy < MapSize; cy++ {
		for cx := 0; cx < MapSize; cx++ {
			at := cx + cy*CornerSize
			if before[at] != w.Heights[at] || before[at+1] != w.Heights[at+1] || before[at+CornerSize] != w.Heights[at+CornerSize] || before[at+CornerSize+1] != w.Heights[at+CornerSize+1] {
				w.ClearEarthTerrain(cx, cy)
				w.ClearWaterTerrain(cx, cy)
			}
		}
	}
}

func (w *World) createEarthquake(owner uint8, x, y, direction, strength int) int {
	if owner > 2 || !inside(x, y) || direction < 0 || direction >= 16 {
		return -1
	}
	id := w.allocateEffect(EffectEarthquake, owner)
	if id < 0 {
		return -1
	}
	e := QuakeEffect{Active: true, Owner: owner, X: x, Y: y, Direction: direction, Descriptor: direction / 2, Life: strength, Delay: 2}
	w.Earth.Quakes[id] = e
	if w.surfaceQuake(&w.Earth.Quakes[id]) < 0 {
		w.releaseEffect(id)
	}
	return id
}

func (w *World) CastEarthquake(owner, x, y, direction int) error {
	if owner < 0 || owner > 1 || !inside(x, y) || direction < 0 || direction > 3 {
		return fmt.Errorf("invalid earthquake target")
	}
	// The original action charges even when no active front is admitted.
	w.createEarthquake(uint8(owner), x, y, int([4]uint8{0, 5, 1, 4}[direction]), 33+int(w.Players[owner].Experience[Earth]))
	return nil
}

// CreateEarthquake is the unpriced environmental creator. Direction identifies
// one of the sixteen named crack branch shapes, not a procedure selector.
func (w *World) CreateEarthquake(owner uint8, x, y, direction, strength int) bool {
	return w.createEarthquake(owner, x, y, direction, strength) >= 0
}

func (w *World) tickEarthquake(id int) {
	e := &w.Earth.Quakes[id]
	if !e.Active {
		return
	}
	finish := func() { e.Active = false; w.releaseEffect(id) }
	previousLife := e.Life
	e.Life = int(int16(uint16(e.Life) - 1))
	if e.Phase != QuakeFading && previousLife <= 1 {
		e.Phase = QuakeFading
		e.Life = 49
		previousLife = 50
	}
	if e.Phase == QuakeFading && previousLife <= 1 {
		finish()
		return
	}
	if e.Phase == QuakeWaiting {
		if w.surfaceQuake(e) < 0 {
			finish()
		}
		return
	}
	previousDelay := e.Delay
	e.Delay = int(int16(uint16(e.Delay) - 1))
	if previousDelay > 1 {
		return
	}
	e.Delay = 4
	height := w.surfaceQuake(e)
	if height < 0 {
		finish()
		return
	}
	if e.Phase == QuakeFading {
		if height != 0 {
			return
		}
		next := e.Descriptor
		if next < 16 {
			next += 8
		}
		if next != e.Descriptor {
			e.Descriptor = next
			w.paintCrack(e.X, e.Y, next)
		}
		return
	}
	if height > 1 {
		return
	}
	e.Phase = QuakeWaiting
	direction := int(quakeBranches[e.Direction][w.random.next()%6])
	d := quakeOffsets[e.Direction]
	child := w.createEarthquake(e.Owner, e.X+d[0], e.Y+d[1], direction, e.Life)
	if child >= id {
		w.Earth.Quakes[child].Life++
	}
}
