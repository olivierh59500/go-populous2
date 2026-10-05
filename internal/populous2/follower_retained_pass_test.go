package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNativeFollowerRetainedFrameCompletePassAgainstCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/follower_retained_pass_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []retainedFrameFixture }
	if e = json.Unmarshal(data, &catalog); e != nil {
		t.Fatal(e)
	}
	if len(catalog.Cases) != 32 {
		t.Fatal("terrain complete pass coverage incomplete")
	}
	rules, e := DecodeNativeFollowerRetainedFrameRules(testBundle(t).Executable)
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
			terrain := NativeFollowerRetainedFrameCallbacks{Memory: w.nativeCleanupMemory(), Frame: &frame, Unlink: w.nativeRuntimeUnlink,
				ClearLeader: func(ref NativeRecordReference, c *NativeFrameRegisterContext) error {
					_, e := ClearFollowerLeaderWithFrame(ref, c, FollowerLeaderCallbacks{Memory: w.nativeCleanupMemory(), Unlink: w.nativeRuntimeUnlink, Insert: w.nativeRuntimeInsert})
					return e
				},
				Attrition: func(ref NativeRecordReference, god int, c *NativeFrameRegisterContext) (bool, error) {
					return ApplyFollowerAttritionWithFrame(ref, god, FollowerAttritionFrameCallbacks{Memory: w.nativeCleanupMemory(), Frame: c, CleanupFrame: func(ref NativeRecordReference, c *NativeFrameRegisterContext) error {
						_, e := CleanupFollowerWithFrame(ref, c, FollowerCleanupCallbacks{Memory: w.nativeCleanupMemory(), Unlink: w.nativeRuntimeUnlink, Insert: w.nativeRuntimeInsert, ClearFarms: w.clearNativeFarms})
						return e
					}})
				},
				Plan: func(ref NativeRecordReference, c *NativeFrameRegisterContext) error {
					hero, e := DecodeNativeFollowerHeroFrameRules(testBundle(t).Executable)
					if e != nil {
						return e
					}
					return hero.Plan(ref, w.nativeHeroFrameCallbacks(c))
				},
			}
			aftermath, e := DecodeNativeFollowerAftermathFrameRules(testBundle(t).Executable)
			if e != nil {
				t.Fatal(e)
			}
			bindings := NativeFollowerFrameBindings{Aftermath: &aftermath, Other: func(ref NativeRecordReference, _ uint16, _ *NativeFrameRegisterContext, _ *NativeFollowerPassState) (NativeFollowerPassFlow, error) {
				boundary, e := rules.Tick(ref, terrain)
				switch boundary {
				case 0x123b4:
					return NativeFollowerCount, e
				case 0x12462:
					return NativeFollowerNext, e
				case 0x112b8:
					return NativeFollowerRedispatch, e
				default:
					return NativeFollowerNext, fmt.Errorf("unproven terrain search continuation%x", boundary)
				}
			}}
			cb := w.nativeFollowerFrameCallbacks(&frame, &state, bindings)
			originalBody := cb.Body
			cb.Body = func(ref NativeRecordReference, target uint16, c *NativeFrameRegisterContext, s *NativeFollowerPassState) (NativeFollowerPassFlow, error) {
				rawState, e := w.nativeCleanupMemory().Read8(cleanupRecordAddress(ref) + 22)
				if e != nil {
					return NativeFollowerNext, e
				}
				if rawState == 0x34 || rawState == 0x46 {
					return bindings.Other(ref, target, c, s)
				}
				return originalBody(ref, target, c, s)
			}
			e = passRules.Tick(cb)
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
