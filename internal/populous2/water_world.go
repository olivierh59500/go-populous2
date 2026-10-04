package populous2

func (w *World) SetEffectView(x, y int) { w.effectViewX, w.effectViewY = x, y }

// TakeEffectSoundCues drains presentation events. Camera-dependent audio does
// not enter the deterministic simulation snapshot.
func (w *World) TakeEffectSoundCues() []int {
	cues := append([]int(nil), w.effectSoundCues...)
	w.effectSoundCues = w.effectSoundCues[:0]
	return cues
}

func (w *World) castWhirlpool(player, x, y int) bool {
	return w.Whirlpools.Create(&w.NativeEffects, player, x, y, w.Experience[player][Water], w.nativeTileAt, w.setWaterTile)
}

func (w *World) setWaterTile(x, y int, tile uint8) {
	pos := x + y*64
	if tile == 0 {
		w.Marks[pos] = Mark{}
	} else {
		w.Marks[pos] = Mark{Spell: Whirlpool, Life: 1, Persistent: true, NativeTile: tile}
	}
	if err := w.Occupancy.SetTile(x, y, w.nativeTileAt(x, y)); err != nil {
		panic(err)
	}
}

func (w *World) lowerWaterVertex(x, y int) {
	before := w.Core.Alt
	if w.Core.DirectLowerTerrain(x, y) {
		w.clearChangedGround(before)
		w.refreshChangedTerrain(before)
	}
}

func (w *World) tickWhirlpool(actor *NativeEffectActor) {
	step, err := w.Whirlpools.Tick(actor, WhirlpoolCallbacks{
		ReadTile: w.nativeTileAt, WriteTile: w.setWaterTile,
		ReadGridByte: func(offset uint16) uint8 {
			value, ok := w.Occupancy.Byte(int(offset))
			if !ok {
				panic("native whirlpool grid offset exceeds map")
			}
			return value
		},
		LowerVertex: w.lowerWaterVertex, Random: w.random,
		ViewX: w.effectViewX, ViewY: w.effectViewY,
	})
	if err != nil {
		panic(err)
	}
	if step.SoundCue >= 0 && len(w.effectSoundCues) < NativeEffectCapacity {
		w.effectSoundCues = append(w.effectSoundCues, step.SoundCue)
	}
}

func (w *World) basaltCallbacks() BasaltCallbacks {
	return BasaltCallbacks{
		ReadGeometry: func(x, y int) uint8 { return w.BasaltRules.Geometry[w.nativeTileAt(x, y)] },
		WriteTile: func(x, y int, tile uint8) {
			w.Marks[x+y*64] = Mark{Spell: Basalt, Life: 1, Persistent: true, NativeTile: tile}
			w.setBasaltLegacyLand(x, y)
			if err := w.Occupancy.SetTile(x, y, w.nativeTileAt(x, y)); err != nil {
				panic(err)
			}
		},
		Random: w.random,
		Link:   func(index int) { w.linkEffect(index) },
		Unlink: func(index int) { w.unlinkActor(NativeEffectPool, index) },
	}
}

func (w *World) castBasalt(player, x, y, direction int) bool {
	_, accepted := w.BasaltRules.Create(&w.NativeEffects, &w.BasaltState, player, x, y, int16(w.BasaltRules.BaseLife+int(w.Experience[player][Water])), uint16(direction), w.basaltCallbacks())
	return accepted
}

func (w *World) tickBasalt(index int) {
	if _, err := w.BasaltRules.Tick(&w.NativeEffects, &w.BasaltState, index, w.basaltCallbacks()); err != nil {
		panic(err)
	}
}
