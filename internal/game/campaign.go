package game

func (g *Game) advanceNativeCampaign() error {
	progress, err := g.World.AdvanceCampaign()
	if err != nil {
		g.notify(err.Error())
		return nil
	}
	g.Profile = g.World.Deity
	custom := g.World.Custom
	if err := g.start(int(progress.NextWorld), custom, false); err != nil {
		return err
	}
	if progress.OpenDeity {
		g.OpenDeity()
	}
	if progress.Complete {
		g.notify("Campagne terminee. Les 1 000 mondes ont ete parcourus.")
	}
	return nil
}
