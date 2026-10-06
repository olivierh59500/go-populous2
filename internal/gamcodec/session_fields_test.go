package gamcodec

import (
	"encoding/binary"
	"reflect"
	"testing"

	"go-populous2/internal/engine"
)

func TestGAMAIAndCampaignStatisticsRoundTrip(t *testing.T) {
	w := &engine.World{}
	w.Players[0] = engine.Player{Mana: 2020, Computer: true, Towns: 17, Statistics: engine.CampaignStatistics{Population: 12345, PeakPopulation: 45678, PeakMana: 7890, Metric: 99, LeaderLosses: 3, BattleWins: 5, ScenarioOptions: 35, WeightedPowerUse: 7}}
	w.Level.Players[0].ReactionDelay, w.Level.Players[0].ArmageddonDeadline = 7, 255
	w.Level.Players[0].Scenario = engine.ScenarioOptions{BuildAnywhere: true, FatalWater: true, HideEnemy: true}
	w.AI[0] = engine.AIState{TerrainRequestFollower: 2, TerrainRequestX: 17, TerrainRequestY: 29, Reaction: -2, ExpansionCooldown: 2, ReleaseCooldown: 7, MagnetCooldown: 250, BestTown: 3, BestPopulation: 1234, ExpansionTown: 4, ChoiceCount: 3, LeaderChoiceCount: 1, ChoiceIndex: 2, Prepared: true, PreparedPower: engine.Batholith, PreparedTarget: engine.PowerTarget{X: 32, Y: 33}}
	w.AI[0].WaterRequestFollower = 3
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

func TestGAMCustomPlayerTemplateDoesNotRevertToCampaignDefaults(t *testing.T) {
	catalog := continuationCatalog()
	w := continuationWorld(t, catalog)
	options := &w.Level.Players[0]
	options.Groups, options.Population, options.MovementSpeed, options.Weapons, options.Mana, options.Attrition = 7, 1234, 48, 5, 4321, 9
	options.ReactionDelay, options.ArmageddonDeadline = 37, 900
	options.FixedMagnet, options.MagnetX, options.MagnetY = true, 11, 22
	options.Scenario = engine.ScenarioOptions{SeaLevelOnly: true, ForbidLower: true, HideEnemy: true, ShallowSwamps: true}
	options.Extra = [5]uint16{594, 37, 900, 81, 11<<8 | 22}
	options.Powers[engine.FireColumn] = false
	options.Powers[engine.Baptism] = true
	document, err := NewDocument(w, catalog, engine.NewDeity("CUSTOM"), 0, 4, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Decode(data, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if restored.World.Level.Players[0] != *options {
		t.Fatalf("custom template reverted to campaign defaults: %+v / %+v", restored.World.Level.Players[0], *options)
	}
}

func TestGAMAIObservationRejectsNonFollowerReference(t *testing.T) {
	data := make([]byte, FileSize)
	ref, err := fileReference(engine.ActorRef{Kind: engine.ActorEffect, Index: 0})
	if err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint16(data[0xe8a4+0x36-fileStart:], ref)
	if err := decodeAIStatistics(fileReader{data}, &engine.Snapshot{}, 0); err == nil {
		t.Fatal("water observation accepted an effect-pool reference")
	}
}
