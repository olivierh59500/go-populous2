package engine

import "fmt"

type CampaignStatistics struct {
	Population, PeakPopulation, PeakMana                                uint32
	Metric, LeaderLosses, BattleWins, ScenarioOptions, WeightedPowerUse uint16
}

type CampaignScore struct {
	Value         uint16
	Ratio         uint16
	RatioOverflow bool
}

// ScoreCampaign preserves the historical unsigned score arithmetic using
// named statistics. Invalid scenario ratios are explicit errors, not machine
// exceptions or reads from an emulated deity structure.
func ScoreCampaign(ticks uint32, local, opponent CampaignStatistics) (CampaignScore, error) {
	if opponent.ScenarioOptions == 0 {
		return CampaignScore{}, fmt.Errorf("campaign score requires nonzero opponent options")
	}
	difference := uint32(uint16(local.BattleWins - opponent.BattleWins))
	product := difference * (uint32(local.BattleWins)<<16 | uint32(local.ScenarioOptions))
	dividend := product&0xffff0000 | uint32(uint16(product)+1)
	quotient := dividend / uint32(opponent.ScenarioOptions)
	result := CampaignScore{RatioOverflow: quotient > 0xffff}
	if result.RatioOverflow {
		result.Ratio = uint16(dividend)
	} else {
		result.Ratio = uint16(quotient)
	}
	result.Value = uint16(5000+ticks) + result.Ratio + uint16(uint32(local.WeightedPowerUse)*150)
	return result, nil
}

type CampaignProgress struct {
	Victory, Complete                    bool
	BoltAward                            uint16
	WorldStep, PresentedWorld, NextWorld int
	AllocateExperience                   bool
}

// AdvanceCampaign applies a completed result exactly once through its caller.
// The final-world victory finishes the campaign and resets its next world.
func AdvanceCampaign(world int, victory bool, score uint16, profile *Deity) (CampaignProgress, error) {
	if world < 0 || world >= 1000 || profile == nil {
		return CampaignProgress{}, fmt.Errorf("invalid campaign result")
	}
	result := CampaignProgress{Victory: victory, PresentedWorld: world, NextWorld: world}
	result.BoltAward = profile.AwardCampaignBolts(score)
	result.WorldStep = CampaignWorldStep(score)
	if !victory {
		if world != 999 {
			result.NextWorld = world + 1
			result.PresentedWorld = result.NextWorld
		}
		return result, nil
	}
	next := world + result.WorldStep
	if next > 999 && world != 999 {
		next = 999
	}
	result.PresentedWorld, result.NextWorld, result.AllocateExperience = next, next, true
	if next >= 1000 {
		result.Complete = true
		result.NextWorld = 0
	}
	return result, nil
}

// RecordCampaignMetrics retains peaks and current named player statistics.
// The packed scenario word remains a file-format quantity for the historical
// score formula, rather than a runtime memory field.
func (w *World) RecordCampaignMetrics() {
	for owner := range w.Players {
		player := &w.Players[owner]
		stats := &player.Statistics
		stats.Population = uint32(max(0, player.Population))
		stats.PeakPopulation = max(stats.PeakPopulation, stats.Population)
		stats.PeakMana = max(stats.PeakMana, uint32(max(0, player.Mana)))
		stats.BattleWins = uint16(player.BattlesWon)
		stats.ScenarioOptions = w.Level.Players[owner].Extra[0]
	}
}

func (w *World) RecordPowerUse(owner int, id PowerID) {
	if owner >= 0 && owner < len(w.Players) && id != RaiseLower {
		w.Players[owner].Statistics.WeightedPowerUse += uint16(id)%6 + 1
	}
}
