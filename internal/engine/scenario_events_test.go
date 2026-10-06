package engine

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
)

func TestScenarioSchedulerStopsAtZeroAndDispatchesOneDueEvent(t *testing.T) {
	var p [60]byte
	binary.BigEndian.PutUint16(p[:], 1)
	p[3] = 90
	p[4] = 20
	p[5] = 20
	binary.BigEndian.PutUint16(p[6:], 1)
	p[9] = 98
	p[10] = 21
	p[11] = 20
	p[21] = 255 // A later command after time zero never participates in decoding.
	s, err := DecodeScenarioEvents(p)
	if err != nil {
		t.Fatal(err)
	}
	w := testFlatWorld()
	w.Scenario = s
	w.Tick = 1
	w.tickScenario()
	if w.Scenario.Cursor != 1 || w.Followers[1].Neutral.Kind != NeutralRoadMaker || w.Followers[2].State != Inactive {
		t.Fatal("scheduler dispatched more than one due event")
	}
	w.tickScenario()
	if w.Scenario.Cursor != 2 || w.Followers[2].Neutral.Kind != NeutralFireMaker {
		t.Fatal("second due event did not follow")
	}
	w.tickScenario()
	if w.Scenario.Cursor != 2 || w.Scenario.Err != "" {
		t.Fatal("zero event did not terminate")
	}
}

func TestScenarioEventDecoderRejectsUnknownCommandAndDecodesQuakeDirection(t *testing.T) {
	var p [60]byte
	binary.BigEndian.PutUint16(p[:], 1)
	p[3] = 40
	p[4] = 128 | 20
	p[5] = 30
	s, err := DecodeScenarioEvents(p)
	if err != nil || s.Events[0].Kind != ScenarioEarthquake || s.Events[0].X != 20 || s.Events[0].Direction != 2 {
		t.Fatal("directed quake decoding")
	}
	p[3] = 255
	if _, err := DecodeScenarioEvents(p); err == nil {
		t.Fatal("unknown command was silently accepted")
	}
}

func TestScenarioEffectsIgnorePlayerAffordabilityAndPreserveFactionLedgers(t *testing.T) {
	for _, kind := range []ScenarioEventKind{ScenarioFireColumn, ScenarioWhirlwind, ScenarioEarthquake, ScenarioTrees, ScenarioVolcano, ScenarioStorm, ScenarioFireRain} {
		w := testFlatWorld()
		before := w.Players
		if err := w.executeScenarioEvent(ScenarioEvent{Kind: kind, X: 30, Y: 30}); err != nil {
			t.Fatalf("kind%d:%v", kind, err)
		}
		if w.Players != before {
			t.Fatal("neutral event altered player mana or availability")
		}
	}
}

func TestNeutralCreationAndMovementUseOriginalSixVelocities(t *testing.T) {
	for kind := NeutralRoadMaker; kind <= NeutralMonster; kind++ {
		w := testFlatWorld()
		id, err := w.CreateNeutral(kind, 20, 20)
		if err != nil {
			t.Fatal(err)
		}
		f := w.Followers[id]
		x, y := f.positionX, f.positionY
		w.tickNeutral(id)
		if w.Followers[id].Owner != 2 || w.Followers[id].Population != 0 || w.Followers[id].positionX != x+f.velocityX || w.Followers[id].positionY != y+f.velocityY {
			t.Fatalf("kind%d motion", kind)
		}
		if kind == NeutralLandLowerer && w.Followers[id+1].Neutral.Kind != kind {
			t.Fatal("paired lowering invention")
		}
	}
}

func TestNeutralMonsterRetainsVictimsUntilItsFourHundredPassTimer(t *testing.T) {
	w := testFlatWorld()
	monster, err := w.CreateNeutral(NeutralMonster, 20, 20)
	if err != nil {
		t.Fatal(err)
	}
	victim := addFollower(w, 21, 21, 0, 100, Walking)
	w.tickNeutral(monster)
	if w.Followers[victim].State != Ruin || w.Followers[victim].Neutral.VictimTime != 400 || w.Occupants[21+21*MapSize] != uint16(victim) {
		t.Fatal("monster victim was deleted immediately")
	}
	for range 399 {
		w.advanceNeutralVictim(victim)
	}
	if w.Followers[victim].State == Inactive {
		t.Fatal("victim lifetime ended early")
	}
	w.advanceNeutralVictim(victim)
	if w.Followers[victim].State != Inactive {
		t.Fatal("victim timer did not complete")
	}
}

func TestScenarioTimingOriginalNumericalCorpus(t *testing.T) {
	data, err := os.ReadFile("testdata/scenario_timing_numbers.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Name                   string
			Parameters             [60]byte
			Cursor, Clock, Updates int
			Trace                  []int
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Cases) < 100 {
		t.Fatal("scenario timing corpus incomplete")
	}
	for _, test := range corpus.Cases {
		t.Run(test.Name, func(t *testing.T) {
			s, err := DecodeScenarioEvents(test.Parameters)
			if err != nil {
				t.Fatal(err)
			}
			// Timing comparisons consume real decoded event kinds while isolating
			// their independently tested creators from the scheduler's cursor rule.
			for index := range s.Events {
				s.Events[index].Kind = ScenarioNoEvent
			}
			s.Cursor = uint8(test.Cursor)
			w := testFlatWorld()
			w.Scenario = s
			w.Tick = uint64(test.Clock)
			for _, cursor := range test.Trace {
				w.tickScenario()
				if int(w.Scenario.Cursor) != cursor {
					t.Fatalf("cursor%d source%d", w.Scenario.Cursor, cursor)
				}
			}
		})
	}
}
