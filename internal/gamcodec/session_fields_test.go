package gamcodec

import (
	"reflect"
	"testing"

	"go-populous2/internal/engine"
)

func TestGAMAIAndCampaignStatisticsRoundTrip(t *testing.T) {
	w := &engine.World{}
	w.Players[0] = engine.Player{Mana: 2020, Computer: true, Towns: 17, Statistics: engine.CampaignStatistics{Population: 12345, PeakPopulation: 45678, PeakMana: 7890, Metric: 99, LeaderLosses: 3, BattleWins: 5, ScenarioOptions: 35, WeightedPowerUse: 7}}
	w.Level.Players[0].ReactionDelay, w.Level.Players[0].ArmageddonDeadline = 7, 255
	w.Level.Players[0].Scenario = engine.ScenarioOptions{BuildAnywhere: true, FatalWater: true, HideEnemy: true}
	w.AI[0] = engine.AIState{Reaction: -2, ExpansionCooldown: 2, ReleaseCooldown: 7, MagnetCooldown: 250, BestTown: 3, BestPopulation: 1234, ExpansionTown: 4, ChoiceCount: 3, LeaderChoiceCount: 1, ChoiceIndex: 2, Prepared: true, PreparedPower: engine.Batholith, PreparedTarget: engine.PowerTarget{X: 32, Y: 33}}
	w.AI[0].Choices[1] = engine.AIPowerChoice{Power: engine.FireColumn, Target: engine.AITargetTown}
	w.AI[0].Choices[2] = engine.AIPowerChoice{Power: engine.Batholith, Target: engine.AITargetPrepared}
	w.AI[0].Choices[3] = engine.AIPowerChoice{Power: engine.Perseus, Target: engine.AITargetLeader}
	data := make([]byte, FileSize)
	if err := encodeAIStatistics(data, w, 0); err != nil {
		t.Fatal(err)
	}
	var snapshot engine.Snapshot
	if err := decodeAIStatistics(fileReader{data}, &snapshot, 0); err != nil {
		t.Fatal(err)
	}
	got := engine.World(snapshot.World)
	if got.Players[0].Statistics != w.Players[0].Statistics || !reflect.DeepEqual(got.AI[0], w.AI[0]) || got.Level.Players[0].Scenario != w.Level.Players[0].Scenario {
		t.Fatal("GAM AI/statistics fields did not retain named state")
	}
}
