package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestScenarioHeightAdmissionOptions(t *testing.T) {
	for _, options := range []uint16{0, 1, 2, 3, 8, 9, 10, 16, 17, 18, 32, 35, 512, 515} {
		s := decodeScenario(options)
		for height := 0; height <= 8; height++ {
			expectedRaise := options&8 == 0 && (options&1 != 0 || options&2 != 0 && height == 0)
			expectedLower := options&16 == 0 && (options&1 != 0 || options&2 != 0 && height <= 1)
			if s.TerrainEditAllowed(height, true) != expectedRaise || s.TerrainEditAllowed(height, false) != expectedLower {
				t.Fatalf("options%d height%d", options, height)
			}
		}
	}
}

func TestPlayerTerrainAdmissionProtectsPropagatedEnemyFarmsAndWalls(t *testing.T) {
	w := testFlatWorld()
	w.Players[0].Mana = 10000
	w.Level.Players[0].Scenario.ForbidEnemyTerrain = true
	w.Farms[31+31*MapSize] = 2
	before := w.Heights
	if w.RaiseAt(0, 32, 32) || w.Heights != before || w.Players[0].Mana != 10000 {
		t.Fatal("enemy farm admission changed terrain or mana")
	}
	w.Farms[31+31*MapSize] = 0
	w.Earth.Walls[0] = WallActor{Active: true, Owner: 0, X: 31, Y: 31}
	if w.RaiseAt(0, 32, 32) || w.Heights != before {
		t.Fatal("wall admission changed terrain")
	}
	w.Earth.Walls[0].Broken = true
	if !w.RaiseAt(0, 32, 32) {
		t.Fatal("broken wall incorrectly protected terrain")
	}
}

func TestScenarioMapVisibilityAndEmigrationOptions(t *testing.T) {
	w := testFlatWorld()
	w.Level.Players[0].Scenario.HideEnemy = true
	w.Level.Players[0].Scenario.HideDisasters = true
	if !w.FollowerVisibleOnMap(0, 0) || w.FollowerVisibleOnMap(0, 1) || w.EffectVisibleOnMap(0) {
		t.Fatal("scenario map visibility")
	}
	id := addFollower(w, 20, 20, 0, 100, Town)
	w.Level.Players[0].Scenario.DisableEmigration = true
	if w.Evacuate(id) || w.Followers[id].State != Town {
		t.Fatal("disabled right-click emigration")
	}
}

func TestFatalWaterRetainsFollowerUntilDeathSequenceCompletes(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Walking)
	w.Tiles[20+20*MapSize] = Cell{}
	w.Level.Players[0].Scenario.FatalWater = true
	w.stepFollower(id)
	if w.Followers[id].State != Ruin || w.Followers[id].Population != 0 || w.Occupants[20+20*MapSize] != uint16(id) {
		t.Fatal("fatal swimmer was not retained")
	}
	for range 6 {
		w.stepFollower(id)
	}
	if w.Followers[id].State != Inactive || w.Occupants[20+20*MapSize] != 0 {
		t.Fatal("water death did not clean up after source seven-frame sequence")
	}
}

func TestNonFatalWaterUsesCampaignAttritionAndReturnsToLand(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Walking)
	w.Tiles[20+20*MapSize] = Cell{}
	w.Level.Players[0].Attrition = 7
	w.stepFollower(id)
	if w.Followers[id].Population != 93 || w.Followers[id].State != Drowning {
		t.Fatal("water survivor attrition")
	}
	w.Tiles[20+20*MapSize] = Cell{Corners: [4]uint8{1, 1, 1, 1}, Shape: 15, Code: 15}
	w.stepFollower(id)
	if w.Followers[id].State == Drowning || w.Followers[id].Population != 79 {
		t.Fatal("rescued follower did not return to ordinary land dispatch")
	}
}

func TestScenarioPermissionOriginalNumericalCorpus(t *testing.T) {
	data, err := os.ReadFile("testdata/scenario_permission_numbers.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Owner, Options, Height int
			Raise, Lower           bool
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 112 {
		t.Fatalf("permission cases%d", len(corpus.Cases))
	}
	for _, test := range corpus.Cases {
		options := decodeScenario(uint16(test.Options))
		if options.TerrainEditAllowed(test.Height, true) != test.Raise || options.TerrainEditAllowed(test.Height, false) != test.Lower {
			t.Fatalf("owner%d options%d height%d", test.Owner, test.Options, test.Height)
		}
	}
}

func TestWaterDeathAndLivingLoopUseDistinctSourceSequences(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Walking)
	w.Tiles[20+20*MapSize] = Cell{}
	for range 8 {
		w.stepFollower(id)
		if w.Followers[id].Frame >= 2 {
			t.Fatal("living drowning loop exceeded its two artwork frames")
		}
	}
	if (Follower{}).WaterDeathFrames() != 7 {
		t.Fatal("terminal swimmer death length")
	}
	for kind := HeroPerseus; kind <= HeroHelen; kind++ {
		if (Follower{Hero: HeroState{Kind: kind}}).WaterDeathFrames() != [6]int{5, 9, 9, 9, 8, 3}[int(kind)-1] {
			t.Fatal("hero terminal water death length")
		}
	}
}
