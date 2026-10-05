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

func TestWorldWhirlwindFullLifecycleAgainstCompleteNativeMemory(t *testing.T) {
	data, err := os.ReadFile("testdata/whirlwind_native_full.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []whirlwindNativeFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	frames := 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			m := whirlwindNativeFixtureMemory(f.Input)
			raw := whirlwindNativeBytes(m)
			w := installNativeFixtureWorld(t, raw, nil)
			s := w.Core.Snapshot()
			s.RNG = f.Input.Seed
			w.Core = legacy.WorldFromSnapshot(s, w.Core.Rules)
			w.nativeCallDepth++
			cb := w.nativeWhirlwindCallbacks()
			cb.SourceD2 = f.Input.Owner
			memory := w.nativeCleanupMemory()
			frameIndex := 0
			for _, op := range f.Input.Operations {
				for _, patch := range op.Initial {
					switch patch.Width {
					case 1:
						err = memory.Write8(patch.Address, uint8(patch.Value))
					case 2:
						err = memory.Write16(patch.Address, uint16(patch.Value))
					case 4:
						err = memory.Write32(patch.Address, patch.Value)
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				for range max(1, op.Repeat) {
					ref := NativeRecordReference(op.Reference)
					switch op.Kind {
					case "create":
						_, err = w.PrimitiveCreators.CreateWhirlwind(f.Input.Owner, f.Input.X, f.Input.Y, w.nativePrimitiveCallbacks())
					case "water":
						_, err = w.NativeWhirlwind.CreateWaterChild(f.Input.Owner, f.Input.X, f.Input.Y, memory)
					case "lift":
						_, err = w.NativeWhirlwind.Lift(ref, NativeRecordReference(op.Target), memory)
					case "release":
						_, err = w.NativeWhirlwind.Release(ref, cb)
					case "tick":
						_, err = w.NativeWhirlwind.Tick(ref, cb)
					case "follower":
						_, err = w.NativeWhirlwind.TickFollower(ref, cb)
					default:
						t.Fatal("unknown native whirlwind operation")
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
						t.Fatalf("World complete whirlwind memory/RNG differs at frame%d", frameIndex)
					}
				}
			}
			w.nativeCallDepth--
		})
	}
	if len(catalog.Cases) != 789 || frames != 19296 {
		t.Fatalf("World whirlwind coverage: cases%d frames%d", len(catalog.Cases), frames)
	}
}

func TestWorldWhirlwindLiftTransportReleaseAndSave(t *testing.T) {
	b := testBundle(t)
	w := lightningWorld(t, 4311)
	w.bindNativeTownEvaluator()
	w.NativeGameMode = 8
	pos := 32 + 32*64
	w.Core.Peeps = []legacy.Peep{{Player: 0, Population: 100, AtPos: pos, Flags: legacy.OnMove, MovementSpeed: 20}, {Player: 1, Population: 200, AtPos: pos, Flags: legacy.InTown, TownStage: 1, MovementSpeed: 20}}
	w.initializeNativeFollower(0)
	w.initializeNativeFollower(1)
	if !w.Cast(0, Whirlwind, Target{X: 32, Y: 32}) {
		t.Fatal("whirlwind rejected")
	}
	if err := w.runNativeFollowerCall(func() error {
		_, err := w.NativeWhirlwind.Lift(nativeActorReference(NativeEffectPool, 0), 52, w.nativeCleanupMemory())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if !w.NativeEntries[0].Managed || w.NativeEntries[0].Actor.Motion.State != 0x14 {
		t.Fatal("native lifted dispatch missing")
	}
	if _, ok := w.ManagedFollowerFrame(0); !ok {
		t.Fatal("native lifted frame missing")
	}
	copy, err := ReadSave(b, bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	for range 10 {
		w.Core.TickWithComputer([2]bool{})
		copy.Core.TickWithComputer([2]bool{})
		w.tickNativeEffects()
		copy.tickNativeEffects()
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, copy)) {
		t.Fatal("native lifted save continuation differs")
	}
	if err := w.runNativeFollowerCall(func() error {
		_, err := w.NativeWhirlwind.Release(nativeActorReference(NativeEffectPool, 0), w.nativeWhirlwindCallbacks())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	a, _ := w.RecordImage.ReadFollowerEntry(52)
	if a.Owner != 0 && a.Motion.State != 0x1a {
		t.Fatal("native release did not use landing state")
	}
	for range 12 {
		w.Core.TickWithComputer([2]bool{})
	}
	if a.Owner != 0 {
		a, _ = w.RecordImage.ReadFollowerEntry(52)
		if a.Motion.Kind == 8 {
			t.Fatal("landing never resumed walker")
		}
	}
}
