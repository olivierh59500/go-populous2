package gamcodec

import (
	"encoding/binary"
	"reflect"
	"testing"

	"go-populous2/internal/engine"
)

func continuationCatalog() Catalog {
	land := engine.Landscape{}
	for stage := 1; stage < engine.TownStages; stage++ {
		land.WorkTicks[stage] = 8
		land.EmigrationDivisor[stage] = 3
		land.PopulationLimit[stage] = 1000
		land.PopulationAdd[stage] = 1
		land.ManaAdd[stage] = 2
	}
	level := engine.Level{Seed: 4311}
	for owner := range level.Players {
		level.Players[owner] = engine.PlayerOptions{Mana: 10000, ReactionDelay: 7}
		for _, id := range []engine.PowerID{engine.RaiseLower, engine.FireColumn, engine.Lightning, engine.Whirlwind} {
			level.Players[owner].Powers[id] = true
		}
	}
	c := Catalog{Levels: []engine.Level{level}, AnimationRoles: make(map[uint16][]AnimationRole)}
	for i := range c.Landscapes {
		c.Landscapes[i] = land
	}
	for i := 0; i < 256; i++ {
		c.Geometry[i] = uint8(i % 16)
	}
	for _, entry := range []struct {
		base   uint16
		length int
		name   string
	}{{376, 9, "death/fire"}, {7796, 12, "combat/death"}, {2500, 6, "combat/hero-death"}, {6956, 14, "combat/victory-blue"}, {7376, 14, "combat/victory-red"}, {1848, 2, "lightning/hit"}, {1872, 10, "lightning/recovery"}, {1236, 12, "airborne/follower"}, {1676, 7, "airborne/landing"}, {2076, 18, "meteor/falling"}} {
		for frame := 0; frame < entry.length; frame++ {
			token := entry.base + uint16(frame*4)
			c.AnimationRoles[token] = append(c.AnimationRoles[token], AnimationRole{Name: entry.name, Frame: frame})
		}
	}
	return c
}
func continuationWorld(t *testing.T, catalog Catalog) *engine.World {
	t.Helper()
	w, err := engine.NewWorld(catalog.Levels[0], catalog.Landscapes[0])
	if err != nil {
		t.Fatal(err)
	}
	s := w.Snapshot()
	for i := range s.World.Heights {
		s.World.Heights[i] = 1
	}
	for i := range s.World.Tiles {
		s.World.Tiles[i] = engine.Cell{Corners: [4]uint8{1, 1, 1, 1}, Shape: 15, Code: 15}
	}
	s.World.Followers = [engine.FollowerCapacity]engine.Follower{}
	s.World.Occupants = [4096]uint16{}
	s.World.Actors = engine.ActorRegistry{}
	s.Motion = [engine.FollowerCapacity]engine.FollowerMotionSnapshot{}
	for owner := range s.World.Players {
		s.World.Players[owner].Computer = false
		s.World.Players[owner].Leader = 0
		s.World.Players[owner].RallyX, s.World.Players[owner].RallyY = 32, 32
		s.World.Magnets[owner] = engine.MagnetActor{Owner: uint8(owner), X: 8320, Y: 8320}
		s.World.Actors.Link(engine.ActorRef{Kind: engine.ActorMagnet, Index: uint16(owner)}, 8320, 8320)
	}
	result, err := s.Restore()
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func addCodecFollower(t *testing.T, w *engine.World, id, x, y, owner, population int, state engine.FollowerState) {
	t.Helper()
	s := w.Snapshot()
	s.World.Followers[id] = engine.Follower{Owner: uint8(owner), X: uint8(x), Y: uint8(y), State: state, Population: population, MovementSpeed: 20}
	s.Motion[id] = engine.FollowerMotionSnapshot{PositionX: x*256 + 128, PositionY: y*256 + 128, PositionSet: true}
	at := x + y*64
	head := int(s.World.Occupants[at])
	s.World.Followers[id].NextFollower = head
	if head != 0 {
		s.World.Followers[head].PreviousFollower = id
	}
	s.World.Occupants[at] = uint16(id)
	s.World.Actors.Link(engine.ActorRef{Kind: engine.ActorFollower, Index: uint16(id)}, x*256+128, y*256+128)
	result, err := s.Restore()
	if err != nil {
		t.Fatal(err)
	}
	*w = *result
}
func compareCodecContinuation(t *testing.T, a, b *engine.World, pass int) {
	t.Helper()
	sa, sb := a.Snapshot(), b.Snapshot()
	if sa.Random != sb.Random || a.Tick != b.Tick {
		t.Fatalf("pass%d clock/RNG differs", pass)
	}
	for side := range a.Players {
		if a.Players[side].Mana != b.Players[side].Mana || a.Players[side].Statistics != b.Players[side].Statistics {
			t.Fatalf("pass%d side%d mana/statistics differ", pass, side)
		}
	}
	for id, f := range a.Followers {
		g := b.Followers[id]
		motionA, motionB := sa.Motion[id], sb.Motion[id]
		if f.ContactWaiting && g.ContactWaiting {
			motionA.LegRemaining, motionB.LegRemaining = 0, 0
		}
		if f.Owner != g.Owner || f.State != g.State || f.Population != g.Population || f.X != g.X || f.Y != g.Y || f.CleanupPrepared != g.CleanupPrepared || f.ContactWait != g.ContactWait || f.ContactWaiting != g.ContactWaiting || motionA != motionB {
			t.Fatalf("pass%d follower%d continuation differs: %+v/%+v", pass, id, f, g)
		}
	}
	if !reflect.DeepEqual(a.Occupants, b.Occupants) || !reflect.DeepEqual(a.Fire, b.Fire) || !reflect.DeepEqual(a.Air, b.Air) {
		t.Fatalf("pass%d actor/effect continuation differs", pass)
	}
}

func TestGAMRetainedDeathContinuationDoesNotRepeatCleanup(t *testing.T) {
	catalog := continuationCatalog()
	w := continuationWorld(t, catalog)
	addCodecFollower(t, w, 1, 20, 20, 0, 100, engine.Walking)
	addCodecFollower(t, w, 2, 50, 50, 1, 500, engine.Walking)
	w.Players[0].Leader = 1
	w.BurnFireCell(20, 20)
	document, err := NewDocument(w, catalog, engine.NewDeity("BLUE"), 0, 2, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Decode(data, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if !restored.World.Followers[1].CleanupPrepared {
		t.Fatal("retained original death did not infer completed cleanup")
	}
	for pass := 1; pass <= 12; pass++ {
		w.Step()
		restored.World.Step()
		compareCodecContinuation(t, w, restored.World, pass)
	}
}

func TestGAMActiveCarryContinuesTypedWorld(t *testing.T) {
	catalog := continuationCatalog()
	w := continuationWorld(t, catalog)
	addCodecFollower(t, w, 1, 20, 20, 0, 500, engine.Walking)
	addCodecFollower(t, w, 2, 50, 50, 1, 500, engine.Walking)
	s := w.Snapshot()
	s.Reservations[0] = engine.EffectReservation{Kind: engine.EffectWhirlwind, Owner: 0, Generation: 1}
	s.World.Air.Whirlwinds[0] = engine.WhirlwindEffect{Active: true, Owner: 0, X: 5248, Y: 5248, VX: 24, Phase: engine.WhirlwindMoving, Life: 150, Timer: 20}
	s.World.Air.Carry[1] = engine.AirCarryState{Phase: engine.AirCarryFlying, Effect: 1, Frames: 12}
	s.World.Followers[1].State = engine.Airborne
	s.World.Followers[1].Weapons = 1
	w, err := s.Restore()
	if err != nil {
		t.Fatal(err)
	}
	document, err := NewDocument(w, catalog, engine.NewDeity("BLUE"), 0, 2, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Decode(data, catalog)
	if err != nil {
		t.Fatal(err)
	}
	for pass := 1; pass <= 12; pass++ {
		w.Step()
		restored.World.Step()
		compareCodecContinuation(t, w, restored.World, pass)
	}
}

func TestGAMActiveLightningVictimContinuesDamageAndRecovery(t *testing.T) {
	catalog := continuationCatalog()
	w := continuationWorld(t, catalog)
	addCodecFollower(t, w, 1, 20, 20, 0, 1000, engine.Walking)
	addCodecFollower(t, w, 2, 50, 50, 1, 500, engine.Walking)
	s := w.Snapshot()
	s.Reservations[0] = engine.EffectReservation{Kind: engine.EffectLightning, Owner: 0, Generation: 1}
	s.Reservations[1] = engine.EffectReservation{Kind: engine.EffectLightning, Owner: 0, Generation: 1}
	s.World.Air.MarkerSlots[0] = 1
	s.World.Air.Markers[0] = engine.LightningMarker{Active: true, Owner: 0, X: 5248, Y: 5248, Life: 150, Phase: engine.LightningSteady, FirstBolt: 2}
	s.World.Air.Bolts[1] = engine.LightningBolt{Active: true, Owner: 0, X: 5248, Y: 5248, Marker: 1}
	s.World.AirVictims[1] = engine.LightningVictimState{Phase: engine.LightningVictimWalkingHit, Bolt: 2, Frames: 2}
	w, err := s.Restore()
	if err != nil {
		t.Fatal(err)
	}
	document, err := NewDocument(w, catalog, engine.NewDeity("BLUE"), 0, 2, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Decode(data, catalog)
	if err != nil {
		t.Fatal(err)
	}
	for pass := 1; pass <= 12; pass++ {
		if pass == 5 {
			w.DismissLightning(0)
			restored.World.DismissLightning(0)
		}
		w.Step()
		restored.World.Step()
		compareCodecContinuation(t, w, restored.World, pass)
	}
}

func TestGAMActiveCombatRetainsTownIdentityAndCleanupStatistics(t *testing.T) {
	catalog := continuationCatalog()
	for frame := 0; frame < 4; frame++ {
		catalog.AnimationRoles[456+uint16(frame*4)] = []AnimationRole{{Name: "combat/attack", Frame: frame}}
	}
	w := continuationWorld(t, catalog)
	addCodecFollower(t, w, 1, 20, 20, 0, 1000, engine.Fighting)
	addCodecFollower(t, w, 2, 20, 20, 1, 1000, engine.Fighting)
	w.Followers[1].BattleWith, w.Followers[1].BattleAggressor, w.Followers[1].Weapons = 2, true, 2
	w.Followers[2].BattleWith, w.Followers[2].BattleWasTown, w.Followers[2].Weapons, w.Followers[2].Stage = 1, true, 1, 9
	document, err := NewDocument(w, catalog, engine.NewDeity("BLUE"), 0, 2, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Decode(data, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if !restored.World.Followers[2].BattleWasTown {
		t.Fatal("combat lost the defender's town identity")
	}
	for pass := 1; pass <= 12; pass++ {
		w.Step()
		restored.World.Step()
		compareCodecContinuation(t, w, restored.World, pass)
	}
}

func TestGAMActiveAIContinuesReactionAndTownEconomy(t *testing.T) {
	catalog := continuationCatalog()
	w := continuationWorld(t, catalog)
	addCodecFollower(t, w, 1, 10, 10, 0, 500, engine.Town)
	addCodecFollower(t, w, 2, 50, 50, 1, 500, engine.Town)
	w.Followers[1].Stage, w.Followers[2].Stage = 9, 9
	w.Players[1].Computer = true
	w.AI[1].Reaction = 7
	w.AI[1].ExpansionCooldown = 2
	w.AI[1].ReleaseCooldown = 11
	w.AI[1].MagnetCooldown = 120
	w.Players[0].Statistics.Metric, w.Players[1].Statistics.Metric = 15, 20
	document, err := NewDocument(w, catalog, engine.NewDeity("BLUE"), 0, 2, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Decode(data, catalog)
	if err != nil {
		t.Fatal(err)
	}
	for pass := 1; pass <= 20; pass++ {
		w.Step()
		restored.World.Step()
		compareCodecContinuation(t, w, restored.World, pass)
		if !reflect.DeepEqual(w.AI, restored.World.AI) {
			t.Fatalf("AI state differs at continuation pass%d", pass)
		}
	}
}

func TestGAMContactWaitingAndArrivalContinueMergeAndBattle(t *testing.T) {
	for _, enemy := range []bool{false, true} {
		t.Run(map[bool]string{false: "friendly-merge", true: "opposing-battle"}[enemy], func(t *testing.T) {
			catalog := continuationCatalog()
			for frame := 0; frame < 2; frame++ {
				catalog.AnimationRoles[3276+uint16(frame*4)] = []AnimationRole{{Name: "contact/waiting", Frame: frame}}
			}
			for frame := 0; frame < 4; frame++ {
				catalog.AnimationRoles[456+uint16(frame*4)] = []AnimationRole{{Name: "combat/attack", Frame: frame}}
			}
			w := continuationWorld(t, catalog)
			owner := 0
			if enemy {
				owner = 1
			}
			addCodecFollower(t, w, 1, 20, 20, 0, 500, engine.Walking)
			addCodecFollower(t, w, 2, 20, 20, owner, 700, engine.Walking)
			addCodecFollower(t, w, 3, 50, 50, 1, 500, engine.Walking)
			s := w.Snapshot()
			s.World.Players[0].Mode, s.World.Players[1].Mode = engine.Rally, engine.Rally
			s.World.Followers[1].ContactWith, s.World.Followers[1].ContactFriendly = 2, !enemy
			s.World.Followers[2].ContactWaiting, s.World.Followers[2].ContactWait = true, 4
			s.Motion[1] = engine.FollowerMotionSnapshot{PositionX: 20*256 + 168, PositionY: 20*256 + 128, VelocityX: -20, LegRemaining: 2, PositionSet: true, Moving: true}
			w, err := s.Restore()
			if err != nil {
				t.Fatal(err)
			}
			document, err := NewDocument(w, catalog, engine.NewDeity("BLUE"), 0, 2, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			data, err := Encode(document)
			if err != nil {
				t.Fatal(err)
			}
			restored, err := Decode(data, catalog)
			if err != nil {
				t.Fatal(err)
			}
			for pass := 1; pass <= 12; pass++ {
				w.Step()
				restored.World.Step()
				compareCodecContinuation(t, w, restored.World, pass)
			}
		})
	}
}

func TestFreshGAMAppearanceVariantOverridesSlotAssignment(t *testing.T) {
	catalog := continuationCatalog()
	w := continuationWorld(t, catalog)
	addCodecFollower(t, w, 1, 20, 20, 0, 500, engine.Walking)
	addCodecFollower(t, w, 2, 50, 50, 1, 500, engine.Walking)
	w.Followers[1].AppearanceVariant, w.Followers[2].AppearanceVariant = 7, 6
	document, err := NewDocument(w, catalog, engine.NewDeity("BLUE"), 0, 2, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Decode(data, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if restored.World.Followers[1].AppearanceVariant != 7 || restored.World.Followers[2].AppearanceVariant != 6 {
		t.Fatal("fresh/imported GAM appearance was reassigned from actor slots")
	}
}

func TestGAMSixHeroClaimsContinueTypedPursuit(t *testing.T) {
	for kind := engine.HeroPerseus; kind <= engine.HeroHelen; kind++ {
		t.Run(heroNames[kind], func(t *testing.T) {
			catalog := continuationCatalog()
			w := continuationWorld(t, catalog)
			addCodecFollower(t, w, 1, 20, 20, 0, 1000, engine.Walking)
			addCodecFollower(t, w, 2, 40, 40, 1, 700, engine.Walking)
			w.Players[0].Mode, w.Players[1].Mode = engine.Rally, engine.Rally
			w.Followers[1].Hero = engine.HeroState{Kind: kind, Phase: engine.HeroPursuing, Target: 2}
			w.Followers[2].Hero.ClaimedBy = 1
			document, err := NewDocument(w, catalog, engine.NewDeity("BLUE"), 0, 2, 0, 0)
			if err != nil {
				t.Fatal(err)
			}
			data, err := Encode(document)
			if err != nil {
				t.Fatal(err)
			}
			restored, err := Decode(data, catalog)
			if err != nil {
				t.Fatal(err)
			}
			if restored.World.Followers[1].Hero.Target != 2 || restored.World.Followers[2].Hero.ClaimedBy != 1 {
				t.Fatal("hero reciprocal claim was lost")
			}
			for pass := 1; pass <= 12; pass++ {
				w.Step()
				restored.World.Step()
				compareCodecContinuation(t, w, restored.World, pass)
				if w.Followers[1].Hero != restored.World.Followers[1].Hero {
					t.Fatalf("hero phase/claim diverged at pass%d", pass)
				}
			}
		})
	}
}

func TestGAMNeutralAndTimedScenarioContinueWithoutRestartingEvents(t *testing.T) {
	catalog := continuationCatalog()
	for frame := 0; frame < 6; frame++ {
		catalog.AnimationRoles[716+uint16(frame*4)] = []AnimationRole{{Name: "neutral/road-maker", Frame: frame}}
	}
	level := catalog.Levels[0]
	binary.BigEndian.PutUint16(level.WorldParameters[:], 2)
	level.WorldParameters[3], level.WorldParameters[4], level.WorldParameters[5] = 90, 20, 20
	catalog.Levels[0] = level
	w := continuationWorld(t, catalog)
	w.Level.WorldParameters = level.WorldParameters
	events, err := engine.DecodeScenarioEvents(level.WorldParameters)
	if err != nil {
		t.Fatal(err)
	}
	w.Scenario = events
	addCodecFollower(t, w, 1, 10, 10, 0, 500, engine.Walking)
	addCodecFollower(t, w, 2, 50, 50, 1, 500, engine.Walking)
	w.Step()
	w.Step()
	w.Step()
	document, err := NewDocument(w, catalog, engine.NewDeity("BLUE"), 0, 2, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Decode(data, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if restored.World.Scenario.Cursor != 1 {
		t.Fatal("GAM restarted an already dispatched scenario event")
	}
	for pass := 1; pass <= 12; pass++ {
		w.Step()
		restored.World.Step()
		compareCodecContinuation(t, w, restored.World, pass)
		if w.Scenario != restored.World.Scenario {
			t.Fatal("scenario cursor changed after GAM load")
		}
	}
}
