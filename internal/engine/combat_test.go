package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestCombatUsesOneAggressorQuotientForBothLosses(t *testing.T) {
	w := testFlatWorld()
	a := addFollower(w, 20, 20, 0, 1000, Walking)
	d := addFollower(w, 21, 20, 1, 500, Walking)
	w.Followers[a].Weapons = 3
	w.Followers[d].Weapons = 7
	w.beginBattle(a, d)
	beforeRandom := w.random
	w.stepBattle(a)
	if w.Followers[a].Population != 920 || w.Followers[d].Population != 460 {
		t.Fatalf("source damage differs: %d/%d", w.Followers[a].Population, w.Followers[d].Population)
	}
	if w.random == beforeRandom {
		t.Fatal("combat did not advance shared RNG")
	}
	aPopulation, dPopulation := w.Followers[a].Population, w.Followers[d].Population
	w.stepBattle(d)
	if w.Followers[a].Population != aPopulation || w.Followers[d].Population != dPopulation {
		t.Fatal("defender incorrectly applied damage a second time")
	}
}

func TestMutualCombatDeathDoesNotAwardBattle(t *testing.T) {
	w := testFlatWorld()
	a := addFollower(w, 20, 20, 0, 10, Walking)
	d := addFollower(w, 21, 20, 1, 10, Walking)
	w.beginBattle(a, d)
	w.stepBattle(a)
	if w.Followers[a].State != Inactive || w.Followers[d].State != Inactive || w.Players[0].BattlesWon != 0 || w.Players[1].BattlesWon != 0 {
		t.Fatal("mutual death winner or stale follower")
	}
}

func TestBattleNumericalReferenceCases(t *testing.T) {
	// Expected losses are scalar results of the original aggressor routine,
	// including its word-quotient overflow boundary, with weapons three/seven.
	for _, test := range []struct{ population, remaining int }{
		{1000, 920}, {6553600, 6553590}, {2147483647, 2147024892},
	} {
		w := testFlatWorld()
		a := addFollower(w, 20, 20, 0, test.population, Walking)
		d := addFollower(w, 21, 20, 1, 1000000, Walking)
		w.Followers[a].Weapons = 3
		w.Followers[d].Weapons = 7
		w.beginBattle(a, d)
		w.stepBattle(a)
		if got := w.Followers[a].Population; got != test.remaining {
			t.Fatalf("population %d: %d", test.population, got)
		}
	}
}

func TestBattleRewardTransfersLedgerAndRetainsDefeatedActor(t *testing.T) {
	w := testFlatWorld()
	a := addFollower(w, 20, 20, 0, 1000, Walking)
	d := addFollower(w, 21, 20, 1, 1, Walking)
	w.Landscape.Parameters = [3]int{100, 200, 300}
	w.Players[0].Mana = 1000
	w.Players[1].Mana = 1000
	w.Players[1].Leader = d
	w.Followers[d].Hero.Kind = HeroPerseus
	w.beginBattle(a, d)
	w.finishBattle(a, d)
	if w.Players[0].Mana != 1600 || w.Players[1].Mana != 400 || w.Players[1].Statistics.LeaderLosses != 1 {
		t.Fatal("battle leader+hero ledger reward")
	}
	if w.Followers[d].State != Ruin || w.Followers[d].CombatAftermath.Frames != 6 || w.Occupants[21+20*MapSize] != uint16(d) {
		t.Fatal("defeated hero was not retained")
	}
	for range 5 {
		w.advanceCombatAftermath(d)
	}
	if w.Followers[d].State == Inactive {
		t.Fatal("hero death ended early")
	}
	w.advanceCombatAftermath(d)
	if w.Followers[d].State != Inactive {
		t.Fatal("hero death cleanup")
	}
}

func TestHeroOnlyRewardUsesFirstConditionalBonus(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 1, 100, Walking)
	w.Followers[id].Hero.Kind = HeroPerseus
	w.Landscape.Parameters = [3]int{100, 200, 300}
	if reward := w.battleReward(id); reward != 300 {
		t.Fatalf("hero-only reward%d", reward)
	}
	w.Followers[id].BattleWasTown = true
	w.Followers[id].Stage = 18
	w.Landscape.Weapons[18] = 500
	if reward := w.battleReward(id); reward != 800 {
		t.Fatal("town reward addition")
	}
}

func TestOrdinaryVictoryRecoversForFourteenFramesBeforeSearching(t *testing.T) {
	w := testFlatWorld()
	a := addFollower(w, 20, 20, 0, 1000, Walking)
	d := addFollower(w, 21, 20, 1, 1, Walking)
	w.beginBattle(a, d)
	w.finishBattle(a, d)
	if w.Followers[a].CombatAftermath.Kind != CombatVictorious || w.Followers[d].CombatAftermath.Frames != 12 {
		t.Fatal("ordinary aftermath sequences")
	}
	for range 13 {
		w.advanceCombatAftermath(a)
	}
	if w.Followers[a].CombatAftermath.Kind == CombatAftermathNone {
		t.Fatal("victory recovery ended early")
	}
	w.advanceCombatAftermath(a)
	if w.Followers[a].State != Walking || w.Followers[a].CombatAftermath.Kind != CombatAftermathNone {
		t.Fatal("victory recovery did not defer ordinary search")
	}
}

func TestBattleRewardsOriginalNumericalCorpus(t *testing.T) {
	data, err := os.ReadFile("testdata/battle_reward_numbers.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Name             string
			Land             int
			WinnerOwner      int      `json:"winner_owner"`
			WinnerHero       HeroKind `json:"winner_hero"`
			WinnerTown       bool     `json:"winner_town"`
			WinnerStage      int      `json:"winner_stage"`
			WinnerPopulation int      `json:"winner_population"`
			LoserHero        HeroKind `json:"loser_hero"`
			LoserLeader      bool     `json:"loser_leader"`
			LoserTown        bool     `json:"loser_town"`
			LoserStage       int      `json:"loser_stage"`
			WinnerMana       uint32   `json:"winner_mana"`
			LoserMana        uint32   `json:"loser_mana"`
			ResultWinnerMana uint32   `json:"result_winner_mana"`
			ResultLoserMana  uint32   `json:"result_loser_mana"`
			Blocked, Full    bool
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 87 {
		t.Fatal("battle reward corpus incomplete")
	}
	for _, test := range corpus.Cases {
		t.Run(test.Name, func(t *testing.T) {
			land, err := os.ReadFile(fmt.Sprintf("../../assets/amiga/land%d.dat", test.Land))
			if os.IsNotExist(err) {
				t.Skip("original landscape assets have not been imported")
			}
			if err != nil {
				t.Fatal(err)
			}
			w := testFlatWorld()
			w.Landscape, err = DecodeLandscape(land)
			if err != nil {
				t.Fatal(err)
			}
			owner := test.WinnerOwner
			enemy := owner ^ 1
			w.Followers[1] = Follower{Owner: uint8(owner), X: 32, Y: 32, State: Fighting, Population: test.WinnerPopulation, Hero: HeroState{Kind: test.WinnerHero}, BattleWasTown: test.WinnerTown, Stage: uint8(test.WinnerStage)}
			w.Followers[2] = Follower{Owner: uint8(enemy), X: 32, Y: 32, State: Fighting, Hero: HeroState{Kind: test.LoserHero}, BattleWasTown: test.LoserTown, Stage: uint8(test.LoserStage)}
			w.Players[owner].Mana = int(test.WinnerMana)
			w.Players[enemy].Mana = int(test.LoserMana)
			if test.LoserLeader {
				w.Players[enemy].Leader = 2
			}
			w.BirthBlocked = test.Blocked
			if test.Full {
				for id := 3; id < FollowerCapacity; id++ {
					w.Followers[id] = Follower{Owner: 0, State: Walking, Population: 1}
				}
			}
			w.finishBattle(1, 2)
			if uint32(w.Players[owner].Mana) != test.ResultWinnerMana || uint32(w.Players[enemy].Mana) != test.ResultLoserMana {
				t.Fatalf("mana%x/%x source%x/%x", w.Players[owner].Mana, w.Players[enemy].Mana, test.ResultWinnerMana, test.ResultLoserMana)
			}
		})
	}
}

func TestConqueredTownReformsWinnerAtCentreAndKeepsWorkCounter(t *testing.T) {
	w := testFlatWorld()
	a := addFollower(w, 20, 20, 0, 1000, Walking)
	d := addFollower(w, 20, 20, 1, 1, Town)
	w.Followers[a].Work = 7
	w.Followers[a].positionX = 20*256 + 173
	w.Followers[a].positionY = 20*256 + 141
	w.Followers[a].positionSet = true
	w.Tick = 123
	w.beginBattle(a, d)
	w.finishBattle(a, d)
	f := w.Followers[a]
	if f.State != Town || f.positionX != 20*256+128 || f.positionY != 20*256+128 || f.Work != 7 || f.FoundedAt != 123 || f.Stage != 18 {
		t.Fatalf("reformed town %+v", f)
	}
}
