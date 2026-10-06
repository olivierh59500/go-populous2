package engine

import (
	"encoding/json"
	"os"
	"testing"
)

type fireTestHabitat struct {
	pool            effectPool
	random          []uint16
	randomAt        int
	experience      uint8
	parcels         [MapSize * MapSize]FireParcel
	hits            int
	damaged         [][2]int
	scorched        [][2]int
	lowered, raised [][2]int
	basalt          [][4]int
	pushes          [][4]int
	rng             randomState
	useRNG          bool
}

func newFireTestHabitat() *fireTestHabitat {
	h := &fireTestHabitat{}
	for i := range h.parcels {
		h.parcels[i] = FireParcel{Altitude: 1, Shape: 15, Tile: 15}
	}
	return h
}
func (h *fireTestHabitat) Reserve(kind EffectKind, owner uint8) int {
	return h.pool.allocate(kind, owner)
}
func (h *fireTestHabitat) Release(id int) { h.pool.free(id) }
func (h *fireTestHabitat) Random() uint16 {
	if h.useRNG {
		h.randomAt++
		return h.rng.next()
	}
	value := uint16(8)
	if h.randomAt < len(h.random) {
		value = h.random[h.randomAt]
	}
	h.randomAt++
	return value
}
func (h *fireTestHabitat) FireExperience(uint8) uint8 { return h.experience }
func (h *fireTestHabitat) Parcel(x, y int) FireParcel {
	if !inside(x, y) {
		return FireParcel{Water: true}
	}
	return h.parcels[x+y*MapSize]
}
func (h *fireTestHabitat) Scorch(x, y int) { h.scorched = append(h.scorched, [2]int{x, y}) }
func (h *fireTestHabitat) Damage(x, y int) int {
	h.damaged = append(h.damaged, [2]int{x, y})
	return h.hits
}
func (h *fireTestHabitat) LowerCorner(x, y int)       { h.lowered = append(h.lowered, [2]int{x, y}) }
func (h *fireTestHabitat) RaiseCorner(x, y int)       { h.raised = append(h.raised, [2]int{x, y}) }
func (h *fireTestHabitat) Paint(x, y int, tile uint8) { h.parcels[x+y*MapSize].Tile = tile }
func (h *fireTestHabitat) CreateBasalt(owner uint8, x, y, direction int) {
	h.basalt = append(h.basalt, [4]int{int(owner), x, y, direction})
}
func (h *fireTestHabitat) PushByLava(x, y, dx, dy int) {
	h.pushes = append(h.pushes, [4]int{x, y, dx, dy})
}

func TestFireColumnOriginalCreationAndPhaseFallthrough(t *testing.T) {
	h, effects := newFireTestHabitat(), &FireEffects{}
	h.experience = 255
	if !effects.CreateColumn(0, 32, 32, h) || h.randomAt != 2 {
		t.Fatal("column creation did not consume its two random draws")
	}
	e := &effects.Columns[0]
	if e.X != 32*256+128 || e.Y != 32*256+128 || e.Life != 455 || e.Phase != FireEmerging {
		t.Fatalf("wrong column creation: %+v", e)
	}
	for range 8 {
		effects.TickColumn(0, h)
	}
	if e.Life != 455 || e.Frame != 8 || len(h.damaged) != 0 {
		t.Fatal("emergence spent active life or damaged the map")
	}
	effects.TickColumn(0, h)
	if e.Phase != FireMoving || e.Frame != 1 || e.Life != 454 || e.Timer != 30 || len(h.damaged) != 1 {
		t.Fatalf("emergence did not execute the first active pass immediately: %+v", e)
	}
	e.Life = 1
	effects.TickColumn(0, h)
	if e.Phase != FireEnding || e.Frame != 1 || len(h.damaged) != 1 {
		t.Fatal("expiry did not enter and advance the ending animation immediately")
	}
	for range 9 {
		effects.TickColumn(0, h)
	}
	if e.Active || h.pool.Slots[0].Kind != EffectNone {
		t.Fatal("completed ending animation kept the shared slot reserved")
	}
}

func TestFireColumnPoolFailureConsumesOnlyJitterAndWaterStopsBeforeMovement(t *testing.T) {
	h, effects := newFireTestHabitat(), &FireEffects{}
	for range EffectCapacity {
		h.Reserve(EffectStorm, 0)
	}
	if effects.CreateColumn(0, 32, 32, h) || h.randomAt != 1 {
		t.Fatal("full-pool rejection changed column admission or random draws")
	}
	h = newFireTestHabitat()
	h.Reserve(EffectFireColumn, 0)
	h.parcels[32+32*MapSize].Water = true
	effects.Columns[0] = FireEffect{Active: true, Owner: 0, X: 32*256 + 128, Y: 32*256 + 128, VX: 16, Phase: FireMoving, Timer: 9, Life: 200}
	effects.TickColumn(0, h)
	e := effects.Columns[0]
	if e.X != 32*256+128 || e.Timer != 9 || e.Phase != FireEnding || e.Frame != 1 || h.randomAt != 0 || len(h.damaged) != 0 {
		t.Fatal("water termination moved, rerouted or damaged before ending")
	}
}

func TestFireColumnRoutingUsesAltitudeAndRetainsNewCellDamage(t *testing.T) {
	h, effects := newFireTestHabitat(), &FireEffects{}
	h.random = []uint16{0}
	h.parcels[33+33*MapSize].Altitude = 7
	e := FireEffect{Active: true, X: 32*256 + 250, Y: 32*256 + 250, Phase: FireMoving, Timer: 1, Life: 200}
	effects.Columns[0] = e
	effects.TickColumn(0, h)
	e = effects.Columns[0]
	if e.VX != 16 || e.VY != 16 || e.Timer != 30 || h.randomAt != 1 {
		t.Fatalf("column did not route toward the highest admitted neighboring cell: %+v", e)
	}
	if len(h.damaged) != 1 || h.damaged[0] != [2]int{33, 33} {
		t.Fatal("column damaged the old cell instead of the cell after movement")
	}
}

func TestFireRainDelayImpactAndPartialPoolAdmission(t *testing.T) {
	h, effects := newFireTestHabitat(), &FireEffects{}
	h.random = []uint16{0}
	if !effects.CreateRain(0, 32, 32, h) {
		t.Fatal("rain rejected a valid shared-pool creation")
	}
	active := 0
	for _, e := range effects.Rain {
		if e.Active {
			active++
		}
	}
	if active != 9 {
		t.Fatalf("original inclusive count created %d meteors, want 9", active)
	}
	e := &effects.Rain[0]
	e.Timer = 2
	effects.TickRain(0, h)
	if e.Phase != MeteorWaiting || e.Frame != 0 {
		t.Fatal("meteor advanced before its delay expired")
	}
	effects.TickRain(0, h)
	if e.Phase != MeteorFalling || e.Frame != 1 || e.Life != 23 {
		t.Fatal("delay expiry did not advance the falling animation in the same pass")
	}
	e.Life = 3
	effects.TickRain(0, h)
	if e.Phase != MeteorImpact || e.Frame != 1 || len(h.scorched) != 1 || len(h.damaged) != 1 {
		t.Fatal("empty-cell impact did not enter and advance its land animation")
	}
	for range 3 {
		effects.TickRain(0, h)
	}
	if e.Active {
		t.Fatal("finished meteor impact remained active")
	}
	h, effects = newFireTestHabitat(), &FireEffects{}
	for range EffectCapacity - 1 {
		h.Reserve(EffectLightning, 1)
	}
	if effects.CreateRain(0, 32, 32, h) || !effects.Rain[EffectCapacity-1].Active {
		t.Fatal("partial creation did not retain its meteor while rejecting the cast")
	}
}

func TestVolcanoStageOrderAndSharedEruptionChildren(t *testing.T) {
	h, effects := newFireTestHabitat(), &FireEffects{}
	h.experience = 255
	if !effects.CreateVolcano(0, 32, 32, h) || h.randomAt != 0 || effects.Volcano[0].Stage != 0 {
		t.Fatal("volcano creation changed stage selection or consumed randomness")
	}
	effects.TickVolcano(0, h)
	if effects.Volcano[0].Stage != 1 || len(h.raised) != 1 || len(h.lowered) == 0 || h.lowered[0] != [2]int{97, 33} {
		t.Fatalf("wrong descending crater visits or cumulative center changes: first lower %v", h.lowered)
	}
	for range 8 {
		effects.TickVolcano(0, h)
	}
	if effects.Volcano[0].Phase != VolcanoFinished {
		t.Fatal("final volcano stage did not erupt")
	}
	columns, lava := 0, 0
	for id := range effects.Columns {
		if effects.Columns[id].Active {
			columns++
			if effects.Columns[id].Life != 200 {
				t.Fatal("eruption fire column received the casting experience bonus")
			}
		}
		if effects.Lava[id].Active {
			lava++
		}
	}
	if columns != 4 || lava == 0 {
		t.Fatalf("eruption did not allocate its shared-pool children: %d fire, %d lava", columns, lava)
	}
	effects.TickVolcano(0, h)
	if effects.Volcano[0].Active || h.pool.Slots[0].Kind != EffectNone {
		t.Fatal("completed volcano controller did not release its slot")
	}
}

func TestLavaPropagationPrecedesLifeUpdateAndWaterCreatesBasalt(t *testing.T) {
	h, effects := newFireTestHabitat(), &FireEffects{}
	h.random = []uint16{0, 0}
	if effects.CreateLava(0, 32, 32, 1, h) != 1 {
		t.Fatal("lava creation rejected flat ground")
	}
	h.parcels[31+32*MapSize].Tile = 220
	effects.TickLava(0, 5, h)
	if !effects.Lava[1].Active || effects.Lava[1].X>>8 != 33 || effects.Lava[0].Timer != 18 || effects.Lava[0].Life != 3 {
		t.Fatal("lava propagation or trailing crater lifetime was lost")
	}
	if len(h.pushes) != 1 || h.pushes[0] != [4]int{32, 32, 20, 0} {
		t.Fatal("lava failed to request its directional fractional push")
	}
	h.parcels[35+35*MapSize].Shape = 0
	if effects.CreateLava(0, 35, 35, 2, h) != 1 || len(h.basalt) != 1 {
		t.Fatal("water lava did not request basalt without reserving a lava actor")
	}
	if effects.CreateLava(0, 32, 32, 1, h) != -1 {
		t.Fatal("duplicate lava actor was admitted")
	}
}

func TestFireColumnNamedStateMatchesOriginalNumericTraces(t *testing.T) {
	type actor struct {
		Kind, Owner, Phase, Speed             uint8
		FixedX, FixedY, Animation             uint16
		VelocityX, VelocityY, MoveTimer, Life int16
	}
	type snapshot struct {
		Tick  int
		Actor actor
		RNG   uint32
	}
	type fixture struct {
		Name       string
		Seed       uint32
		Experience uint8
		Target     [2]int
		Accepted   bool
		Initial    actor
		InitialRNG uint32
		Trace      []snapshot
	}
	data, err := os.ReadFile("../populous2/testdata/fire_column_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Fixtures []fixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Fixtures) != 12 {
		t.Fatal("the independent numeric trace catalog is incomplete")
	}
	for _, f := range catalog.Fixtures {
		t.Run(f.Name, func(t *testing.T) {
			h, effects := newFireTestHabitat(), &FireEffects{}
			h.experience, h.rng, h.useRNG = f.Experience, randomState(f.Seed), true
			if f.Name == "water" {
				for i := range h.parcels {
					h.parcels[i] = FireParcel{Water: true}
				}
			}
			if f.Name == "uphill" {
				for y := 0; y < MapSize; y++ {
					for x := 0; x < MapSize; x++ {
						height := 1 + max(0, min(7, x-28))
						if x >= 35 {
							height--
						}
						h.parcels[x+y*MapSize].Altitude = uint8(height)
					}
				}
			} else if f.Name != "water" {
				for i := range h.parcels {
					h.parcels[i].Altitude = 0
				}
			}
			if got := effects.CreateColumn(0, f.Target[0], f.Target[1], h); got != f.Accepted {
				t.Fatalf("cast admission %v, original %v", got, f.Accepted)
			}
			assert := func(tick int, want actor, rng uint32) {
				t.Helper()
				e := effects.Columns[0]
				phase, animation := uint8(2), 416+e.Frame*4
				if e.Phase == FireMoving {
					phase, animation = 4, 1208+e.Frame*4
				}
				if e.Phase == FireEnding {
					phase, animation = 6, 1632+e.Frame*4
				}
				got := actor{Kind: 34, Phase: phase, Speed: 16, FixedX: uint16(e.X), FixedY: uint16(e.Y), Animation: uint16(animation), VelocityX: int16(e.VX), VelocityY: int16(e.VY), MoveTimer: int16(e.Timer), Life: int16(e.Life)}
				if e.Active {
					got.Owner = 1
				}
				if got != want || uint32(h.rng) != rng {
					t.Fatalf("tick %d: state %+v / original %+v; RNG %x / %x", tick, got, want, uint32(h.rng), rng)
				}
			}
			assert(0, f.Initial, f.InitialRNG)
			for _, sample := range f.Trace {
				effects.TickColumn(0, h)
				assert(sample.Tick, sample.Actor, sample.RNG)
			}
		})
	}
}
