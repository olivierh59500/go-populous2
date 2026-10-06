package engine

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestSnapshotContinuesMotionAIAndSharedEffectsIdentically(t *testing.T) {
	w := testFlatWorld()
	w.random = 4311
	w.Level.Players[0].Powers[FireColumn] = true
	w.Level.Players[0].Powers[Fungus] = true
	w.Players[0].Mana, w.Players[1].Mana = 1000000, 1000000
	w.Players[0].RallyX, w.Players[0].RallyY = 10, 10
	w.Players[1].RallyX, w.Players[1].RallyY = 50, 50
	w.Players[1].Computer = true
	a := addFollower(w, 32, 32, 0, 1000, Walking)
	b := addFollower(w, 50, 50, 1, 900, Town)
	w.Followers[b].Stage = 18
	w.Players[0].Leader, w.Players[1].Leader = a, b
	w.beginLeg(a, 33, 32)
	w.advanceLeg(a)
	if err := w.CastFireColumn(0, 20, 20); err != nil {
		t.Fatal(err)
	}
	if err := w.CastFungus(0, 40, 40); err != nil {
		t.Fatal(err)
	}
	// A separate water parcel gives basalt a live propagation controller.
	for _, p := range [4][2]int{{5, 5}, {6, 5}, {5, 6}, {6, 6}} {
		w.Heights[p[0]+p[1]*CornerSize] = 0
	}
	w.rebuildCells()
	if !w.CreateBasalt(0, 5, 5, 1, 100) {
		t.Fatal("basalt fixture was not admitted")
	}
	w.tickFireEffect(0)
	var output bytes.Buffer
	if err := WriteSnapshot(&output, w); err != nil {
		t.Fatal(err)
	}
	restored, err := ReadSnapshot(bytes.NewReader(output.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(w, restored) {
		t.Fatal("snapshot lost private motion, AI or effect state")
	}
	for tick := 0; tick < 60; tick++ {
		w.Step()
		restored.Step()
		if !reflect.DeepEqual(w, restored) {
			t.Fatalf("snapshot replay diverged at continuation tick%d", tick+1)
		}
	}
}

func TestSnapshotRejectsInvalidVersionsAndBrokenLinksWithoutMutatingLiveWorld(t *testing.T) {
	w := testFlatWorld()
	w.Players[0].RallyX, w.Players[0].RallyY = 32, 32
	w.Players[1].RallyX, w.Players[1].RallyY = 32, 32
	id := addFollower(w, 32, 32, 0, 100, Walking)
	before := *w
	for name, corrupt := range map[string]func(*Snapshot){
		"version":           func(s *Snapshot) { s.Version++ },
		"height":            func(s *Snapshot) { s.World.Heights[0] = 9 },
		"cycle":             func(s *Snapshot) { s.World.Followers[id].NextFollower = id },
		"wrong-cell":        func(s *Snapshot) { s.World.Followers[id].X = 12 },
		"invalid-motion":    func(s *Snapshot) { s.Motion[id].PositionX = -1 },
		"absent-controller": func(s *Snapshot) { s.Reservations[0] = EffectReservation{Kind: EffectFireColumn, Owner: 0} },
		"unreserved-controller": func(s *Snapshot) {
			s.World.Fire.Columns[0] = FireEffect{Active: true, Owner: 0, X: 32 * 256, Y: 32 * 256}
		},
		"invalid-carry": func(s *Snapshot) {
			s.World.Air.Carry[id] = AirCarryState{Phase: AirCarryFlying, Effect: 999, Frames: 12}
		},
	} {
		t.Run(name, func(t *testing.T) {
			s := w.Snapshot()
			corrupt(&s)
			data, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			if candidate, err := ReadSnapshot(bytes.NewReader(data)); err == nil || candidate != nil {
				t.Fatal("corrupt save returned a playable world")
			}
			if !reflect.DeepEqual(*w, before) {
				t.Fatal("failed load changed the live world")
			}
		})
	}
}

func TestSnapshotContinuesAirborneAndRetainedDeathState(t *testing.T) {
	w := testFlatWorld()
	w.Players[0].RallyX, w.Players[0].RallyY = 32, 32
	w.Players[1].RallyX, w.Players[1].RallyY = 40, 40
	carried := addFollower(w, 32, 32, 0, 555, Walking)
	dying := addFollower(w, 40, 40, 1, 100, Walking)
	w.BurnFireCell(40, 40)
	w.stepFollower(dying)
	id := w.allocateEffect(EffectWhirlwind, 0)
	w.Air.Whirlwinds[id] = WhirlwindEffect{Active: true, Owner: 0, X: 32*256 + 128, Y: 32*256 + 128, Phase: WhirlwindMoving, Life: 100, Timer: 20, VX: 24, VY: 0}
	w.liftAirFollowers(id, 32, 32)
	w.stepFollower(carried)
	var output bytes.Buffer
	if err := WriteSnapshot(&output, w); err != nil {
		t.Fatal(err)
	}
	restored, err := ReadSnapshot(bytes.NewReader(output.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	for pass := 0; pass < 12; pass++ {
		w.Step()
		restored.Step()
		if !reflect.DeepEqual(w, restored) {
			t.Fatalf("retained lifecycle diverged at pass%d", pass+1)
		}
	}
}

func TestSnapshotMixedActorRegistryRejectsMalformedRelations(t *testing.T) {
	var valid ActorRegistry
	for _, ref := range []ActorRef{{Kind: ActorMagnet, Index: 0}, {Kind: ActorFollower, Index: 1}, {Kind: ActorEffect, Index: 0}, {Kind: ActorScenery, Index: 0}, {Kind: ActorWall, Index: 0}} {
		if !valid.Link(ref, 32*256+128, 32*256+128) {
			t.Fatal("mixed registry setup failed")
		}
	}
	if err := validateActorRegistry(&valid); err != nil {
		t.Fatal(err)
	}
	for name, corrupt := range map[string]func(*ActorRegistry){
		"cycle":            func(r *ActorRegistry) { r.Walls[0].Next = ActorRef{Kind: ActorWall, Index: 0} },
		"wrong-pool":       func(r *ActorRegistry) { r.Walls[0].Next = ActorRef{Kind: ActorFollower, Index: 999} },
		"detached-head":    func(r *ActorRegistry) { r.Walls[0].Linked = false },
		"wrong-cell":       func(r *ActorRegistry) { r.Walls[0].X = 12 * 256 },
		"broken-back-link": func(r *ActorRegistry) { r.Scenery[0].Previous = ActorRef{} },
	} {
		t.Run(name, func(t *testing.T) {
			r := valid
			corrupt(&r)
			if err := validateActorRegistry(&r); err == nil {
				t.Fatal("malformed mixed registry was accepted")
			}
		})
	}
}

func TestSnapshotPreservesMixedActorRegistryAndMagnetFractions(t *testing.T) {
	w := testFlatWorld()
	w.Players[0].RallyX, w.Players[0].RallyY = 32, 32
	w.Players[1].RallyX, w.Players[1].RallyY = 32, 32
	id := addFollower(w, 32, 32, 0, 100, Walking)
	w.Magnets[0] = MagnetActor{Owner: 0, X: 32*256 + 211, Y: 32*256 + 17}
	w.Actors.Link(ActorRef{Kind: ActorMagnet, Index: 0}, w.Magnets[0].X, w.Magnets[0].Y)
	w.Actors.Link(ActorRef{Kind: ActorFollower, Index: uint16(id)}, w.Followers[id].positionX, w.Followers[id].positionY)
	var output bytes.Buffer
	if err := WriteSnapshot(&output, w); err != nil {
		t.Fatal(err)
	}
	restored, err := ReadSnapshot(bytes.NewReader(output.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(w.Actors, restored.Actors) || w.Magnets != restored.Magnets {
		t.Fatal("save lost mixed actor order or fractional magnet coordinates")
	}
}

func TestSnapshotContinuesNeutralScenarioAndWindState(t *testing.T) {
	w := testFlatWorld()
	w.random = 4311
	w.Players[0].RallyX, w.Players[0].RallyY = 10, 10
	w.Players[1].RallyX, w.Players[1].RallyY = 50, 50
	addFollower(w, 10, 10, 0, 500, Town)
	addFollower(w, 50, 50, 1, 500, Town)
	for kind := NeutralRoadMaker; kind <= NeutralMonster; kind++ {
		if _, err := w.CreateNeutral(kind, 20+int(kind)*3, 25); err != nil {
			t.Fatal(err)
		}
	}
	w.Scenario = ScenarioState{Events: [10]ScenarioEvent{{Time: 3, Kind: ScenarioFireColumn, X: 30, Y: 30}, {Time: 8, Kind: ScenarioRoadMaker, X: 15, Y: 15}}}
	if err := w.CastWind(0, 30, 30, 1); err != nil {
		t.Fatal(err)
	}
	w.Step()
	w.Step()
	var output bytes.Buffer
	if err := WriteSnapshot(&output, w); err != nil {
		t.Fatal(err)
	}
	restored, err := ReadSnapshot(bytes.NewReader(output.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	for pass := 0; pass < 60; pass++ {
		w.Step()
		restored.Step()
		if !reflect.DeepEqual(w, restored) {
			t.Fatalf("neutral/scenario/wind continuation diverged at pass%d", pass+1)
		}
	}
}

func TestSnapshotAcceptsRetainedTerrainDeathAndRejectsInvalidNeutralOwner(t *testing.T) {
	w := testFlatWorld()
	w.Players[0].RallyX, w.Players[0].RallyY = 32, 32
	w.Players[1].RallyX, w.Players[1].RallyY = 32, 32
	id := addFollower(w, 32, 32, 0, 0, Ruin)
	w.Followers[id].TerrainDeath = TerrainDeathState{Active: true, Frame: 1, Frames: 2}
	if _, err := w.Snapshot().Restore(); err != nil {
		t.Fatal(err)
	}
	s := w.Snapshot()
	s.World.Followers[id].Owner = 2
	if _, err := s.Restore(); err == nil {
		t.Fatal("owner2 ordinary follower was accepted without a neutral kind")
	}
	s = w.Snapshot()
	s.World.Followers[id].TerrainDeath.Frame = 2
	if _, err := s.Restore(); err == nil {
		t.Fatal("already completed terrain death was accepted as active")
	}
}

func TestSnapshotVersionOneRetiredConstructionFieldsRemainReadable(t *testing.T) {
	w := testFlatWorld()
	w.Players[0].RallyX, w.Players[0].RallyY = 32, 32
	w.Players[1].RallyX, w.Players[1].RallyY = 32, 32
	s := w.Snapshot()
	s.Version = 1
	s.Construction = &ConstructionSnapshot{}
	s.Construction.TerrainTargets[100] = 3
	s.Construction.TerrainTargetSet[100] = true
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := ReadSnapshot(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if restored.Snapshot().Version != 2 || restored.Snapshot().Construction != nil || !reflect.DeepEqual(w, restored) {
		t.Fatal("retired planner data altered the current simulation")
	}
}

func TestSnapshotPreservesRetainedTownRuinCountdown(t *testing.T) {
	w := testFlatWorld()
	w.Players[0].RallyX, w.Players[0].RallyY = 32, 32
	w.Players[1].RallyX, w.Players[1].RallyY = 32, 32
	id := addFollower(w, 32, 32, 0, 0, Ruin)
	w.Followers[id].CombatAftermath = CombatAftermathState{Kind: CombatTownRuin, RuinTime: 399}
	var output bytes.Buffer
	if err := WriteSnapshot(&output, w); err != nil {
		t.Fatal(err)
	}
	restored, err := ReadSnapshot(bytes.NewReader(output.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	for range 30 {
		w.Step()
		restored.Step()
		if !reflect.DeepEqual(w, restored) {
			t.Fatal("town ruin countdown changed after loading")
		}
	}
	s := w.Snapshot()
	s.World.Followers[id].CombatAftermath.RuinTime = 401
	if _, err := s.Restore(); err == nil {
		t.Fatal("invalid retained town ruin countdown accepted")
	}
}

func TestSnapshotMixedRegistryRejectsInactiveActorsAndStalePositions(t *testing.T) {
	for name, corrupt := range map[string]func(*World){
		"inactive-scenery": func(w *World) { w.Actors.Link(ActorRef{ActorScenery, 0}, 20*256+128, 20*256+128) },
		"inactive-effect":  func(w *World) { w.Actors.Link(ActorRef{ActorEffect, 0}, 20*256+128, 20*256+128) },
		"inactive-wall":    func(w *World) { w.Actors.Link(ActorRef{ActorWall, 0}, 20*256+128, 20*256+128) },
		"stale-follower-position": func(w *World) {
			id := addFollower(w, 20, 20, 0, 100, Walking)
			w.Actors.Move(ActorRef{ActorFollower, uint16(id)}, 20*256+140, 20*256+128)
		},
		"waiting-meteor-mapped": func(w *World) {
			w.effects.Slots[0] = EffectReservation{Kind: EffectFireRain, Owner: 0}
			w.Fire.Rain[0] = FireEffect{Active: true, Owner: 0, X: 20*256 + 128, Y: 20*256 + 128, Phase: MeteorWaiting, Life: 24}
			w.Actors.Link(ActorRef{ActorEffect, 0}, 20*256+128, 20*256+128)
		},
	} {
		t.Run(name, func(t *testing.T) {
			w := testFlatWorld()
			w.Players[0].RallyX, w.Players[0].RallyY = 32, 32
			w.Players[1].RallyX, w.Players[1].RallyY = 32, 32
			corrupt(w)
			if _, err := w.Snapshot().Restore(); err == nil {
				t.Fatal("live-pool registry mismatch was accepted")
			}
		})
	}
}

func TestSnapshotAcceptsValidUnmappedControllerAndKeepsChainOrder(t *testing.T) {
	w := testFlatWorld()
	w.Players[0].RallyX, w.Players[0].RallyY = 32, 32
	w.Players[1].RallyX, w.Players[1].RallyY = 32, 32
	w.effects.Slots[0] = EffectReservation{Kind: EffectVolcano, Owner: 0}
	w.Fire.Volcano[0] = FireEffect{Active: true, Owner: 0, X: 20 * 256, Y: 20 * 256, Phase: VolcanoGrowing, Stage: 3}
	addFollower(w, 20, 20, 0, 100, Walking)
	w.Nature.Scenery[0] = SceneryActor{Kind: SceneryTree, X: 20, Y: 20}
	w.Actors.Link(ActorRef{ActorScenery, 0}, 20*256+129, 20*256+143)
	before := w.Actors
	restored, err := w.Snapshot().Restore()
	if err != nil {
		t.Fatal(err)
	}
	if restored.Actors != before {
		t.Fatal("validation rebuilt chronological actor chains")
	}
}

func TestSnapshotRejectsInvalidCrossingTerrainObservation(t *testing.T) {
	w := testFlatWorld()
	w.Players[0].RallyX, w.Players[0].RallyY = 32, 32
	w.Players[1].RallyX, w.Players[1].RallyY = 32, 32
	w.AI[0].TerrainRequestFollower, w.AI[0].TerrainRequestX, w.AI[0].TerrainRequestY = 1, 64, 20
	if _, err := w.Snapshot().Restore(); err == nil {
		t.Fatal("out-of-map pending terrain request was accepted")
	}
	w.AI[0].TerrainRequestFollower = 0
	if _, err := w.Snapshot().Restore(); err != nil {
		t.Fatal("unused stale terrain coordinates rejected a cleared request")
	}
	w.AI[0].WaterRequestFollower = FollowerCapacity
	if _, err := w.Snapshot().Restore(); err == nil {
		t.Fatal("out-of-range water request follower was accepted")
	}
}

func TestSnapshotStrictJSONAndWriterErrors(t *testing.T) {
	w := testFlatWorld()
	w.Players[0].RallyX, w.Players[0].RallyY = 32, 32
	w.Players[1].RallyX, w.Players[1].RallyY = 32, 32
	var output bytes.Buffer
	if err := WriteSnapshot(&output, w); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSnapshot(strings.NewReader(output.String() + "{}")); err == nil {
		t.Fatal("trailing JSON was accepted")
	}
	if _, err := ReadSnapshot(strings.NewReader(`{"version":1,"machineAddress":123}`)); err == nil {
		t.Fatal("unknown save field was accepted")
	}
	if _, err := ReadSnapshot(strings.NewReader(`{"version":`)); err == nil {
		t.Fatal("truncated JSON was accepted")
	}
	if _, err := ReadSnapshot(nil); err == nil {
		t.Fatal("nil reader was accepted")
	}
	if err := WriteSnapshot(nil, w); err == nil {
		t.Fatal("nil writer was accepted")
	}
}
