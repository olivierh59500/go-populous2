package game

// localPlayer follows the native selected-profile side, including imported
// games whose human deity occupies the second physical owner slot.
func (g *Game) localPlayer() int { return int(g.World.NativeProfileSide) - 1 }
