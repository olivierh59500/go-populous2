package engine

import "testing"

func TestCursorRightsUseEntireVisibleOwnedParcelView(t *testing.T) {
	w := testFlatWorld()
	w.Level.Players[0].Scenario = ScenarioOptions{}
	walker := addFollower(w, 27, 27, 0, 100, Walking)
	rights := w.CursorTerrainRights(0, Viewport{20, 20, 8})
	if rights.BuildAnywhere || !rights.SeaLevelOnly || !rights.TerrainEditAllowed(0, true) || rights.TerrainEditAllowed(1, true) {
		t.Fatal("visible walker should grant sea-level editing throughout the view")
	}
	w.Followers[walker].X = 28
	w.Followers[walker].Y = 27
	w.Actors.Move(ActorRef{Kind: ActorFollower, Index: uint16(walker)}, 28*256+128, 27*256+128)
	rights = w.CursorTerrainRights(0, Viewport{20, 20, 8})
	if rights.BuildAnywhere || rights.SeaLevelOnly {
		t.Fatal("actor beyond the eight-parcel boundary granted rights")
	}
}

func TestCursorRightsTownOwnershipAndStaticScenarioExceptions(t *testing.T) {
	w := testFlatWorld()
	w.Level.Players[0].Scenario = ScenarioOptions{}
	addFollower(w, 23, 23, 1, 100, Town)
	rights := w.CursorTerrainRights(0, Viewport{20, 20, 8})
	if rights.BuildAnywhere || rights.SeaLevelOnly {
		t.Fatal("enemy town granted local editing")
	}
	town := addFollower(w, 25, 25, 0, 100, Town)
	w.Followers[town].Stage = 1
	rights = w.CursorTerrainRights(0, Viewport{20, 20, 8})
	if !rights.BuildAnywhere || !rights.TerrainEditAllowed(7, true) {
		t.Fatal("small visible town did not grant full height editing")
	}
	w.remove(town)
	w.Level.Players[0].Scenario = ScenarioOptions{BuildAnywhere: true, ForbidRaise: true}
	rights = w.CursorTerrainRights(0, Viewport{20, 20, 8})
	if !rights.BuildAnywhere || rights.TerrainEditAllowed(1, true) {
		t.Fatal("static exception or prohibition was lost")
	}
}

func TestCursorRightsExcludeUnmappedWaitingAndRetainedActors(t *testing.T) {
	w := testFlatWorld()
	w.Level.Players[0].Scenario = ScenarioOptions{}
	id := w.allocate(Follower{Owner: 0, X: 23, Y: 23, State: Walking, Population: 100})
	if w.CursorTerrainRights(0, Viewport{20, 20, 8}).SeaLevelOnly {
		t.Fatal("unmapped follower granted rights")
	}
	w.linkFollower(id)
	w.Followers[id].ContactWaiting = true
	if w.CursorTerrainRights(0, Viewport{20, 20, 8}).SeaLevelOnly {
		t.Fatal("waiting contact target granted walking rights")
	}
	w.Followers[id].ContactWaiting = false
	w.PrepareFollowerDeath(id)
	w.Followers[id].State = Ruin
	rights := w.CursorTerrainRights(0, Viewport{20, 20, 8})
	if rights.BuildAnywhere || rights.SeaLevelOnly {
		t.Fatal("retained corpse granted rights")
	}
}

func TestCursorRightsFollowActualVictimAndCarryDrawStates(t *testing.T) {
	w := testFlatWorld()
	w.Level.Players[0].Scenario = ScenarioOptions{}
	id := addFollower(w, 23, 23, 0, 100, Walking)
	for _, phase := range []LightningVictimPhase{LightningVictimWalkingHit, LightningVictimDeath, LightningVictimRecovery} {
		w.AirVictims[id] = LightningVictimState{Phase: phase}
		if w.CursorTerrainRights(0, Viewport{20, 20, 8}).SeaLevelOnly {
			t.Fatal("lightning continuation granted ordinary walking rights")
		}
	}
	w.AirVictims[id] = LightningVictimState{}
	w.Air.Carry[id] = AirCarryState{Phase: AirCarryLanding}
	if w.CursorTerrainRights(0, Viewport{20, 20, 8}).SeaLevelOnly {
		t.Fatal("landing actor granted ordinary walking rights")
	}
}

func TestViewportCastUsesEffectiveRightsWithoutMutatingScenario(t *testing.T) {
	w := testFlatWorld()
	w.Level.Players[0].Scenario = ScenarioOptions{}
	w.Level.Players[0].Powers[RaiseLower] = true
	w.Players[0].Mana = 1000
	before := w.Level.Players[0].Scenario
	if err := w.CastFromViewport(0, RaiseLower, PowerTarget{X: 23, Y: 23}, Viewport{20, 20, 8}); err == nil {
		t.Fatal("empty view allowed human sculpting")
	}
	addFollower(w, 25, 25, 0, 100, Town)
	if err := w.CastFromViewport(0, RaiseLower, PowerTarget{X: 23, Y: 23}, Viewport{20, 20, 8}); err != nil {
		t.Fatal(err)
	}
	if w.Level.Players[0].Scenario != before || w.Players[0].Mana != 980 {
		t.Fatal("viewport cast leaked dynamic rights or changed the source debit")
	}
	if err := w.CastFromViewport(0, RaiseLower, PowerTarget{X: 29, Y: 23}, Viewport{20, 20, 8}); err == nil {
		t.Fatal("human target outside visible corners was accepted")
	}
}
