package populous2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type terrainStateFixture struct {
	Input struct {
		Name, Handler                   string
		Owner, Flags, Tile              uint8
		Hero, Scenario, Animation, X, Y uint16
		VX, VY                          int16
		Population                      int32
		Amount                          uint32
	}
	Hash, SourceRaw string
	Boundary        uint32
	Callbacks       []terrainStateCall
}
type terrainStateCall struct {
	Name       string
	Mode, X, Y uint16
}

func terrainStateFixtureMemory(t *testing.T, f terrainStateFixture) *cleanupMemory {
	t.Helper()
	m := &cleanupMemory{}
	c, at := f.Input, 0x76f4
	for i := range 4096 {
		m.putByte(0xf44+i*4, 0x88)
		m.putByte(0xf44+i*4+1, 15)
	}
	m.putByte(0xf44+(int(c.X>>8)+int(c.Y>>8)*64)*4+1, c.Tile)
	kind, state := uint8(2), uint8(0x3c)
	if c.Handler == "water" {
		kind, state = 10, 0x16
	}
	if c.Handler == "conversion" {
		kind, state = 14, 0x36
	}
	m.putByte(at, kind)
	m.putByte(at+12, c.Owner)
	m.putByte(at+13, c.Flags)
	m.putByte(at+19, 0xaa)
	m.putByte(at+18, 20)
	m.putByte(at+23, 2)
	m.putByte(at+22, state)
	m.putWord(at+6, c.X)
	m.putWord(at+8, c.Y)
	m.putWord(at+10, c.Animation)
	m.putWord(at+14, uint16(c.VX))
	m.putWord(at+16, uint16(c.VY))
	m.putWord(at+40, c.Hero)
	if err := m.write32(at+26, uint32(c.Population)); err != nil {
		t.Fatal(err)
	}
	for owner := uint8(0); owner < 3; owner++ {
		god, marker := 0xe76a+int(owner)*314, 0xe740+int(owner)*14
		ref := NativeRecordReference(marker - 0x76c0)
		if err := m.write32(god+20, c.Amount); err != nil {
			t.Fatal(err)
		}
		m.putWord(god+68, 30)
		m.putWord(god+8, 52)
		m.putWord(god+10, uint16(ref))
		m.putByte(marker, 20)
		m.putByte(marker+12, owner)
		m.putWord(marker+6, 0x0880)
		m.putWord(marker+8, 0x0980)
		if err := m.insert(ref); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.insert(52); err != nil {
		t.Fatal(err)
	}
	if m.err != nil {
		t.Fatal(m.err)
	}
	return m
}

func TestFollowerTerrainStatesAgainstCompleteOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_terrain_states_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []terrainStateFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 554 {
		t.Fatal("native terrain-state catalog incomplete")
	}
	r, err := DecodeFollowerTerrainRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	if r.BurnShift != 4 || len(r.Frames) == 0 {
		t.Fatal("native terrain-state tables differ")
	}
	counts := map[string]int{}
	for _, f := range catalog.Cases {
		counts[f.Input.Handler]++
		t.Run(f.Input.Name, func(t *testing.T) {
			m := terrainStateFixtureMemory(t, f)
			read := func(ref NativeRecordReference) (FollowerEntryActor, error) { return m.records.ReadFollowerEntry(ref) }
			write := func(ref NativeRecordReference, a FollowerEntryActor) error {
				old, err := read(ref)
				if err != nil {
					return err
				}
				_, err = m.records.PatchFollowerEntry(ref, old, a)
				return err
			}
			mem := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
			calls := []terrainStateCall{}
			cb := FollowerTerrainCallbacks{
				Read: read, Write: write,
				Scenario:  func(uint8) (uint16, error) { return f.Input.Scenario, nil },
				Attrition: func(owner uint8) (uint32, error) { return m.read32(0xe76a + int(owner)*314 + 20) },
				SetWaterReference: func(owner uint8, ref NativeRecordReference) error {
					return m.write16(0xe76a+int(owner)*314+54, uint16(ref))
				},
				Tile: func(p NativePackedTile) (uint8, error) {
					return m.read8(0xf44 + (int(uint8(p))+int(uint8(p>>8))*64)*4 + 1)
				},
				Cleanup: func(ref NativeRecordReference, mode uint16) error {
					calls = append(calls, terrainStateCall{Name: "cleanup", Mode: mode})
					_, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: uint32(mode)}, FollowerCleanupCallbacks{Memory: mem, Unlink: m.unlink, Insert: m.insert, ClearFarms: func(NativeRecordReference, uint8) error {
						return fmt.Errorf("terrain-state ordinary follower unexpectedly cleared farms")
					}})
					return err
				},
				ClearLeader: func(ref NativeRecordReference) error {
					calls = append(calls, terrainStateCall{Name: "clearLeader"})
					_, err := ClearFollowerLeader(ref, FollowerCleanupRegisters{}, FollowerLeaderCallbacks{Memory: mem, Unlink: m.unlink, Insert: m.insert})
					return err
				},
				Move: func(ref NativeRecordReference, x, y uint16) error {
					calls = append(calls, terrainStateCall{Name: "move", X: x, Y: y})
					var graph NativeOccupancyState
					for i := range graph.Cells {
						at := 0xf44 + i*4
						graph.Cells[i] = NativeOccupancyCell{Header: m.byte(at), Tile: m.byte(at + 1), Head: NativeRecordReference(m.word(at + 2))}
					}
					_, err := graph.Move(ref, x, y, m.runtime().RecordAccess())
					if err != nil {
						return err
					}
					for i, cell := range graph.Cells {
						at := 0xf44 + i*4
						m.putByte(at, cell.Header)
						m.putWord(at+2, uint16(cell.Head))
					}
					return m.err
				},
			}
			var step FollowerTerrainStep
			var err error
			switch f.Input.Handler {
			case "water":
				step, err = r.TickWater(52, cb)
			case "conversion":
				step, err = r.TickConversion(52, cb)
			case "burning":
				step, err = r.TickBurning(52, cb)
			default:
				t.Fatal("unknown native handler")
			}
			if err != nil {
				t.Fatal(err)
			}
			boundary := uint32(0)
			if step.Search {
				boundary = 0x1131c
			}
			if step.CurrentTotal {
				if boundary != 0 {
					t.Fatal("ambiguous native return")
				}
				boundary = 0x123b4
			}
			if step.NextFollower {
				if boundary != 0 {
					t.Fatal("ambiguous native return")
				}
				boundary = 0x12462
			}
			if boundary != f.Boundary {
				t.Fatalf("native branch differs: %x want %x", boundary, f.Boundary)
			}
			if !reflect.DeepEqual(calls, f.Callbacks) {
				t.Fatalf("native external call sequence differs: %+v want %+v", calls, f.Callbacks)
			}
			offset := 0x76f4 - NativeRecordImageStart
			if hex.EncodeToString(m.records.Bytes[offset:offset+52]) != f.SourceRaw {
				t.Fatalf("native source record differs: %s want %s", hex.EncodeToString(m.records.Bytes[offset:offset+52]), f.SourceRaw)
			}
			bytes := append([]byte(nil), m.lower[:]...)
			bytes = append(bytes, m.records.Bytes[:]...)
			bytes = append(bytes, m.globals.Bytes[:]...)
			if fmt.Sprintf("%x", sha256.Sum256(bytes)) != f.Hash {
				t.Fatal("complete native terrain-state BSS/grid/globals differs")
			}
		})
	}
	if !reflect.DeepEqual(counts, map[string]int{"water": 194, "conversion": 216, "burning": 144}) {
		t.Fatalf("native handler coverage differs: %v", counts)
	}
}
