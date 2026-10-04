package populous2

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"testing"

	legacy "go-populous2/internal/legacy"
)

func oceanWorld(t *testing.T, seed uint32) *World {
	t.Helper()
	w := flatGroundWorld(t)
	state := w.Core.Snapshot()
	state.Alt, state.MapAlt, state.MapBlk, state.MapBk2, state.MapWho = [65 * 65]int{}, [4096]byte{}, [4096]byte{}, [4096]byte{}, [4096]uint16{}
	state.Peeps, state.RNG = nil, seed
	w.Core = legacy.WorldFromSnapshot(state, w.Core.Rules)
	w.Scenery, w.Walls, w.NativeEffects, w.NativeFollowers, w.Marks = [SceneryCapacity]SceneryActor{}, WallState{}, [NativeEffectCapacity]NativeEffectActor{}, [legacy.MaxPeeps]NativeFollower{}, [4096]Mark{}
	w.rebuildSceneryIndex()
	w.initializeActorGraph()
	return w
}

func TestWhirlpoolWorldUsesExactWaterQuadAndNativeDebit(t *testing.T) {
	w := oceanWorld(t, 4311)
	before, rng := w.Core.Magnets[0].Mana, w.Core.Snapshot().RNG
	if !w.Cast(0, Whirlpool, Target{X: 32, Y: 32}) {
		t.Fatal("four-water native cast rejected")
	}
	if w.Core.Magnets[0].Mana != before-4000 || w.Core.Snapshot().RNG != rng || len(w.Effects) != 0 || w.NativeEffects[0].Kind != WhirlpoolActorKind {
		t.Fatal("whirlpool used provisional damage, wrong debit or a creation random draw")
	}
	for index, offset := range w.Whirlpools.Footprint {
		packed := uint16(32|32<<8) + offset
		if w.nativeTileAt(int(uint8(packed)), int(uint8(packed>>8))) != uint8(0x98+index) {
			t.Fatal("native terrain quad not stamped at creation")
		}
	}
	before = w.Core.Magnets[0].Mana
	if w.Cast(0, Whirlpool, Target{X: 32, Y: 32}) || w.Core.Magnets[0].Mana != before {
		t.Fatal("animated water quad passed exact-zero admission or failed cast spent mana")
	}
	if linked, _ := w.Occupancy.Linked(nativeActorReference(NativeEffectPool, 0)); linked {
		t.Fatal("unmapped whirlpool controller entered actor occupancy")
	}
}

func TestNativeWaterEffectsSaveAndCameraIndependentState(t *testing.T) {
	w := oceanWorld(t, 4311)
	if !w.Cast(0, Whirlpool, Target{X: 32, Y: 32}) || !w.Cast(0, Basalt, Target{X: 10, Y: 10, Direction: 2}) {
		t.Fatal("native water cast rejected")
	}
	for range 25 {
		w.tickNativeEffects()
	}
	restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	w.SetEffectView(30, 30)
	restored.SetEffectView(0, 0)
	for range 100 {
		w.tickNativeEffects()
		restored.tickNativeEffects()
		if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
			t.Fatal("saved water pool, graph, pressure, terrain or randomness changed")
		}
	}
	if len(w.TakeEffectSoundCues()) == 0 || len(restored.TakeEffectSoundCues()) != 0 {
		t.Fatal("native view-gated audio entered shared simulation state")
	}
}

func worldEffectPoolHash(w *World) string {
	var raw [NativeEffectCapacity * 32]byte
	for index, actor := range w.NativeEffects {
		at := raw[index*32 : (index+1)*32]
		entry := w.Occupancy.Effects[index].Record
		at[0] = actor.Kind
		binary.BigEndian.PutUint16(at[2:], uint16(entry.Next))
		binary.BigEndian.PutUint16(at[4:], uint16(entry.Previous))
		binary.BigEndian.PutUint16(at[6:], uint16(actor.X))
		binary.BigEndian.PutUint16(at[8:], uint16(actor.Y))
		binary.BigEndian.PutUint16(at[10:], uint16(actor.Animation))
		if actor.Active {
			at[12] = actor.Player + 1
		}
		binary.BigEndian.PutUint16(at[14:], uint16(actor.VX))
		binary.BigEndian.PutUint16(at[16:], uint16(actor.VY))
		at[18] = actor.Speed
		binary.BigEndian.PutUint16(at[20:], uint16(actor.Timer))
		at[22] = actor.State
		binary.BigEndian.PutUint16(at[24:], uint16(actor.Life))
		binary.BigEndian.PutUint16(at[26:], w.BasaltState.Directions[index])
	}
	return fmt.Sprintf("%x", sha256.Sum256(raw[:]))
}

func worldNativeGridHash(w *World) string {
	var raw [16384]byte
	for index, cell := range w.Occupancy.Grid.Cells {
		raw[index*4], raw[index*4+1] = cell.Header, cell.Tile
		binary.BigEndian.PutUint16(raw[index*4+2:], uint16(cell.Head))
	}
	return fmt.Sprintf("%x", sha256.Sum256(raw[:]))
}

func TestBasaltWorldAgainstNativeOrderedPoolTraces(t *testing.T) {
	cases := 0
	for _, fixture := range nativeBasaltFixtures(t) {
		if len(fixture.Trace) == 0 {
			continue
		}
		cases++
		t.Run(fixture.Input.Name, func(t *testing.T) {
			input := fixture.Input
			w := oceanWorld(t, input.Seed)
			for index := range w.NativeEffects {
				w.NativeEffects[index] = NativeEffectActor{Active: index < input.Occupied, VX: -19, VY: 71, Speed: 25}
				w.BasaltState.Directions[index] = 0xeeee
			}
			cells := append([]nativeBasaltCell{{Index: input.X + input.Y*64, Header: input.Header, Tile: input.Tile}}, input.Obstacles...)
			for _, cell := range cells {
				w.Occupancy.Grid.Cells[cell.Index].Header, w.Occupancy.Grid.Cells[cell.Index].Tile = cell.Header, cell.Tile
				if cell.Tile != 0 {
					w.Marks[cell.Index] = Mark{Spell: Flowers, Life: 1, Persistent: true, NativeTile: cell.Tile}
				}
			}
			if input.Follower {
				pos := input.X + input.Y*64
				w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 1000, AtPos: pos, Flags: legacy.OnMove}}
				w.placeActor(NativeFollowerPool, 0, uint16(input.X*256), uint16(input.Y*256))
			}
			_, accepted := w.BasaltRules.Create(&w.NativeEffects, &w.BasaltState, int(input.Owner)-1, input.X, input.Y, input.Life, input.Direction, w.basaltCallbacks())
			if !accepted || worldEffectPoolHash(w) != fixture.InitialPoolSHA256 || worldNativeGridHash(w) != fixture.InitialGridSHA256 || w.Core.Snapshot().RNG != fixture.InitialRNG {
				t.Fatal("world basalt creation differs from complete native pool/grid/RNG")
			}
			next := 0
			for tick := 1; next < len(fixture.Trace); tick++ {
				w.tickNativeEffects()
				if golden := fixture.Trace[next]; golden.Tick == tick {
					if worldEffectPoolHash(w) != golden.PoolSHA256 || worldNativeGridHash(w) != golden.GridSHA256 || w.Core.Snapshot().RNG != golden.RNG {
						t.Fatalf("pass %d world ordered pool, pressure/heads, tiles or RNG differ", tick)
					}
					next++
				}
			}
		})
	}
	if cases != 40 {
		t.Fatal("world native basalt trace coverage incomplete")
	}
}

func TestWorldWhirlwindChildrenAgainstCompleteNativePoolPasses(t *testing.T) {
	for _, fixture := range nativeWhirlwindWaterFixtures(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			w := oceanWorld(t, fixture.Seed)
			w.Experience[0][Air], w.Experience[0][Water] = fixture.AirExperience, fixture.WaterExperience
			if !w.Cast(0, Whirlwind, Target{X: fixture.Target[0], Y: fixture.Target[1]}) {
				t.Fatal("native parent creation rejected")
			}
			compare := func(snapshot nativeWhirlwindWaterSnapshot) {
				t.Helper()
				if worldEffectPoolHash(w) != snapshot.PoolSHA256 || worldNativeGridHash(w) != snapshot.GridSHA256 || w.Core.Snapshot().RNG != snapshot.RNG {
					t.Fatalf("pass %d combined world pool/grid/RNG differs: pool %s want %s; grid %s want %s; rng %x want %x", snapshot.Tick, worldEffectPoolHash(w), snapshot.PoolSHA256, worldNativeGridHash(w), snapshot.GridSHA256, w.Core.Snapshot().RNG, snapshot.RNG)
				}
			}
			compare(fixture.Initial)
			next := 0
			for tick := 1; tick <= fixture.Passes; tick++ {
				w.tickNativeEffects()
				if next < len(fixture.Snapshots) && fixture.Snapshots[next].Tick == tick {
					compare(fixture.Snapshots[next])
					next++
				}
			}
		})
	}
}
