package engine

// SetupScenery seeds clustered trees and boulders before either faction is
// placed. The two passes share the original 200-scenery allocation budget.
func (w *World) SetupScenery() {
	for _, kind := range []SceneryKind{SceneryTree, SceneryBoulder} {
		clusters := int(w.random.next()%9) + 4
		for cluster := 0; cluster <= clusters; cluster++ {
			point := w.random.next() & 0x3f3f
			x, y := int(uint8(point)), int(uint8(point>>8))
			bonus := 0
			// The original startup reuses a reserved scenery row once this
			// slot is populated. Preserve its visible eight-attempt bonus as
			// a named setup rule, without reading another record's bytes.
			if kind == SceneryTree && w.Nature.Scenery[45].Kind != SceneryNone {
				bonus = 8
			}
			w.plantStartupCluster(kind, x, y, bonus)
		}
	}
}

func (w *World) plantStartupCluster(kind SceneryKind, x, y, bonus int) {
	bits := int(w.random.next())
	variant := uint8(bits % 8 / 2)
	count := bits%14 + bonus
	for attempt := 0; attempt <= count; attempt++ {
		nx, ny, valid := w.natureSample(x, y)
		if !valid {
			continue
		}
		at := nx + ny*MapSize
		code := w.Cell(nx, ny).Code
		if code == 0 || roadCode(code) || w.Occupants[at] != 0 || w.Nature.sceneryAt(nx, ny) >= 0 {
			continue
		}
		free := -1
		for id, a := range w.Nature.Scenery {
			if a.Kind == SceneryNone {
				free = id
				break
			}
		}
		if free < 0 {
			// Tree clusters stop immediately; the original boulder creator
			// continues the sampled attempts despite a full shared pool.
			if kind == SceneryTree {
				return
			}
			continue
		}
		chosen := variant
		if w.random.next()%90 == 0 {
			chosen = 0
		}
		w.Nature.Scenery[free] = SceneryActor{Kind: kind, X: uint8(nx), Y: uint8(ny), Age: 24, Variant: chosen}
		w.Actors.Link(ActorRef{Kind: ActorScenery, Index: uint16(free)}, nx*256+128, ny*256+128)
	}
}
