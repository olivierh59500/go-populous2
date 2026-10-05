package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

// This proves the complete original policy corpus followed by its isolated
// later command stage. The original policy output supplies every incoming
// command data register explicitly; it does not invent frame initialization
// or claim to include the intervening FX/scenery/presentation register flow.
func TestWorldNativeAIPolicyAndDeferredCommandsAgainstCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/command_ai_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct {
		Cases []struct {
			Input                      aiNativeInput
			PolicyHash, Hash           string
			PolicyRegisters, Registers [8]uint32
			PolicyChanges, Changes     []nativeHeroChange
			ErrorPC                    int
			Calls                      []struct {
				Address int
				Context [8]uint32
			}
		}
	}
	if e := json.Unmarshal(data, &catalog); e != nil {
		t.Fatal(e)
	}
	if len(catalog.Cases) != 512 {
		t.Fatal("native full AI command replay corpus incomplete")
	}
	bundle := testBundle(t)
	rules, e := DecodeNativeCommandRules(bundle.Executable)
	if e != nil {
		t.Fatal(e)
	}
	wall, e := DecodeNativeWallRules(bundle.Executable)
	if e != nil {
		t.Fatal(e)
	}
	faults := 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			m := aiNativeFixtureMemory(f.Input)
			initial := make([]byte, 131072)
			copy(initial, m[:])
			initial[0xeb5e], initial[0xeb68] = 2, 4
			w := aiFixtureWorld(t, initial)
			w.nativeCallDepth++
			policyContext := NativeAIRegisterContext{D4: 0xccdd, D5: 0x6778}
			if _, e := w.tickNativeAI(&w.NativeAI, &policyContext); e != nil {
				t.Fatal(e)
			}
			export := func() []byte {
				b := aiFixtureWorldBytes(w, initial)
				copy(b[0xeb90:0x11280], w.NativeRedrawBytes[:])
				return b
			}
			if fmt.Sprintf("%x", sha256.Sum256(export())) != f.PolicyHash {
				t.Fatal("native full AI policy memory/RNG differs before command stage")
			}
			if policyContext.D4 != uint16(f.PolicyRegisters[4]) || policyContext.D5 != uint16(f.PolicyRegisters[5]) {
				t.Fatal("native AI context differs before command stage")
			}
			// The typed policy adapter currently exposes D4/D5. All remaining
			// original caller registers are explicit fixture inputs to this stage.
			input := NativeCommandRegisterContext{D: f.PolicyRegisters}
			saved := input
			calls := []struct {
				Address int
				Context [8]uint32
			}{}
			placement := NativeWallPlacementState{}
			_, err := w.executeNativeDeferredCommands(input, NativeDeferredCommandCallbacks{Execute: func(address int, context *NativeCommandRegisterContext) error {
				calls = append(calls, struct {
					Address int
					Context [8]uint32
				}{address, context.D})
				_, e := w.executeNativeNormalCommand(&rules, address, context, NativeCommandWorldBindings{WallRules: &wall, WallPlacement: &placement})
				return e
			}})
			w.nativeCallDepth--
			if f.ErrorPC != 0 {
				faults++
				if f.ErrorPC != 0x13052 || err == nil {
					t.Fatalf("native invalid state-table fault%x did not remain explicit: %v", f.ErrorPC, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if input != saved {
				t.Fatal("native1744c changed enclosing caller context")
			}
			if f.ErrorPC == 0 && input.D != f.Registers {
				t.Fatal("native1744c restored registers differ")
			}
			if !reflect.DeepEqual(calls, f.Calls) {
				t.Fatalf("native side1/side2 command contexts differ: %+v native%+v", calls, f.Calls)
			}
			if fmt.Sprintf("%x", sha256.Sum256(export())) != f.Hash {
				t.Fatal("native AI/deferred full memory/RNG differs")
			}
		})
	}
	if faults != 24 {
		t.Fatalf("native synthetic state0 fault coverage%d", faults)
	}
}
