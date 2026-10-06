package app

import (
	"fmt"
	"go-populous2/internal/engine"
)

func (g *Game) applyCampaignResult() error {
	if g.World == nil {
		return fmt.Errorf("campaign result has no world")
	}
	if g.resultApplied {
		return nil
	}
	if g.CustomGame || g.Network != nil {
		if g.Network != nil {
			g.Network.Close()
			g.Network = nil
		}
		g.resultApplied, g.Screen = true, MainMenu
		return nil
	}
	if g.ResultScoreError != "" {
		return fmt.Errorf("campaign score: %s", g.ResultScoreError)
	}
	progress, err := engine.AdvanceCampaign(g.LevelIndex, g.World.Result == g.playerSide()+1, g.ResultScore.Value, &g.Profile)
	if err != nil {
		return err
	}
	g.ResultProgress = progress
	g.resultApplied = true
	g.LevelIndex = progress.NextWorld
	if progress.AllocateExperience && g.Profile.Bolts != 0 {
		g.openDeityProfile(ConquestBriefing)
	} else {
		g.Screen = ConquestBriefing
	}
	if progress.Complete {
		g.Message = "CAMPAIGN COMPLETE"
		g.messageUntil = g.Updates + 500
		return g.startEnding()
	}
	return nil
}

// finishWorld uses the local camp for both the verdict and score. Network
// victories are standalone games and do not advance the solo conquest.
func (g *Game) finishWorld() {
	if g.World == nil || g.World.Result == 0 || g.Screen == CampaignResult {
		return
	}
	side := g.playerSide()
	score, err := engine.ScoreCampaign(uint32(g.World.Tick), g.World.Players[side].Statistics, g.World.Players[1-side].Statistics)
	g.ResultScore, g.ResultScoreError = score, ""
	if err != nil {
		g.ResultScoreError = err.Error()
	}
	g.resultApplied = false
	g.resultAt, g.Screen = g.Updates, CampaignResult
}

func (g *Game) drawCampaignResult() {
	g.drawWorld()
	g.drawOriginalCampaignResult()
	if g.Updates < g.messageUntil {
		g.drawMessage(181)
	}
}
