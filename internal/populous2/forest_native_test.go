package populous2

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type nativeForestInput struct {
	Name, Mode                  string
	Owner                       uint16
	X, Y, Tile                  uint8
	Seed                        uint32
	FreeStart, FreeCount, Ticks int
	Clock                       uint16
	Initial                     []nativeHeroPatch
	Links                       []uint16
}
type nativeForestTrace struct {
	Update int
	Hash   string
	RNG    uint32
}
type nativeForestCall struct {
	Kind      string
	Reference uint16
}
type nativeForestFixture struct {
	MetricAfter                      uint16
	Input                            nativeForestInput
	Hash                             string
	Changes                          []nativeHeroChange
	D0, RNG                          uint32
	RandomDraws, Attempts, RareDraws int
	Calls                            []nativeForestCall
	Trace                            []nativeForestTrace
}

func forestFixtureMemory(input nativeForestInput) *cleanupMemory {
	m := &cleanupMemory{}
	for index := range 4096 {
		m.putByte(0xf44+index*4, 0xa8)
		m.putByte(0xf45+index*4, input.Tile)
		m.putByte(0x4f44+index, 0x77)
	}
	for index := range 200 {
		a := 0x6bd0 + index*14
		for off := range 14 {
			m.putByte(a+off, uint8(index*14+off+7))
		}
		owner := uint8(1)
		if index >= input.FreeStart && index < input.FreeStart+input.FreeCount {
			owner = 0
		}
		if input.Mode == "age" {
			owner = 0
		}
		m.putByte(a+12, owner)
	}
	for owner := uint8(1); owner <= 2; owner++ {
		god := 0xe76a + int(owner)*314
		marker := 0xe740 + int(owner)*14
		m.putWord(god+10, uint16(marker-0x76c0))
		m.putWord(god+0x44, 120)
		m.putByte(marker+12, owner)
		m.putWord(marker+6, uint16(10+owner)<<8|128)
		m.putWord(marker+8, 0x0a80)
	}
	m.putWord(0xf42, input.Clock)
	for _, p := range input.Initial {
		switch p.Width {
		case 1:
			m.putByte(p.Address, uint8(p.Value))
		case 2:
			m.putWord(p.Address, uint16(p.Value))
		case 4:
			if err := m.write32(p.Address, p.Value); err != nil {
				m.err = err
			}
		}
	}
	for _, ref := range append([]uint16{0x708e, 0x709c}, input.Links...) {
		if err := m.insert(NativeRecordReference(ref)); err != nil {
			m.err = err
		}
	}
	m.putByte(0x600, uint8(input.Owner))
	m.putByte(0x601, 46)
	m.putByte(0x602, input.X)
	m.putByte(0x603, input.Y)
	return m
}

// Full original bodies compare native command metric updates, signed lookup
// aliases, allocation/RNG and all signed aging/burial states. Tree fire uses
// real town destruction and processes later scenery within the same pool pass.
func TestForestRenewAndSceneryAgainstCompleteNativeBodies(t *testing.T) {
	data, err := os.ReadFile("testdata/forest_renew_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativeForestFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 4522 {
		t.Fatal("native forest/renew catalog incomplete")
	}
	bundle := testBundle(t)
	forest, err := DecodeForestNativeRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	renew, err := DecodeRenewNativeRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	town, err := DecodeNativeTownEvaluator(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	combat, err := DecodeTownCombatRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range []struct {
		number  uint8
		spell   SpellID
		handler int
	}{{46, Trees, 0x179be}, {80, Flowers, 0x17a5c}} {
		found := false
		for _, action := range bundle.Actions {
			if action.Command == command.number {
				found = true
				if action.Spell != command.spell || action.Handler != command.handler {
					t.Fatal("nativeforest/renew actionidentity differs")
				}
			}
		}
		if !found {
			t.Fatal("nativeforest/renew command missing")
		}
	}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			m := forestFixtureMemory(fixture.Input)
			if m.err != nil {
				t.Fatal(m.err)
			}
			var commandImage [0xeb90 - NativeRuntimeImageEnd]byte
			memory := FollowerCleanupMemory{Read8: m.read8, Read16: func(a int) (uint16, error) {
				if a >= NativeRuntimeImageEnd && a+2 <= 0xeb90 {
					return binary.BigEndian.Uint16(commandImage[a-NativeRuntimeImageEnd:]), nil
				}
				return m.read16(a)
			}, Read32: m.read32, Write8: m.write8, Write16: func(a int, v uint16) error {
				if a >= NativeRuntimeImageEnd && a+2 <= 0xeb90 {
					binary.BigEndian.PutUint16(commandImage[a-NativeRuntimeImageEnd:], v)
					return nil
				}
				return m.write16(a, v)
			}, Write32: m.write32}
			rng, draws := fixture.Input.Seed, 0
			random := func() uint16 {
				draws++
				if rng == 0 {
					rng = 0xbc614e
				}
				rng *= 0xbb40e62d
				return uint16(rng >> 8 & 0x7fff)
			}
			calls := []nativeForestCall{}
			clear := func(ref NativeRecordReference, tile uint8) error {
				return town.ClearFarms(ref, tile, winTownCallbacks(m))
			}
			cleanup := func(ref NativeRecordReference, mode uint16) error {
				_, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: uint32(mode)}, FollowerCleanupCallbacks{Memory: memory, Unlink: m.unlink, Insert: m.insert, ClearFarms: clear})
				return err
			}
			cb := ForestNativeCallbacks{Memory: memory, Random: random, Link: func(ref NativeRecordReference) error {
				calls = append(calls, nativeForestCall{Kind: "link", Reference: uint16(ref)})
				return m.insert(ref)
			}, Unlink: func(ref NativeRecordReference) error {
				calls = append(calls, nativeForestCall{Kind: "unlink", Reference: uint16(ref)})
				return m.unlink(ref)
			}, DestroyTown: func(ref NativeRecordReference) error {
				calls = append(calls, nativeForestCall{Kind: "destroy", Reference: uint16(ref)})
				_, err := combat.Destroy(ref, TownCombatCallbacks{Read: func(ref NativeRecordReference) (FollowerEntryActor, error) { return winEntryRead(m, ref) }, Write: func(ref NativeRecordReference, a FollowerEntryActor) error { return winEntryWrite(m, ref, a) }, ClearFarms: clear, Cleanup: cleanup, EvaluateTown: func(NativeRecordReference) (int, error) { return 0, fmt.Errorf("unexpectedforest townreform") }})
				return err
			}}
			switch fixture.Input.Mode {
			case "renew":
				step, err := renew.Create(fixture.Input.Owner, fixture.Input.X, fixture.Input.Y, RenewNativeCallbacks{Memory: memory, Random: random})
				if err != nil {
					t.Fatal(err)
				}
				if step.Attempts != fixture.Attempts || step.RandomDraws != fixture.RandomDraws {
					t.Fatal("native renew count/RNG differs")
				}
			case "plant", "cast":
				var step ForestNativePlacement
				var err error
				if fixture.Input.Mode == "cast" {
					step, err = forest.Cast(uint8(fixture.Input.Owner), fixture.Input.X, fixture.Input.Y, cb)
				} else {
					step, err = forest.Plant(fixture.Input.Owner, NativePackedTile(uint16(fixture.Input.Y)<<8|uint16(fixture.Input.X)), cb)
				}
				if err != nil {
					t.Fatal(err)
				}
				if step.Attempts != fixture.Attempts || step.RandomDraws != fixture.RandomDraws || int(step.Count) != fixture.RareDraws {
					t.Fatal("native forest attempt/allocation/RNG differs")
				}
				if fixture.Input.Mode == "plant" && uint32(step.Count) != fixture.D0 {
					t.Fatal("native forest count return differs")
				}
			case "age":
				for _, trace := range fixture.Trace {
					clock := fixture.Input.Clock + uint16(trace.Update-1)
					m.putWord(0xf42, clock)
					for index := range 200 {
						ref := NativeRecordReference(uint16(0x6bd0 + index*14 - 0x76c0))
						step, err := forest.Tick(ref, clock, cb)
						if err != nil {
							t.Fatal(err)
						}
						if step.SpreadFire { // Tick already invoked BurnNeighbors; retain its original call position before nestedtown callbacks.
							// Original call order is reconstructed from the helper outcome below.
							at := len(calls)
							for at > 0 && calls[at-1].Kind == "destroy" {
								at--
							}
							calls = append(calls, nativeForestCall{})
							copy(calls[at+1:], calls[at:])
							calls[at] = nativeForestCall{Kind: "spread", Reference: uint16(ref)}
						}
					}
					if stormFixtureHash(m) != trace.Hash || rng != trace.RNG {
						t.Fatalf("native scenerypass%d retainedBSS/RNG differs", trace.Update)
					}
				}
			default:
				t.Fatal("unknown native forest fixture")
			}
			if !reflect.DeepEqual(calls, fixture.Calls) {
				t.Fatalf("nativeforest callbacks differ: got%v want%v", calls, fixture.Calls)
			}
			if stormFixtureHash(m) != fixture.Hash {
				for _, change := range fixture.Changes {
					got, _ := m.read8(change.Address)
					if got != change.Value {
						t.Errorf("native byte%x got%x want%x", change.Address, got, change.Value)
					}
				}
				t.Fatal("complete native forest/renew BSS differs")
			}
			if fixture.Input.Mode == "cast" {
				metric, err := memory.Read16(primitiveDeityAddress(uint16(uint8(fixture.Input.Owner))) + 0x44)
				if err != nil || metric != fixture.MetricAfter {
					t.Fatal("native Forest command/player metric alias differs")
				}
			}
			if rng != fixture.RNG || draws != fixture.RandomDraws {
				t.Fatal("nativeforest/renew final RNG differs")
			}
		})
	}
}
