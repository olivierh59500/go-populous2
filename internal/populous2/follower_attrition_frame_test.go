package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type followerAttritionFrameFixture struct {
	Input struct {
		Name                                string
		Owner, Flags, Kind, Stage, HeroType uint8
		Population, Amount                  uint32
		Registers                           [8]uint32
	}
	Registers [8]uint32
	Hash      string
	Writes    []cleanupWrite
	Calls     []struct {
		PC        uint32
		Reference uint16
		Registers [8]uint32
	}
}

func TestFollowerAttritionFullFrameAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_attrition_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []followerAttritionFrameFixture
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 678 {
		t.Fatal("native full-register attrition corpus incomplete")
	}
	town, err := DecodeNativeTownEvaluator(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	deaths, leaders, ruins := 0, 0, 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			c := f.Input
			marker, _ := NativeMagnetReference(c.Owner)
			baseline := cleanupFixture{}
			baseline.Input.Owner, baseline.Input.Kind, baseline.Input.Flags = c.Owner, c.Kind, c.Flags
			baseline.Input.Stage, baseline.Input.Byte19 = c.Stage, 0xaa
			baseline.Input.X, baseline.Input.Y, baseline.Input.Popularity = 0x2011, 0x202b, 30
			baseline.Input.SourceHero, baseline.Input.GodHero, baseline.Input.Head = uint16(c.HeroType), 3, 104
			baseline.MarkerReference = uint16(marker)
			m := cleanupFixtureMemory(t, baseline)
			m.record = false
			m.putByte(0x76f4+22, 0x24)
			m.putWord(0x76f4+10, 8)
			if err := m.write32(0x76f4+26, c.Population); err != nil {
				t.Fatal(err)
			}
			god := 0xe76a + int(c.Owner)*314
			if err := m.write32(god+20, c.Amount); err != nil {
				t.Fatal(err)
			}
			m.writes, m.record = nil, true
			frame := NativeFrameRegisterContext{D: c.Registers, AddressBase: 0x200000}
			mem := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
			calls := 0
			died, err := ApplyFollowerAttritionWithFrame(52, god, FollowerAttritionFrameCallbacks{Memory: mem, Frame: &frame, CleanupFrame: func(ref NativeRecordReference, frame *NativeFrameRegisterContext) error {
				if calls >= len(f.Calls) {
					t.Fatal("unexpected attrition cleanup")
				}
				want := f.Calls[calls]
				calls++
				if want.PC != 0x124a2 || want.Reference != uint16(ref) || want.Registers != frame.D {
					t.Fatalf("native attrition cleanup context differs: got%x/%x want%+v", ref, frame.D, want)
				}
				_, err := CleanupFollowerWithFrame(ref, frame, FollowerCleanupCallbacks{Memory: mem, Unlink: m.unlink, Insert: m.insert, ClearFarms: func(ref NativeRecordReference, tile uint8) error {
					return town.ClearFarms(ref, tile, m.town(t))
				}})
				return err
			}})
			if err != nil {
				t.Fatal(err)
			}
			if frame.D != f.Registers || died != (f.Registers[0] != 0) || calls != len(f.Calls) {
				t.Fatalf("native attrition full context/result differs: got%x/%v/%d want%x/%d", frame.D, died, calls, f.Registers, len(f.Calls))
			}
			if !reflect.DeepEqual(m.writes, f.Writes) {
				t.Fatalf("native attrition ordered writes differ: got%+v want%+v", m.writes, f.Writes)
			}
			bytes := append([]byte(nil), m.lower[:]...)
			bytes = append(bytes, m.records.Bytes[:]...)
			bytes = append(bytes, m.globals.Bytes[:]...)
			bytes = append(bytes, make([]byte, 65536-len(bytes))...)
			if fmt.Sprintf("%x", sha256.Sum256(bytes)) != f.Hash {
				t.Fatal("native attrition entire64KiB BSS differs")
			}
			if died {
				deaths++
				if c.Flags&1 != 0 {
					leaders++
				}
				if c.Kind == 4 {
					ruins++
				}
				if m.byte(0x76f4+12) != c.Owner || m.byte(0x76f4+22) != map[bool]uint8{true: 0x40, false: 0x2c}[m.byte(0x76f4+13)&2 != 0] {
					t.Fatal("native attrition death must retain owner and native state")
				}
			}
		})
	}
	if deaths == 0 || leaders == 0 || ruins == 0 {
		t.Fatal("native cleanup/leader/town attrition branches untested")
	}
}

func TestFollowerAttritionFramePreservesPartialFailure(t *testing.T) {
	m := &scenarioScriptMemory{}
	_ = m.write32(0x76f4+26, 1)
	_ = m.write32(0xe76a+314+20, 1)
	c := NativeFrameRegisterContext{D: [8]uint32{100, 2, 3, 4, 5, 6, 7, 8}}
	died, err := ApplyFollowerAttritionWithFrame(52, 0xe76a+314, FollowerAttritionFrameCallbacks{Memory: m.callbacks(), Frame: &c})
	pop, _ := m.read32(0x76f4 + 26)
	if err == nil || died || pop != 0 || c.D != [8]uint32{1, 2, 3, 4, 5, 6, 7, 8} || m[0x76f4+22] != 0 {
		t.Fatalf("missing real cleanup child silently completed: %v/%v/%x/%x", died, err, pop, c.D)
	}
	_ = m.write32(0x76f4+26, 0x12345678)
	c.D[0] = 99
	_, err = ApplyFollowerAttritionWithFrame(52, 65535, FollowerAttritionFrameCallbacks{Memory: m.callbacks(), Frame: &c})
	if err == nil || c.D[0] != 0x12345678 {
		t.Fatal("failed deity read discarded the preceding native MOVE.L")
	}
}
