package populous2

func (w *World) patchNativeByte(ref NativeRecordReference, offset int, value uint8) {
	old, err := w.RecordImage.Read8(ref, offset)
	if err != nil {
		panic(err)
	}
	if old != value {
		if _, err := w.RecordImage.Write8(ref, offset, value); err != nil {
			panic(err)
		}
	}
}
func (w *World) patchNativeWord(ref NativeRecordReference, offset int, value uint16) {
	old, err := w.RecordImage.Read16(ref, offset)
	if err != nil {
		panic(err)
	}
	if old != value {
		if _, err := w.RecordImage.Write16(ref, offset, value); err != nil {
			panic(err)
		}
	}
}

// refreshNativeRecordImage patches common fields without regenerating records.
// Controller-specific retained bytes remain in the contiguous image. Aliased
// writes use hydrateNativePatch before the next typed controller can overwrite
// an affected field from an older cached value.
func (w *World) refreshNativeRecordImage() {
	for _, pool := range []NativeRecordPool{NativeWallPool, NativeSceneryPool, NativeFollowerPool, NativeEffectPool} {
		count := 0
		switch pool {
		case NativeWallPool:
			count = WallCapacity
		case NativeSceneryPool:
			count = SceneryCapacity
		case NativeFollowerPool:
			count = len(w.Core.Peeps)
		case NativeEffectPool:
			count = NativeEffectCapacity
		}
		for index := 0; index < count; index++ {
			ref := nativeActorReference(pool, index)
			active := false
			switch pool {
			case NativeWallPool:
				active = w.Walls.Actors[index].Active
			case NativeSceneryPool:
				active = w.Scenery[index].Active
			case NativeFollowerPool:
				active = w.Core.Peeps[index].Population > 0 || w.flameDeathIndex[index] || w.LightningVictims[index].Active
			case NativeEffectPool:
				active = w.NativeEffects[index].Active
			}
			if !active {
				w.patchNativeByte(ref, 12, 0)
				continue
			}
			graph, _ := w.Occupancy.Record(ref)
			for _, field := range []struct {
				off   int
				value uint16
			}{{2, uint16(graph.Next)}, {4, uint16(graph.Previous)}, {6, graph.X}, {8, graph.Y}} {
				w.patchNativeWord(ref, field.off, field.value)
			}
			switch pool {
			case NativeWallPool:
				a := w.Walls.Actors[index]
				kind := uint8(0x1a)
				if a.Broken {
					kind = 0x1c
				}
				w.patchNativeByte(ref, 0, kind)
				w.patchNativeByte(ref, 1, a.Variant)
				w.patchNativeWord(ref, 10, uint16(a.Animation))
				owner := uint8(0)
				if a.Active {
					owner = a.Player + 1
				}
				w.patchNativeByte(ref, 12, owner)
				next := uint16(0)
				if a.Next != 0 {
					next = uint16(nativeActorReference(NativeWallPool, int(a.Next)-1))
				}
				w.patchNativeWord(ref, 14, next)
			case NativeSceneryPool:
				a := w.Scenery[index]
				kind := uint8(a.Kind)
				if a.Removing {
					kind = 0x1e
				}
				w.patchNativeByte(ref, 0, kind)
				w.patchNativeByte(ref, 1, uint8(a.Age))
				w.patchNativeWord(ref, 10, uint16(a.Animation+a.Frame*4))
				owner := uint8(0)
				if a.Active {
					owner = 3
				}
				w.patchNativeByte(ref, 12, owner)
			case NativeFollowerPool:
				a, err := w.readEntryRecord(ref)
				if err != nil {
					panic(err)
				}
				before, err := w.RecordImage.ReadFollowerEntry(ref)
				if err != nil {
					panic(err)
				}
				if _, err := w.RecordImage.PatchFollowerEntry(ref, before, a); err != nil {
					panic(err)
				}
				w.patchNativeByte(ref, 24, uint8(w.Core.Peeps[index].IQ))
			case NativeEffectPool:
				w.projectNativeEffectRecord(index)
			}
		}
	}
}

// projectNativeEffectRecord also records final controller writes before an
// actor becomes inactive; later inactive refreshes preserve these bytes.
func (w *World) projectNativeEffectRecord(index int) {
	ref := nativeActorReference(NativeEffectPool, index)
	a := w.NativeEffects[index]
	if a.Kind == FungusActorKind || a.Kind == WhirlpoolActorKind {
		w.patchNativeWord(ref, 6, uint16(a.X))
		w.patchNativeWord(ref, 8, uint16(a.Y))
	}
	w.patchNativeByte(ref, 0, a.Kind)
	w.patchNativeWord(ref, 10, uint16(a.Animation))
	owner := uint8(0)
	if a.Active {
		owner = a.Player + 1
	}
	w.patchNativeByte(ref, 12, owner)
	for _, field := range []struct {
		off   int
		value uint16
	}{{14, uint16(a.VX)}, {16, uint16(a.VY)}, {20, uint16(a.Timer)}, {24, uint16(a.Life)}} {
		w.patchNativeWord(ref, field.off, field.value)
	}
	w.patchNativeByte(ref, 18, a.Speed)
	w.patchNativeByte(ref, 22, a.State)
	switch a.Kind {
	case BasaltActorKind:
		w.patchNativeWord(ref, 26, w.BasaltState.Directions[index])
	case 0x28, 0x2a:
		w.patchNativeWord(ref, 26, uint16(w.LightningState.Word26[index]))
		w.patchNativeWord(ref, 28, uint16(w.LightningState.Word28[index]))
		w.patchNativeWord(ref, 30, w.LightningState.RandomWords[index])
	case FungusActorKind:
		b := w.FungusState.Bounds[index]
		w.patchNativeByte(ref, 26, b.DX)
		w.patchNativeByte(ref, 27, b.DY)
		w.patchNativeByte(ref, 28, b.WorkDX)
		w.patchNativeByte(ref, 29, b.WorkDY)
	}
}

func (w *World) hydrateNativePatch(patch NativeRecordImagePatch) {
	for _, slot := range patch.Slots {
		if slot.Reserved {
			continue
		}
		ref := slot.Reference
		if slot.Location.Pool == NativeFollowerPool {
			index := slot.Location.Index - 1
			if index >= len(w.Core.Peeps) {
				continue
			}
			a, err := w.RecordImage.ReadFollowerEntry(ref)
			if err != nil {
				panic(err)
			}
			w.NativeEntries[index] = NativeFollowerEntry{Initialized: true, Actor: a}
			p := &w.Core.Peeps[index]
			p.MovementSpeed, p.Weapons, p.Population = a.Motion.Speed, int(a.Weapon), int(a.Motion.Population)
			search, err := w.RecordImage.Read8(ref, 24)
			if err != nil {
				panic(err)
			}
			p.IQ = int(search)
			// The next ordinary-controller read overlays its complete motion
			// record, so every modeled field must reflect the retained image.
			w.NativeFollowers[index].Actor = a.Motion
		}
	}
}
