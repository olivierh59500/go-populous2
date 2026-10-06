package app

import "go-populous2/internal/visualassets"

func (g *Game) resultPresentationState() visualassets.ResultState {
	side := g.playerSide()
	local, other := g.World.Players[side].Statistics, g.World.Players[1-side].Statistics
	return visualassets.ResultState{Winner: g.World.Result - 1, Ticks: uint32(g.World.Tick), Score: g.ResultScore.Value,
		Local:    visualassets.ResultStatistics{PeakPopulation: local.PeakPopulation, PeakMana: local.PeakMana, BattleWins: local.BattleWins, LeaderLosses: local.LeaderLosses},
		Opponent: visualassets.ResultStatistics{PeakPopulation: other.PeakPopulation, PeakMana: other.PeakMana, BattleWins: other.BattleWins, LeaderLosses: other.LeaderLosses},
	}
}
func (g *Game) drawOriginalCampaignResult() bool {
	if g.Assets == nil || g.Assets.Result == nil || g.World == nil {
		return false
	}
	layout := g.Assets.Result.Layout
	layout.Palette = g.Assets.Visual.Palettes[g.World.Level.Landscape]
	layout.Draw(g.framebuffer, g.Assets.Visual.Font, g.Assets.Result.Values(g.resultPresentationState()), nil)
	return true
}
func (g *Game) resultContinueHit(x, y int) bool {
	return g.Assets != nil && g.Assets.Result != nil && g.Assets.Result.Layout.ActionAt(x, y) == "continue"
}
