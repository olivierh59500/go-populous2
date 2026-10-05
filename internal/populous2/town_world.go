package populous2

import legacy "go-populous2/internal/legacy"

func (w *World) nativeTownCallbacks() NativeTownCallbacks {
	return NativeTownCallbacks{
		Record: func(ref NativeRecordReference) (NativeTownRecord, bool) {
			if location, ok := LocateNativeMagnet(ref); ok {
				graph, _ := w.Occupancy.Record(ref)
				owner, err := w.runtimeMemory().Read8(location.BSSOffset + 12)
				return NativeTownRecord{Kind: 0x14, Owner: owner, Next: graph.Next, X: graph.X, Y: graph.Y}, err == nil
			}
			location, ok := LocateNativeRecord(ref)
			if !ok {
				return NativeTownRecord{}, false
			}
			graph, _ := w.Occupancy.Record(ref)
			if location.Pool == NativeFollowerPool {
				a, err := w.readEntryRecord(ref)
				if err != nil {
					return NativeTownRecord{}, false
				}
				return NativeTownRecord{Kind: a.Motion.Kind, Owner: a.Owner, Stage: a.Byte1, State: a.Motion.State, Animation: uint16(a.Motion.Animation), Next: graph.Next, X: graph.X, Y: graph.Y}, true
			}
			v, ok := w.lightningRecord(ref)
			return NativeTownRecord{Kind: v.Kind, Owner: v.Owner, State: v.State, Animation: uint16(v.Animation), Next: graph.Next, X: graph.X, Y: graph.Y}, ok
		},
		SetRecord: func(ref NativeRecordReference, r NativeTownRecord) {
			a, err := w.readEntryRecord(ref)
			if err != nil {
				panic(err)
			}
			a.Motion.Kind, a.Motion.State, a.Motion.Animation, a.Byte1 = r.Kind, r.State, int(r.Animation), r.Stage
			if err := w.writeEntryRecord(ref, a); err != nil {
				panic(err)
			}
		},
		Head:         func(x, y int) NativeRecordReference { return w.Occupancy.Grid.Cells[x+y*64].Head },
		ReadTile:     w.nativeTileAt,
		WriteTile:    w.writeNativeTownTile,
		WriteOverlay: func(x, y int, code uint8) { w.NativeOverlays[x+y*64] = code },
	}
}

func (w *World) writeNativeTownTile(x, y int, tile uint8) {
	pos := x + y*64
	switch tile {
	case 47, 63:
		owner := 0
		if tile == 63 {
			owner = 1
		}
		w.Core.MapBlk[pos] = uint8(legacy.FarmBlock + owner)
		w.Marks[pos] = Mark{}
	case 15:
		w.Core.MapBlk[pos] = legacy.FlatBlock
		w.Marks[pos] = Mark{}
	default:
		w.Marks[pos] = Mark{Life: 1, Persistent: true, NativeTile: tile, NativeCodeValid: true}
	}
	if err := w.Occupancy.SetTile(x, y, tile); err != nil {
		panic(err)
	}
}

func (w *World) evaluateNativeTown(ref NativeRecordReference) (int, error) {
	stage, err := w.TownEvaluator.Evaluate(ref, uint16(w.Core.GameTurn), w.nativeTownCallbacks())
	return int(stage), err
}

func (w *World) clearNativeFarms(ref NativeRecordReference, tile uint8) error {
	return w.TownEvaluator.ClearFarms(ref, tile, w.nativeTownCallbacks())
}

func (w *World) bindNativeTownEvaluator() {
	w.Core.NativeFollowerPassBegin = w.beginNativeFollowerPass
	w.Core.NativeTownAllocationFailed = func() { w.setNativeBirthBlockWord(1) }
	w.Core.NativeTownEvaluate = func(index int) int {
		stage, err := w.evaluateNativeTown(nativeActorReference(NativeFollowerPool, index))
		if err != nil {
			panic(err)
		}
		return stage
	}
	w.Core.NativeTownClear = func(index int) {
		if err := w.clearNativeFarms(nativeActorReference(NativeFollowerPool, index), 15); err != nil {
			panic(err)
		}
	}
}
