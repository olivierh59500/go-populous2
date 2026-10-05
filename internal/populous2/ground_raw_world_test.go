package populous2

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	legacy "go-populous2/internal/legacy"
)

func TestWorldGroundEffectsAgainstCompleteNativeMemory(t *testing.T) {
	data, err := os.ReadFile("testdata/ground_native_full.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []groundNativeFixture }
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
			castOnly := true
			for _, operation := range input.Operations {
				castOnly = castOnly && (operation.Kind == "font" || operation.Kind == "swamp")
			}
			if castOnly {
				// Creators inspect raw occupancy words without traversing them.
				// Dangling and reserved-slot heads are valid admission fixtures.
				for pos := range 4096 {
					setup[0xf46+pos*4], setup[0xf47+pos*4] = 0, 0
				}
			}
			w := installNativeFixtureWorld(t, setup, nil)
			if castOnly {
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
					cb := w.nativeGroundCallbacks(Baptism, int(input.Owner)-1)
					cb.SourceD2Upper = input.SourceD2Upper
					ref := NativeRecordReference(operation.Reference)
					switch operation.Kind {
					case "font", "swamp":
						id := Baptism
						if operation.Kind == "swamp" {
							id = Swamp
						}
						_, err = w.NativeGround.Create(id, input.Owner, input.X, input.Y, cb)
					case "prepass":
						_, err = w.CommonPrepass.Tick(ref, cb.Prepass)
					case "consumer":
						_, err = w.NativeGround.TickFollower(ref, cb)
					default:
						t.Fatalf("unknown native ground operation %s", operation.Kind)
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
						t.Fatalf("World complete ground memory/RNG differs at frame%d", frameIndex)
					}
				}
			}
		})
	}
	if len(catalog.Cases) != 5430 || frames != 28062 {
		t.Fatalf("World ground coverage: %d cases/%d frames", len(catalog.Cases), frames)
	}
}

func TestGroundAnimationsUseFollowerPassAndSurviveSave(t *testing.T) {
	w := flatGroundWorld(t)
	w.NativeGameMode = 8
	w.Core.Peeps = []legacy.Peep{
		{Player: 0, Population: 100, AtPos: 2000, Flags: legacy.OnMove, MovementSpeed: 20},
		{Player: 1, Population: 200, AtPos: 2001, Flags: legacy.OnMove, MovementSpeed: 20},
	}
	w.Marks[2000] = Mark{Spell: Baptism, Life: 1, Persistent: true, NativeTile: 143}
	w.Marks[2001] = Mark{Spell: Swamp, Life: 1, Persistent: true, NativeTile: 168}
	for index := range w.Core.Peeps {
		w.initializeNativeFollower(index)
	}
	w.Core.TickWithComputer([2]bool{})
	font, _ := w.RecordImage.ReadFollowerEntry(nativeActorReference(NativeFollowerPool, 0))
	swamp, _ := w.RecordImage.ReadFollowerEntry(nativeActorReference(NativeFollowerPool, 1))
	if font.Motion.State != 0x36 || font.Owner != 1 || font.Motion.Population != 100 {
		t.Fatal("ordinary follower pass skipped delayed conversion")
	}
	if swamp.Motion.State != 0x38 || swamp.Owner != 2 || swamp.Motion.Population != 0 {
		t.Fatal("ordinary follower pass skipped retained swamp death")
	}
	restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil {
		t.Fatal(err)
	}
	for range 80 {
		w.Core.TickWithComputer([2]bool{})
		restored.Core.TickWithComputer([2]bool{})
	}
	if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
		t.Fatal("saved ground animations diverged after the native follower pass")
	}
}
