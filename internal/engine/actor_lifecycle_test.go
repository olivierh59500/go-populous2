package engine

import "testing"

func TestCreatorMembershipPreservesCrossFamilyChronologyAndRelease(t *testing.T) {
	w := testFlatWorld()
	// Feature state is committed before the creator lifecycle is exposed.
	effect := w.allocateEffect(EffectFireColumn, 0)
	w.Fire.Columns[effect] = FireEffect{Active: true, Owner: 0, X: 20*256 + 128, Y: 20*256 + 128}
	w.syncEffectActor(effect)
	if err := w.EditorPlaceScenery(SceneryBoulder, 20, 20); err != nil {
		t.Fatal(err)
	}
	follower := w.allocate(Follower{Owner: 0, X: 20, Y: 20, State: Walking, Population: 100})
	w.linkFollower(follower)
	head := w.Actors.Heads[20+20*MapSize]
	if head != (ActorRef{Kind: ActorFollower, Index: uint16(follower)}) || w.Actors.Next(head).Kind != ActorScenery || w.Actors.Next(w.Actors.Next(head)).Kind != ActorEffect {
		t.Fatal("cross-family creator chronology changed")
	}
	w.releaseEffect(effect)
	if _, _, linked := w.Actors.Position(ActorRef{Kind: ActorEffect, Index: uint16(effect)}); linked {
		t.Fatal("released effect retained registry membership")
	}
}

func TestActualFireAndLightningCreatorsLinkImmediately(t *testing.T) {
	w := testFlatWorld()
	w.random = randomState(4311)
	if !w.Fire.CreateColumn(0, 20, 20, worldFireHabitat{w}) {
		t.Fatal("fire creation")
	}
	if _, _, linked := w.Actors.Position(ActorRef{Kind: ActorEffect, Index: 0}); !linked {
		t.Fatal("fire creator required a later family rebuild")
	}
	w.Level.Players[0].Powers[Lightning] = true
	if err := w.PlaceLightning(0, 30, 30); err != nil {
		t.Fatal(err)
	}
	slot := w.Air.MarkerSlots[0] - 1
	x, y, linked := w.Actors.Position(ActorRef{Kind: ActorEffect, Index: uint16(slot)})
	if !linked || x != 30*256+128 || y != 30*256+128 {
		t.Fatal("lightning marker did not link at creation")
	}
	if err := w.PlaceLightning(0, 31, 30); err != nil {
		t.Fatal(err)
	}
	x, y, linked = w.Actors.Position(ActorRef{Kind: ActorEffect, Index: uint16(slot)})
	if !linked || x != 31*256+128 || y != 30*256+128 {
		t.Fatal("lightning relocation did not move its existing actor")
	}
}

func TestUnmappedControllersNeverJoinActorRegistry(t *testing.T) {
	w := testFlatWorld()
	for _, kind := range []EffectKind{EffectVolcano, EffectFungus, EffectWhirlpool, EffectEarthquake} {
		id := w.allocateEffect(kind, 0)
		w.syncEffectActor(id)
		if _, _, linked := w.Actors.Position(ActorRef{Kind: ActorEffect, Index: uint16(id)}); linked {
			t.Fatalf("controller kind%d incorrectly mapped", kind)
		}
	}
}

func TestRealCrossFamilyCreatorsPrependWithoutFamilyRebuild(t *testing.T) {
	w := testFlatWorld()
	w.random = randomState(4311)
	if !w.Fire.CreateColumn(0, 20, 20, worldFireHabitat{w}) {
		t.Fatal("fire creator")
	}
	fire := w.Fire.Columns[0]
	x, y := fire.X>>8, fire.Y>>8
	if err := w.EditorPlaceScenery(SceneryBoulder, x, y); err != nil {
		t.Fatal(err)
	}
	w.Level.Players[0].Powers[Lightning] = true
	if err := w.PlaceLightning(0, x, y); err != nil {
		t.Fatal(err)
	}
	marker := ActorRef{Kind: ActorEffect, Index: uint16(w.Air.MarkerSlots[0] - 1)}
	if w.Actors.Heads[x+y*MapSize] != marker || w.Actors.Next(marker).Kind != ActorScenery || w.Actors.Next(w.Actors.Next(marker)) != (ActorRef{Kind: ActorEffect, Index: 0}) {
		t.Fatal("creators were reordered by family instead of insertion time")
	}
}

func TestBasaltCreatorLinksMappedImpactAndTerminalUnlinks(t *testing.T) {
	w := &World{Editor: true}
	if !w.CreateBasalt(0, 20, 20, 1, 2) {
		t.Fatal("basalt creation")
	}
	ref := ActorRef{Kind: ActorEffect, Index: 0}
	x, y, linked := w.Actors.Position(ref)
	if !linked || x != 20*256+128 || y != 20*256+128 || w.Actors.Heads[20+20*MapSize] != ref {
		t.Fatal("basalt impact omitted its original map insertion")
	}
	w.tickWaterEffect(0)
	w.tickWaterEffect(0)
	if _, _, linked = w.Actors.Position(ref); linked || w.effects.Slots[0].Kind != EffectNone {
		t.Fatal("completed basalt retained mixed membership")
	}
	if !w.Water.Painted[20+20*MapSize] || w.Water.Tiles[20+20*MapSize] != 224 {
		t.Fatal("terminal basalt removed permanent terrain")
	}
}
