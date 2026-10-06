package populous2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNativeRuntimeMainEditorChildAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/render_frame_main_editor_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Input struct {
				Name   string
				X, Y   uint16
				D      [8]uint32
				Events []struct {
					Keys   []byte
					VBlank bool
				}
			}
			Frames []struct {
				PC                             int
				D                              [8]uint32
				BSSHash, ChipHash, PointerHash string
				ScratchHash                    string
				Fields, Debug, ImageBank       []byte
				LastY                          uint16
				Selector, Patch, Copper        uint32
			}
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 20 {
		t.Fatal("main/editor reference corpus incomplete", err)
	}
	bundle := testBundle(t)
	rules, err := nativeLiveActorRulesTest(t, bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}

	snapshots := 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			h := nativeRuntimeHostTest(t)
			presentation := h.Session.Presentation
			var err error
			sprites, err := DecodeNativeSpriteBitmapBank(bundle, 0)
			if err != nil {
				t.Fatal(err)
			}
			tiles, err := DecodeNativeTileBitmapBank(bundle.Raw["block0.pak"])
			if err != nil {
				t.Fatal(err)
			}

			for i := 0x408; i < len(presentation.Chip); i++ {
				presentation.Chip[i] = byte(i*17 + 3 + ((i-0x408)/32000)*91)
			}
			presentation.InterruptChain = false
			if _, err := presentation.Initialize(bundle.Executable, NativeMouseSample{}); err != nil {
				t.Fatal(err)
			}
			memory := h.Memory.BSS
			for _, patch := range []nativeHeroPatch{
				{0xeb42, 2, 1}, {0xeb44, 2, 8}, {0xeb6a, 4, 0x200000 + 0xeb56}, {0xdde, 2, 123}, {0xde0, 2, 46}, {0xde2, 1, 20}, {0xde3, 1, 30},
				{0x140, 2, 1}, {0x134, 2, uint32(f.Input.X)}, {0x136, 2, uint32(f.Input.Y)}, {0x22, 4, 0xa20000}, {0xf0c, 2, 8}, {0xf0e, 2, 1}, {0x5f44, 2, 20}, {0x5f46, 2, 20},
			} {
				renderFramePatch(memory, patch)
			}
			code := h.Code.RawData()
			codeMemory := h.Memory.Code
			_ = codeMemory.Write16(0x3ea, 0)
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			image := &h.Session.Image
			state := NativeMainRenderState{}
			children, err := h.NewRenderChildren(NativeRuntimeRenderChildrenCallbacks{Sound: nativeRuntimeDisabledRenderSound(t, h)})
			if err != nil {
				t.Fatal(err)
			}
			background := make([]byte, 32000)
			for i := range background {
				background[i] = byte(i*53 + 17)
			}
			resolve := func(address uint32) ([]byte, error) {
				at, err := presentation.chipAt(address, 32000)
				if err != nil {
					return nil, err
				}
				return presentation.Chip[at : at+32000], nil
			}
			var editorStep NativeEditorFrameStep
			advance := func() (bool, error) {
				target, err := memory.Read32(0x1e)
				if err != nil {
					return false, err
				}
				bitmap, err := resolve(target)
				if err != nil {
					return false, err
				}
				at := int(target - presentation.ChipBase)
				window := NativeBitmapWindow{Bytes: presentation.Chip, BitmapOffset: at}
				world := NativeWorldRenderCallbacks{
					Effects: NativeActorEffectsCallbacks{NativeRenderFrameCallbacks: NativeRenderFrameCallbacks{Memory: memory, Frame: &frame, Image: image, Input: &presentation.Input, Bitmap: bitmap, Sprite: sprites.Paint}, Cropped: sprites.PaintCropped, Reinterpreted: sprites.PaintReinterpreted},
					Tiles:   tiles, Background: background, Tile: func(q NativeTileChunkRequest, _ []byte) error { return tiles.PaintChunkWindow(q, window) },
				}
				return rules.AdvanceMain(NativeMainRenderCallbacks{World: world, Code: codeMemory,
					RefreshTargets: func(w *NativeWorldRenderCallbacks) error {
						target, err := memory.Read32(0x1e)
						if err != nil {
							return err
						}
						bitmap, err := resolve(target)
						if err != nil {
							return err
						}
						w.Effects.Bitmap = bitmap
						window := NativeBitmapWindow{Bytes: presentation.Chip, BitmapOffset: int(target - presentation.ChipBase)}
						w.Tile = func(q NativeTileChunkRequest, _ []byte) error { return tiles.PaintChunkWindow(q, window) }
						return nil
					},
					PaintingAdvance: func(c *NativeFrameRegisterContext) (bool, error) {
						done, err := children.AdvanceEditor(c)
						editorStep = children.EditorStep
						return done, err
					},
					DebugOverlay: func(c *NativeFrameRegisterContext) error {
						_, err := RenderNativeDebugFrame(NativeDebugFrameCallbacks{Code: codeMemory, CodeBase: 0x100000, Frame: c, FormatAddress: 0x100000 + 0xef6, Bitmap: func(address uint32) (NativeBitmapWindow, error) {
							at, err := presentation.chipAt(address, 32000)
							return NativeBitmapWindow{Bytes: presentation.Chip, BitmapOffset: at}, err
						}})
						return err
					},
				}, &state)
			}
			check := func(index int, complete bool) {
				t.Helper()
				want := f.Frames[index]
				pc := editorStep.PC
				if editorStep.Waiting && editorStep.ChildRoutine == 0 {
					pc = 0x786
				}
				if complete {
					pc = 0
				}
				if pc != want.PC || frame.D != want.D {
					t.Errorf("main/editor registers/PC at%d: PC%x/%x D%x/%x", index, pc, want.PC, frame.D, want.D)
				}
				if got := fileFrameHash(fileFrameMemoryBytes(t, memory)); got != want.BSSHash {
					t.Errorf("main/editor BSS at%d differs: %s/%s", index, got, want.BSSHash)
				}
				if got := fileFrameHash(presentation.Chip); got != want.ChipHash {
					t.Errorf("main/editor chip at%d differs: %s/%s", index, got, want.ChipHash)
				}
				if got := fileFrameHash(presentation.PointerData); got != want.PointerHash {
					t.Errorf("main/editor pointer at%d differs: %s/%s", index, got, want.PointerHash)
				}
				if !bytes.Equal(code[0x37bc:0x381e], want.Fields) || fileFrameHash(code[0xab4e:0xab4e+2048]) != want.ScratchHash || !bytes.Equal(code[0x2e2a:0x2f08], want.Debug) {
					t.Errorf("main/editor retained CODE at%d differs", index)
				}
				if !bytes.Equal(image.AudioBank[:], want.ImageBank) || image.LastY != want.LastY {
					t.Errorf("main/editor shared image bank at%d differs", index)
				}
				if presentation.CopperSelector != want.Selector || presentation.SpritePatchPointer != want.Patch || presentation.ActiveCopper != want.Copper {
					t.Errorf("main/editor Copper at%d differs", index)
				}
				if !complete && (state.Step != 4 || !state.painting) {
					t.Fatal("main renderer did not retain the actual editor call")
				}
				snapshots++
			}
			for i := 0; i < 2; i++ {
				complete, err := advance()
				if err != nil {
					t.Fatal(err)
				}
				check(i, complete)
			}
			for i, event := range f.Input.Events {
				if event.VBlank {
					if _, err := presentation.VBlank(NativeMouseSample{}, memory, &frame); err != nil {
						t.Fatal(err)
					}
				}
				for _, wire := range event.Keys {
					if err := presentation.Input.KeyboardInterrupt(wire); err != nil {
						t.Fatal(err)
					}
				}
				complete, err := advance()
				if err != nil {
					t.Fatal(err)
				}
				check(i+2, complete)
			}
			if !children.Editor.Complete || state.Step != 11 {
				t.Fatal("main renderer did not resume to the actual physics boundary")
			}
		})
	}
	if snapshots != 96 {
		t.Fatal(fmt.Sprintf("main/editor reference snapshots: %d", snapshots))
	}
}
