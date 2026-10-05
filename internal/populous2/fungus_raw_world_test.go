package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"bytes"
)

func TestWorldFungusAgainstCompleteNativeMemory(t *testing.T) {
	data, err := os.ReadFile("testdata/fungus_native_full.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []fungusNativeFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	frames := 0
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			input := fixture.Input
			m := whirlwindNativeFixtureMemory(input.whirlwindNativeInput)
			raw := whirlwindNativeBytes(m)
			setup := append([]byte(nil), raw...)
			independentOfGraph := true
			for _, operation := range input.Operations {
				independentOfGraph = independentOfGraph && (operation.Kind != "consumer")
			}
			if independentOfGraph {
				// The fungus controller reads raw terrain without traversing heads.
				// Preserve arbitrary raw occupancy words in its memory fixtures.
				for pos := range 4096 {
					setup[0xf46+pos*4], setup[0xf47+pos*4] = 0, 0
				}
			}
			w := installNativeFixtureWorld(t, setup, nil)
			if independentOfGraph {
				for pos := range w.Occupancy.Grid.Cells {
					w.Occupancy.Grid.Cells[pos].Head = NativeRecordReference(binary.BigEndian.Uint16(raw[0xf46+pos*4:]))
				}
			}
			w.Core.SetRandomState(input.Seed)
			w.nativeCallDepth++
			defer func() { w.nativeCallDepth-- }()
			memory := w.nativeCleanupMemory()
			frameIndex := 0
			for _, operation := range input.Operations {
				for _, patch := range operation.Initial {
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
				for range max(1, operation.Repeat) {
					current := nativeFixtureWorldImage(w, raw)
					w.Rules[0] = DecodeScenarioRules(binary.BigEndian.Uint16(current[0xeb2c:]))
					w.Rules[1] = DecodeScenarioRules(binary.BigEndian.Uint16(current[0xeb2e:]))
					ground := w.nativeGroundCallbacks(Fungus, int(input.Owner)-1)
					ground.SourceD2Upper = input.SourceD2Upper
					ref := NativeRecordReference(operation.Reference)
					switch operation.Kind {
					case "create", "createat":
						x, y := input.X, input.Y
						if operation.Kind == "createat" {
							x, y = uint8(operation.Target), uint8(operation.Target>>8)
						}
						_, err = w.NativeFungus.Create(input.Owner, x, y, w.nativeFungusCallbacks())
					case "fungus", "fault":
						_, err = w.NativeFungus.Tick(ref, w.nativeFungusCallbacks())
						if operation.Kind == "fault" {
							if err == nil {
								t.Fatal("native zero age-divisor fault was lost")
							}
							err = nil
						}
					case "consumer":
						_, err = w.NativeFungus.Ground.TickFollower(ref, ground)
					default:
						t.Fatalf("unknown native fungus operation %s", operation.Kind)
					}
					if err != nil {
						t.Fatal(err)
					}
					want := fixture.Frames[frameIndex]
					frameIndex++
					frames++
					all := nativeFixtureWorldImage(w, raw)
					if got := fmt.Sprintf("%x", sha256.Sum256(all)); got != want.Hash || w.Core.RandomState() != want.RNG {
						for _, change := range want.Changes {
							if all[change.Address] != change.Value {
								t.Errorf("native byte%x got%x want%x", change.Address, all[change.Address], change.Value)
							}
						}
						t.Fatalf("World complete fungus memory/RNG differs at frame%d", frameIndex)
					}
				}
			}
		})
	}
	if len(catalog.Cases) != 1465 || frames != 24748 {
		t.Fatalf("World fungus coverage: %d cases/%d frames", len(catalog.Cases), frames)
	}
}

func TestRawFungusPendingMigratesFromSave25WithoutRecasting(t *testing.T) {
	w := flatGroundWorld(t)
	w.Experience[0][Plants] = 96
	for _, x := range []int{31, 32, 33} {
		w.Cast(0, Fungus, Target{X: x, Y: 32})
	}
	for range 35 {
		w.tickNativeEffects()
	}
	old := w.Snapshot()
	old.Version = 25
	for owner := uint8(1); owner <= 2; owner++ {
		god, _ := NativeDeityAddress(owner)
		if _, err := (NativeRuntimeMemory{Records: &old.RecordImage, Globals: &old.NativeGlobals}).Write16(god+14, 0); err != nil {
			t.Fatal(err)
		}
	}
	untouched := old.NativeGlobals
	restored, err := Restore(testBundle(t), old)
	if err != nil {
		t.Fatal(err)
	}
	if old.NativeGlobals != untouched || restored.Core.RandomState() != old.Core.RNG || restored.Core.Magnets[0].Mana != old.Core.Magnets[0].Mana {
		t.Fatal("migration changed caller data, RNG or mana")
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
		t.Fatal("legacy collecting slot did not restore its native deity reference")
	}
	for range 180 {
		w.tickNativeEffects()
		restored.tickNativeEffects()
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
		t.Fatal("migrated native collection or later generation diverged")
	}
}
