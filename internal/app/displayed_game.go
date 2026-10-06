package app

// displayedGame provides the immutable geometry and actors currently on
// screen. Commands still mutate the live world, never the presentation copy.
func (g *Game) displayedGame() Game {
	if g.Screen == Playing && g.presentation.Ready {
		return g.presentation.Renderer(g)
	}
	if g.Screen == EditorScreen && g.Editor != nil {
		view := *g
		view.World = g.Editor.Draft
		if g.Editor.Presentation.Ready {
			view = g.Editor.Presentation.Renderer(&view)
		}
		view.CameraX, view.CameraY = g.CameraX, g.CameraY
		return view
	}
	return *g
}
