package populous

// GenerateOlympianTerrain follows CODE:$cd22/$cd76. The four starting hills
// use their own local X/Y random walks, while the shared random generator only
// supplies their initial seeds. The first game's three height-six hills are
// not an equivalent world generator.
func (w *World) GenerateOlympianTerrain(seed uint32, hills [4][4]int) {
	w.Alt = [EndWidth * EndWidth]int{}
	w.MapAlt = [MapWidth * MapHeight]byte{}
	w.MapBlk = [MapWidth * MapHeight]byte{}
	w.MapBk2 = [MapWidth * MapHeight]byte{}
	w.MapWho = [MapWidth * MapHeight]uint16{}
	w.MapSteps = [MapWidth * MapHeight]uint16{}
	w.rng = lcg(seed)
	for _, hill := range hills {
		xState, yState := uint16(w.rng.next()), uint16(w.rng.next())
		y := int(xState)%hill[0] + hill[1]
		x := int(yState)%hill[2] + hill[3]
		for step := 0; step < 65536; step++ {
			xState = nextRandom(xState)
			yState = nextRandom(yState)
			x += int(xState)%7 - 3
			y += int(yState)%7 - 3
			if x < 0 || x > MapWidth || y < 0 || y > MapHeight {
				break
			}
			if w.raisePoint(x, y) >= 8 {
				break
			}
		}
	}
	w.makeMap(0, 0, MapWidth-1, MapHeight-1)
}
