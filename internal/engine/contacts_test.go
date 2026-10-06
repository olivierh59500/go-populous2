package engine

import "testing"

func TestFriendlyEntryHomesBeforeMergingPopulation(t *testing.T) {
	w := testFlatWorld()
	a := w.allocate(Follower{Owner: 0, X: 20, Y: 20, State: Walking, Population: 100, MovementSpeed: 20})
	w.linkFollower(a)
	b := w.allocate(Follower{Owner: 0, X: 21, Y: 20, State: Walking, Population: 200, MovementSpeed: 20})
	w.linkFollower(b)
	w.beginLeg(a, 21, 20)
	for range 7 {
		w.advanceLeg(a)
	}
	if w.Followers[a].State == Inactive || w.Followers[a].ContactWith != b || w.Followers[b].Population != 200 {
		t.Fatal("entry merged before homing")
	}
	var ids [FollowerCapacity]int
	if w.FollowersAt(21, 20, ids[:]) != 2 {
		t.Fatal("contact actors do not share typed parcel chain")
	}
	for range 12 {
		if w.Followers[a].State != Inactive {
			w.stepContact(a)
		}
	}
	if w.Followers[a].State != Inactive || w.Followers[b].Population != 300 || w.Occupants[21+20*MapSize] != uint16(b) || w.Followers[b].PreviousFollower != 0 {
		t.Fatal("completed contact did not merge and unlink")
	}
}

func TestEnemyEntryHomesBeforeBeginningBattle(t *testing.T) {
	w := testFlatWorld()
	a := w.allocate(Follower{Owner: 0, X: 20, Y: 20, State: Walking, Population: 100, MovementSpeed: 20})
	w.linkFollower(a)
	b := w.allocate(Follower{Owner: 1, X: 21, Y: 20, State: Walking, Population: 200, MovementSpeed: 20})
	w.linkFollower(b)
	w.beginLeg(a, 21, 20)
	for range 7 {
		w.advanceLeg(a)
	}
	if w.Followers[a].State == Fighting || w.Followers[a].ContactWith != b {
		t.Fatal("entry began combat before contact")
	}
	for range 12 {
		if w.Followers[a].State != Fighting {
			w.stepContact(a)
		}
	}
	if w.Followers[a].State != Fighting || w.Followers[b].State != Fighting || w.Followers[a].BattleWith != b {
		t.Fatal("completed hostile contact did not create reciprocal combat")
	}
}

func TestRemovedContactTargetReleasesArrivingGroup(t *testing.T) {
	w := testFlatWorld()
	a := addFollower(w, 20, 20, 0, 100, Walking)
	b := addFollower(w, 21, 20, 0, 200, Walking)
	w.prepareContact(a, b)
	w.remove(b)
	for range 14 {
		if w.Followers[a].ContactWith != 0 {
			w.stepContact(a)
		}
	}
	if w.Followers[a].State == Inactive || w.Followers[a].ContactWith != 0 || w.Followers[a].Population != 100 {
		t.Fatalf("removed target redispatch: state%d target%d population%d", w.Followers[a].State, w.Followers[a].ContactWith, w.Followers[a].Population)
	}
}

func TestSourceContactMergeTransfersLeaderWithoutDeathStatistics(t *testing.T) {
	w := testFlatWorld()
	a := addFollower(w, 20, 20, 0, 100, Walking)
	b := addFollower(w, 20, 20, 0, 200, Walking)
	w.Players[0].Leader = a
	w.Players[0].Statistics.Metric = 100
	w.Followers[a].Weapons = 7
	w.Followers[b].Weapons = 3
	w.mergeFollowers(a, b)
	if w.Players[0].Leader != b || w.Players[0].Statistics.Metric != 100 || w.Players[0].Statistics.LeaderLosses != 0 || w.Followers[b].Population != 300 || w.Followers[b].Weapons != 7 || w.Followers[a].State != Inactive {
		t.Fatal("join counted a death or lost the source leader attributes")
	}
}

func TestContactRedispatchHasBoundedTransitionsAndKeepsWorldTick(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 1000, Walking)
	w.Players[0].Mode = Rally
	w.Players[0].RallyX = 20
	w.Players[0].RallyY = 20
	w.Followers[id].ContactWith = id
	w.Followers[id].moving = false
	before := w.Tick
	w.stepFollower(id)
	if w.Tick != before || w.followerTransitionDepth[id] != 0 || w.Followers[id].State != Walking {
		t.Fatal("contact redispatch changed world time or leaked transition budget")
	}
}
