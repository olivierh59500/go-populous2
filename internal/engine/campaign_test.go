package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestCampaignScoresAndProgressMatchOriginalNumbers(t *testing.T) {
	raw, err := os.ReadFile("testdata/campaign_scores.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Name, Kind, Stop                                                string
			World                                                           int
			Score, InitialScore, ExpectedRatio, InitialBolts, ExpectedBolts uint16
			Ticks                                                           uint32
			Local, Opponent                                                 CampaignStatistics
			Overflow, InvalidRatio, Campaign, Victory                       bool
			ExpectedWorld                                                   int
		}
	}
	if err := json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) != 1096 {
		t.Fatal("incomplete reference corpus", len(corpus.Cases))
	}
	for _, test := range corpus.Cases {
		if test.Kind == "score" {
			actual, err := ScoreCampaign(test.Ticks, test.Local, test.Opponent)
			if test.InvalidRatio {
				if err == nil {
					t.Fatal("invalid scenario ratio accepted", test.Name)
				}
				continue
			}
			if err != nil || actual.Value != test.Score || actual.Ratio != test.ExpectedRatio || actual.RatioOverflow != test.Overflow {
				t.Fatalf("score %s: %+v %v", test.Name, actual, err)
			}
		} else if test.Campaign && test.World >= 0 && test.World < 1000 {
			profile := Deity{Bolts: test.InitialBolts}
			actual, err := AdvanceCampaign(test.World, test.Victory, test.InitialScore, &profile)
			if err != nil || actual.NextWorld != test.ExpectedWorld || profile.Bolts != test.ExpectedBolts {
				t.Fatalf("progress %s: %+v %+v %v", test.Name, actual, profile, err)
			}
			stop := "reload"
			if actual.AllocateExperience {
				stop = "deity"
			}
			if actual.Complete {
				stop = "complete"
			}
			if stop != test.Stop {
				t.Fatal("progress destination differs", test.Name, stop, test.Stop)
			}
		}
	}
}

func TestCampaignInvalidResultDoesNotChangeProfile(t *testing.T) {
	profile := NewDeity("KEEP")
	before := profile
	for _, world := range []int{-1, 1000} {
		if _, err := AdvanceCampaign(world, true, 65035, &profile); err == nil || profile != before {
			t.Fatal("invalid result changed profile", world, err)
		}
	}
}
