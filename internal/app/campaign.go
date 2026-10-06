package app

import (
	"fmt"
	"go-populous2/internal/engine"
	"image"
	"image/color"
	"image/draw"
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
	progress, err := engine.AdvanceCampaign(g.LevelIndex, g.World.Result == 1, g.ResultScore.Value, &g.Profile)
	if err != nil {
		return err
	}
	g.ResultProgress = progress
	g.resultApplied = true
	g.LevelIndex = progress.NextWorld
	if progress.AllocateExperience {
		g.Screen = DeityProfile
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
	draw.Draw(g.framebuffer, image.Rect(24, 21, 296, 181), image.NewUniform(color.RGBA{40, 45, 18, 255}), image.Point{}, draw.Src)
	title := "CONQUEST WON"
	side := g.playerSide()
	if g.World.Result != side+1 {
		title = "CONQUEST LOST"
	}
	g.text(title, 104, 29)
	if g.ResultScoreError != "" {
		g.text("SCORE UNAVAILABLE", 88, 140)
	}
	local, opponent := g.World.Players[side].Statistics, g.World.Players[1-side].Statistics
	for index, line := range []string{
		fmt.Sprintf("YOUR PEAK PEOPLE %d", local.PeakPopulation),
		fmt.Sprintf("ENEMY PEAK PEOPLE %d", opponent.PeakPopulation),
		fmt.Sprintf("YOUR PEAK MANA %d", local.PeakMana),
		fmt.Sprintf("BATTLES WON %d", local.BattleWins),
		fmt.Sprintf("SCORE %d", g.ResultScore.Value),
		fmt.Sprintf("BOLT REWARD %d", min(g.ResultScore.Value/13007, uint16(5))),
	} {
		g.text(line, 40, 52+index*15)
	}
	g.button("CONTINUE", 112, 150, 104)
}
