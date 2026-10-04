package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type aftermathFixtureEvent struct {
	Name                      string
	Reference, Argument       uint16
	BeforeSHA256, AfterSHA256 string
	Changes                   []struct {
		Offset int
		Value  uint8
	}
}

type aftermathFixtureFrame struct {
	Tick    int
	Exit    string
	Records []struct {
		Reference uint16
		Raw       [52]uint8
	}
	Events                                              []aftermathFixtureEvent
	GridSHA256, OverlaySHA256, ActorsSHA256, GodsSHA256 string
	RNG                                                 uint32
}

type aftermathFixture struct {
	Input struct {
		Name               string
		X, Y               int
		Owner, Flags, Tile uint8
		FullDispatch       bool
		Neighbors          []struct {
			Reference uint16
			X, Y      int
			Previous  uint16
		}
	}
	Initial aftermathFixtureFrame
	Trace   []aftermathFixtureFrame
}

func aftermathMemory(f aftermathFixture) *entryFixtureMemory {
	m := &entryFixtureMemory{}
	for index := range 4096 {
		m.bytes[0xf44+index*4], m.bytes[0xf44+index*4+1], m.bytes[0x4f44+index] = 0xa8, f.Input.Tile, 0x77
	}
	for _, r := range f.Initial.Records {
		at := 0x76c0 + int(int16(r.Reference))
		copy(m.bytes[at:at+52], r.Raw[:])
	}
	m.putWord(0xf42, 123)
	m.putLong(0xeb28, 4311)
	m.putWord(0xf44+(f.Input.X+f.Input.Y*64)*4+2, 52)
	for owner := 1; owner <= 2; owner++ {
		m.putWord(0xe76a+owner*314+10, uint16(0x7080+owner*14))
		m.putWord(0xf44+((owner+8)+(owner+8)*64)*4+2, uint16(0x7080+owner*14))
		if f.Input.Flags&1 != 0 && uint8(owner) == f.Input.Owner {
			m.putWord(0xe76a+owner*314+8, 52)
		}
	}
	for _, n := range f.Input.Neighbors {
		if n.Previous == 0 {
			m.putWord(0xf44+(n.X+n.Y*64)*4+2, n.Reference)
		}
	}
	return m
}

func aftermathUnlink(m *entryFixtureMemory, ref NativeRecordReference) error {
	at := 0x76c0 + int(int16(ref))
	next, previous := m.word(at+2), m.word(at+4)
	if previous != 0 {
		m.putWord(0x76c0+int(int16(previous))+2, next)
	} else {
		x, y := int(m.word(at+6)>>8), int(m.word(at+8)>>8)
		m.putWord(0xf44+(x+y*64)*4+2, next)
	}
	if next != 0 {
		m.putWord(0x76c0+int(int16(next))+4, previous)
	}
	m.putWord(at+2, 0)
	m.putWord(at+4, 0)
	return nil
}

func aftermathRawHash(m *entryFixtureMemory) string {
	return fmt.Sprintf("%x", sha256.Sum256(m.bytes[:]))
}

func assertAftermathFrame(t *testing.T, m *entryFixtureMemory, f aftermathFixtureFrame) {
	t.Helper()
	for _, r := range f.Records {
		at := 0x76c0 + int(int16(r.Reference))
		if !reflect.DeepEqual(m.bytes[at:at+52], r.Raw[:]) {
			t.Fatalf("native record%04x differs at update%d\ngot%x\nwant%x", r.Reference, f.Tick, m.bytes[at:at+52], r.Raw)
		}
	}
	for _, h := range []struct {
		at, length int
		want, name string
	}{{0xf44, 16384, f.GridSHA256, "map"}, {0x4f44, 4096, f.OverlaySHA256, "overlays"}, {0x5f50, 0xe740 - 0x5f50, f.ActorsSHA256, "complete actor pools"}, {0xe740, 0xeb18 - 0xe740, f.GodsSHA256, "magnets/deities"}} {
		if got := fmt.Sprintf("%x", sha256.Sum256(m.bytes[h.at:h.at+h.length])); got != h.want {
			t.Fatalf("native%s differs at update%d: got%s want%s", h.name, f.Tick, got, h.want)
		}
	}
	if m.long(0xeb28) != f.RNG {
		t.Fatal("native aftermath RNG changed")
	}
}

// Ninety independently executed original CPU cases cover1214 dispatches.
// Prepass and leader relocation are explicit external callback contracts:
// their original before/after BSS snapshots are replayed at that boundary.
// Town destruction, settlement cleanup and raw unlink run actual Go helpers.
func TestFollowerAftermathAgainstOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_aftermath_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []aftermathFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 90 {
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
	if updates != 1214 {
		t.Fatalf("native aftermath trajectory count differs: %d", updates)
	}
}

func TestFollowerAftermathArtExistsInEveryLandscape(t *testing.T) {
	b := testBundle(t)
	rules, err := DecodeFollowerAftermathRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for pointer, frame := range rules.Frames {
		if len(frame.Layers) == 0 || frame.SoundCue < 0 || frame.SoundCue >= 133 {
			t.Fatalf("native aftermath composite/cue absent at%04x", pointer)
		}
		for _, bank := range b.Sprites {
			for _, layer := range frame.Layers {
				if layer.Sprite < 0 || layer.Sprite >= len(bank) {
					t.Fatalf("native aftermath sprite absent at%04x", pointer)
				}
			}
		}
	}
}
