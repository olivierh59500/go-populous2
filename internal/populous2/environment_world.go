package populous2

import "fmt"

type NativeEnvironmentController uint8

const (
	NativeEnvironmentNone NativeEnvironmentController = iota
	NativeEnvironmentQuake
	NativeEnvironmentVolcano
	NativeEnvironmentLava
)

func (w *World) quakeCallbacks() EarthquakeCallbacks {
	return EarthquakeCallbacks{Memory: w.nativeCleanupMemory(), Tile: func(x, y int) (uint8, error) {
		if !inside(x, y) {
			return 0, fmt.Errorf("native earthquake tile outside world")
		}
		return w.nativeTileAt(x, y), nil
	},
		Header: func(x, y int) (uint8, error) {
			if !inside(x, y) {
				return 0, fmt.Errorf("native earthquake header outside world")
			}
			return w.Occupancy.Grid.Cells[x+y*64].Header, nil
		},
		SetTile: func(x, y int, code uint8) error {
			if !inside(x, y) {
				return fmt.Errorf("native earthquake write outside world")
			}
			w.writeNativeTownTile(x, y, code)
			return nil
		},
		Lower: func(x, y int) error {
			before := w.Core.Alt
			if w.Core.DirectLowerTerrain(x, y) {
				w.clearChangedGround(before)
				w.refreshChangedTerrain(before)
			}
			return nil
		},
		Random: w.random, Dirty: func() error { w.NativeEnvironmentDirty++; return nil }, Shake: func() error { w.NativeEnvironmentShake++; return nil }, CameraX: uint8(w.effectViewX), CameraY: uint8(w.effectViewY)}
}

func (w *World) castNativeEarthquake(player, x, y int, direction uint8) bool {
	created := false
	err := w.runNativeFollowerCall(func() error {
		dir := w.EarthquakeRules.Directions[direction&3]
		result, err := w.EarthquakeRules.Create(uint8(player+1), uint8(x), uint8(y), dir, w.EarthquakeRules.BaseStrength+uint16(w.Experience[player][Earth]), 0, w.quakeCallbacks())
		if err != nil {
			return err
		}
		if result.Allocated {
			index := (result.Address - 0xc800) / 32
			if result.Active {
				w.NativeEnvironment[index] = NativeEnvironmentQuake
			}
			created = result.Active
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return created
}

func (w *World) tickNativeEnvironment(index int) error {
	return w.runNativeFollowerCall(func() error {
		address := 0xc800 + index*32
		ref := nativeActorReference(NativeEffectPool, index)
		switch w.NativeEnvironment[index] {
		case NativeEnvironmentQuake:
			step, err := w.EarthquakeRules.Tick(address, w.quakeCallbacks())
			if err != nil {
				return err
			}
			if step.Branched && step.ChildAddress >= 0xc800 && step.ChildAddress < 0xe740 {
				child := (step.ChildAddress - 0xc800) / 32
				owner, _ := w.runtimeMemory().Read8(step.ChildAddress + 12)
				if owner != 0 {
					w.NativeEnvironment[child] = NativeEnvironmentQuake
				}
			}
		case NativeEnvironmentVolcano:
			if _, err := w.VolcanoRules.Tick(ref, w.volcanoCallbacks()); err != nil {
				return err
			}
		case NativeEnvironmentLava:
			if _, err := w.LavaRules.Tick(ref, w.lavaCallbacks()); err != nil {
				return err
			}
		default:
			return fmt.Errorf("native environment controller not implemented")
		}
		owner, _ := w.runtimeMemory().Read8(address + 12)
		if owner == 0 {
			w.NativeEnvironment[index] = NativeEnvironmentNone
		}
		return nil
	})
}

func (w *World) nativeEnvironmentCell(tile NativePackedTile) (NativeOccupancyCell, error) {
	x, y := int(uint8(tile)), int(uint8(uint16(tile)>>8))
	if !inside(x, y) {
		return NativeOccupancyCell{}, fmt.Errorf("native environment cell outside world")
	}
	return w.Occupancy.Grid.Cells[x+y*64], nil
}

func (w *World) volcanoCallbacks() VolcanoCallbacks {
	return VolcanoCallbacks{Memory: w.nativeCleanupMemory(), FireExperience: func(owner uint8) (uint8, error) {
		deity, ok := NativeDeityAddress(owner)
		if !ok {
			return 0, fmt.Errorf("native volcano owner outside deities")
		}
		return w.runtimeMemory().Read8(deity + 0x56)
	}, Cell: w.nativeEnvironmentCell, WriteTile: w.nativeCommonPrepassCallbacks().WriteTile,
		Lower: func(x, y int) error {
			before := w.Core.Alt
			if w.Core.DirectLowerTerrain(x, y) {
				w.clearChangedGround(before)
				w.refreshChangedTerrain(before)
			}
			return nil
		},
		Raise: func(x, y int) error {
			before := w.Core.Alt
			if w.Core.DirectRaiseTerrain(x, y) {
				w.clearChangedGround(before)
				w.refreshChangedTerrain(before)
			}
			return nil
		},
		CreateFireColumn: func(owner uint16, x, y uint8) error {
			_, err := w.PrimitiveCreators.CreateFireColumn(owner, x, y, w.nativePrimitiveCallbacks())
			return err
		},
		CreateLava: func(owner uint8, tile NativePackedTile, direction uint16) error {
			_, err := w.LavaRules.Create(owner, tile, direction, w.lavaCallbacks())
			return err
		}, Random: func() uint16 { return uint16(w.random()) }}
}

func (w *World) lavaCallbacks() NativeLavaCallbacks {
	return NativeLavaCallbacks{Memory: w.nativeCleanupMemory(), Cell: w.nativeEnvironmentCell, Random: func() uint16 { return uint16(w.random()) }, Clock: uint16(w.Core.GameTurn),
		Link: func(ref NativeRecordReference) error {
			if err := w.nativeRuntimeInsert(ref); err != nil {
				return err
			}
			location, ok := LocateNativeRecord(ref)
			if !ok || location.Pool != NativeEffectPool {
				return fmt.Errorf("native lava link outside effect pool")
			}
			w.NativeEnvironment[location.Index] = NativeEnvironmentLava
			return nil
		},
		Unlink: w.nativeRuntimeUnlink, Move: w.nativeTerrainCallbacks().Move,
		CreateBasalt: func(owner uint8, tile NativePackedTile, direction uint16) error {
			return w.createRawBaseBasalt(owner, tile, direction)
		},
		Scorch: func(ref NativeRecordReference) error {
			a, err := w.runtimeMemory().ReadFollowerEntry(ref)
			if err != nil {
				return err
			}
			tile := entryTile(a)
			cell, err := w.nativeEnvironmentCell(tile)
			if err != nil {
				return err
			}
			if w.LavaRules.Geometry[cell.Tile]&15 == 15 && w.GroundRules.Properties[cell.Tile]&0x400 == 0 {
				return w.nativeCommonPrepassCallbacks().WriteTile(tile, 95)
			}
			return nil
		},
		DestroyTown: w.nativeDestroyTown}
}

func (w *World) createRawBaseBasalt(owner uint8, tile NativePackedTile, direction uint16) error {
	var pool [NativeEffectCapacity]NativeEffectActor
	var state BasaltState
	m := w.runtimeMemory()
	for index := range pool {
		ref := nativeActorReference(NativeEffectPool, index)
		byteOwner, _ := m.Read8(cleanupRecordAddress(ref) + 12)
		vx, _ := m.Read16(cleanupRecordAddress(ref) + 14)
		vy, _ := m.Read16(cleanupRecordAddress(ref) + 16)
		speed, _ := m.Read8(cleanupRecordAddress(ref) + 18)
		pool[index] = NativeEffectActor{Active: byteOwner != 0, VX: int16(vx), VY: int16(vy), Speed: speed}
	}
	var callbackErr error
	_, _ = w.BasaltRules.Create(&pool, &state, int(owner)-1, int(uint8(tile)), int(uint8(uint16(tile)>>8)), int16(w.BasaltRules.BaseLife), direction, BasaltCallbacks{
		ReadGeometry: func(x, y int) uint8 { return w.BasaltRules.Geometry[w.nativeTileAt(x, y)] }, WriteTile: func(x, y int, code uint8) { w.writeNativeTownTile(x, y, code) }, Random: w.random,
		Link: func(index int) {
			a := pool[index]
			at := 0xc800 + index*32
			for _, field := range []struct {
				offset int
				value  uint8
			}{{0, a.Kind}, {12, owner}, {22, a.State}} {
				if _, err := m.Write8(at+field.offset, field.value); err != nil {
					callbackErr = err
					return
				}
			}
			for _, field := range []struct {
				offset int
				value  uint16
			}{{6, uint16(a.X)}, {8, uint16(a.Y)}, {10, uint16(a.Animation)}, {20, uint16(a.Timer)}, {24, uint16(a.Life)}, {26, state.Directions[index]}} {
				if _, err := m.Write16(at+field.offset, field.value); err != nil {
					callbackErr = err
					return
				}
			}
			callbackErr = w.nativeRuntimeInsert(nativeActorReference(NativeEffectPool, index))
		}})
	return callbackErr
}

func (w *World) castNativeVolcano(player, x, y int) bool {
	created := false
	err := w.runNativeFollowerCall(func() error {
		ref, ok, err := w.VolcanoRules.Create(uint8(player+1), uint8(x), uint8(y), w.volcanoCallbacks())
		if err != nil {
			return err
		}
		if ok {
			location, _ := LocateNativeRecord(ref)
			w.NativeEnvironment[location.Index] = NativeEnvironmentVolcano
			created = true
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return created
}
