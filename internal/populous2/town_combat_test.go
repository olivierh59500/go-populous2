package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

type townCombatFixture struct {
	Input struct {
		Name, Mode                                    string
		X, Y                                          int
		Owner, Stage, Flags, State, Kind, DefaultTile uint8
		Support                                       int
		Clock, OriginalA0                             uint16
		CompetitorOwner                               uint8
	}
	Before, After                            [][52]uint8
	Calls                                    []string
	GridSHA256, OverlaySHA256, RecordsSHA256 string
	GodsSHA256                               string
	RNG                                      uint32
}

func townCombatMemory(f townCombatFixture, evaluator *NativeTownEvaluator) *entryFixtureMemory {
	m := &entryFixtureMemory{}
	for index := range 4096 {
		m.bytes[0xf44+index*4], m.bytes[0xf44+index*4+1], m.bytes[0x4f44+index] = 0xa8, f.Input.DefaultTile, 0x77
	}
	for index, raw := range f.Before {
		copy(m.bytes[0x76f4+index*52:], raw[:])
	}
	m.putWord(0xf42, f.Input.Clock)
	m.putLong(0xeb28, 4311)
	if f.Input.Support >= 0 {
		origin := uint16(f.Input.X | f.Input.Y<<8)
		for _, offset := range evaluator.Footprint[:f.Input.Support] {
			x, y, inside := nativeTownParcel(origin, offset)
			if inside {
				m.bytes[0xf44+(x+y*64)*4+1] = 15
			}
		}
	}
	m.putWord(0xf44+(f.Input.X+f.Input.Y*64)*4+2, 52)
	if f.Input.CompetitorOwner != 0 {
		m.putWord(0xf44+(f.Input.X+1+f.Input.Y*64)*4+2, 104)
	}
	return m
}

func townCombatEvaluatorCallbacks(m *entryFixtureMemory) NativeTownCallbacks {
	return NativeTownCallbacks{
		Record: func(ref NativeRecordReference) (NativeTownRecord, bool) {
			a, err := m.read(ref)
			return NativeTownRecord{Kind: a.Motion.Kind, Owner: a.Owner, Stage: a.Byte1, State: a.Motion.State, Animation: uint16(a.Motion.Animation), Next: NativeRecordReference(a.Motion.Next), X: uint16(a.Motion.X), Y: uint16(a.Motion.Y)}, err == nil
		},
		SetRecord: func(ref NativeRecordReference, r NativeTownRecord) {
			a, _ := m.read(ref)
			a.Motion.Kind, a.Owner, a.Byte1, a.Motion.State = r.Kind, r.Owner, r.Stage, r.State
			a.Motion.Animation, a.Motion.Next = int(r.Animation), uint16(r.Next)
			a.Motion.X, a.Motion.Y = int16(r.X), int16(r.Y)
			_ = m.write(ref, a)
		},
		Head:         func(x, y int) NativeRecordReference { return NativeRecordReference(m.word(0xf44 + (x+y*64)*4 + 2)) },
		ReadTile:     func(x, y int) uint8 { return m.bytes[0xf44+(x+y*64)*4+1] },
		WriteTile:    func(x, y int, v uint8) { m.bytes[0xf44+(x+y*64)*4+1] = v },
		WriteOverlay: func(x, y int, v uint8) { m.bytes[0x4f44+x+y*64] = v },
	}
}

// Each case executes original $16184/$12bd8 all the way through actual
// $135ca, $13352 and $124a2. Comparisons cover every map/overlay/follower/deity
// byte, rather than only the visible town or expected helper assignments.
func TestTownCombatAgainstOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/town_combat_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []townCombatFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 70 {
		t.Fatal("native town combat catalog incomplete")
	}
	bundle := testBundle(t)
	rules, err := DecodeTownCombatRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := DecodeNativeTownEvaluator(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			m := townCombatMemory(fixture, &evaluator)
			evalCallbacks := townCombatEvaluatorCallbacks(m)
			calls := []string{}
			clear := func(ref NativeRecordReference, replacement uint8) error {
				calls = append(calls, "135ca")
				return evaluator.ClearFarms(ref, replacement, evalCallbacks)
			}
			memory := FollowerCleanupMemory{
				Read8:   func(at int) (uint8, error) { return m.bytes[at], nil },
				Read16:  func(at int) (uint16, error) { return m.word(at), nil },
				Read32:  func(at int) (uint32, error) { return m.long(at), nil },
				Write8:  func(at int, v uint8) error { m.bytes[at] = v; return nil },
				Write16: func(at int, v uint16) error { m.putWord(at, v); return nil },
				Write32: func(at int, v uint32) error { m.putLong(at, v); return nil },
			}
			cb := TownCombatCallbacks{Read: m.read, Write: m.write, ClearFarms: clear,
				EvaluateTown: func(ref NativeRecordReference) (int, error) {
					calls = append(calls, "13352")
					stage, err := evaluator.Evaluate(ref, fixture.Input.Clock, evalCallbacks)
					// The evaluator composes competitor cleanup internally, with
					// the same original $135ca boundary represented in the trace.
					if fixture.Input.CompetitorOwner != 0 {
						calls = append(calls, "135ca")
					}
					return int(stage), err
				},
				Cleanup: func(ref NativeRecordReference, mode uint16) error {
					calls = append(calls, "124a2")
					_, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: uint32(mode)}, FollowerCleanupCallbacks{Memory: memory, ClearFarms: clear,
						Unlink: func(NativeRecordReference) error {
							return fmt.Errorf("mode1 town unexpectedly removed graph allocation")
						},
						Insert: func(NativeRecordReference) error { return fmt.Errorf("nonleader town unexpectedly moved a magnet") },
					})
					// $14654 is inlined by CleanupFollower; its fields are covered
					// by complete raw hashes, and its position follows farm cleanup.
					calls = append(calls, "14654")
					return err
				},
			}
			var step TownCombatStep
			if fixture.Input.Mode == "destroy" {
				step, err = rules.Destroy(52, cb)
			} else {
				step, err = rules.Reform(52, NativeRecordReference(fixture.Input.OriginalA0), fixture.Input.Clock, cb)
			}
			if err != nil {
				t.Fatal(err)
			}
			for index, expected := range fixture.After {
				if !reflect.DeepEqual(m.bytes[0x76f4+index*52:0x76f4+(index+1)*52], expected[:]) {
					t.Fatalf("complete native record%d differs: %+v", index, step)
				}
			}
			for _, check := range []struct {
				Start, Length int
				Hash, Name    string
			}{
				{0xf44, 16384, fixture.GridSHA256, "map"}, {0x4f44, 4096, fixture.OverlaySHA256, "overlays"},
				{0x76c0, 20800, fixture.RecordsSHA256, "followers"}, {0xe76a, 942, fixture.GodsSHA256, "deities"},
			} {
				if got := fmt.Sprintf("%x", sha256.Sum256(m.bytes[check.Start:check.Start+check.Length])); got != check.Hash {
					t.Fatalf("complete native%s differs: got%s want%s", check.Name, got, check.Hash)
				}
			}
			expectedCalls := []string{}
			for _, call := range fixture.Calls {
				expectedCalls = append(expectedCalls, strings.Split(call, ":")[0])
			}
			if !reflect.DeepEqual(calls, expectedCalls) || m.long(0xeb28) != fixture.RNG {
				t.Fatalf("native calls/RNG differ: got%v want%v", calls, expectedCalls)
			}
			actor, _ := m.read(52)
			if fixture.Input.Mode == "destroy" && (step.Guarded != (fixture.Input.State == 0x30) || step.Destroyed == step.Guarded) {
				t.Fatal("native destruction guard differs")
			}
			if fixture.Input.Mode == "reform" && (step.Walking != (actor.Motion.Kind == 2) || step.Reformed == step.Walking) {
				t.Fatal("native reform result differs")
			}
		})
	}
}

func TestTownCombatDestructionArtExistsInEveryLandscape(t *testing.T) {
	b := testBundle(t)
	rules, err := DecodeTownCombatRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, pointer := range rules.DeathAnimations {
		frame, ok := rules.Frames[int(pointer)]
		if !ok || len(frame.Layers) == 0 {
			t.Fatal("native town destruction frame missing")
		}
		for _, bank := range b.Sprites {
			for _, layer := range frame.Layers {
				if layer.Sprite < 0 || layer.Sprite >= len(bank) {
					t.Fatal("native town destruction sprite absent")
				}
			}
		}
	}
}

// The decoder bounds stage lookup; the original caller supplies stage0..18.
func TestTownCombatRejectsInvalidDestructionStage(t *testing.T) {
	r, err := DecodeTownCombatRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	m := &entryFixtureMemory{}
	m.bytes[0x76f4+1] = 19
	cb := TownCombatCallbacks{Read: m.read, Write: m.write, ClearFarms: func(NativeRecordReference, uint8) error { t.Fatal("invalid stage changed terrain"); return nil }, Cleanup: func(NativeRecordReference, uint16) error { return nil }}
	if _, err := r.Destroy(52, cb); err == nil {
		t.Fatal("native stage table overread accepted")
	}
}
