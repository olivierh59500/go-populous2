package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestAICompileKeepsOriginalWeightedOrderAndLeaderChoices(t *testing.T) {
	w := testFlatWorld()
	w.Level.Players[1].Powers[Swamp] = true
	w.Level.Players[1].Powers[Storm] = true
	w.Level.Players[1].Powers[Adonis] = true
	w.compileAIPowers(1)
	a := w.AI[1]
	if a.ChoiceCount != 8 || a.LeaderChoiceCount != 2 {
		t.Fatalf("counts%d/%d", a.ChoiceCount, a.LeaderChoiceCount)
	}
	for i := 1; i <= 4; i++ {
		if a.Choices[i].Power != Swamp {
			t.Fatal("swamp weighting")
		}
	}
	for i := 5; i <= 7; i++ {
		if a.Choices[i].Power != Storm {
			t.Fatal("storm weighting")
		}
	}
	if a.Choices[8].Power != Adonis || a.Choices[9].Power != Adonis {
		t.Fatal("Adonis leader weighting")
	}
}

func TestAIOffensiveRetainsUnaffordableChoiceAndUsesStrictBasePrice(t *testing.T) {
	w := testFlatWorld()
	town := addFollower(w, 20, 20, 0, 1000, Town)
	w.AI[0].BestTown = town
	w.Tick = 250
	a := &w.AI[1]
	a.ChoiceCount = 2
	a.Choices[1] = AIPowerChoice{Power: Swamp, Target: AITargetTown}
	a.ChoiceIndex = 1
	w.Players[1].Mana = 7000
	if w.chooseAIOffensive(1) || a.ChoiceIndex != 1 {
		t.Fatal("equal base price incorrectly affordable")
	}
	w.Players[1].Mana = 7001
	if !w.chooseAIOffensive(1) || a.ChoiceIndex != 0 || a.Order.Power != Swamp || a.Order.Target.X != 20 || a.Order.Target.Y != 20 {
		t.Fatal("affordable saved choice was not queued")
	}
}

func TestAIPreparedPowerRunsUrgentOnFollowingPolicyPass(t *testing.T) {
	w := testFlatWorld()
	town := addFollower(w, 20, 20, 0, 1000, Town)
	w.AI[0].BestTown = town
	w.Tick = 250
	w.Players[1].Mana = 50000
	a := &w.AI[1]
	a.ChoiceCount = 2
	a.Choices[1] = AIPowerChoice{Power: Batholith, Target: AITargetPrepared}
	a.ChoiceIndex = 1
	if w.chooseAIOffensive(1) || !a.Prepared || a.Order.Kind != AINoOrder {
		t.Fatal("prepared power executed in offensive phase")
	}
	if !w.chooseAIUrgent(1) || a.Order.Power != Batholith || a.Order.Target.X != 20 {
		t.Fatal("urgent policy did not emit prepared power")
	}
}

func TestAIFinalBattleHonoursDeadlineAndPopulationAdvantage(t *testing.T) {
	w := testFlatWorld()
	town := addFollower(w, 20, 20, 0, 1000, Town)
	w.AI[0].BestTown = town
	w.Tick = 250
	w.Level.Players[1].ArmageddonDeadline = 10
	w.Players[1].Mana = 1000000
	w.Players[0].Population = 1000
	w.Players[1].Population = 2000
	a := &w.AI[1]
	a.ChoiceCount = 2
	a.Choices[1] = AIPowerChoice{Power: Armageddon, Target: AITargetFinalBattle}
	a.ChoiceIndex = 1
	if !w.chooseAIOffensive(1) {
		t.Fatal("eligible final battle")
	}
	a.Order = AIOrder{}
	a.ChoiceIndex = 1
	w.Tick = 641
	if w.chooseAIOffensive(1) || a.ChoiceIndex != 0 {
		t.Fatal("final-battle deadline")
	}
}

func TestAIMagnetSwitchUsesPopulationLowBitsAndThirtyTownBoundary(t *testing.T) {
	for _, test := range []struct {
		population int
		mode       Mode
	}{{1000, Settle}, {1002, Join}, {1003, Fight}} {
		w := testFlatWorld()
		w.Players[1].Towns = 30
		w.Players[1].Mode = Rally
		w.Players[1].Population = test.population
		if !w.chooseAIMagnet(1) || w.AI[1].Order.Mode != test.mode || w.AI[1].MagnetCooldown != 10 {
			t.Fatal("small-town magnet tactical switch")
		}
	}
	w := testFlatWorld()
	w.Players[1].Towns = 31
	w.Players[1].Mode = Settle
	if !w.chooseAIMagnet(1) || w.AI[1].Order.Mode != Rally || w.AI[1].MagnetCooldown != 100 {
		t.Fatal("missing leader did not request rally")
	}
}

func TestAIOffensiveOriginalNumericalCorpus(t *testing.T) {
	data, err := os.ReadFile("testdata/ai_offensive_numbers.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Name             string
			Owner            int
			Seed             uint32
			Tick             uint64
			Mana, Population int
			EnemyPopulation  int  `json:"enemy_population"`
			EnemyPresent     bool `json:"enemy_present"`
			EnemyX           int  `json:"enemy_x"`
			EnemyY           int  `json:"enemy_y"`
			Leader           int
			LeaderPopulation int `json:"leader_population"`
			Deadline, Choice int
			ChoiceCount      int `json:"choice_count"`
			LeaderCount      int `json:"leader_count"`
			Catalog          []AIPowerChoice
			Chosen           bool
			ResultChoice     int     `json:"result_choice"`
			ResultPower      PowerID `json:"result_power"`
			ResultX          int     `json:"result_x"`
			ResultY          int     `json:"result_y"`
			RNG              uint32
			Prepared         bool
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 216 {
		t.Fatal("offensive AI corpus incomplete")
	}
	for _, test := range corpus.Cases {
		t.Run(test.Name, func(t *testing.T) {
			w := testFlatWorld()
			w.Tick = test.Tick
			w.random = randomState(test.Seed)
			owner, enemy := test.Owner, test.Owner^1
			w.Players[owner].Mana = test.Mana
			w.Players[owner].Population = test.Population
			w.Players[enemy].Population = test.EnemyPopulation
			w.Players[owner].Leader = test.Leader
			w.Level.Players[owner].ArmageddonDeadline = test.Deadline
			w.Followers[1] = Follower{Owner: uint8(owner), X: 32, Y: 32, State: Walking, Population: test.LeaderPopulation}
			w.Followers[2] = Follower{Owner: uint8(enemy), X: uint8(test.EnemyX), Y: uint8(test.EnemyY), State: Town, Population: 100}
			if test.EnemyPresent {
				w.AI[enemy].BestTown = 2
			}
			a := &w.AI[owner]
			a.ChoiceCount = test.ChoiceCount
			a.LeaderChoiceCount = test.LeaderCount
			a.ChoiceIndex = test.Choice
			copy(a.Choices[:], test.Catalog)
			got := w.chooseAIOffensive(owner)
			if got != test.Chosen || a.ChoiceIndex != test.ResultChoice || uint32(w.random) != test.RNG || a.Prepared != test.Prepared {
				t.Fatalf("chosen%v choice%d rng%x prepared%v source%v/%d/%x/%v", got, a.ChoiceIndex, w.random, a.Prepared, test.Chosen, test.ResultChoice, test.RNG, test.Prepared)
			}
			if got && a.Order.Power != test.ResultPower {
				t.Fatalf("power%d source%d", a.Order.Power, test.ResultPower)
			}
			if got && a.Choices[test.Choice].Target != AITargetLeader && (a.Order.Target.X != test.ResultX || a.Order.Target.Y != test.ResultY) {
				t.Fatalf("target%d,%d source%d,%d", a.Order.Target.X, a.Order.Target.Y, test.ResultX, test.ResultY)
			}

		})
	}
}

func TestAIMagnetOriginalNumericalCorpus(t *testing.T) {
	data, err := os.ReadFile("testdata/ai_magnet_numbers.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Name                         string
			Owner                        int
			Seed                         uint32
			Towns, Mode, Leader          int
			LeaderPopulation             int `json:"leader_population"`
			Population, Cooldown, Choice int
			FirstCount                   int  `json:"first_count"`
			LeaderCount                  int  `json:"leader_count"`
			MarkerX                      int  `json:"marker_x"`
			MarkerY                      int  `json:"marker_y"`
			TargetX                      int  `json:"target_x"`
			TargetY                      int  `json:"target_y"`
			EnemyAvailable               bool `json:"enemy_available"`
			Chosen                       bool
			ResultKind                   int `json:"result_kind"`
			ResultChoice                 int `json:"result_choice"`
			ResultCooldown               int `json:"result_cooldown"`
			RNG                          uint32
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 768 {
		t.Fatal("magnet AI corpus incomplete")
	}
	for _, test := range corpus.Cases {
		t.Run(test.Name, func(t *testing.T) {
			w := testFlatWorld()
			w.random = randomState(test.Seed)
			p := &w.Players[test.Owner]
			p.Towns = test.Towns
			p.Population = test.Population
			p.Leader = test.Leader
			p.RallyX, p.RallyY = test.MarkerX, test.MarkerY
			switch test.Mode {
			case 14:
				p.Mode = Settle
			case 16:
				p.Mode = Rally
			case 18:
				p.Mode = Join
			case 20:
				p.Mode = Fight
			}
			w.Followers[1] = Follower{Owner: uint8(test.Owner), X: 32, Y: 32, State: Walking, Population: test.LeaderPopulation}
			w.Followers[2] = Follower{Owner: uint8(test.Owner), X: uint8(test.TargetX), Y: uint8(test.TargetY), State: Town, Population: 100, FoundedAt: 1}
			if test.EnemyAvailable {
				w.Followers[2].Owner = uint8(test.Owner ^ 1)
				w.Followers[3] = Follower{Owner: uint8(test.Owner), X: uint8(test.TargetX), Y: uint8(test.TargetY), State: Town, Population: 100, FoundedAt: 1}
			}
			a := &w.AI[test.Owner]
			a.MagnetCooldown = test.Cooldown
			a.ChoiceIndex = test.Choice
			a.ChoiceCount = test.FirstCount
			a.LeaderChoiceCount = test.LeaderCount
			got := w.chooseAIMagnet(test.Owner)
			if got != test.Chosen || a.ChoiceIndex != test.ResultChoice || a.MagnetCooldown != test.ResultCooldown || uint32(w.random) != test.RNG {
				t.Fatalf("chosen%v choice%d cooldown%d rng%x source%v/%d/%d/%x", got, a.ChoiceIndex, a.MagnetCooldown, w.random, test.Chosen, test.ResultChoice, test.ResultCooldown, test.RNG)
			}
		})
	}
}
