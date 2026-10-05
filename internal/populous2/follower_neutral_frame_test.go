package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type neutralFrameFixture struct {
	Input  frameContextInput
	Frames []struct {
		Boundary uint32
		D        [8]uint32
		Hash     string
		ErrorPC  uint32
	}
}

func TestNativeFollowerNeutralFrameAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/follower_neutral_frame_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []neutralFrameFixture }
	if e = json.Unmarshal(data, &catalog); e != nil {
		t.Fatal(e)
	}
	if len(catalog.Cases) != 372 {
		t.Fatal("native terrain follower context coverage incomplete")
	}
	rules, e := DecodeNativeFollowerNeutralFrameRules(testBundle(t).Executable)
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			initial := frameContextInitial(f.Input)
			w := aiFixtureWorld(t, initial)
			w.nativeCallDepth++
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			cb := NativeFollowerNeutralFrameCallbacks{Memory: w.nativeCleanupMemory(), Frame: &frame, Unlink: w.nativeRuntimeUnlink, Random: func() uint16 { return uint16(w.random()) }, Call: func(routine uint32, c *NativeFrameRegisterContext) error {
				command := c.CommandContext()
				var e error
				if routine == 0xd7f0 {
					_, e = w.commandDirectTerrain(NativeCommandCall{Routine: int(routine), Context: &command}, false)
				} else {
					_, e = w.nativeNormalCommandCallbacks(NativeCommandWorldBindings{}).Call(NativeCommandCall{Routine: int(routine), Context: &command})
				}
				c.SetCommandContext(command)
				return e
			},
				Cleanup: func(ref NativeRecordReference, c *NativeFrameRegisterContext) error {
					_, e := CleanupFollowerWithFrame(ref, c, FollowerCleanupCallbacks{Memory: w.nativeCleanupMemory(), Unlink: w.nativeRuntimeUnlink, Insert: w.nativeRuntimeInsert, ClearFarms: w.clearNativeFarms})
					return e
				},
				Move: func(ref NativeRecordReference, c *NativeFrameRegisterContext) error {
					command := c.CommandContext()
					e := w.commandMove(cleanupRecordAddress(ref), &command)
					c.SetCommandContext(command)
					return e
				},
			}
			boundary, e := rules.Tick(NativeRecordReference(f.Input.Actor), cb)
			w.nativeCallDepth--
			want := f.Frames[0]
			if want.ErrorPC == 0 {
				if e != nil {
					t.Fatal(e)
				}
				if boundary != want.Boundary {
					t.Fatalf("body boundary got%x native%x", boundary, want.Boundary)
				}
			} else {
				if e == nil || want.ErrorPC != 0x119b0 {
					t.Fatalf("original address fault not retained: %x/%v", want.ErrorPC, e)
				}
			}
			if frame.D != want.D {
				t.Fatalf("body registers got%08x native%08x", frame.D, want.D)
			}

			b := aiFixtureWorldBytes(w, initial)
			copy(b[0xeb90:], w.NativeRedrawBytes[:])
			if fmt.Sprintf("%x", sha256.Sum256(b)) != want.Hash {
				t.Fatal("complete terrain follower BSS/RNG differs")
			}
		})
	}
}
