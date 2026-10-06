package engine

import (
	"encoding/json"
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
