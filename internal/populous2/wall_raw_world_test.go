package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestWorldCityWallsAgainstCompleteOriginalMemory(t *testing.T) {
	data, err := os.ReadFile("testdata/wall_native_full.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []wallNativeFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 1193 {
		t.Fatal("complete World wall catalog missing")
	}
	rules, err := DecodeNativeWallRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	frames := 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			m := wallNativeFixtureMemory(f.Input)
			raw := whirlwindNativeBytes(m)
			w := installNativeFixtureWorld(t, raw, nil)
			w.Core.SetRandomState(binary.BigEndian.Uint32(raw[0xeb28:]))
			w.nativeCallDepth++
			defer func() { w.nativeCallDepth-- }()
			memory := w.nativeCleanupMemory()
			base := w.nativeWallCallbacks(f.Input.SourceD7)
			cb := base
			calls := []whirlwindNativeCall{}
			cb.Link = func(ref NativeRecordReference) error {
				calls = append(calls, whirlwindNativeCall{Kind: "link", Reference: uint16(ref)})
				return base.Link(ref)
			}
			cb.Unlink = func(ref NativeRecordReference) error {
				calls = append(calls, whirlwindNativeCall{Kind: "unlink", Reference: uint16(ref)})
				return base.Unlink(ref)
			}
			cb.Move = func(ref NativeRecordReference, x, y uint16) (bool, error) {
				calls = append(calls, whirlwindNativeCall{Kind: "move", Reference: uint16(ref), X: x, Y: y})
				return base.Move(ref, x, y)
			}
			cb.Enter = func(ref NativeRecordReference) error {
				calls = append(calls, whirlwindNativeCall{Kind: "entry", Reference: uint16(ref)})
				return base.Enter(ref)
			}
			placement := NativeWallPlacementState{}
			frameIndex := 0
			for opIndex, op := range f.Input.Operations {
				for _, p := range op.Initial {
					switch p.Width {
					case 1:
						err = memory.Write8(p.Address, uint8(p.Value))
					case 2:
						err = memory.Write16(p.Address, uint16(p.Value))
					case 4:
						err = memory.Write32(p.Address, p.Value)
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				for iteration := range max(1, op.Repeat) {
					calls = []whirlwindNativeCall{}
					ref := NativeRecordReference(op.Reference)
					created, probe, exit := false, int16(0), "return"
					switch op.Kind {
					case "create", "createat":
						x, y := f.Input.X, f.Input.Y
						if op.Kind == "createat" {
							x, y = uint8(op.Target), uint8(op.Target>>8)
						}
						var step NativeWallCreation
						step, err = rules.Create(f.Input.Owner, x, y, &placement, cb)
						created = step.Created
					case "pool":
						_, err = rules.TickPool(cb)
					case "probe":
						probe, err = rules.Hero.Probe(ref, NativePackedTile(uint16(f.Input.Y)<<8|uint16(f.Input.X)), 1, 0, memory)
					case "follower", "fault":
						at := cleanupRecordAddress(ref)
						owner, e := memory.Read8(at + 12)
						if e != nil {
							t.Fatal(e)
						}
						state, e := memory.Read8(at + 22)
						if e != nil {
							t.Fatal(e)
						}
						if owner != 0 && (state == 4 || state == 0x2a) {
							var step NativeWallFollowerStep
							step, err = rules.TickFollower(ref, cb)
							exit = "123b4"
							if step.Redispatch {
								exit = "112b8"
							}
						}
						if op.Kind == "fault" {
							if err == nil {
								t.Fatal("original wall odd-stage fault disappeared")
							}
							err = nil
							exit = "fault"
						}
					default:
						t.Fatal("unknown native World wall operation")
					}
					if err != nil {
						t.Fatalf("operation%d iteration%d: %v", opIndex, iteration, err)
					}
					if frameIndex >= len(f.Frames) {
						t.Fatal("native World wall trace ended early")
					}
					want := f.Frames[frameIndex]
					frameIndex++
					frames++
					all := nativeFixtureWorldImage(w, raw)
					if got := fmt.Sprintf("%x", sha256.Sum256(all)); got != want.Hash || w.Core.RandomState() != want.RNG {
						for _, change := range want.Changes {
							if all[change.Address] != change.Value {
								t.Errorf("native byte%x got%x want%x", change.Address, all[change.Address], change.Value)
							}
						}
						t.Fatalf("World wall complete BSS/RNG differs at frame%d", frameIndex)
					}
					if placement.Neighbors != want.Scratch || !reflect.DeepEqual(calls, want.Calls) {
						t.Fatalf("native wall scratch/callback order differs: %+v/%+v calls%+v/%+v", placement.Neighbors, want.Scratch, calls, want.Calls)
					}
					if (op.Kind == "create" || op.Kind == "createat") && created != want.Admitted {
						t.Fatal("World wall creation admission differs")
					}
					if op.Kind == "probe" && int32(probe) != want.Result {
						t.Fatal("World wall probe condition differs")
					}
					if (op.Kind == "follower" || op.Kind == "fault") && exit != want.Exit {
						t.Fatal("World wall follower route differs")
					}
					for _, image := range want.Images {
						frame, offset, e := rules.Frame(NativeRecordReference(image.Reference), memory)
						if e != nil {
							t.Fatal(e)
						}
						layers := make([][3]int, len(frame.Layers))
						for i, layer := range frame.Layers {
							layers[i] = [3]int{layer.X, layer.Y, layer.Sprite}
						}
						if offset != image.Offset || uint16(frame.SoundCue*10) != image.Cue || !reflect.DeepEqual(layers, image.Layers) {
							t.Fatal("World wall original frame/layers/cue differs")
						}
					}
				}
			}
			if frameIndex != len(f.Frames) {
				t.Fatal("native World wall trace had unused frames")
			}
		})
	}
	if frames != 17763 {
		t.Fatalf("World wall original frame count%d want17763", frames)
	}
}
