package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func aiFixtureWorld(t *testing.T, raw []byte) *World {
	t.Helper()
	w := installNativeFixtureWorld(t, raw, nil)
	w.NativeGameMode = uint16(raw[0xeb44])<<8 | uint16(raw[0xeb45])
	w.NativeFreeCommands = uint16(raw[0xf0e])<<8 | uint16(raw[0xf0f])
	copy(w.NativeViewBytes[:], raw[0x5f44:0x5f50])
	seed := uint32(raw[0xeb28])<<24 | uint32(raw[0xeb29])<<16 | uint32(raw[0xeb2a])<<8 | uint32(raw[0xeb2b])
	if err := w.nativeCleanupMemory().Write32(0xeb28, seed); err != nil {
		t.Fatal(err)
	}
	return w
}
func aiFixtureWorldBytes(w *World, initial []byte) []byte {
	result := nativeFixtureWorldImage(w, initial)
	copy(result[0x5f44:0x5f50], w.NativeViewBytes[:])
	result[0xeb44], result[0xeb45] = uint8(w.NativeGameMode>>8), uint8(w.NativeGameMode)
	result[0xf0e], result[0xf0f] = uint8(w.NativeFreeCommands>>8), uint8(w.NativeFreeCommands)
	return result
}

// The complete dispatcher corpus executes all actual side1/side2 policies.
// World callbacks borrow its raw map/deity/command images and RNG, and no cast
// or command consumption occurs at this original $1383c stage.
func TestWorldNativeAICompletePolicyAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/ai_native_complete.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			aiNativeFixture
			D4, D5 uint16
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 512 {
		t.Fatal("native AI complete corpus incomplete")
	}
	rules, err := DecodeNativeAIRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			initial := aiNativeFixtureMemory(fixture.Input)
			raw := append([]byte(nil), initial[:]...)
			w := aiFixtureWorld(t, raw)
			context := NativeAIRegisterContext{D4: 0xccdd, D5: 0x6778}
			_, err := w.tickNativeAI(&rules, &context)
			if err != nil {
				t.Fatal(err)
			}
			after := aiFixtureWorldBytes(w, raw)
			if fmt.Sprintf("%x", sha256.Sum256(after)) != fixture.Hash || w.Core.RandomState() != fixture.RNG {
				t.Fatalf("native World AI complete BSS/RNG differs")
			}
			changes := []nativeHeroChange{}
			for at, value := range raw {
				if value != after[at] {
					changes = append(changes, nativeHeroChange{Address: at, Value: after[at]})
				}
			}
			if !reflect.DeepEqual(changes, fixture.Changes) {
				t.Fatal("native World AI changed ranges differ")
			}
			if context.D4 != fixture.D4 || context.D5 != fixture.D5 {
				t.Fatalf("native AI caller context differs: %+v native%04x/%04x", context, fixture.D4, fixture.D5)
			}
			if w.SpellSerial != 0 || w.LastSpell != 0 {
				t.Fatal("AI policy executed a deferred command early")
			}
		})
	}
}

// Individual original policy corpora exercise their World memory/RNG adapters
// as well. Partial dispatcher captures are covered by the separate complete
// 512-case corpus above, whose offensive/magnet bodies execute unmodified.
func TestWorldNativeAIIndividualPoliciesAgainstOriginalCPU(t *testing.T) {
	rules, err := DecodeNativeAIRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, name := range []string{"ai_native_initial", "ai_native_extended"} {
		data, err := os.ReadFile("testdata/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var catalog struct {
			Cases []struct {
				aiNativeFixture
				D4, D5 uint16
			}
		}
		if err := json.Unmarshal(data, &catalog); err != nil {
			t.Fatal(err)
		}
		for _, fixture := range catalog.Cases {
			if fixture.Input.Mode == "tick" {
				continue
			}
			checked++
			t.Run(fixture.Input.Name, func(t *testing.T) {
				initial := aiNativeFixtureMemory(fixture.Input)
				raw := append([]byte(nil), initial[:]...)
				w := aiFixtureWorld(t, raw)
				context := NativeAIRegisterContext{D4: 0xccdd, D5: 0x6778}
				cb := w.nativeAICallbacks(&context)
				god, command := 0xe76a+fixture.Input.Side*314, 0xeb56+(fixture.Input.Side-1)*10
				result := false
				switch fixture.Input.Mode {
				case "urgent":
					result, err = rules.Urgent(god, command, cb)
				case "expand":
					result, err = rules.Expand(god, command, cb)
				case "release":
					result, err = rules.ReleaseTown(god, command, cb)
				case "choices":
					err = rules.CompileChoices(god, cb.Memory)
				case "offensive":
					result, err = rules.Offensive(god, command, &context, cb)
				case "magnet":
					result, err = rules.MagnetMode(god, command, cb)
				case "water":
					result, err = rules.WaterTarget(0x76f4, uint16(fixture.Input.Side), &context, cb.Memory)
				default:
					t.Fatal("unknown native World AI policy")
				}
				if err != nil {
					t.Fatal(err)
				}
				if fixture.Input.Mode != "choices" && result != fixture.Result {
					t.Fatal("native AI policy condition differs")
				}
				after := aiFixtureWorldBytes(w, raw)
				if fmt.Sprintf("%x", sha256.Sum256(after)) != fixture.Hash || w.Core.RandomState() != fixture.RNG {
					t.Fatal("native World policy complete BSS/RNG differs")
				}
				if fixture.Input.Mode == "offensive" || fixture.Input.Mode == "water" {
					if context.D4 != fixture.D4 || context.D5 != fixture.D5 {
						t.Fatal("native policy caller context differs")
					}
				}
				changes := []nativeHeroChange{}
				for at, v := range raw {
					if v != after[at] {
						changes = append(changes, nativeHeroChange{Address: at, Value: after[at]})
					}
				}
				if !reflect.DeepEqual(changes, fixture.Changes) {
					t.Fatal("native World policy changed ranges differ")
				}
			})
		}
	}
	if checked < 1900 {
		t.Fatalf("native World policy coverage incomplete: %d", checked)
	}
	t.Logf("individual original CPU policies%d", checked)
}

type aiWorldSetupFixture struct {
	Input struct {
		Name, Mode string
		Raw        [250]byte
		Initial    []nativeHeroPatch
	}
	Hash       string
	Changes    []nativeHeroChange
	Experience [2][6]uint8
	Calls      []struct {
		Address   int
		Record    [10]byte
		Registers [8]uint32
	}
	Registers [8]uint32
}

func aiSetupFixtureRaw(f aiWorldSetupFixture) []byte {
	raw := make([]byte, 65536)
	for side := 0; side < 2; side++ {
		god := 0xe8a4 + side*314
		for i := range 314 {
			raw[god+i] = uint8(side*71 + i*3 + 17)
		}
	}
	for _, p := range f.Input.Initial {
		switch p.Width {
		case 1:
			raw[p.Address] = uint8(p.Value)
		case 2:
			raw[p.Address], raw[p.Address+1] = uint8(p.Value>>8), uint8(p.Value)
		case 4:
			for i := range 4 {
				raw[p.Address+i] = uint8(p.Value >> uint(24-i*8))
			}
		}
	}
	return raw
}

// Startup/profile evidence executes the real $10df2/$10e90 and control suffix
// independently, including all1000 supplied campaign templates. The command
// scheduler captures actual $17500 boundaries with only its body replaced by
// a register-preserving RTS; player body semantics have their separate proof.
func TestWorldNativeAISetupAndDeferredCommandStageAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/ai_world_setup_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []aiWorldSetupFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 1236 {
		t.Fatal("native World AI startup corpus incomplete")
	}
	rules, err := DecodeNativeAIRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			raw := aiSetupFixtureRaw(fixture)
			w := aiFixtureWorld(t, raw)
			counts[fixture.Input.Mode]++
			switch fixture.Input.Mode {
			case "controls":
				err = w.initializeNativeAIControls()
			case "templates":
				level := Level{Raw: fixture.Input.Raw}
				var xp [2][6]uint8
				xp, err = w.loadNativeAITemplates(&rules, level)
				if xp != fixture.Experience {
					t.Fatalf("actual final raw XP differs: actual%v native%v", xp, fixture.Experience)
				}
			case "commands":
				input := NativeCommandRegisterContext{D: [8]uint32{0x11000001, 0x22000002, 0x33000003, 0x44000004, 0x55000005, 0x66000006, 0x77000007, 0x88000008}}
				calls := 0
				_, err = w.executeNativeDeferredCommands(input, NativeDeferredCommandCallbacks{Execute: func(address int, context *NativeCommandRegisterContext) error {
					if calls >= len(fixture.Calls) {
						return fmt.Errorf("unexpected deferred command")
					}
					expected := fixture.Calls[calls]
					calls++
					var record [10]byte
					for i := range record {
						value, e := w.nativeCleanupMemory().Read8(address + i)
						if e != nil {
							return e
						}
						record[i] = value
					}
					if address != expected.Address || record != expected.Record {
						return fmt.Errorf("native raw ten-byte record/order differs")
					}
					// $17456 initializes D0 to the byte8 dispatch-table offset.
					// Other caller registers, notably D7, reach $17500 intact.
					for i := 0; i < 8; i++ {
						if context.D[i] != expected.Registers[i] {
							return fmt.Errorf("native deferred registerD%d differs", i)
						}
					}
					return nil
				}})
				if calls != len(fixture.Calls) {
					t.Fatal("native deferred calls missing")
				}
				if input.D != fixture.Registers {
					t.Fatal("native1744c failed to retain caller register input")
				}
			default:
				t.Fatal("unknown native AI startup mode")
			}
			if err != nil {
				t.Fatal(err)
			}
			after := aiFixtureWorldBytes(w, raw)
			if fmt.Sprintf("%x", sha256.Sum256(after)) != fixture.Hash {
				for _, c := range fixture.Changes {
					if after[c.Address] != c.Value {
						t.Errorf("BSS%04x actual%02x native%02x", c.Address, after[c.Address], c.Value)
					}
				}
				t.Fatal("complete native AI startup/scheduler World BSS differs")
			}
			changes := []nativeHeroChange{}
			for at, value := range raw {
				if value != after[at] {
					changes = append(changes, nativeHeroChange{Address: at, Value: after[at]})
				}
			}
			if !reflect.DeepEqual(changes, fixture.Changes) {
				t.Fatal("native AI startup write ranges differ")
			}
		})
	}
	if !reflect.DeepEqual(counts, map[string]int{"controls": 24, "templates": 1192, "commands": 20}) {
		t.Fatalf("native AI setup coverage differs: %v", counts)
	}
}
