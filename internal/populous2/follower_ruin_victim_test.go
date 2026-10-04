package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// Original full-dispatch runs cover the creator's kind2/kind4 victims, both
// owners, static image retention, the entire400-update lifetime, signed word
// overflow, bad raster removal, leaders and water-prepass replacement.
func TestFollowerRuinVictimAgainstOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_ruin_victim_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []aftermathFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 101 {
		t.Fatal("native retained victim catalog incomplete")
	}
	rules, err := DecodeFollowerRuinVictimRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	updates := 0
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			m := aftermathMemory(fixture)
			assertAftermathFrame(t, m, fixture.Initial)
			memory := FollowerCleanupMemory{Read8: func(at int) (uint8, error) { return m.bytes[at], nil }, Read16: func(at int) (uint16, error) { return m.word(at), nil }, Read32: func(at int) (uint32, error) { return m.long(at), nil }, Write8: func(at int, v uint8) error { m.bytes[at] = v; return nil }, Write16: func(at int, v uint16) error { m.putWord(at, v); return nil }, Write32: func(at int, v uint32) error { m.putLong(at, v); return nil }}
			for _, native := range fixture.Trace {
				updates++
				eventIndex := 0
				call := func(name string, ref NativeRecordReference, operation func() error) error {
					if eventIndex >= len(native.Events) {
						return fmt.Errorf("unexpected retained victim callback%s", name)
					}
					e := native.Events[eventIndex]
					eventIndex++
					if e.Name != name || e.Reference != uint16(ref) || aftermathRawHash(m) != e.BeforeSHA256 {
						return fmt.Errorf("native%s input differs at update%d", name, native.Tick)
					}
					if operation == nil {
						for _, change := range e.Changes {
							m.bytes[change.Offset] = change.Value
						}
					} else if err := operation(); err != nil {
						return err
					}
					if aftermathRawHash(m) != e.AfterSHA256 {
						return fmt.Errorf("native%s output differs at update%d", name, native.Tick)
					}
					return nil
				}
				cb := FollowerRuinVictimCallbacks{Memory: memory,
					Prepass: func(ref NativeRecordReference) error { return call("Prepass", ref, nil) },
					Tile: func(tile NativePackedTile) (uint8, error) {
						x, y := int(uint8(tile)), int(uint8(uint16(tile)>>8))
						return m.bytes[0xf44+(x+y*64)*4+1], nil
					},
					ClearLeader: func(ref NativeRecordReference) error {
						return call("ClearLeader", ref, func() error {
							_, err := ClearFollowerLeader(ref, FollowerCleanupRegisters{}, FollowerLeaderCallbacks{Memory: memory, Unlink: func(ref NativeRecordReference) error { return aftermathUnlink(m, ref) }, Insert: func(ref NativeRecordReference) error { m.insert(cleanupRecordAddress(ref)); return nil }})
							return err
						})
					},
					Unlink: func(ref NativeRecordReference) error {
						return call("Unlink", ref, func() error { return aftermathUnlink(m, ref) })
					},
				}
				step, err := rules.Tick(52, cb)
				if err != nil {
					t.Fatal(err)
				}
				if eventIndex != len(native.Events) {
					t.Fatalf("native callback omitted at update%d", native.Tick)
				}
				assertAftermathFrame(t, m, native)
				if native.Exit == "12462" && (!step.Handled || !step.NextFollower) || native.Exit == "11d1a" && step.Handled {
					t.Fatalf("native retained victim dispatch differs: %+v", step)
				}
			}
		})
	}
	if updates != 540 {
		t.Fatalf("native retained victim trajectory count differs: %d", updates)
	}
}

func TestFollowerRuinVictimStaticArtExistsInEveryLandscape(t *testing.T) {
	b := testBundle(t)
	rules, err := DecodeFollowerRuinVictimRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules.Frame.Layers) == 0 || rules.Frame.SoundCue < 0 || rules.Frame.SoundCue >= 133 {
		t.Fatal("native retained victim composite missing")
	}
	for _, bank := range b.Sprites {
		for _, layer := range rules.Frame.Layers {
			if layer.Sprite < 0 || layer.Sprite >= len(bank) {
				t.Fatal("native retained victim sprite absent")
			}
		}
	}
}
