package game

func (g *Game) advanceNativeCampaign() error {
	progress, err := g.World.AdvanceCampaign()
	if err != nil {
		g.notify(err.Error())
		return nil
	}
	g.Profile = g.World.Deity
	custom := g.World.Custom
	if progress.Complete {
		return g.beginNativeEnding(progress, custom)
	}
	if err := g.start(int(progress.NextWorld), custom, false); err != nil {
		return err
	}
	if progress.OpenDeity {
		g.OpenDeity()
	}
	return nil
}
