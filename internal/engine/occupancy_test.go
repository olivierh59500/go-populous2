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
