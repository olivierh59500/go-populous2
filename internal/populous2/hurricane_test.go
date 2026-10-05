package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type hurricaneFixtureRecord struct {
	Reference uint16
	Raw       [52]uint8
}
type hurricaneFixtureEvent struct {
	Name                      string
	Reference, X, Y           uint16
	BeforeSHA256, AfterSHA256 string
}
type hurricaneFixtureFrame struct {
	Tick                                           int
	Hash, GridSHA256, OverlaySHA256, RecordsSHA256 string
	Records                                        []hurricaneFixtureRecord
	RNG                                            uint32
	Events                                         []hurricaneFixtureEvent
}
type hurricaneFixture struct {
	Input struct {
		Name        string
		Owner, X, Y uint8
		Direction   uint16
		Ticks       int
		Full        bool
		God0Pointer uint32
		Victims     []struct {
			Reference                        uint16
			Kind, Owner, Flags, State, Stage uint8
			X, Y                             uint16
			Population                       int32
			Hero                             uint16
		}
	}
	Initial, Creation hurricaneFixtureFrame
	InitialRecords    []hurricaneFixtureRecord
	Created           uint16
	Admitted          bool
	Trace             []hurricaneFixtureFrame
}

func hurricaneGrid(m *entryFixtureMemory) NativeOccupancyState {
	var grid NativeOccupancyState
	for i := range grid.Cells {
		at := 0xf44 + i*4
		grid.Cells[i] = NativeOccupancyCell{Header: m.bytes[at], Tile: m.bytes[at+1], Head: NativeRecordReference(m.word(at + 2))}
	}
	return grid
}
func hurricaneAccess(m *entryFixtureMemory) NativeRecordAccess {
	return NativeRecordAccess{
		Record: func(ref NativeRecordReference) (NativeOccupancyRecord, bool) {
			at := cleanupRecordAddress(ref)
			return NativeOccupancyRecord{Next: NativeRecordReference(m.word(at + 2)), Previous: NativeRecordReference(m.word(at + 4)), X: m.word(at + 6), Y: m.word(at + 8)}, true
		},
		SetLinks: func(ref, next, previous NativeRecordReference) {
			at := cleanupRecordAddress(ref)
			m.putWord(at+2, uint16(next))
			m.putWord(at+4, uint16(previous))
		},
		SetPosition: func(ref NativeRecordReference, x, y uint16) {
			at := cleanupRecordAddress(ref)
			m.putWord(at+6, x)
			m.putWord(at+8, y)
		},
	}
}
func hurricaneStoreGrid(m *entryFixtureMemory, grid *NativeOccupancyState) {
	for i, c := range grid.Cells {
		at := 0xf44 + i*4
		m.bytes[at], m.bytes[at+1] = c.Header, c.Tile
		m.putWord(at+2, uint16(c.Head))
	}
}
func hurricaneRemoveFixture(m *entryFixtureMemory, ref NativeRecordReference) error {
	grid := hurricaneGrid(m)
	err := grid.Remove(ref, hurricaneAccess(m))
	hurricaneStoreGrid(m, &grid)
	return err
}
func hurricaneInsertFixture(m *entryFixtureMemory, ref NativeRecordReference) error {
	grid := hurricaneGrid(m)
	at := cleanupRecordAddress(ref)
	err := grid.Insert(ref, int(m.word(at+6)>>8), int(m.word(at+8)>>8), hurricaneAccess(m))
	hurricaneStoreGrid(m, &grid)
	return err
}

func hurricaneMemory(f hurricaneFixture) *entryFixtureMemory {
	m := &entryFixtureMemory{}
	for i := range 4096 {
		m.bytes[0xf44+i*4], m.bytes[0xf44+i*4+1], m.bytes[0x4f44+i] = 0xa8, 15, 0x77
	}
	for slot := 0; slot < 250; slot++ {
		at := 0xc800 + slot*32
		for j := 0; j < 32; j++ {
			m.bytes[at+j] = uint8(0x80 + j)
		}
		m.bytes[at+12] = 0
		if f.Input.Full {
			m.bytes[at+12] = 1
		}
	}
	m.putLong(0xeb28, 4311)
	m.putLong(0xe76a, f.Input.God0Pointer)
	for owner := 1; owner <= 2; owner++ {
		marker := 0xe740 + owner*14
		m.putWord(0xe76a+owner*314+10, uint16(marker-0x76c0))
		m.bytes[marker], m.bytes[marker+12] = 0x14, uint8(owner)
		m.putWord(marker+6, 0x2080)
		m.putWord(marker+8, 0x2080)
		_ = hurricaneInsertFixture(m, NativeRecordReference(marker-0x76c0))
	}
	for _, v := range f.Input.Victims {
		at := cleanupRecordAddress(NativeRecordReference(v.Reference))
		if v.Reference == 0x708e || v.Reference == 0x709c {
			_ = hurricaneRemoveFixture(m, NativeRecordReference(v.Reference))
		}
		m.bytes[at], m.bytes[at+1], m.bytes[at+12], m.bytes[at+13], m.bytes[at+22] = v.Kind, v.Stage, v.Owner, v.Flags, v.State
		m.putWord(at+6, v.X)
		m.putWord(at+8, v.Y)
		m.putWord(at+10, 0x744)
		m.putWord(at+20, 23)
		m.putLong(at+26, uint32(v.Population))
		m.putWord(at+40, v.Hero)
		if v.Flags&1 != 0 {
			m.putWord(0xe76a+int(v.Owner)*314+8, v.Reference)
		}
		if v.Kind == 4 {
			farm := uint8(47)
			if v.Owner != 1 {
				farm = 63
			}
			x, y := int(v.X>>8), int(v.Y>>8)
			for yy := max(0, y-3); yy <= min(63, y+3); yy++ {
				for xx := max(0, x-3); xx <= min(63, x+3); xx++ {
					m.bytes[0xf44+(xx+yy*64)*4+1] = farm
				}
			}
		}
		_ = hurricaneInsertFixture(m, NativeRecordReference(v.Reference))
	}
	return m
}

func assertHurricaneFrame(t *testing.T, m *entryFixtureMemory, f hurricaneFixtureFrame) {
	t.Helper()
	for _, h := range []struct {
		at, size   int
		want, name string
	}{{0, 65536, f.Hash, "complete BSS"}, {0xf44, 16384, f.GridSHA256, "complete map"}, {0x4f44, 4096, f.OverlaySHA256, "overlay plane"}, {0x5f50, 0xeb18 - 0x5f50, f.RecordsSHA256, "all actor pools/deities"}} {
		if got := fmt.Sprintf("%x", sha256.Sum256(m.bytes[h.at:h.at+h.size])); got != h.want {
			t.Fatalf("native%s differs at update%d: got%s want%s", h.name, f.Tick, got, h.want)
		}
	}
	if m.long(0xeb28) != f.RNG {
		t.Fatal("hurricane consumed randomness")
	}
}

func TestHurricaneAgainstOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/hurricane_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []hurricaneFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 47 {
		t.Fatal("native hurricane catalog incomplete")
	}
	b := testBundle(t)
	rules, err := DecodeHurricaneRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := DecodeNativeTownEvaluator(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	updates := 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			m := hurricaneMemory(f)
			assertHurricaneFrame(t, m, f.Initial)
			memory := FollowerCleanupMemory{Read8: func(at int) (uint8, error) { return m.bytes[at], nil }, Read16: func(at int) (uint16, error) { return m.word(at), nil }, Read32: func(at int) (uint32, error) { return m.long(at), nil }, Write8: func(at int, v uint8) error { m.bytes[at] = v; return nil }, Write16: func(at int, v uint16) error { m.putWord(at, v); return nil }, Write32: func(at int, v uint32) error { m.putLong(at, v); return nil }}
			created, err := rules.Create(f.Input.Owner, f.Input.X, f.Input.Y, f.Input.Direction, HurricaneCallbacks{Memory: memory})
			if err != nil {
				t.Fatal(err)
			}
			if created.Admitted != f.Admitted || f.Admitted && uint16(created.Reference) != f.Created {
				t.Fatal("native hurricane creator admission/slot differs")
			}
			assertHurricaneFrame(t, m, f.Creation)
			for _, native := range f.Trace {
				updates++
				eventIndex := 0
				call := func(name string, ref NativeRecordReference, x, y uint16, operation func() error) error {
					if eventIndex >= len(native.Events) {
						return fmt.Errorf("unexpected hurricane callback%s", name)
					}
					e := native.Events[eventIndex]
					eventIndex++
					if e.Name != name || e.Reference != uint16(ref) || name == "Move" && (e.X != x || e.Y != y) || aftermathRawHash(m) != e.BeforeSHA256 {
						return fmt.Errorf("native%s input differs", name)
					}
					if err := operation(); err != nil {
						return err
					}
					if aftermathRawHash(m) != e.AfterSHA256 {
						return fmt.Errorf("native%s output differs", name)
					}
					return nil
				}
				cb := HurricaneCallbacks{Memory: memory, PointerBase: 0x200000, WriteOverlay: func(index int, value uint8) error { m.bytes[0x4f44+index] = value; return nil },
					Move: func(ref NativeRecordReference, x, y uint16) error {
						return call("Move", ref, x, y, func() error {
							grid := hurricaneGrid(m)
							_, err := grid.Move(ref, x, y, hurricaneAccess(m))
							hurricaneStoreGrid(m, &grid)
							return err
						})
					},
					Unlink: func(ref NativeRecordReference) error {
						return call("Unlink", ref, 0, 0, func() error { return hurricaneRemoveFixture(m, ref) })
					},
					Cleanup: func(ref NativeRecordReference, mode uint16) error {
						return call("Cleanup", ref, 0, 0, func() error {
							_, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: uint32(mode)}, FollowerCleanupCallbacks{Memory: memory, Unlink: func(ref NativeRecordReference) error { return hurricaneRemoveFixture(m, ref) }, Insert: func(ref NativeRecordReference) error { return hurricaneInsertFixture(m, ref) }, ClearFarms: func(ref NativeRecordReference, tile uint8) error {
								return evaluator.ClearFarms(ref, tile, townCombatEvaluatorCallbacks(m))
							}})
							return err
						})
					},
				}
				if _, err := rules.Tick(created.Reference, cb); err != nil {
					t.Fatal(err)
				}
				if eventIndex != len(native.Events) {
					t.Fatal("native hurricane callback omitted")
				}
				assertHurricaneFrame(t, m, native)
			}
		})
	}
	if updates != 586 {
		t.Fatalf("native hurricane update count differs: %d", updates)
	}
}
