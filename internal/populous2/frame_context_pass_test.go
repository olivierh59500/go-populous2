package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestNativeComposedFrameStagesAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/frame_composed_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []frameContextFixture }
	if e = json.Unmarshal(data, &catalog); e != nil {
		t.Fatal(e)
	}
	if len(catalog.Cases) != 54 {
		t.Fatal("composed frame coverage incomplete")
	}
	follower, e := DecodeNativeFollowerPassRules(testBundle(t).Executable)
	if e != nil {
		t.Fatal(e)
	}
	wall, e := DecodeNativeWallRules(testBundle(t).Executable)
	if e != nil {
		t.Fatal(e)
	}
	commandRules, e := DecodeNativeCommandRules(testBundle(t).Executable)
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			initial := frameContextInitial(f.Input)
			w := aiFixtureWorld(t, initial)
			w.nativeCallDepth++
			c := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			audio, e := DecodeNativeFrameAudioState(testBundle(t).Executable)
			if e != nil {
				t.Fatal(e)
			}
			pass := NativeFramePassState{}
			deferred := NativeDeferredFrameState{}
			followers := NativeFollowerPassState{}
			bindings := NativeFrameContinuationBindings{World: NativeFrameWorldBindings{Audio: &audio, Commands: NativeCommandWorldBindings{WallRules: &wall}}, Audio: NativeFrameAudioCallbacks{Command: func(_, _ uint16, input uint32) (uint32, error) { return input, nil }},
				Followers: func(frame *NativeFrameRegisterContext) (bool, error) {
					e := w.tickNativeFollowerFrame(&follower, frame, &followers, NativeFollowerFrameBindings{})
					return e == nil, e
				},
				// The fixture's two screen pointers and vblank word are zero. $072e
				// preserves D0-D7; its Copper/pixel operation remains a real Game callback.
				Swap: func(*NativeFrameRegisterContext) (bool, error) { return true, nil },
				Commands: func(frame *NativeFrameRegisterContext) (bool, error) {
					return deferred.TickDeferredFrame(frame, NativeDeferredFrameCallbacks{Memory: w.nativeCleanupMemory(), Execute: func(at int, context *NativeCommandRegisterContext, _ *uint32) (bool, error) {
						_, e := w.executeNativeNormalCommand(&commandRules, at, context, NativeCommandWorldBindings{WallRules: &wall})
						return e == nil, e
					}})
				},
			}
			cb := w.nativeFrameCallbacks(bindings)
			index := 0
			check := func(routine uint32, callback NativeFrameStageCallback) NativeFrameStageCallback {
				return func(frame *NativeFrameRegisterContext) (bool, error) {
					done, e := callback(frame)
					if e != nil {
						return done, e
					}
					if !done {
						return false, nil
					}
					if index >= len(f.Frames) {
						t.Fatal("unexpected composed frame boundary")
					}
					want := f.Frames[index]
					index++
					if want.Routine != routine {
						t.Fatalf("stage got%x native%x", routine, want.Routine)
					}
					if frame.D != want.D {
						t.Fatalf("stage%x registers got%08x native%08x", routine, frame.D, want.D)
					}
					b := aiFixtureWorldBytes(w, initial)
					copy(b[0xeb90:], w.NativeRedrawBytes[:])
					if fmt.Sprintf("%x", sha256.Sum256(b)) != want.Hash {
						t.Fatalf("stage%x complete BSS/RNG differs", routine)
					}
					if !reflect.DeepEqual(audio.Entries[:], want.Audio) || audio.Channels != want.Channels {
						t.Fatalf("stage%x mutable audio differs", routine)
					}
					return true, nil
				}
			}
			cb.Followers = check(0x11252, cb.Followers)
			cb.AI = check(0x1383c, cb.AI)
			cb.FX = check(0x1482e, cb.FX)
			cb.Walls = check(0x161cc, cb.Walls)
			cb.Forest = check(0xde36, cb.Forest)
			cb.Scenario = check(0x17e9a, cb.Scenario)
			cb.Audio = check(0x182ce, cb.Audio)
			cb.Swap = check(0x072e, cb.Swap)
			originalCommands := cb.Commands
			cb.Commands = func(frame *NativeFrameRegisterContext) (bool, error) {
				if index >= len(f.Frames) || f.Frames[index].Routine != 0x1a58e {
					t.Fatal("native swap-adjacent RTS missing")
				}
				if frame.D != f.Frames[index].D {
					t.Fatal("$1a58e changed native data registers")
				}
				index++
				return check(0x1744c, originalCommands)(frame)
			}
			complete, e := pass.TickFramePass(&c, cb)
			w.nativeCallDepth--
			if e != nil {
				t.Fatal(e)
			}
			if !complete || index != len(f.Frames) {
				t.Fatal("composed stage sequence incomplete")
			}
		})
	}
}
