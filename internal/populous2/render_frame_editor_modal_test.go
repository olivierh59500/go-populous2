package populous2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type editorModalFixture struct {
	Input struct {
		Name   string
		Action int
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
		Fields                         []byte
		Selector, Patch, Copper        uint32
	}
}

func TestNativeEditorNumericModalCompleteAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/render_frame_editor_modal_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []editorModalFixture }
	if e := json.Unmarshal(data, &catalog); e != nil || len(catalog.Cases) != 20 {
		t.Fatalf("native editor modal corpus incomplete: %v", e)
	}
	bundle := testBundle(t)
	rules, e := DecodeNativeEditorFrameRules(bundle.Executable)
	if e != nil {
		t.Fatal(e)
	}
	sprites, e := DecodeNativeSpriteBitmapBank(bundle, 0)
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			p, e := NewNativeFramePresentationState(bundle.Executable, 0x500000, 0x400000)
			if e != nil {
				t.Fatal(e)
			}
			for i := 0x408; i < len(p.Chip); i++ {
				p.Chip[i] = byte(i*17 + 3 + ((i-0x408)/32000)*91)
			}
			p.InterruptChain = false
			if _, e := p.Initialize(bundle.Executable, NativeMouseSample{}); e != nil {
				t.Fatal(e)
			}
			raw := make([]byte, 0x11280)
			m := p.Memory(commandFrameBacking(raw))
			for _, v := range []nativeHeroPatch{{0xeb42, 2, 1}, {0xeb44, 2, 8}, {0xeb6a, 4, 0x200000 + 0xeb56}, {0xdde, 2, 123}, {0xde0, 2, 46}, {0xde2, 1, 20}, {0xde3, 1, 30}, {0x140, 2, 1}, {0x134, 2, uint32(f.Input.X)}, {0x136, 2, uint32(f.Input.Y)}} {
				renderFramePatch(m, v)
			}
			code := fileFrameRelocatedCode(t)
			cm := commandFrameBacking(code)
			_ = cm.Write16(0x3ea, 0)
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			image := rules.Frames.Images.NewImageState()
			state := NativeEditorFrameState{}
			resolver := func(address uint32) ([]byte, error) {
				at := int(int64(address) - int64(p.ChipBase))
				if at < 0 || at > len(p.Chip)-32000 {
					return nil, fmt.Errorf("native modal bitmap address outsidechip")
				}
				return p.Chip[at : at+32000], nil
			}
			cb := NativeEditorFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Code: cm, Memory: m, CodeBase: 0x100000, Frame: &frame, Presentation: p, Bitmap: resolver, Sound: func(uint16, *NativeFrameRegisterContext) error { return nil }}, Image: &image, Sprite: sprites.Paint}
			check := func(i int, step NativeEditorFrameStep) {
				t.Helper()
				want := f.Frames[i]
				pc := step.PC
				if step.Complete {
					pc = 0
				}
				if step.Waiting && step.ChildRoutine == 0 {
					pc = 0x786
				}
				if pc != want.PC || frame.D != want.D {
					t.Errorf("native completeeditor registers/PC differ at%d: PC%x/%x D%x/%x", i, pc, want.PC, frame.D, want.D)
				}
				if fileFrameHash(fileFrameMemoryBytes(t, m)) != want.BSSHash || fileFrameHash(p.Chip) != want.ChipHash || fileFrameHash(p.PointerData) != want.PointerHash {
					t.Errorf("native completeeditor BSS/chip/pointer differs at%d", i)
				}
				if !bytes.Equal(code[0x37bc:0x381e], want.Fields) || fileFrameHash(code[0xab4e:0xab4e+2048]) != want.ScratchHash {
					t.Errorf("native completeeditor fields/scratch differ at%d", i)
				}
				if p.CopperSelector != want.Selector || p.SpritePatchPointer != want.Patch || p.ActiveCopper != want.Copper {
					t.Errorf("native completeeditor Copper state differs at%d", i)
				}
			}
			step, e := state.Advance(&rules, cb)
			if e != nil {
				t.Fatal(e)
			}
			check(0, step)
			step, e = state.Advance(&rules, cb)
			if e != nil {
				t.Fatal(e)
			}
			check(1, step)
			for i, ev := range f.Input.Events {
				if ev.VBlank {
					if _, e := p.VBlank(NativeMouseSample{}, m, &frame); e != nil {
						t.Fatal(e)
					}
				}
				for _, wire := range ev.Keys {
					if e := p.Input.KeyboardInterrupt(wire); e != nil {
						t.Fatal(e)
					}
				}
				step, e = state.Advance(&rules, cb)
				if e != nil {
					t.Fatal(e)
				}
				check(i+2, step)
			}
			if !state.Complete || f.Frames[len(f.Frames)-1].PC != 0 {
				t.Fatal("native numeric editor did notcomplete")
			}
		})
	}
}
