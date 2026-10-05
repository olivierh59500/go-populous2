package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestWorldNativeFollowerWaitContactFullFrameAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_wait_contact_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		BSSBytes int
		Cases    []waitContactFrameFixture
	}
	if err := json.Unmarshal(data, &corpus); err != nil || corpus.BSSBytes != 0x11280 || len(corpus.Cases) != 947 {
		t.Fatalf("native waiting/contact World corpus incomplete: %v", err)
	}
	rules, err := DecodeNativeFollowerWaitContactFrameRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	modes, children, reserved := map[string]int{}, map[string]int{}, map[string]int{}
	traps := 0
	for _, f := range corpus.Cases {
		modes[f.Input.Mode]++
		t.Run(f.Input.Name, func(t *testing.T) {
			raw := waitContactFrameMemory(f.Input)
			expected := append([]byte(nil), raw...)
			for _, p := range f.Changes {
				if p.Address < 0 || p.Address >= len(expected) {
					t.Fatal("native waiting/contact changed byte outside full BSS")
				}
				expected[p.Address] = p.Value
			}
			w := installNativeFixtureWorld(t, raw, nil)
			copy(w.NativeRedrawBytes[:], raw[0xeb90:])
			w.hydrateNativeRuntimeGraph()
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			cb := w.nativeWaitContactFrameCallbacks(&frame)
			merge, contact := cb.Merge, cb.Contact
			calls := []heroFrameCall{}
			cb.Merge = func(source, target NativeRecordReference, c *NativeFrameRegisterContext) error {
				children["merge"]++
				if target == 0 {
					reserved["merge"]++
				}
				calls = append(calls, heroFrameCall{Kind: "merge", Source: uint16(source), Target: uint16(target), D: c.D})
				return merge(source, target, c)
			}
			cb.Contact = func(source, target NativeRecordReference, c *NativeFrameRegisterContext) (FollowerContactStep, error) {
				children["contact"]++
				if target == 0 {
					reserved["contact"]++
				}
				calls = append(calls, heroFrameCall{Kind: "contact", Source: uint16(source), Target: uint16(target), D: c.D})
				return contact(source, target, c)
			}
			var step NativeFollowerWaitContactFrameStep
			w.nativeCallDepth++
			if f.Input.Mode == "wait" {
				step, err = rules.TickWaiting(52, cb)
			} else if f.Input.Mode == "contact" {
				step, err = rules.CompleteContact(52, cb)
			} else {
				t.Fatal("unknown native waiting/contact mode")
			}
			w.nativeCallDepth--
			if f.Trap != "" {
				traps++
				if f.Trap != "odd_code_word" || err == nil || !strings.Contains(err.Error(), "is odd") {
					t.Fatalf("native waiting/contact odd-address prefix changed: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if frame.D != f.D {
				t.Errorf("native waiting/contact World all8D differ: got%x want%x", frame.D, f.D)
			}
			if !reflect.DeepEqual(calls, f.Calls) {
				t.Errorf("native waiting/contact World actual child context differs: got%+v want%+v", calls, f.Calls)
			}
			wantMerged, wantContact := false, false
			var target uint16
			for _, call := range f.Calls {
				wantMerged = wantMerged || call.Kind == "merge"
				wantContact = wantContact || call.Kind == "contact"
				target = call.Target
			}
			// A zero target can be the reserved physical record at $76c0.
			// Admission is represented by Merged/Contact, not target!=0.
			if step.Merged != wantMerged || step.Contact != wantContact || ((wantMerged || wantContact) && uint16(step.Target) != target) {
				t.Errorf("native waiting/contact World target admission differs: %+v calls%+v", step, f.Calls)
			}
			all := nativeFixtureWorldImage(w, raw)
			copy(all[0xeb90:], w.NativeRedrawBytes[:])
			if got := fmt.Sprintf("%x", sha256.Sum256(all)); got != f.Hash {
				shown := 0
				for a := range all {
					if all[a] != expected[a] && shown < 12 {
						t.Logf("raw BSS byte%#x differs: got%02x native%02x", a, all[a], expected[a])
						shown++
					}
				}
				t.Errorf("native waiting/contact World full BSS hash differs: got%s want%s", got, f.Hash)
			}
			if binary.BigEndian.Uint32(all[0xeb28:]) != f.RNG {
				t.Error("native waiting/contact World RNG differs")
			}
			if f.Trap == "" && fmt.Sprintf("%x", step.Continuation) != f.Exit {
				t.Errorf("native waiting/contact World continuation differs: %+v exit%s", step, f.Exit)
			}
			if f.ReturnedA0 != 52 {
				t.Errorf("native waiting/contact source was not restored: %x", f.ReturnedA0)
			}
		})
	}
	if modes["wait"] != 486 || modes["contact"] != 461 || traps != 36 || children["merge"] != 148 || children["contact"] != 229 || reserved["merge"] != 1 || reserved["contact"] != 62 {
		t.Fatalf("native waiting/contact World coverage incomplete: modes%v traps%d children%v reserved%v", modes, traps, children, reserved)
	}
}
