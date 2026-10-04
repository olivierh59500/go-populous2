package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type economyFixture struct {
	Input struct {
		Name                                          string
		Land, Slot, FreeSlot                          int
		Stage, Owner, Flags                           uint8
		Work, PoolBlocked                             uint16
		Population                                    int32
		Mana, Clock, Deadline                         uint32
		Selected, SupportFailure, FullPass, ExtraFree bool
	}
	Before, After []struct {
		Reference uint16
		Raw       [52]uint8
	}
	BeforeGod, AfterGod [314]uint8
	Events              []struct {
		Name                      string
		Reference, Argument       uint16
		Result                    int16
		D0, D1, D2                uint32
		BeforeSHA256, AfterSHA256 string
		Changes                   []struct {
			Offset int
			Value  uint8
		}
	}
	Visited                                                          []uint16
	PoolBlocked, Selected                                            uint16
	Deadline, RNG                                                    uint32
	Exit, GridSHA256, OverlaySHA256, RecordsSHA256, GodsSHA256, Hash string
}

func economyMemory(f economyFixture) *entryFixtureMemory {
	m := &entryFixtureMemory{}
	for index := range 4096 {
		m.bytes[0xf44+index*4], m.bytes[0xf44+index*4+1], m.bytes[0x4f44+index] = 0xa8, 15, 0x77
		if f.Input.SupportFailure {
			m.bytes[0xf44+index*4+1] = 0
		}
	}
	for slot := 1; slot < 400; slot++ {
		if slot == f.Input.FreeSlot || slot == f.Input.Slot || f.Input.ExtraFree && slot == 251 {
			continue
		}
		at := 0x76c0 + slot*52
		m.bytes[at], m.bytes[at+12], m.bytes[at+22] = 4, 1, 0x30
		m.putWord(at+20, 32767)
		m.putWord(at+6, 128)
		m.putWord(at+8, 128)
		m.putWord(at+10, 0x2a8)
	}
	for _, r := range f.Before {
		copy(m.bytes[0x76c0+int(r.Reference):], r.Raw[:])
	}
	copy(m.bytes[0xe76a+int(f.Input.Owner)*314:], f.BeforeGod[:])
	m.putWord(0xf44+(32+32*64)*4+2, uint16(f.Input.Slot*52))
	m.putLong(0xf40, f.Input.Clock)
	m.putLong(0xdd8, f.Input.Deadline)
	m.putWord(0xdc2, f.Input.PoolBlocked)
	m.putLong(0xeb28, 4311)
	m.putWord(0xeb44, 8)
	if f.Input.Selected {
		m.putLong(0xf36, 0x200000+0x76c0+uint32(f.Input.Slot*52))
	}
	return m
}

func TestNativeTownEconomyAgainstOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/town_economy_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases, PassCases []economyFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 253 || len(catalog.PassCases) != 2 {
		t.Fatal("native town economic catalog incomplete")
	}
	b := testBundle(t)
	evaluator, err := DecodeNativeTownEvaluator(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			rules, err := DecodeNativeTownEconomyRules(b.Landscapes[f.Input.Land])
			if err != nil {
				t.Fatal(err)
			}
			m := economyMemory(f)
			state := NativeTownEconomyState{PoolBlocked: f.Input.PoolBlocked, Clock: f.Input.Clock, Deadline: f.Input.Deadline}
			if f.Input.Selected {
				state.Selected = NativeRecordReference(f.Input.Slot * 52)
			}
			syncState := func() {
				m.putWord(0xdc2, state.PoolBlocked)
				m.putLong(0xdd8, state.Deadline)
				pointer := uint32(0)
				if state.Selected != 0 {
					pointer = 0x200000 + 0x76c0 + uint32(state.Selected)
				}
				m.putLong(0xf36, pointer)
			}
			eventIndex := 0
			call := func(name string, ref NativeRecordReference, operation func() error) error {
				syncState()
				if eventIndex >= len(f.Events) {
					return fmt.Errorf("unexpected economic callback%s", name)
				}
				e := f.Events[eventIndex]
				eventIndex++
				if e.Name != name || e.Reference != uint16(ref) || aftermathRawHash(m) != e.BeforeSHA256 {
					return fmt.Errorf("native%s callback input differs", name)
				}
				if operation == nil {
					for _, c := range e.Changes {
						m.bytes[c.Offset] = c.Value
					}
				} else if err := operation(); err != nil {
					return err
				}
				if aftermathRawHash(m) != e.AfterSHA256 {
					return fmt.Errorf("native%s callback output differs", name)
				}
				return nil
			}
			evalCallbacks := townCombatEvaluatorCallbacks(m)
			memory := FollowerCleanupMemory{Read8: func(at int) (uint8, error) { return m.bytes[at], nil }, Read16: func(at int) (uint16, error) { return m.word(at), nil }, Read32: func(at int) (uint32, error) { return m.long(at), nil }, Write8: func(at int, v uint8) error { m.bytes[at] = v; return nil }, Write16: func(at int, v uint16) error { m.putWord(at, v); return nil }, Write32: func(at int, v uint32) error { m.putLong(at, v); return nil }}
			cb := NativeTownEconomyCallbacks{Memory: memory,
				EvaluateTown: func(ref NativeRecordReference) (int, error) {
					stage := uint8(0)
					err := call("EvaluateTown", ref, func() error {
						var err error
						stage, err = evaluator.Evaluate(ref, uint16(state.Clock), evalCallbacks)
						return err
					})
					if err == nil && int16(stage) != f.Events[eventIndex-1].Result {
						err = fmt.Errorf("native evaluator return differs")
					}
					return int(stage), err
				},
				ClearFarms: func(ref NativeRecordReference, tile uint8) error {
					return call("ClearFarms", ref, func() error { return evaluator.ClearFarms(ref, tile, evalCallbacks) })
				},
				Insert: func(ref NativeRecordReference) error {
					return call("Insert", ref, func() error { m.insert(cleanupRecordAddress(ref)); return nil })
				},
				Random: func() int {
					seed := m.long(0xeb28)
					if seed == 0 {
						seed = 0xbc614e
					}
					seed *= 0xbb40e62d
					m.putLong(0xeb28, seed)
					return int(seed >> 8 & 0x7fff)
				},
				LandAI: func(request NativeTownLandRequest) error {
					if eventIndex >= len(f.Events) {
						return fmt.Errorf("native rare creator omitted")
					}
					e := f.Events[eventIndex]
					if request.Registers != (FollowerCleanupRegisters{D0: e.D0, D1: e.D1, D2: e.D2}) {
						return fmt.Errorf("native rare creator register request differs: %+v", request)
					}
					return call("LandAI", NativeRecordReference(f.Input.Slot*52), nil)
				},
			}
			step, err := rules.Tick(NativeRecordReference(f.Input.Slot*52), &state, cb)
			if err != nil {
				t.Fatal(err)
			}
			syncState()
			if eventIndex != len(f.Events) || state.PoolBlocked != f.PoolBlocked || uint16(state.Selected) != f.Selected || state.Deadline != f.Deadline || m.long(0xeb28) != f.RNG {
				t.Fatalf("native globals or callback count differ: %+v", step)
			}
			for _, r := range f.After {
				at := 0x76c0 + int(r.Reference)
				if !reflect.DeepEqual(m.bytes[at:at+52], r.Raw[:]) {
					t.Fatalf("complete native actor%04x differs\ngot%x\nwant%x", r.Reference, m.bytes[at:at+52], r.Raw)
				}
			}
			if !reflect.DeepEqual(m.bytes[0xe76a+int(f.Input.Owner)*314:0xe76a+(int(f.Input.Owner)+1)*314], f.AfterGod[:]) {
				t.Fatal("complete native deity differs")
			}
			for _, h := range []struct {
				at, size   int
				want, name string
			}{{0xf44, 16384, f.GridSHA256, "map"}, {0x4f44, 4096, f.OverlaySHA256, "overlays"}, {0x76c0, 20800, f.RecordsSHA256, "follower pool"}, {0xe76a, 942, f.GodsSHA256, "deities"}, {0, 65536, f.Hash, "full BSS"}} {
				if got := fmt.Sprintf("%x", sha256.Sum256(m.bytes[h.at:h.at+h.size])); got != h.want {
					t.Fatalf("native%s hash differs: got%s want%s", h.name, got, h.want)
				}
			}
			if f.Exit == "1131c" && (!step.NeedsDecision || step.CurrentTotal) || f.Exit == "123b4" && (!step.CurrentTotal || step.NeedsDecision) {
				t.Fatalf("native town dispatch boundary differs: %+v", step)
			}
		})
	}
	// These original whole-pass runs dispatch every allocated owner byte in
	// ascending address order. The lower newborn was already passed while
	// its owner was zero; the higher newborn receives a dispatch immediately.
	for _, f := range catalog.PassCases {
		t.Run(f.Input.Name, func(t *testing.T) {
			parent, child := uint16(f.Input.Slot*52), uint16(f.Input.FreeSlot*52)
			born := false
			for _, e := range f.Events {
				if e.Name == "Insert" && e.Reference == child {
					born = true
				}
			}
			visitedChild := false
			for _, ref := range f.Visited {
				if ref == child {
					visitedChild = true
				}
			}
			if !born || visitedChild != (child > parent) {
				t.Fatal("original same-pass allocation evidence differs")
			}
			if f.Exit != "e00000" {
				t.Fatal("original follower pass did not finish")
			}
			m := economyMemory(f)
			rules, err := DecodeNativeTownEconomyRules(b.Landscapes[f.Input.Land])
			if err != nil {
				t.Fatal(err)
			}
			state := NativeTownEconomyState{Clock: f.Input.Clock, Deadline: f.Input.Deadline}
			evalCallbacks := townCombatEvaluatorCallbacks(m)
			memory := FollowerCleanupMemory{Read8: func(at int) (uint8, error) { return m.bytes[at], nil }, Read16: func(at int) (uint16, error) { return m.word(at), nil }, Read32: func(at int) (uint32, error) { return m.long(at), nil }, Write8: func(at int, v uint8) error { m.bytes[at] = v; return nil }, Write16: func(at int, v uint16) error { m.putWord(at, v); return nil }, Write32: func(at int, v uint32) error { m.putLong(at, v); return nil }}
			step, err := rules.Tick(NativeRecordReference(parent), &state, NativeTownEconomyCallbacks{Memory: memory,
				EvaluateTown: func(ref NativeRecordReference) (int, error) {
					stage, err := evaluator.Evaluate(ref, uint16(state.Clock), evalCallbacks)
					return int(stage), err
				},
				ClearFarms: func(ref NativeRecordReference, tile uint8) error {
					return evaluator.ClearFarms(ref, tile, evalCallbacks)
				},
				Insert: func(ref NativeRecordReference) error { m.insert(cleanupRecordAddress(ref)); return nil },
			})
			if err != nil {
				t.Fatal(err)
			}
			if uint16(step.Born) != child || step.SamePassEligible != visitedChild {
				t.Fatalf("Go birth schedule differs from original full pass: %+v", step)
			}
		})
	}
}
