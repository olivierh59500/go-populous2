package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestAllHeroConversionsPreserveMotionAndPopulationRules(t *testing.T) {
	for kind := HeroPerseus; kind <= HeroHelen; kind++ {
		for _, speed := range []uint8{0, 20, 240, 255} {
			w := testFlatWorld()
			id := addFollower(w, 20, 20, 0, 1001, Walking)
			f := &w.Followers[id]
			f.MovementSpeed = speed
			f.positionX = 20*256 + 135
			f.positionY = 20*256 + 147
			f.velocityX = 13
			f.velocityY = -13
			f.legRemaining = 7
			f.Target = 135
			f.Work = 11
			f.Weapons = 4
			f.Search = 18
			w.Players[0].Leader = id
			w.Players[0].Experience[int(kind)-1] = 64
			beforeRandom := w.random
			got, err := w.CreateHero(0, kind)
			if err != nil || got != id {
				t.Fatalf("kind %d: %v", kind, err)
			}
			if !f.IsHero() || f.Hero.Kind != kind || f.Frame != 0 || w.Players[0].Leader != 0 || w.random != beforeRandom {
				t.Fatal("conversion state or RNG")
			}
			population := 1001
			if kind == HeroHeracles {
				population *= 2
			}
			expectedSpeed := int(speed) + 8
			if kind == HeroOdysseus {
				expectedSpeed += int(speed)
			}
			if f.Population != population || int(f.MovementSpeed) != min(255, expectedSpeed) {
				t.Fatal("hero population/speed")
			}
			if f.positionX != 20*256+135 || f.positionY != 20*256+147 || f.velocityX != 13 || f.velocityY != -13 || f.legRemaining != 7 || f.Target != 135 || f.Work != 11 || f.Weapons != 4 || f.Search != 18 {
				t.Fatal("conversion destroyed motion or unrelated fields")
			}
		}
	}
}

func TestHeraclesPopulationRetainsSignedLongOverflow(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 2147483647, Walking)
	if err := w.ConvertHero(id, HeroHeracles); err != nil {
		t.Fatal(err)
	}
	if w.Followers[id].Population != -2 {
		t.Fatal("Heracles population overflow was clamped instead of retained")
	}
}

func TestHeroElementalImmunitiesAndNamedDeathLengths(t *testing.T) {
	for kind := HeroPerseus; kind <= HeroHelen; kind++ {
		f := Follower{Hero: HeroState{Kind: kind}}
		if f.ImmuneToSwamp() != (kind == HeroAdonis) || f.ImmuneToFungus() != (kind == HeroAdonis) || f.ImmuneToBurning() != (kind == HeroAchilles) || f.ImmuneToDrowning() != (kind == HeroHelen) || f.ImmuneToFatalGround() != (kind == HeroHeracles) {
			t.Fatal("hero elemental immunity")
		}
		if f.HazardDeathFrames(HazardSwamp) != [6]int{5, 0, 9, 9, 8, 3}[int(kind)-1] {
			t.Fatal("hero death length")
		}
	}
	if (Follower{}).HazardDeathFrames(HazardFungus) != 2 {
		t.Fatal("ordinary death length")
	}
}

func TestHeroTargetClaimsAndLaterSlotDistanceTie(t *testing.T) {
	w := testFlatWorld()
	hero := addFollower(w, 20, 20, 0, 100, Walking)
	first := addFollower(w, 19, 20, 1, 100, Walking)
	second := addFollower(w, 21, 20, 1, 100, Walking)
	w.Followers[hero].Hero.Kind = HeroPerseus
	if got := w.SelectHeroTarget(hero); got != second || w.Followers[second].Hero.ClaimedBy != hero {
		t.Fatal("hero distance tie or reciprocal claim")
	}
	w.Followers[first].Hero.CaptiveOf = hero
	w.Followers[second].Hero.ClaimedBy = hero
	if got := w.SelectHeroTarget(hero); got != second {
		t.Fatal("all-claimed fallback must use the last eligible slot")
	}
}

func TestMissingHeroLeaderDoesNotChangeWorld(t *testing.T) {
	w := testFlatWorld()
	before := *w
	if _, err := w.CreateHero(0, HeroPerseus); err == nil || !reflect.DeepEqual(*w, before) {
		t.Fatal("missing leader conversion changed world")
	}
}

func TestHeroCreationOriginalNumericalCorpus(t *testing.T) {
	data, err := os.ReadFile("testdata/hero_creation_numbers.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Name                     string
			Kind                     HeroKind
			Owner                    int
			Town                     bool
			Speed, Experience        uint8
			Population               uint32
			Missing, Direct, Created bool
			ResultPopulation         int   `json:"result_population"`
			ResultSpeed              uint8 `json:"result_speed"`
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 1104 {
		t.Fatal("hero creation numerical corpus incomplete")
	}
	for _, test := range corpus.Cases {
		t.Run(test.Name, func(t *testing.T) {
			w := testFlatWorld()
			state := Walking
			if test.Town {
				state = Town
			}
			id := addFollower(w, 32, 33, test.Owner, int(int32(test.Population)), state)
			w.Followers[id].MovementSpeed = test.Speed
			w.Players[test.Owner].Experience[int(test.Kind)-1] = test.Experience
			if !test.Missing {
				w.Players[test.Owner].Leader = id
			}
			var err error
			if test.Direct {
				err = w.ConvertHero(id, test.Kind)
			} else {
				_, err = w.CreateHero(test.Owner, test.Kind)
			}
			if (err == nil) != test.Created {
				t.Fatal("creation admission differs")
			}
			f := w.Followers[id]
			if f.Population != test.ResultPopulation || f.MovementSpeed != test.ResultSpeed {
				t.Fatalf("result %d/%d; source %d/%d", f.Population, f.MovementSpeed, test.ResultPopulation, test.ResultSpeed)
			}
		})
	}
}

func TestHeroSelectionThenPursuitRetainsSourcePassBoundary(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 1000, Walking)
	enemy := addFollower(w, 24, 20, 1, 100, Town)
	w.Followers[id].Hero.Kind = HeroPerseus
	w.Level.Players[0].Attrition = 3
	beforeRandom := w.random
	w.stepFollower(id)
	f := &w.Followers[id]
	if f.Hero.Target != enemy || f.Hero.Phase != HeroPursuing || f.Population != 1000 || f.moving {
		t.Fatal("target selection did not end its pass")
	}
	w.stepFollower(id)
	if f.Population != 997 || f.positionX != 20*256+148 || f.positionY != 20*256+128 || f.legRemaining != 11 || !f.moving || w.random != beforeRandom {
		t.Fatalf("first pursuit differs: %+v", f)
	}
}

func TestHeroPlannerUsesOrderedAlternativeAndExcludesReverse(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 1000, Walking)
	enemy := addFollower(w, 24, 20, 1, 100, Town)
	w.Followers[id].Hero = HeroState{Kind: HeroPerseus, Phase: HeroPursuing, Target: enemy}
	w.Tiles[21+20*MapSize] = Cell{}
	w.stepHero(id)
	if f := w.Followers[id]; f.velocityX != 20 || f.velocityY != 20 {
		t.Fatalf("east fallback did not prefer southeast: %d,%d", f.velocityX, f.velocityY)
	}
	// With all forward alternatives blocked, source consumes a centered leg
	// with zero velocity instead of reversing into the previous direction.
	w = testFlatWorld()
	id = addFollower(w, 20, 20, 0, 1000, Walking)
	enemy = addFollower(w, 24, 20, 1, 100, Town)
	w.Followers[id].Hero = HeroState{Kind: HeroPerseus, Phase: HeroPursuing, Target: enemy}
	for _, d := range directions {
		if d == [2]int{-1, 0} {
			continue
		}
		w.Tiles[20+d[0]+(20+d[1])*MapSize] = Cell{}
	}
	w.stepHero(id)
	if f := w.Followers[id]; f.velocityX != 0 || f.velocityY != 0 || !f.moving || f.legRemaining != 11 {
		t.Fatal("blocked planner reversed or discarded source zero-velocity leg")
	}
}

func TestHeroNoEnemyWaitsTwentyPassesAndRetargets(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 1000, Walking)
	w.Followers[id].Hero = HeroState{Kind: HeroPerseus, Phase: HeroPursuing}
	w.stepHero(id)
	if w.Followers[id].Hero.Wait != 20 || w.Followers[id].Hero.Phase != HeroWaiting {
		t.Fatal("no-target wait")
	}
	enemy := addFollower(w, 24, 20, 1, 100, Town)
	for range 20 {
		w.stepHero(id)
	}
	if w.Followers[id].Hero.Phase != HeroWaiting {
		t.Fatal("wait expired early")
	}
	w.stepHero(id)
	if w.Followers[id].Hero.Target != enemy || w.Followers[id].Hero.Phase != HeroPursuing {
		t.Fatal("wait did not redispatch selection")
	}
}

func TestHeroDeathClearsReciprocalClaimsBeforePoolReuse(t *testing.T) {
	w := testFlatWorld()
	hero := addFollower(w, 20, 20, 0, 100, Walking)
	enemy := addFollower(w, 22, 20, 1, 100, Town)
	w.Followers[hero].Hero.Kind = HeroPerseus
	w.SelectHeroTarget(hero)
	w.remove(enemy)
	if w.Followers[hero].Hero.Target != 0 {
		t.Fatal("dead target retained hero claim")
	}
	next := addFollower(w, 30, 30, 1, 100, Walking)
	if next != enemy || w.Followers[next].Hero.ClaimedBy != 0 {
		t.Fatal("pool reuse inherited target claim")
	}
	w.SelectHeroTarget(hero)
	w.remove(hero)
	if w.Followers[next].Hero.ClaimedBy != 0 {
		t.Fatal("dead hero retained enemy backlink")
	}
}

func TestHeroReachesEnemyAndBecomesAggressor(t *testing.T) {
	w := testFlatWorld()
	hero := addFollower(w, 20, 20, 0, 1000, Walking)
	enemy := addFollower(w, 22, 20, 1, 100, Town)
	w.Followers[hero].Hero.Kind = HeroHeracles
	for pass := 0; pass < 40 && w.Followers[hero].State != Fighting; pass++ {
		w.stepFollower(hero)
	}
	if w.Followers[hero].State != Fighting || !w.Followers[hero].BattleAggressor || w.Followers[hero].BattleWith != enemy {
		t.Fatal("hero did not contact its enemy")
	}
	if w.Followers[enemy].Hero.ClaimedBy != 0 || w.Followers[hero].Hero.Target != 0 {
		t.Fatal("contact retained chase claim")
	}
}

func TestHelenCapturesSeveralEnemiesWithoutChangingFaith(t *testing.T) {
	w := testFlatWorld()
	hero := addFollower(w, 20, 20, 0, 100, Walking)
	a := addFollower(w, 21, 20, 1, 100, Town)
	b := addFollower(w, 22, 20, 1, 200, Walking)
	w.Followers[hero].Hero.Kind = HeroHelen
	if !w.CaptureByHelen(hero, a) || !w.CaptureByHelen(hero, b) {
		t.Fatal("capture failed")
	}
	for _, id := range []int{a, b} {
		f := w.Followers[id]
		if f.Owner != 1 || f.Hero.CaptiveOf != hero || f.State == Town {
			t.Fatal("capture converted faith or released earlier captive")
		}
	}
	if w.Followers[a].Population != 100 || w.Followers[b].Population != 200 {
		t.Fatal("capture changed population")
	}
	w.remove(hero)
	for _, id := range []int{a, b} {
		f := w.Followers[id]
		if f.Owner != 1 || f.Hero.CaptiveOf != 0 || f.State != Walking {
			t.Fatal("Helen death did not release captives")
		}
	}
}

func TestAdonisSplitHalvesOddPopulationAndKeepsTypedMotion(t *testing.T) {
	w := testFlatWorld()
	hero := addFollower(w, 20, 20, 0, 101, Walking)
	w.Followers[hero].Hero.Kind = HeroAdonis
	w.Followers[hero].velocityX = 7
	w.Followers[hero].legRemaining = 11
	child := w.SplitAdonis(hero)
	if child == 0 || w.Followers[hero].Population != 50 || w.Followers[child].Population != 50 || w.Followers[child].Hero.Kind != HeroAdonis || w.Followers[child].velocityX != 7 || w.Followers[child].legRemaining != 11 {
		t.Fatal("Adonis clone differs")
	}
	w.Followers[hero].Population = 20
	if w.SplitAdonis(hero) != 0 || w.Followers[hero].Population != 20 {
		t.Fatal("small Adonis split")
	}
	for id := 1; id < FollowerCapacity; id++ {
		if w.Followers[id].State == Inactive {
			w.Followers[id] = Follower{State: Walking, Population: 1}
		}
	}
	w.Followers[hero].Population = 101
	if w.SplitAdonis(hero) != 0 || w.Followers[hero].Population != 50 {
		t.Fatal("full-pool Adonis did not retain source halving")
	}
}

func TestHelenContactUsesCaptureInsteadOfBattle(t *testing.T) {
	w := testFlatWorld()
	hero := addFollower(w, 20, 20, 0, 1000, Walking)
	enemy := addFollower(w, 21, 20, 1, 100, Town)
	w.Followers[hero].Hero.Kind = HeroHelen
	for pass := 0; pass < 30 && w.Followers[enemy].Hero.CaptiveOf == 0; pass++ {
		w.stepFollower(hero)
	}
	if w.Followers[enemy].Hero.CaptiveOf != hero || w.Followers[hero].State == Fighting || w.Followers[enemy].Owner != 1 {
		t.Fatal("Helen contact entered ordinary combat")
	}
}

func TestHeroTargetSelectionOriginalNumericalCorpus(t *testing.T) {
	data, err := os.ReadFile("testdata/hero_target_numbers.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Name   string
			Actors []struct {
				Slot, Owner, X, Y, Population, Claimed int
				Captive                                bool
			}
			Target int
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 60 {
		t.Fatalf("selector corpus %d", len(corpus.Cases))
	}
	for _, test := range corpus.Cases {
		t.Run(test.Name, func(t *testing.T) {
			w := testFlatWorld()
			for _, actor := range test.Actors {
				w.Followers[actor.Slot] = Follower{Owner: uint8(actor.Owner), X: uint8(actor.X), Y: uint8(actor.Y), State: Walking, Population: actor.Population, Hero: HeroState{ClaimedBy: actor.Claimed}}
				if actor.Captive {
					w.Followers[actor.Slot].Hero.CaptiveOf = 1
				}
			}
			w.Followers[1].Hero.Kind = HeroPerseus
			if got := w.SelectHeroTarget(1); got != test.Target {
				t.Fatalf("target %d; source %d", got, test.Target)
			}
		})
	}
}

func TestAllSixHeroesPursueWithTheirDistinctContactBehaviour(t *testing.T) {
	for kind := HeroPerseus; kind <= HeroHelen; kind++ {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			w := testFlatWorld()
			hero := addFollower(w, 20, 20, 0, 1000, Walking)
			enemy := addFollower(w, 22, 20, 1, 100, Town)
			w.Followers[hero].Hero.Kind = kind
			for pass := 0; pass < 60 && w.Followers[hero].State != Fighting && w.Followers[enemy].Hero.CaptiveOf == 0; pass++ {
				w.stepFollower(hero)
			}
			if kind == HeroHelen {
				if w.Followers[enemy].Hero.CaptiveOf != hero {
					t.Fatal("Helen capture")
				}
			} else {
				if w.Followers[hero].State != Fighting || !w.Followers[hero].BattleAggressor {
					t.Fatal("hero combat contact")
				}
			}
		})
	}
}

func TestArmageddonHeroRequestsDirectTerrainWithoutManaCost(t *testing.T) {
	w := testFlatWorld()
	hero := addFollower(w, 20, 20, 0, 1000, Walking)
	enemy := addFollower(w, 24, 20, 1, 100, Town)
	for y := 20; y <= 21; y++ {
		for x := 21; x <= 22; x++ {
			w.Heights[x+y*CornerSize] = 0
		}
	}
	w.rebuildCells()
	w.Armageddon = true
	w.Players[0].Mana = 0
	w.Followers[hero].Hero = HeroState{Kind: HeroPerseus, Phase: HeroPursuing, Target: enemy}
	before := w.Heights[21+20*CornerSize]
	w.stepHero(hero)
	if w.Heights[21+20*CornerSize] <= before || w.Players[0].Mana != 0 {
		t.Fatal("Armageddon hero did not raise blocking ground directly")
	}
}
