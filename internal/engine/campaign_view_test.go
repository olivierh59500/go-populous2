package engine

import (
	"reflect"
	"testing"
)

func TestViewportStepChangesOnlyItsCurrentFrameScoringObservation(t *testing.T) {
	w := testFlatWorld()
	for owner := range w.Level.Players {
		w.Level.Players[owner].Scenario = ScenarioOptions{}
		w.Level.Players[owner].Extra[0] = 0
		w.Players[owner].Computer = false
	}
	addFollower(w, 22, 22, 0, 100, Town)
	addFollower(w, 23, 23, 1, 100, Town)
	ordinary, err := w.Snapshot().Restore()
	if err != nil {
		t.Fatal(err)
	}
	view := Viewport{20, 20, 8}
	w.StepWithViewport(view)
	ordinary.Step()
	for owner := range w.Players {
		if w.Players[owner].Statistics.ScenarioOptions != 1 || ordinary.Players[owner].Statistics.ScenarioOptions != 0 {
			t.Fatal("viewport rights did not stay separate from ordinary/headless steps")
		}
		ordinary.Players[owner].Statistics.ScenarioOptions = w.Players[owner].Statistics.ScenarioOptions
	}
	if !reflect.DeepEqual(w, ordinary) {
		t.Fatal("viewport scoring observation changed physics, AI, allocation or RNG")
	}
	if _, err := w.Snapshot().Restore(); err != nil {
		t.Fatal("observed scoring rights did not survive a valid snapshot", err)
	}
}

func TestViewportStepDoesNotInventPermissionsForAnEmptyView(t *testing.T) {
	w := testFlatWorld()
	for owner := range w.Level.Players {
		w.Level.Players[owner].Scenario = ScenarioOptions{}
		w.Level.Players[owner].Extra[0] = 0
		w.Players[owner].Computer = false
	}
	addFollower(w, 22, 22, 0, 100, Town)
	addFollower(w, 23, 23, 1, 100, Town)
	w.StepWithViewport(Viewport{40, 40, 8})
	if w.Players[0].Statistics.ScenarioOptions != 0 || w.Players[1].Statistics.ScenarioOptions != 0 {
		t.Fatal("unrendered towns were used as a guessed nonzero scoring divisor")
	}
	if _, err := ScoreCampaign(uint32(w.Tick), w.Players[0].Statistics, w.Players[1].Statistics); err == nil {
		t.Fatal("empty-view arithmetic fault was silently masked")
	}
}

func TestViewportStepPreservesCompletedResultObservations(t *testing.T) {
	w := testFlatWorld()
	w.Result = 1
	w.Players[0].Statistics.ScenarioOptions = 1
	w.Players[1].Statistics.ScenarioOptions = 3
	before := *w
	w.StepWithViewport(Viewport{40, 40, 8})
	if !reflect.DeepEqual(*w, before) {
		t.Fatal("finished result was reinterpreted using a later camera position")
	}
}

func TestViewportStepRetainsUninterpretedSavedOptionBits(t *testing.T) {
	w := testFlatWorld()
	for owner := range w.Level.Players {
		w.Level.Players[owner].Scenario = ScenarioOptions{}
		w.Level.Players[owner].Extra[0] = 0xa400
		w.Players[owner].Computer = false
	}
	addFollower(w, 22, 22, 0, 100, Town)
	addFollower(w, 23, 23, 1, 100, Walking)
	w.StepWithViewport(Viewport{20, 20, 8})
	if w.Players[0].Statistics.ScenarioOptions != 0xa401 || w.Players[1].Statistics.ScenarioOptions != 0xa402 {
		t.Fatal("viewport observation discarded saved high option bits")
	}
}
