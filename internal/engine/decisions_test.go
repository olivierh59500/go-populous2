package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestOrdinarySearchRetainsPreferredOrderAndTownRadius(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Walking)
	w.Followers[id].Search = 2
	x, y, ok := w.chooseMove(id)
	if !ok || x != 19 || y != 19 {
		t.Fatalf("preferred order %d,%d", x, y)
	}
	for _, d := range preferredSettlements[:8] {
		w.Tiles[20+d[0]+(20+d[1])*MapSize] = Cell{}
	}
	w.Followers[id].Search = 10
	x, y, ok = w.chooseMove(id)
	if !ok || x != 18 || y != 18 {
		t.Fatalf("stage-five emigrant search %d,%d", x, y)
	}
}

func TestJoinAndFightSelectMatchingActorBeforeFallback(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Walking)
	friend := addFollower(w, 21, 20, 0, 100, Walking)
	enemy := addFollower(w, 20, 21, 1, 100, Town)
	w.SetMode(0, Join)
	x, y, ok := w.chooseMove(id)
	if !ok || x != int(w.Followers[friend].X) || y != int(w.Followers[friend].Y) {
		t.Fatal("join did not target a friendly walking group")
	}
	w.SetMode(0, Fight)
	x, y, ok = w.chooseMove(id)
	if !ok || x != int(w.Followers[enemy].X) || y != int(w.Followers[enemy].Y) {
		t.Fatal("fight did not target enemy town")
	}
}

func TestPressureFallbackChoosesLowestVisitPressure(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Walking)
	for _, d := range decisionNeighbours[:8] {
		at := 20 + d[0] + (20+d[1])*MapSize
		w.Tiles[at].Corners = [4]uint8{1, 2, 2, 1}
		w.Tiles[at].Code = 6
		w.Pressure[at] = 200
	}
	w.Pressure[21+21*MapSize] = 0
	x, y, ok := w.chooseMove(id)
	if !ok || x != 21 || y != 21 {
		t.Fatalf("pressure fallback %d,%d", x, y)
	}
}

func TestMagnetPlacementDoesNotChangeTacticalMode(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Walking)
	w.Players[0].Leader = id
	w.Players[0].Mana = 1000
	w.Level.Players[0].Powers[PapalMagnet] = true
	if err := w.Cast(0, PapalMagnet, PowerTarget{X: 30, Y: 30}); err != nil {
		t.Fatal(err)
	}
	if w.Players[0].Mode != Settle || w.Players[0].RallyX != 30 || w.Players[0].Mana != 900 {
		t.Fatal("magnet changed tactic or price")
	}
}

func TestRoadFallbackPrecedesPressureAndUsesSourceSpeedBonus(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 1000, Walking)
	w.Players[0].Mode = Fight
	// Eliminate settlement candidates so fallback inspects cardinal roads.
	for _, d := range decisionNeighbours[:8] {
		at := 20 + d[0] + (20+d[1])*MapSize
		w.Tiles[at] = Cell{Corners: [4]uint8{1, 2, 2, 1}, Code: 3, Shape: 3}
		w.Pressure[at] = 0
	}
	w.Earth.Roads[21+20*MapSize] = RoadParcel{Active: true, Code: 197}
	w.Pressure[21+20*MapSize] = 248
	w.Followers[id].MovementSpeed = 20
	w.stepFollower(id)
	f := w.Followers[id]
	if !f.RoadLeg || f.velocityX != 40 || f.velocityY != 0 || f.MovementSpeed != 20 || f.legRemaining != 5 {
		t.Fatalf("road motion %+v", f)
	}
}

func TestSaturatedRoadLegRetainsOriginalVelocityAndSpeedRestoration(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 1000, Walking)
	w.Players[0].Mode = Fight
	for _, d := range decisionNeighbours[:8] {
		at := 20 + d[0] + (20+d[1])*MapSize
		w.Tiles[at] = Cell{Corners: [4]uint8{1, 2, 2, 1}, Code: 3, Shape: 3}
	}
	w.Earth.Roads[21+20*MapSize] = RoadParcel{Active: true, Code: 197}
	w.Followers[id].MovementSpeed = 250
	w.stepFollower(id)
	f := w.Followers[id]
	if f.velocityX != 255 || f.MovementSpeed != 235 {
		t.Fatal("saturated road speed did not retain the source restore rule")
	}
}

func TestOrdinaryDecisionsOriginalNumericalCorpus(t *testing.T) {
	data, err := os.ReadFile("testdata/decision_numbers.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Name                string
			Mode, Search, Speed int
			Seed                uint32
			VX, VY              int
			DefaultPressure     uint8 `json:"default_pressure"`
			DefaultCode         uint8 `json:"default_code"`
			Cells               []struct {
				X, Y           int
				Pressure, Code uint8
			}
			Actors      []struct{ Slot, Kind, Owner, Next, X, Y int }
			TargetX     int `json:"target_x"`
			TargetY     int `json:"target_y"`
			Route       string
			ResultSpeed int `json:"result_speed"`
			ResultVX    int `json:"result_vx"`
			ResultVY    int `json:"result_vy"`
			Timer       int
			RNG         uint32
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	for _, test := range corpus.Cases {
		t.Run(test.Name, func(t *testing.T) {
			w := testFlatWorld()
			w.random = randomState(test.Seed)
			for at := range w.Tiles {
				w.Tiles[at].Code = test.DefaultCode
				if test.DefaultCode == 0 {
					w.Tiles[at].Corners = [4]uint8{}
				}
				w.Pressure[at] = test.DefaultPressure
			}
			for _, cell := range test.Cells {
				w.Tiles[cell.X+cell.Y*MapSize].Code = cell.Code
				w.Pressure[cell.X+cell.Y*MapSize] = cell.Pressure
			}
			for _, actor := range test.Actors {
				if actor.Kind == 24 {
					w.Nature.Scenery[0] = SceneryActor{Kind: SceneryBoulder, X: uint8(actor.X), Y: uint8(actor.Y)}
				} else {
					state := Walking
					if actor.Kind == 4 {
						state = Town
					}
					w.Followers[actor.Slot] = Follower{Owner: uint8(actor.Owner), X: uint8(actor.X), Y: uint8(actor.Y), State: state, Population: 100, NextFollower: actor.Next}
					if w.Occupants[actor.X+actor.Y*MapSize] == 0 {
						w.Occupants[actor.X+actor.Y*MapSize] = uint16(actor.Slot)
					}
				}
			}
			// Build canonical mixed actor order independently of follower-only heads.
			for i := len(test.Actors) - 1; i >= 0; i-- {
				actor := test.Actors[i]
				if actor.Kind == 24 {
					w.Actors.Link(ActorRef{Kind: ActorScenery, Index: 0}, actor.X*256+128, actor.Y*256+128)
				} else {
					w.Actors.Link(ActorRef{Kind: ActorFollower, Index: uint16(actor.Slot)}, actor.X*256+128, actor.Y*256+128)
				}
			}
			id := 1
			w.Followers[id] = Follower{Owner: 0, X: 32, Y: 32, State: Walking, Population: 1000, MovementSpeed: uint8(test.Speed), Search: test.Search, velocityX: test.VX, velocityY: test.VY}
			w.Occupants[32+32*MapSize] = 1
			switch test.Mode {
			case 18:
				w.Players[0].Mode = Join
			case 20:
				w.Players[0].Mode = Fight
			default:
				w.Players[0].Mode = Settle
			}
			x, y, ok := w.chooseMove(id)
			if ok != (test.Route != "none") || ok && (x != test.TargetX || y != test.TargetY) || uint32(w.random) != test.RNG {
				t.Fatalf("target%d,%d ok%v rng%x source%d,%d/%s/%x", x, y, ok, w.random, test.TargetX, test.TargetY, test.Route, test.RNG)
			}
			if ok {
				f := &w.Followers[id]
				if f.RoadLeg {
					f.MovementSpeed = uint8(min(255, int(f.MovementSpeed)+20))
					w.beginSearchLeg(id, x, y)
					f.MovementSpeed -= 20
				} else {
					w.beginSearchLeg(id, x, y)
				}
				if int(f.MovementSpeed) != test.ResultSpeed || f.velocityX != test.ResultVX || f.velocityY != test.ResultVY || f.legRemaining != test.Timer {
					t.Fatalf("motion speed%d vx/y%d,%d timer%d source%d/%d,%d/%d", f.MovementSpeed, f.velocityX, f.velocityY, f.legRemaining, test.ResultSpeed, test.ResultVX, test.ResultVY, test.Timer)
				}
			}

		})
	}
}
