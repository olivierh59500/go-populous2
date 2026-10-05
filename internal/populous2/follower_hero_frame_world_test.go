package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

// These references run the genuine native child bodies. World callbacks must
// produce their raw mutations and register outputs, not replay fixture deltas.
func TestWorldNativeFollowerHeroFullFrameAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_hero_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		BSSBytes int
		Cases    []heroFrameFixture
	}
	if err := json.Unmarshal(data, &corpus); err != nil || corpus.BSSBytes != 0x11280 || len(corpus.Cases) != 2462 {
		t.Fatalf("native hero World corpus incomplete: %v", err)
	}
	rules, err := DecodeNativeFollowerHeroFrameRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	modes, traps, children := map[string]int{}, map[string]int{}, map[string]int{}
	for _, f := range corpus.Cases {
		modes[f.Input.Mode]++
		traps[f.Trap]++
		t.Run(f.Input.Name, func(t *testing.T) {
			raw := heroFrameMemory(f.Input)
			expected := append([]byte(nil), raw...)
			for _, p := range f.Changes {
				if p.Address < 0 || p.Address >= len(expected) {
					t.Fatal("native hero changed byte outside full BSS fixture")
				}
				expected[p.Address] = p.Value
			}
			w := installNativeFixtureWorld(t, raw, nil)
			// Direct terrain can queue beyond $eb90. Preserve the whole retained
			// redraw backing rather than letting the fixture installer omit it.
			copy(w.NativeRedrawBytes[:], raw[0xeb90:])
			// Retained unlinked towns are still readable by the farm compositor.
			// Import every common prefix, preserving the explicit linked flags.
			w.hydrateNativeRuntimeGraph()
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			cb := w.nativeHeroFrameCallbacks(&frame)
			calls := []heroFrameCall{}
			raise, contact, cleanup := cb.Raise, cb.Contact, cb.Cleanup
			cb.Raise = func(c *NativeFrameRegisterContext) error {
				children["raise"]++
				calls = append(calls, heroFrameCall{Kind: "raise", D: c.D})
				return raise(c)
			}
			cb.Contact = func(source, target NativeRecordReference, c *NativeFrameRegisterContext) (FollowerContactStep, error) {
				children["contact"]++
				calls = append(calls, heroFrameCall{Kind: "contact", Source: uint16(source), Target: uint16(target), D: c.D})
				return contact(source, target, c)
			}
			cb.Cleanup = func(ref NativeRecordReference, c *NativeFrameRegisterContext) error {
				children["cleanup"]++
				calls = append(calls, heroFrameCall{Kind: "cleanup", Source: uint16(ref), D: c.D})
				return cleanup(ref, c)
			}
			step := NativeFollowerHeroFrameStep{RedispatchSource: 52}
			w.nativeCallDepth++
			switch f.Input.Mode {
			case "select":
				_, err = rules.Select(52, cb)
			case "probe":
				frame.Word(0, uint16(raw[0x76f4+8])<<8|uint16(raw[0x76f4+6]))
				frame.Word(2, uint16(f.Input.DX))
				frame.Word(3, uint16(f.Input.DY))
				err = rules.Probe(52, cb)
			case "plan":
				frame.Byte(0, f.Input.TargetX)
				frame.Byte(1, f.Input.TargetY)
				err = rules.Plan(52, cb)
			case "decision24", "decision26":
				step, err = rules.Tick(52, cb)
			case "chase":
				step, err = rules.Chase(52, cb)
			default:
				t.Fatal("unknown native hero fixture mode")
			}
			w.nativeCallDepth--
			if f.Trap == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				if err == nil {
					t.Fatal("native hero exception prefix silently continued")
				}
				if f.Trap == "zero_speed" && !errors.Is(err, ErrFollowerZeroSpeed) {
					t.Fatalf("native hero zero-speed exception changed: %v", err)
				}
				if f.Trap == "odd_code_word" && !strings.Contains(err.Error(), "is odd") {
					t.Fatalf("native hero odd-address exception changed: %v", err)
				}
			}
			if frame.D != f.D {
				t.Errorf("native hero World all8D differ: got%x want%x", frame.D, f.D)
			}
			if !reflect.DeepEqual(calls, f.Calls) {
				t.Errorf("native hero World actual child context differs: got%+v want%+v", calls, f.Calls)
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
				t.Errorf("native hero World full BSS hash differs: got%s want%s", got, f.Hash)
			}
			if binary.BigEndian.Uint32(all[0xeb28:]) != f.RNG {
				t.Error("native hero World RNG differs")
			}
			if f.Input.Mode == "decision24" || f.Input.Mode == "decision26" || f.Input.Mode == "chase" {
				if f.Trap == "" && fmt.Sprintf("%x", step.Continuation) != f.Exit {
					t.Errorf("native hero World continuation differs: %+v exit%s", step, f.Exit)
				}
				if uint16(step.RedispatchSource) != f.ReturnedA0 {
					t.Errorf("native hero World redispatch actor differs: got%x want%x", step.RedispatchSource, f.ReturnedA0)
				}
			}
		})
	}
	if modes["select"] != 66 || modes["probe"] != 1608 || modes["plan"] != 317 || modes["decision24"] != 66 || modes["decision26"] != 81 || modes["chase"] != 324 || traps["odd_code_word"] != 263 || traps["zero_speed"] != 2 || children["raise"] != 23 || children["contact"] != 100 || children["cleanup"] != 60 {
		t.Fatalf("native hero World branch coverage incomplete: modes%v traps%v children%v", modes, traps, children)
	}
}
