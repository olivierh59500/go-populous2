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

type completePhysicsInput struct {
	frameContextInput
	Landscape  int
	ScreenSeed uint8
	Frames     int
}
type completePhysicsFixture struct {
	Input               completePhysicsInput
	InitialHash         string
	D                   [8]uint32
	ErrorPC, ErrorStage uint32
	Frames              []struct {
		Changes                       []nativeHeroChange
		Iteration                     int
		Routine                       uint32
		D                             [8]uint32
		Hash, ScreenHash, TerrainHash string
		Audio                         []byte
		Channels                      [4]uint16
		RNG                           uint32
	}
}

func TestCompleteNativePhysicsFrameAgainstOriginalMain(t *testing.T) {
	data, e := os.ReadFile("testdata/complete_physics_frame_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []completePhysicsFixture }
	if e = json.Unmarshal(data, &catalog); e != nil {
		t.Fatal(e)
	}
	if len(catalog.Cases) != 168 {
		t.Fatal("complete main physics coverage incomplete")
	}
	bundle := testBundle(t)
	rules := [4]*NativeFollowerFrameRules{}
	for land := range rules {
		rules[land], e = DecodeNativeFollowerFrameRules(bundle, land)
		if e != nil {
			t.Fatal(e)
		}
	}
	wall, e := DecodeNativeWallRules(bundle.Executable)
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			initial := frameContextInitial(f.Input.frameContextInput)
			w := aiFixtureWorld(t, initial)
			w.Landscape = bundle.Landscapes[f.Input.Landscape]
			w.NativeAI.Code = append([]byte(nil), w.NativeAI.Code...)
			if e := w.retainNativeLAND(bundle.Raw[fmt.Sprintf("land%d.dat", f.Input.Landscape)]); e != nil {
				t.Fatal(e)
			}
			w.FollowerWin, e = DecodeFollowerWinRules(bundle.Executable, w.Landscape)
			if e != nil {
				t.Fatal(e)
			}
			presentation, e := NewNativeFramePresentationState(bundle.Executable, 0x500000, 0x400000)
			if e != nil {
				t.Fatal(e)
			}
			for at := 0; at < 0xdc2; at++ {
				if e := presentation.Memory(w.nativeCleanupMemory()).Write8(at, initial[at]); e != nil {
					t.Fatal(e)
				}
			}
			bitmap, e := presentation.BackBuffer()
			if e != nil {
				t.Fatal(e)
			}
			for i := range bitmap {
				bitmap[i] = byte((i*73 + int(f.Input.ScreenSeed)*19) % 256)
			}
			terrainBitmap := presentation.Chip[0x408 : 0x408+32000]
			for i := range terrainBitmap {
				terrainBitmap[i] = byte((i*31 + int(f.Input.ScreenSeed)*43) % 256)
			}
			w.nativeTerrainPoint = func(c *NativeCommandRegisterContext) error {
				point := NativeFrameRegisterContext{D: c.D, AddressBase: 0x200000}
				plan, e := PlanNativeMapPoint(&point)
				c.D = point.D
				if e != nil {
					return e
				}
				return plan.Paint(terrainBitmap)
			}
			memory := presentation.Memory(w.nativeCleanupMemory())
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			audio, e := DecodeNativeFrameAudioState(bundle.Executable)
			if e != nil {
				t.Fatal(e)
			}
			for _, p := range f.Input.Code {
				if p.Address >= 0x185a8 && p.Address < 0x18ada {
					at := p.Address - 0x185a8
					switch p.Width {
					case 1:
						audio.Entries[at] = byte(p.Value)
					case 2:
						binary.BigEndian.PutUint16(audio.Entries[at:], uint16(p.Value))
					case 4:
						binary.BigEndian.PutUint32(audio.Entries[at:], p.Value)
					}
				} else if p.Address >= 0x18426 && p.Address < 0x1842e {
					audio.Channels[(p.Address-0x18426)/2] = uint16(p.Value)
				}
			}
			imageRules, e := DecodeNativeEditorCursorRules(bundle.Executable)
			if e != nil {
				t.Fatal(e)
			}
			imageState := imageRules.NewImageState()
			copy(imageState.AudioBank[:], audio.Entries[:])
			paint := func(c *NativeFrameRegisterContext) error {
				plan, e := PlanNativeMapPoint(c)
				if e != nil {
					return e
				}
				return plan.Paint(bitmap)
			}
			commands := NativeCommandWorldBindings{WallRules: &wall}
			state := rules[f.Input.Landscape].NewState()
			bindings := NativeFrameContinuationBindings{World: NativeFrameWorldBindings{Audio: &audio, MapPoint: paint, Commands: commands}, Followers: func(c *NativeFrameRegisterContext) (bool, error) {
				e := rules[f.Input.Landscape].Tick(w, c, state, NativeFollowerFrameOutputs{MapPoint: func(_ uint16, c *NativeFrameRegisterContext) error { return paint(c) }, Commands: commands})
				return e == nil, e
			}, Audio: NativeFrameAudioCallbacks{Command: func(_, _ uint16, input uint32) (uint32, error) { return input, nil }}}
			w.nativeCallDepth++
			index := 0
			iterations := f.Input.Frames
			if iterations == 0 {
				iterations = 1
			}
			for iteration := 0; iteration < iterations; iteration++ {
				// The source starts after the last prephysics EE32 draw. Both producers
				// and183CE consume the same descriptor bank, retained across boundaries.
				copy(audio.Entries[:], imageState.AudioBank[:])
				cb := w.nativeFrameCallbacks(bindings)
				cb.Memory = memory
				capture := func(routine uint32, call NativeFrameStageCallback) error {
					complete, e := call(&frame)
					if e != nil {
						return e
					}
					if !complete {
						return fmt.Errorf("unexpected pending physics routine%x", routine)
					}
					if index >= len(f.Frames) {
						return fmt.Errorf("unexpected physics boundary%x", routine)
					}
					want := f.Frames[index]
					index++
					if want.Routine != routine || want.Iteration != iteration {
						return fmt.Errorf("physics order got%x/%d native%x/%d", routine, iteration, want.Routine, want.Iteration)
					}
					if frame.D != want.D {
						return fmt.Errorf("physics%x registers got%08x native%08x", routine, frame.D, want.D)
					}
					all := aiFixtureWorldBytes(w, initial)
					copy(all[0xeb90:], w.NativeRedrawBytes[:])
					for at := 0; at < 0xdc2; at++ {
						v, e := memory.Read8(at)
						if e != nil {
							return e
						}
						all[at] = v
					}
					if got := fmt.Sprintf("%x", sha256.Sum256(all)); got != want.Hash {
						expected := append([]byte(nil), initial...)
						for _, change := range want.Changes {
							expected[change.Address] = change.Value
						}
						diff := []string{}
						for at, v := range all {
							if v != expected[at] && len(diff) < 12 {
								diff = append(diff, fmt.Sprintf("%x:%02x/%02x", at, v, expected[at]))
							}
						}
						return fmt.Errorf("physics%x fullBSS/RNG differs got%s native%s at%v", routine, got, want.Hash, diff)
					}
					if !reflect.DeepEqual(audio.Entries[:], want.Audio) || audio.Channels != want.Channels {
						return fmt.Errorf("physics%x sharedaudio descriptors/channels differ", routine)
					}
					if got := fmt.Sprintf("%x", sha256.Sum256(bitmap)); got != want.ScreenHash {
						return fmt.Errorf("physics%x actual back-screen pixels differ", routine)
					}
					if got := fmt.Sprintf("%x", sha256.Sum256(terrainBitmap)); got != want.TerrainHash {
						return fmt.Errorf("physics%x separate BSS22 terrain pixels differ", routine)
					}
					return nil
				}
				paused, _ := memory.Read16(0xf3c)
				skip, _ := memory.Read16(0xf3e)
				if paused == 0 {
					if skip == 0 {
						for _, step := range []struct {
							routine uint32
							call    NativeFrameStageCallback
						}{{0x11252, cb.Followers}, {0x1383c, cb.AI}, {0x1482e, cb.FX}} {
							if e := capture(step.routine, step.call); e != nil {
								t.Fatal(e)
							}
						}
					}
					for _, step := range []struct {
						routine uint32
						call    NativeFrameStageCallback
					}{{0x161cc, cb.Walls}, {0xde36, cb.Forest}, {0x17e9a, cb.Scenario}, {0x182ce, cb.Audio}} {
						if e := capture(step.routine, step.call); e != nil {
							t.Fatal(e)
						}
					}
				}
				copy(imageState.AudioBank[:], audio.Entries[:])
				clock, e := memory.Read32(0xf40)
				if e != nil {
					t.Fatal(e)
				}
				if e := memory.Write32(0xf40, clock+1); e != nil {
					t.Fatal(e)
				}
			}
			w.nativeCallDepth--
			if f.ErrorPC != 0 {
				t.Fatalf("unexpected original main fault%x at stage%x", f.ErrorPC, f.ErrorStage)
			}
			if index != len(f.Frames) || frame.D != f.D {
				t.Fatal("complete physics boundary sequence incomplete")
			}
		})
	}
}
