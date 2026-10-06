package engine

import "testing"

func TestRetainedCleanupChargesLeaderMetricOnceAndKeepsMembership(t *testing.T) {
	w := testFlatWorld()
	id := w.allocate(Follower{Owner: 0, X: 20, Y: 20, State: Walking, Population: 100, Hero: HeroState{Kind: HeroHelen}})
	w.linkFollower(id)
	victim := w.allocate(Follower{Owner: 1, X: 21, Y: 20, State: Walking, Population: 100, Hero: HeroState{CaptiveOf: id}})
	w.linkFollower(victim)
	w.Players[0].Leader = id
	w.Players[0].Statistics.Metric = 100
	w.PrepareFollowerDeath(id)
	w.PrepareFollowerDeath(id)
	f := w.Followers[id]
	if f.State != Walking || f.Population != 0 || !f.CleanupPrepared || w.Occupants[20+20*MapSize] != uint16(id) {
		t.Fatal("retained cleanup removed identity or membership")
	}
	if w.Players[0].Statistics.Metric != 88 || w.Players[0].Statistics.LeaderLosses != 1 || w.Players[0].Leader != 0 {
		t.Fatal("cleanup statistics were charged more than once")
	}
	if w.Magnets[0].X != 20*256+128 || w.Magnets[0].Y != 20*256+128 || w.Followers[victim].Hero.CaptiveOf != 0 {
		t.Fatal("leader marker or captive claims survived cleanup")
	}
	w.remove(id)
	if w.Players[0].Statistics.Metric != 88 || w.Players[0].Statistics.LeaderLosses != 1 || w.Followers[id].State != Inactive || w.Occupants[20+20*MapSize] != 0 {
		t.Fatal("terminal cleanup charged statistics again")
	}
}

func TestOrdinaryRemovalUsesTheSameCleanupPath(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Walking)
	w.Players[0].Statistics.Metric = 1
	w.remove(id)
	if w.Players[0].Statistics.Metric != 65535 || w.Players[0].Statistics.LeaderLosses != 0 {
		t.Fatal("ordinary cleanup did not retain unsigned word arithmetic")
	}
}

func TestTownDestructionScorchesOnlyItsOwnedFarmFootprint(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Town)
	w.Followers[id].Stage = 18
	w.Farms[20+20*MapSize] = 1
	w.Farms[21+20*MapSize] = 2
	w.Farms[19+20*MapSize] = 1
	w.rebuildCells()
	w.destroyCombatTown(id)
	if w.Nature.Ground[20+20*MapSize].Mark != GroundScorched || w.Nature.Ground[19+20*MapSize].Mark != GroundScorched || w.Farms[21+20*MapSize] != 2 || w.Nature.Ground[21+20*MapSize].Mark != GroundNone {
		t.Fatal("town destruction overwrote enemy farmland")
	}
}

func TestHazardHitChargesRetainedCleanupBeforeTerminal(t *testing.T) {
	for _, hazard := range []string{"swamp", "water", "fire", "monster"} {
		t.Run(hazard, func(t *testing.T) {
			w := testFlatWorld()
			id := addFollower(w, 20, 20, 0, 100, Walking)
			w.Players[0].Leader = id
			w.Players[0].Statistics.Metric = 100
			switch hazard {
			case "swamp":
				w.Nature.Ground[20+20*MapSize] = GroundParcel{Mark: GroundSwamp}
				w.EnterNatureHazard(id)
			case "water":
				w.Tiles[20+20*MapSize] = Cell{}
				w.Level.Players[0].Scenario.FatalWater = true
				w.advanceWater(id)
			case "fire":
				w.damageFireParcel(20, 20, false)
			case "monster":
				monster, _ := w.CreateNeutral(NeutralMonster, 19, 19)
				w.tickNeutral(monster)
			}
			if !w.Followers[id].CleanupPrepared || w.Players[0].Statistics.LeaderLosses != 1 || w.Players[0].Statistics.Metric != 88 || w.Followers[id].State == Inactive {
				t.Fatal("hazard deferred statistics until actor removal")
			}
			w.remove(id)
			if w.Players[0].Statistics.LeaderLosses != 1 || w.Players[0].Statistics.Metric != 88 {
				t.Fatal("hazard terminal charged cleanup twice")
			}
		})
	}
}

func TestLightningTownReformUsesSettlementEvaluationAndKeepsWork(t *testing.T) {
	w := testFlatWorld()
	id := w.allocate(Follower{Owner: 0, X: 20, Y: 20, State: Town, Population: 1000, Stage: 9, Work: 7, positionSet: true, positionX: 20*256 + 173, positionY: 20*256 + 147})
	w.linkFollower(id)
	competitor := w.allocate(Follower{Owner: 1, X: 21, Y: 20, State: Town, Population: 200, Stage: 9})
	w.linkFollower(competitor)
	w.Tick = 123
	w.AirVictims[id] = LightningVictimState{Phase: LightningVictimTownHit, Bolt: 0, Frames: 2}
	w.AdvanceLightningVictim(id)
	f := w.Followers[id]
	if f.State != Town || f.Stage != 18 || f.Work != 7 || f.FoundedAt != 123 || f.positionX != 20*256+128 || f.positionY != 20*256+128 || w.Followers[competitor].State != Walking {
		t.Fatal("lightning reform used a read-only support preview instead of the source settlement body")
	}
}
