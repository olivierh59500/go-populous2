package app

// consumeSelectionTransfers follows the selected group through the source's
// ordered merge/emigration changes once per completed simulation pass.
func (g *Game) consumeSelectionTransfers() {
	if g.World == nil || g.selectionTransferTick == g.World.Tick {
		return
	}
	g.SelectedFollower = g.World.SelectionTransfers.Resolve(g.SelectedFollower)
	g.selectionTransferTick = g.World.Tick
}
