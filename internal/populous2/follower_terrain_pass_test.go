package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNativeFollowerTerrainFrameCompletePassAgainstCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/follower_terrain_pass_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []terrainFrameFixture }
	if e = json.Unmarshal(data, &catalog); e != nil {
		t.Fatal(e)
	}
	if len(catalog.Cases) != 72 {
		t.Fatal("terrain complete pass coverage incomplete")
	}
	rules, e := DecodeNativeFollowerTerrainFrameRules(testBundle(t).Executable)
	if e != nil {
		t.Fatal(e)
	}
	passRules, e := DecodeNativeFollowerPassRules(testBundle(t).Executable)
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			initial := frameContextInitial(f.Input)
			w := aiFixtureWorld(t, initial)
			w.nativeCallDepth++
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			state := NativeFollowerPassState{}
			terrain := NativeFollowerTerrainFrameCallbacks{Memory: w.nativeCleanupMemory(), Frame: &frame, Unlink: w.nativeRuntimeUnlink,
				Cleanup: func(ref NativeRecordReference, c *NativeFrameRegisterContext) error {
					_, e := CleanupFollowerWithFrame(ref, c, FollowerCleanupCallbacks{Memory: w.nativeCleanupMemory(), Unlink: w.nativeRuntimeUnlink, Insert: w.nativeRuntimeInsert, ClearFarms: w.clearNativeFarms})
					return e
				},
				ClearLeader: func(ref NativeRecordReference, c *NativeFrameRegisterContext) error {
					_, e := ClearFollowerLeaderWithFrame(ref, c, FollowerLeaderCallbacks{Memory: w.nativeCleanupMemory(), Unlink: w.nativeRuntimeUnlink, Insert: w.nativeRuntimeInsert})
					return e
				},
				Move: func(ref NativeRecordReference, c *NativeFrameRegisterContext) error {
					command := c.CommandContext()
					e := w.commandMove(cleanupRecordAddress(ref), &command)
					c.SetCommandContext(command)
					return e
				},
			}
			bindings := NativeFollowerFrameBindings{Terrain: &rules, Aftermath: &rules.Aftermath, Other: func(ref NativeRecordReference, _ uint16, _ *NativeFrameRegisterContext, _ *NativeFollowerPassState) (NativeFollowerPassFlow, error) {
				boundary, e := rules.Tick(ref, terrain)
				switch boundary {
				case 0x123b4:
					return NativeFollowerCount, e
				case 0x12462:
					return NativeFollowerNext, e
				default:
					return NativeFollowerNext, fmt.Errorf("unproven terrain search continuation%x", boundary)
				}
			}}
			e = w.tickNativeFollowerFrame(&passRules, &frame, &state, bindings)
			w.nativeCallDepth--
			if e != nil {
				t.Fatal(e)
			}
			want := f.Frames[0]
			if want.ErrorPC != 0 {
				t.Fatal("unexpected original CPU fault")
			}
			if frame.D != want.D {
				t.Fatalf("complete pass registers got%08x native%08x", frame.D, want.D)
			}
			b := aiFixtureWorldBytes(w, initial)
			copy(b[0xeb90:], w.NativeRedrawBytes[:])
			if fmt.Sprintf("%x", sha256.Sum256(b)) != want.Hash {
				t.Fatal("complete400slot terrain pass BSS/RNG differs")
			}
		})
	}
}
