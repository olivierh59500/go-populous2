package engine

// FireHabitat supplies the interactions shared by fire powers. Damage owns
// follower, hero, tree and town lifecycles; controllers do not substitute an
// immediate deletion for the victim's own animation and cleanup.
type FireHabitat interface {
	Reserve(EffectKind, uint8) int
	Release(int)
	Random() uint16
	FireExperience(uint8) uint8
	Parcel(x, y int) FireParcel
	Scorch(x, y int)
	Damage(x, y int) int
	LowerCorner(x, y int)
	RaiseCorner(x, y int)
	Paint(x, y int, tile uint8)
	CreateBasalt(owner uint8, x, y, direction int)
	PushByLava(x, y, dx, dy int)
}

type FireParcel struct {
	Altitude  uint8
	Shape     uint8
	Tile      uint8
	Water     bool
	ExtraRise bool
}

type FirePhase uint8

const (
	FireEmerging FirePhase = iota
	FireMoving
	FireEnding
	MeteorWaiting
	MeteorFalling
	MeteorImpact
	VolcanoGrowing
	VolcanoFinished
	LavaFlowing
)

// FireEffect stores named simulation fields. Coordinates and velocities use
// 1/256-tile units so movement and boundary tests retain sub-cell precision.
type FireEffect struct {
	Active           bool
	Owner            uint8
	X, Y, VX, VY     int
	Phase            FirePhase
	Frame            int
	Timer, Life      int
	Stage, Direction int
	WaterImpact      bool
}

// FireEffects is indexed by the common effect reservation pool. Each family
// shares that pool's 250-slot budget rather than receiving another budget.
type FireEffects struct {
	Columns [EffectCapacity]FireEffect
	Rain    [EffectCapacity]FireEffect
	Volcano [EffectCapacity]FireEffect
	Lava    [EffectCapacity]FireEffect
}

var fireJitter = [9][2]int{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {0, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}}
var fireNeighbors = [16][2]int{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}, {-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {1, 0}, {-1, 1}, {0, 1}, {1, 1}}

// offsetFireParcel retains carry between the two coordinate components. This
// matters at map edges: an invalid origin must not become a wrapped map cell.
func offsetFireParcel(x, y, dx, dy int) (int, int, bool) {
	packed := uint16(y<<8|x) + uint16(int16(dy*256+dx))
	return int(uint8(packed)), int(uint8(packed >> 8)), packed&0xc0c0 == 0
}

func (s *FireEffects) CreateColumn(owner uint8, x, y int, h FireHabitat) bool {
	return s.createColumn(owner, x, y, true, h)
}

func (s *FireEffects) createColumn(owner uint8, x, y int, experience bool, h FireHabitat) bool {
	jitter := fireJitter[(h.Random()%18)/2]
	x, y, valid := offsetFireParcel(x, y, jitter[0], jitter[1])
	if !valid {
		return false
	}
	id := h.Reserve(EffectFireColumn, owner)
	if id < 0 {
		return false
	}
	h.Random()
	old := s.Columns[id]
	life := 200
	if experience && owner < 2 {
		life += int(h.FireExperience(owner))
	}
	defer syncFireActor(h, id)
	s.Columns[id] = FireEffect{Active: true, Owner: owner, X: x*256 + 128, Y: y*256 + 128, VX: old.VX, VY: old.VY, Phase: FireEmerging, Timer: 1, Life: life}
	return true
}

// TickColumn advances the emergence and ending phases without spending life.
// Transition into movement falls through in the same pass, as does expiry
// into the ending sequence. Water is inspected before moving to the next cell.
func (s *FireEffects) TickColumn(id int, h FireHabitat) {
	defer syncFireActor(h, id)
	e := &s.Columns[id]
	if !e.Active {
		return
	}
	end := func() {
		if e.Frame+1 >= 10 {
			e.Active = false
			h.Release(id)
		} else {
			e.Frame++
		}
	}
	if e.Phase == FireEmerging {
		e.Frame++
		if e.Frame < 9 {
			return
		}
		e.Phase, e.Frame = FireMoving, 0
	}
	if e.Phase == FireEnding {
		end()
		return
	}
	previousLife := e.Life
	e.Life = int(int16(uint16(e.Life) - 1))
	if previousLife <= 1 {
		e.Phase, e.Frame = FireEnding, 0
		end()
		return
	}
	e.Frame = (e.Frame + 1) % 3
	x, y := e.X>>8, e.Y>>8
	if h.Parcel(x, y).Water {
		e.Phase, e.Frame = FireEnding, 0
		end()
		return
	}
	previousTimer := e.Timer
	e.Timer = int(int16(uint16(e.Timer) - 1))
	if previousTimer <= 1 {
		s.routeColumn(e, h)
	}
	nextX, nextY := int(int16(uint16(e.X+e.VX))), int(int16(uint16(e.Y+e.VY)))
	if nextX < 0 || nextY < 0 || nextX >= MapSize*256 || nextY >= MapSize*256 {
		e.Active = false
		h.Release(id)
		return
	}
	e.X, e.Y = nextX, nextY
	h.Scorch(e.X>>8, e.Y>>8)
	h.Damage(e.X>>8, e.Y>>8)
}

func (s *FireEffects) routeColumn(e *FireEffect, h FireHabitat) {
	x, y := e.X>>8, e.Y>>8
	height := h.Parcel(x, y).Altitude
	bits := h.Random()
	start, selected := int(bits&14)/2, 0
	for i := 0; i < 8; i++ {
		d := fireNeighbors[start+i]
		nx, ny, valid := offsetFireParcel(x, y, d[0], d[1])
		if !valid {
			continue
		}
		candidate := h.Parcel(nx, ny).Altitude
		if candidate < height {
			continue
		}
		if candidate == height {
			accept := bits&1 != 0
			bits >>= 1
			if !accept {
				continue
			}
		}
		height, selected = candidate, start+i
	}
	if selected == 0 {
		selected = int(bits&0x3c) / 4
	}
	d := fireNeighbors[selected]
	e.VX, e.VY, e.Timer = d[0]*16, d[1]*16, 30
}

// CreateRain keeps the original positive 0..7 scatter, delay and partial-pool
// admission rules. A clipped final attempt can admit a cast with no meteors;
// exhausting the shared pool rejects even after earlier successful allocations.
func (s *FireEffects) CreateRain(owner uint8, x, y int, h FireHabitat) bool {
	base := uint16(32)
	if owner < 2 {
		base += uint16(h.FireExperience(owner) >> 5)
	}
	count := h.Random()%base + base/4
	admitted := false
	for attempt := 0; attempt <= int(count); attempt++ {
		id := h.Reserve(EffectFireRain, owner)
		if id < 0 {
			return false
		}
		bits := h.Random() & 0x0707
		nx, ny, valid := offsetFireParcel(x, y, int(bits&255), int(bits>>8))
		if !valid {
			h.Release(id)
			admitted = true
			continue
		}
		s.Rain[id] = FireEffect{Active: true, Owner: owner, X: nx*256 + 128, Y: ny*256 + 128, Phase: MeteorWaiting, Timer: int(h.Random() % 9), Life: 24}
		admitted = true
	}
	return admitted
}

func (s *FireEffects) TickRain(id int, h FireHabitat) {
	defer syncFireActor(h, id)
	e := &s.Rain[id]
	if !e.Active {
		return
	}
	if e.Phase == MeteorWaiting {
		previous := e.Timer
		e.Timer--
		if previous > 1 {
			return
		}
		e.Phase = MeteorFalling
	}
	remove := func() { e.Active = false; h.Release(id) }
	if e.Phase == MeteorImpact {
		if e.Frame+1 >= 4 {
			remove()
		} else {
			e.Frame++
		}
		return
	}
	e.Frame++
	impact := e.Frame >= 18
	parcel := h.Parcel(e.X>>8, e.Y>>8)
	if !impact {
		e.Life--
		height := int(parcel.Altitude)
		if parcel.ExtraRise {
			height++
		}
		impact = height*3 >= e.Life
	}
	if !impact {
		return
	}
	h.Scorch(e.X>>8, e.Y>>8)
	if h.Damage(e.X>>8, e.Y>>8) != 0 {
		remove()
		return
	}
	e.Phase, e.Frame, e.WaterImpact = MeteorImpact, 1, parcel.Water
}

var volcanoStageSizes = [9]int{3, 5, 7, 9, 11, 13, 15, 15, 15}
var volcanoCenters = [9][2]int{{1, 0}, {1, 0}, {1, 0}, {1, 0}, {1, 0}, {1, 0}, {1, 0}, {-1, -1}, {1, 0}}

func (s *FireEffects) CreateVolcano(owner uint8, x, y int, h FireHabitat) bool {
	id := h.Reserve(EffectVolcano, owner)
	if id < 0 {
		return false
	}
	stage := 3
	if owner < 2 {
		stage = 3 - int(h.FireExperience(owner)>>6)
	}
	s.Volcano[id] = FireEffect{Active: true, Owner: owner, X: x * 256, Y: y * 256, Phase: VolcanoGrowing, Stage: stage}
	return true
}

// TickVolcano visits crater cells in descending order before applying the
// cumulative center changes and painting slopes. The last stage emits fire
// and lava; the controller itself stays outside the parcel occupancy graph.
func (s *FireEffects) TickVolcano(id int, h FireHabitat) {
	defer syncFireActor(h, id)
	e := &s.Volcano[id]
	if !e.Active {
		return
	}
	if e.Phase == VolcanoFinished {
		e.Active = false
		h.Release(id)
		return
	}
	x, y, size := e.X>>8, e.Y>>8, volcanoStageSizes[e.Stage]
	left, top := max(0, x-size/2-1), max(0, y-size/2-1)
	visit := func(f func(int, int, FireParcel)) {
		for dy := size; dy >= 0; dy-- {
			for dx := size; dx >= 0; dx-- {
				nx, ny := left+dx, top+dy
				if inside(nx, ny) {
					f(nx, ny, h.Parcel(nx, ny))
				}
			}
		}
	}
	visit(func(nx, ny int, parcel FireParcel) {
		if parcel.Altitude == 0 && parcel.Tile == 0 {
			return
		}
		linear := nx + ny*MapSize
		for n := 0; n <= int(parcel.Altitude); n++ {
			// Crater lowering passes the full low byte of its linear index.
			// Invalid coordinates 64..255 must not become wrapped corners.
			h.LowerCorner(int(uint8(linear)), linear>>6)
		}
	})
	for _, changes := range volcanoCenters[:e.Stage+1] {
		for _, change := range changes {
			if change > 0 {
				h.RaiseCorner(x, y)
			} else if change < 0 {
				h.LowerCorner(x, y)
			}
		}
	}
	visit(func(nx, ny int, parcel FireParcel) {
		if parcel.Shape != 0 && parcel.Shape != 15 {
			h.Paint(nx, ny, 224+parcel.Shape)
		}
	})
	if e.Stage < 8 {
		e.Stage++
		return
	}
	e.Phase = VolcanoFinished
	for range 4 {
		s.createColumn(e.Owner, x, y, false, h)
	}
	for _, overlay := range []struct {
		dx, dy      int
		shape, tile uint8
	}{{1, -1, 9, 240}, {1, 0, 9, 241}, {-1, 1, 3, 242}, {0, 1, 3, 243}, {-1, -1, 15, 220}, {0, -1, 15, 220}, {-1, 0, 15, 220}, {0, 0, 15, 220}} {
		nx, ny, valid := offsetFireParcel(x, y, overlay.dx, overlay.dy)
		if valid && h.Parcel(nx, ny).Shape == overlay.shape {
			h.Paint(nx, ny, overlay.tile)
		}
	}
	sources := [8][3]int{{-1, -2, 0}, {0, -2, 0}, {1, -1, 1}, {1, 0, 1}, {-1, 1, 2}, {0, 1, 2}, {-2, -1, 3}, {-2, 0, 3}}
	attempts := int(h.Random()%5) + 1
	for range attempts {
		source := sources[(h.Random()%32)/4]
		nx, ny, valid := offsetFireParcel(x, y, source[0], source[1])
		if valid {
			s.CreateLava(e.Owner, nx, ny, source[2], h)
		}
	}
}

func lavaSupports(shape uint8) bool {
	return shape == 3 || shape == 6 || shape == 9 || shape == 12 || shape == 15
}

var lavaDirections = [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}

// CreateLava returns one for creation or a basalt request, minus one for an
// existing flow or unsupported slope, and zero for bounds or pool exhaustion.
func (s *FireEffects) CreateLava(owner uint8, x, y, direction int, h FireHabitat) int {
	if !inside(x, y) || direction < 0 || direction > 3 {
		return 0
	}
	if s.lavaAt(x, y) {
		return -1
	}
	parcel := h.Parcel(x, y)
	if parcel.Shape == 0 {
		h.CreateBasalt(owner, x, y, direction)
		return 1
	}
	id := h.Reserve(EffectLava, owner)
	if id < 0 {
		return 0
	}
	if !lavaSupports(parcel.Shape) {
		h.Release(id)
		return -1
	}
	delay := int(h.Random()%9) + 1
	defer syncFireActor(h, id)
	s.Lava[id] = FireEffect{Active: true, Owner: owner, X: x * 256, Y: y * 256, Phase: LavaFlowing, Direction: direction, Timer: delay, Life: delay}
	return 1
}

func (s *FireEffects) lavaAt(x, y int) bool {
	for _, effect := range s.Lava {
		if effect.Active && effect.X>>8 == x && effect.Y>>8 == y {
			return true
		}
	}
	return false
}

func (s *FireEffects) TickLava(id int, clock uint64, h FireHabitat) {
	defer syncFireActor(h, id)
	e := &s.Lava[id]
	if !e.Active {
		return
	}
	x, y := e.X>>8, e.Y>>8
	d := lavaDirections[e.Direction]
	finish := func() { e.Active = false; h.Release(id) }
	e.Timer = int(int16(uint16(e.Timer) - 1))
	if e.Timer == 0 {
		nx, ny, valid := offsetFireParcel(x, y, d[0], d[1])
		result := 0
		if valid {
			result = s.CreateLava(e.Owner, nx, ny, e.Direction, h)
		}
		if result != 0 {
			e.Timer = 18
		}
	}
	parcel := h.Parcel(x, y)
	if !lavaSupports(parcel.Shape) {
		finish()
		return
	}
	e.Frame = int((clock + uint64(x+y*MapSize)) & 1)
	previousLife := e.Life
	e.Life = int(int16(uint16(e.Life) - 1))
	if previousLife <= 1 {
		h.Scorch(x, y)
		e.Life = 3
		bx, by, valid := offsetFireParcel(x, y, -d[0], -d[1])
		if !valid || h.Parcel(bx, by).Tile != 220 && !s.lavaAt(bx, by) {
			finish()
			return
		}
	}
	h.PushByLava(x, y, d[0]*20, d[1]*20)
}

func syncFireActor(h FireHabitat, id int) {
	if lifecycle, ok := h.(interface{ SyncEffect(int) }); ok {
		lifecycle.SyncEffect(id)
	}
}
