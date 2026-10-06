package network

import (
	"reflect"
	"testing"

	"go-populous2/internal/engine"
)

func TestTerrainSecondaryNetworkCommandMatchesLocalLowering(t *testing.T) {
	w := networkWorld(t)
	w.Level.Players[1].Scenario.BuildAnywhere = true
	var heights [engine.CornerSize * engine.CornerSize]uint8
	for i := range heights {
		heights[i] = 2
	}
	if err := w.EditorSetTerrain(heights); err != nil {
		t.Fatal(err)
	}
	local, err := w.Snapshot().Restore()
	if err != nil {
		t.Fatal(err)
	}
	target := engine.PowerTarget{X: 20, Y: 20}
	if local.Sprog(1, target.X, target.Y) {
		t.Fatal("test target unexpectedly contains a releasable town")
	}
	target.Lower = true
	if err := local.Cast(1, engine.RaiseLower, target); err != nil {
		t.Fatal(err)
	}
	command := Command{Kind: "terrain-secondary", Target: engine.PowerTarget{X: 20, Y: 20}}
	if err := validateCommands([]Command{command}, 1); err != nil {
		t.Fatal(err)
	}
	if err := applyCommand(w, 1, command); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(w.Snapshot(), local.Snapshot()) {
		t.Fatal("network secondary click differs from local terrain action")
	}
}

func TestTerrainSecondaryReleasesTownBeforeLoweringProhibition(t *testing.T) {
	w := networkWorld(t)
	for range 120 {
		w.Step()
	}
	town := 0
	for id, follower := range w.Followers {
		if follower.State == engine.Town && follower.Owner == 1 {
			town = id
			break
		}
	}
	if town == 0 {
		t.Fatal("settlement did not establish a town")
	}
	w.Level.Players[1].Scenario.ForbidLower = true
	f := w.Followers[town]
	mana, heights := w.Players[1].Mana, w.Heights
	if err := applyCommand(w, 1, Command{Kind: "terrain-secondary", Target: engine.PowerTarget{X: int(f.X), Y: int(f.Y)}}); err != nil {
		t.Fatal(err)
	}
	if !w.Followers[town].ForceEmigration || w.Players[1].Mana != mana || w.Heights != heights {
		t.Fatal("town release spent mana or changed forbidden terrain")
	}
}
