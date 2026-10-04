package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// These supplementary original CPU traces exercise raw dispatch aliases and
// hero animation pointers independently of the production family mapping.
func TestFollowerAftermathAliasesAgainstOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_aftermath_alias_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []aftermathFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 84 {
		t.Fatal("native aftermath catalog incomplete")
	}
	b := testBundle(t)
	rules, err := DecodeFollowerAftermathRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	towns, err := DecodeTownCombatRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := DecodeNativeTownEvaluator(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	updates := 0
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			m := aftermathMemory(fixture)
			assertAftermathFrame(t, m, fixture.Initial)
			evalCallbacks := townCombatEvaluatorCallbacks(m)
			clear := func(ref NativeRecordReference, tile uint8) error {
				return evaluator.ClearFarms(ref, tile, evalCallbacks)
			}
			memory := FollowerCleanupMemory{Read8: func(at int) (uint8, error) { return m.bytes[at], nil }, Read16: func(at int) (uint16, error) { return m.word(at), nil }, Read32: func(at int) (uint32, error) { return m.long(at), nil }, Write8: func(at int, v uint8) error { m.bytes[at] = v; return nil }, Write16: func(at int, v uint16) error { m.putWord(at, v); return nil }, Write32: func(at int, v uint32) error { m.putLong(at, v); return nil }}
			cleanup := func(ref NativeRecordReference, mode uint16) error {
				_, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: uint32(mode)}, FollowerCleanupCallbacks{Memory: memory, ClearFarms: clear, Unlink: func(ref NativeRecordReference) error { return aftermathUnlink(m, ref) }, Insert: func(ref NativeRecordReference) error { m.insert(0x76c0 + int(int16(ref))); return nil }})
				return err
			}
			for _, native := range fixture.Trace {
				updates++
				eventIndex := 0
				call := func(name string, ref NativeRecordReference, operation func() error) error {
					if eventIndex >= len(native.Events) {
						return fmt.Errorf("unexpected native callback%s", name)
					}
					e := native.Events[eventIndex]
					eventIndex++
					if e.Name != name || e.Reference != uint16(ref) || aftermathRawHash(m) != e.BeforeSHA256 {
						return fmt.Errorf("native callback%s input differs at update%d", name, native.Tick)
					}
					if operation == nil {
						for _, change := range e.Changes {
							m.bytes[change.Offset] = change.Value
						}
					} else if err := operation(); err != nil {
						return err
					}
					if aftermathRawHash(m) != e.AfterSHA256 {
						return fmt.Errorf("native callback%s output differs at update%d", name, native.Tick)
					}
					return nil
				}
				cb := FollowerAftermathCallbacks{Read: m.read, Write: m.write,
					Head: func(tile NativePackedTile) (NativeRecordReference, error) {
						x, y := int(uint8(tile)), int(uint8(uint16(tile)>>8))
						return NativeRecordReference(m.word(0xf44 + (x+y*64)*4 + 2)), nil
					},
					Tile: func(tile NativePackedTile) (uint8, error) {
						x, y := int(uint8(tile)), int(uint8(uint16(tile)>>8))
						return m.bytes[0xf44+(x+y*64)*4+1], nil
					},
					ClearLeader: func(ref NativeRecordReference) error { return call("ClearLeader", ref, nil) },
					Unlink: func(ref NativeRecordReference) error {
						return call("Unlink", ref, func() error { return aftermathUnlink(m, ref) })
					},
					Cleanup: func(ref NativeRecordReference, mode uint16) error {
						return call("Cleanup", ref, func() error { return cleanup(ref, mode) })
					},
					DestroyTown: func(ref NativeRecordReference) error {
						return call("DestroyTown", ref, func() error {
							_, err := towns.Destroy(ref, TownCombatCallbacks{Read: m.read, Write: m.write, ClearFarms: clear, Cleanup: cleanup})
							return err
						})
					},
				}
				if fixture.Input.FullDispatch {
					cb.Prepass = func(ref NativeRecordReference) error { return call("Prepass", ref, nil) }
				}
				step, err := rules.Tick(52, cb)
				if err != nil {
					t.Fatal(err)
				}
				if eventIndex != len(native.Events) {
					t.Fatalf("native external callback omitted at update%d", native.Tick)
				}
				assertAftermathFrame(t, m, native)
				switch native.Exit {
				case "123b4":
					if !step.Handled || !step.CurrentTotal || step.NextFollower {
						t.Fatalf("native totals boundary differs: %+v", step)
					}
				case "12462":
					if !step.Handled || !step.NextFollower || step.CurrentTotal {
						t.Fatalf("native next-follower boundary differs: %+v", step)
					}
				case "1131c", "11d1a", "12044":
					if step.Handled {
						t.Fatalf("external native state substituted: %+v", step)
					}
				default:
					t.Fatalf("unknown original exit%s", native.Exit)
				}
				if step.DeferredSearch {
					a, _ := m.read(52)
					if native.Exit != "123b4" || a.Motion.State != 2 || a.Motion.Animation != 0 {
						t.Fatal("winner decision happened before native deferred boundary")
					}
				}
			}
		})
	}
	if updates != 496 {
		t.Fatalf("native aftermath trajectory count differs: %d", updates)
	}
}
