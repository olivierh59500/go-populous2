package engine

import "testing"

func TestTypedOccupancyPrependRemoveAndMovePreserveChain(t *testing.T) {
	w := testFlatWorld()
	a := w.allocate(Follower{X: 20, Y: 20, State: Walking, Population: 100})
	w.linkFollower(a)
	b := w.allocate(Follower{X: 20, Y: 20, State: Walking, Population: 100})
	w.linkFollower(b)
	c := w.allocate(Follower{X: 20, Y: 20, State: Walking, Population: 100})
	w.linkFollower(c)
	var ids [FollowerCapacity]int
	if n := w.FollowersAt(20, 20, ids[:]); n != 3 || ids[0] != c || ids[1] != b || ids[2] != a {
		t.Fatal("prepend order")
	}
	w.unlinkFollower(b)
	if w.Followers[c].NextFollower != a || w.Followers[a].PreviousFollower != c || w.Followers[b].NextFollower != 0 || w.Followers[b].PreviousFollower != 0 {
		t.Fatal("middle removal")
	}
	w.moveFollowerCell(c, 21, 20)
	if w.Occupants[20+20*MapSize] != uint16(a) || w.Followers[a].PreviousFollower != 0 || w.Occupants[21+20*MapSize] != uint16(c) || w.Pressure[21+20*MapSize] != 8 {
		t.Fatal("move did not splice chain or increment pressure")
	}
	w.remove(a)
	if w.Occupants[20+20*MapSize] != 0 {
		t.Fatal("dead final actor retained head")
	}
}

func TestAdonisClonesShareTypedParcelWithoutOverwritingParent(t *testing.T) {
	w := testFlatWorld()
	id := w.allocate(Follower{X: 20, Y: 20, State: Walking, Population: 101, Hero: HeroState{Kind: HeroAdonis}})
	w.linkFollower(id)
	child := w.SplitAdonis(id)
	var ids [FollowerCapacity]int
	if n := w.FollowersAt(20, 20, ids[:]); n != 2 || ids[0] != child || ids[1] != id {
		t.Fatal("clone lost parent occupancy")
	}
	w.remove(child)
	if w.Occupants[20+20*MapSize] != uint16(id) || w.Followers[id].PreviousFollower != 0 {
		t.Fatal("clone death lost parent")
	}
}

func TestNeutralFollowerDoesNotIndexPlayerLedger(t *testing.T) {
	w := testFlatWorld()
	id := w.allocate(Follower{Owner: 2, X: 20, Y: 20, State: Walking, Population: 100})
	w.linkFollower(id)
	w.Step()
	if w.Followers[id].Population != 100 || w.Summaries()[0].Population != 0 || w.Summaries()[1].Population != 0 {
		t.Fatal("neutral actor entered deity statistics")
	}
}

func TestFollowerMovementKeepsMixedActorLinksAndFractionalCoordinates(t *testing.T) {
	w := testFlatWorld()
	w.Magnets[0] = MagnetActor{Owner: 0, X: 21*256 + 128, Y: 20*256 + 128}
	w.Actors.Link(ActorRef{Kind: ActorMagnet, Index: 0}, w.Magnets[0].X, w.Magnets[0].Y)
	id := w.allocate(Follower{X: 20, Y: 20, State: Walking, Population: 100, MovementSpeed: 20})
	w.linkFollower(id)
	w.beginLeg(id, 21, 20)
	for range 7 {
		w.advanceLeg(id)
	}
	ref := ActorRef{Kind: ActorFollower, Index: uint16(id)}
	x, y, ok := w.Actors.Position(ref)
	if !ok || x != w.Followers[id].positionX || y != w.Followers[id].positionY || w.Actors.Next(ref).Kind != ActorMagnet {
		t.Fatal("follower crossing lost marker membership or fixed position")
	}
	w.remove(id)
	if w.Actors.Heads[21+20*MapSize].Kind != ActorMagnet {
		t.Fatal("follower death removed magnet")
	}
}

func TestMagnetRelocationMovesRealActorWithoutPressure(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Walking)
	w.Players[0].Leader = id
	if !w.PlaceMagnet(0, 30, 30) {
		t.Fatal("placement failed")
	}
	x, y, ok := w.Actors.Position(ActorRef{Kind: ActorMagnet, Index: 0})
	if !ok || x != 30*256+128 || y != 30*256+128 || w.Pressure[30+30*MapSize] != 0 {
		t.Fatal("magnet position or pressure")
	}
}
