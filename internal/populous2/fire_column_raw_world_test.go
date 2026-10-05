package populous2

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	legacy "go-populous2/internal/legacy"
)

func TestWorldFireColumnAgainstCompleteNativeController(t *testing.T) {
	data, err := os.ReadFile("testdata/fire_column_native_full.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []fireColumnNativeFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	frames := 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			m := fireColumnNativeFixtureMemory(f.Input)
			raw := whirlwindNativeBytes(m)
			w := installNativeFixtureWorld(t, raw, nil)
			s := w.Core.Snapshot()
			s.RNG = f.Input.Seed
			w.Core = legacy.WorldFromSnapshot(s, w.Core.Rules)
			w.nativeCallDepth++
			memory := w.nativeCleanupMemory()
			cb := w.nativeFireColumnCallbacks()
			frameIndex := 0
			for _, op := range f.Input.Operations {
				for _, p := range op.Initial {
					switch p.Width {
					case 1:
						err = memory.Write8(p.Address, uint8(p.Value))
					case 2:
						err = memory.Write16(p.Address, uint16(p.Value))
					case 4:
						err = memory.Write32(p.Address, p.Value)
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				for range max(1, op.Repeat) {
					ref := NativeRecordReference(op.Reference)
					switch op.Kind {
					case "create":
						_, err = w.PrimitiveCreators.CreateFireColumn(f.Input.Owner, f.Input.X, f.Input.Y, w.nativePrimitiveCallbacks())
					case "scorch":
						err = w.NativeFireColumn.Storm.Scorch(ref, memory)
					case "damage":
						_, err = w.NativeFireColumn.Storm.Damage(ref, w.stormCallbacks())
					case "town":
						err = w.nativeDestroyTown(ref)
					case "burnneighbors":
						err = w.ForestNative.BurnNeighbors(ref, w.nativeForestCallbacks())
					case "tree":
						_, err = w.ForestNative.Tick(ref, uint16(w.NativeClock), w.nativeForestCallbacks())
					case "tick":
						_, err = w.NativeFireColumn.Tick(ref, cb)
					case "faulttick":
						_, err = w.NativeFireColumn.Tick(ref, cb)
						if err == nil {
							t.Fatal("native zero-speed fault lost")
						}
						err = nil
					default:
						t.Fatal("unknown fire column native operation")
					}
					if err != nil {
						t.Fatal(err)
					}
					want := f.Frames[frameIndex]
					frameIndex++
					frames++
					all := nativeFixtureWorldImage(w, raw)
					if got := fmt.Sprintf("%x", sha256.Sum256(all)); got != want.Hash || w.Core.RandomState() != want.RNG {
						for _, change := range want.Changes {
							if all[change.Address] != change.Value {
								t.Errorf("native byte%x got%x want%x", change.Address, all[change.Address], change.Value)
							}
						}
						t.Fatalf("World complete fire column memory/RNG differs at frame%d", frameIndex)
					}
				}
			}
			w.nativeCallDepth--
		})
	}
	if len(catalog.Cases) != 1169 || frames != 12094 {
		t.Fatalf("World fire column coverage: %d cases/%d frames", len(catalog.Cases), frames)
	}
}

func TestRawFireColumnMixedDeathAndSavedContinuation(t *testing.T) {
	w := lightningWorld(t, 4311)
	w.bindNativeTownEvaluator()
	w.NativeGameMode = 8
	pos := 32 + 32*64
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 100, AtPos: pos, Flags: legacy.OnMove, MovementSpeed: 20}, {Player: 1, Population: 200, AtPos: pos, Flags: legacy.OnMove, MovementSpeed: 20}, {Player: 1, Population: 1000, AtPos: pos, Flags: legacy.InTown, TownStage: 1, MovementSpeed: 20}}
	for i := range w.Core.Peeps {
		w.initializeNativeFollower(i)
	}
	if err := w.runNativeFollowerCall(func() error {
		step, err := w.PrimitiveCreators.CreateFireColumn(1, 32, 32, w.nativePrimitiveCallbacks())
		if err != nil {
			return err
		}
		a := cleanupRecordAddress(step.Reference)
		m := w.nativeCleanupMemory()
		if _, err := w.nativeFireColumnCallbacks().Move(step.Reference, 0x2080, 0x2080); err != nil {
			return err
		}
		for _, f := range []struct {
			off int
			v   uint16
		}{{6, 0x2080}, {8, 0x2080}, {14, 0}, {16, 0}, {20, 20}, {24, 100}, {10, 0x4b8}} {
			if err := m.Write16(a+f.off, f.v); err != nil {
				return err
			}
		}
		return m.Write8(a+22, 4)
	}); err != nil {
		t.Fatal(err)
	}
	w.tickNativeEffects()
	for index := 0; index < 2; index++ {
		a, _ := w.RecordImage.ReadFollowerEntry(nativeActorReference(NativeFollowerPool, index))
		if a.Motion.Population != 0 || a.Owner == 0 || a.Motion.State != 8 || !w.Occupancy.Followers[index].Linked {
			t.Fatal("walker death lost native reservation or linked state")
		}
	}
	if len(w.FlameDeaths) != 0 {
		t.Fatal("raw controller used inherited mortality sidecar")
	}
	copy, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	for range 50 {
		w.Core.TickWithComputer([2]bool{})
		copy.Core.TickWithComputer([2]bool{})
		w.tickNativeEffects()
		copy.tickNativeEffects()
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, copy)) {
		t.Fatal("mixed native fire/death save continuation differs")
	}
}
