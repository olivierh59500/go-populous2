package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestAIUsesCampaignReactionAndDefersTerrainCommands(t *testing.T) {
	w := testFlatWorld()
	id := w.allocate(Follower{Owner: 1, X: 20, Y: 20, State: Town, Population: 100, Stage: 9})
	w.linkFollower(id)
	w.Players[1].Computer = true
	w.Players[1].Mana = 10000
	w.Level.Players[1].ReactionDelay = 3
	w.AI[1].ExpansionTown = id
	w.AI[1].Reaction = 2
	w.Heights[20+19*CornerSize] = 0
	before := w.Heights
	w.thinkAI(1)
	if w.AI[1].Order.Kind != AINoOrder {
		t.Fatal("AI acted before reaction expiry")
	}
	w.thinkAI(1)
	if w.AI[1].Order.Kind != AIRaise || w.AI[1].Order.X != 20 || w.AI[1].Order.Y != 19 || w.AI[1].Reaction != 3 {
		t.Fatal("campaign reaction or original expansion order")
	}
	if w.Heights != before {
		t.Fatal("AI changed terrain during thinking")
	}
	w.executeAIOrders()
	if w.Heights[20+19*CornerSize] != 1 || w.AI[1].Order.Kind != AINoOrder {
		t.Fatal("deferred AI terrain order did not execute")
	}
}

func TestAIExpansionSkipsFriendlyWalkingOccupants(t *testing.T) {
	w := testFlatWorld()
	town := w.allocate(Follower{Owner: 1, X: 20, Y: 20, State: Town, Population: 100, Stage: 9})
	w.linkFollower(town)
	walker := w.allocate(Follower{Owner: 1, X: 20, Y: 19, State: Walking, Population: 100})
	w.linkFollower(walker)
	w.Heights[20+19*CornerSize] = 0
	w.Heights[21+19*CornerSize] = 0
	w.Players[1].Mana = 10000
	w.AI[1].ExpansionTown = town
	if !w.chooseAIExpansion(1) || w.AI[1].Order.X != 21 || w.AI[1].Order.Y != 19 {
		t.Fatal("AI expansion disturbed friendly walking group")
	}
}

func TestAIReleaseUsesSeparateCooldownAndPopulationThreshold(t *testing.T) {
	w := testFlatWorld()
	town := w.allocate(Follower{Owner: 1, X: 20, Y: 20, State: Town, Population: 500})
	w.linkFollower(town)
	w.Players[1].Mana = 1000
	w.AI[1].BestTown = town
	if !w.chooseAIRelease(1) || w.AI[1].Order.Kind != AIReleaseTown || w.AI[1].ReleaseCooldown != 4 {
		t.Fatal("AI release policy")
	}
	w.AI[1].Order = AIOrder{}
	if w.chooseAIRelease(1) {
		t.Fatal("AI release ignored its cooldown")
	}
}

func TestAIObservationsSelectLastChangedTownAndSignedPopulation(t *testing.T) {
	w := testFlatWorld()
	a := w.allocate(Follower{Owner: 1, X: 20, Y: 20, State: Town, Population: 100, Stage: 9})
	b := w.allocate(Follower{Owner: 1, X: 22, Y: 20, State: Town, Population: 200, Stage: 10})
	w.observeAITown(a, 0)
	w.observeAITown(b, 0)
	if w.AI[1].ExpansionTown != b || w.AI[1].BestTown != b {
		t.Fatal("AI town observations do not preserve pool order")
	}
	w.Followers[b].Population = 65535
	w.beginAIObservations()
	w.observeAITown(a, 0)
	w.observeAITown(b, 0)
	if w.AI[1].BestTown != a {
		t.Fatal("AI strongest town comparison lost signed low-word rule")
	}
}

func TestAILandPoliciesOriginalNumericalCorpus(t *testing.T) {
	data, err := os.ReadFile("testdata/ai_land_numbers.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Name, Mode           string
			Owner                int
			Seed                 uint32
			Mana, Cooldown, Town int
			BestPopulation       int `json:"best_population"`
			Population, Stage    int
			Rally                bool
			Cells                []struct{ Index, Base, Code int }
			Chosen               bool
			Kind, X, Y           int
			RNG                  uint32
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 624 {
		t.Fatal("AI land corpus incomplete")
	}
	for _, test := range corpus.Cases {
		t.Run(test.Name, func(t *testing.T) {
			w := testFlatWorld()
			w.random = randomState(test.Seed)
			w.Players[test.Owner].Mana = test.Mana
			if test.Rally {
				w.Players[test.Owner].Mode = Rally
			}
			w.Followers[1] = Follower{Owner: uint8(test.Owner), X: 32, Y: 32, State: Town, Stage: uint8(test.Stage), Population: test.Population}
			w.Occupants[32+32*MapSize] = 1
			for _, cell := range test.Cells {
				w.Tiles[cell.Index].Code = uint8(cell.Code)
				w.Tiles[cell.Index].Shape = uint8(cell.Code & 15)
				w.Tiles[cell.Index].BaseAltitude = uint8(cell.Base)
				shape := cell.Code & 15
				if cell.Code >= 143 && cell.Code <= 151 || cell.Code >= 168 && cell.Code <= 171 {
					shape = 15
				}
				if cell.Code >= 152 && cell.Code <= 167 {
					shape = 0
				}
				w.Heights[cell.Index%MapSize+cell.Index/MapSize*CornerSize] = uint8(cell.Base) + uint8(shape&1)
			}
			a := &w.AI[test.Owner]
			var chosen bool
			if test.Mode == "expand" {
				a.ExpansionTown = test.Town
				a.ExpansionCooldown = test.Cooldown
				chosen = w.chooseAIExpansion(test.Owner)
			} else {
				a.BestTown = test.Town
				a.BestPopulation = test.BestPopulation
				a.ReleaseCooldown = test.Cooldown
				chosen = w.chooseAIRelease(test.Owner)
			}
			if chosen != test.Chosen {
				t.Fatalf("chosen%v source%v", chosen, test.Chosen)
			}
			if chosen && test.Mode == "expand" {
				kind := 2
				if a.Order.Kind == AILower {
					kind = 4
				}
				if kind != test.Kind || a.Order.X != test.X || a.Order.Y != test.Y {
					t.Fatalf("order%d %d,%d source%d %d,%d", kind, a.Order.X, a.Order.Y, test.Kind, test.X, test.Y)
				}
			}
			if uint32(w.random) != test.RNG {
				t.Fatal("random draws differ")
			}
		})
	}
}
