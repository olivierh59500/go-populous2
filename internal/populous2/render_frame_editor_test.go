package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type editorFrameFixture struct {
	Input struct {
		Name          string
		Initial, Code []nativeHeroPatch
		D             [8]uint32
	}
	D                          [8]uint32
	Changes, CodeChanges       []nativeHeroPatch
	BSSHash, BitmapHash, Error string
	Child                      uint32
}

func TestNativeEditorRendererAllRegistersAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/render_frame_editor_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []editorFrameFixture }
	if e := json.Unmarshal(data, &catalog); e != nil || len(catalog.Cases) != 256 {
		t.Fatalf("native editor corpus incomplete: %v", e)
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
	modal := 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			if f.Error != "" {
				t.Fatal("original editor capture failed", f.Error)
			}
			raw := make([]byte, 0x11280)
			m := commandNativeMemory(raw)
			_ = m.Write32(0x1e, 0xa10000)
			_ = m.Write32(0xeb6a, 0x200000+0xeb56)
			_ = m.Write16(0xeb42, 1)
			_ = m.Write16(0xeb44, 8)
			for _, p := range f.Input.Initial {
				renderFramePatch(m, p)
			}
			code := fileFrameRelocatedCode(t)
			cm := commandNativeMemory(code)
			for _, p := range f.Input.Code {
				renderFramePatch(cm, p)
			}
			expectedCode := fileFrameRelocatedCode(t)
			for _, p := range f.CodeChanges {
				renderFramePatch(commandNativeMemory(expectedCode), p)
			}
			bitmap := make([]byte, 32000)
			for i := range bitmap {
				bitmap[i] = byte(i*7 + 13)
			}
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			image := rules.Frames.Images.NewImageState()
			presentation, e := NewNativeFramePresentationState(bundle.Executable, 0x500000, 0x400000)
			if e != nil {
				t.Fatal(e)
			}
			cb := NativeEditorFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Code: cm, Memory: m, CodeBase: 0x100000, Frame: &frame, Presentation: presentation, Bitmap: func(uint32) ([]byte, error) { return bitmap, nil }, Sound: func(uint16, *NativeFrameRegisterContext) error { return nil }}, Image: &image, Sprite: sprites.Paint}
			state := NativeEditorFrameState{}
			step, e := state.Advance(&rules, cb)
			if e != nil {
				t.Fatal(e)
			}
			if f.Child != 0 {
				modal++
				if !step.Waiting || step.ChildRoutine != int(f.Child) || state.PC != int(f.Child) {
					t.Errorf("native editor realmodal entry differs: %+v", step)
				}
			} else if !step.Complete {
				t.Errorf("native editor normalreturn incomplete: %+v", step)
			}
			if frame.D != f.D {
				t.Errorf("native editor all8D differ: got%x want%x", frame.D, f.D)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != f.BSSHash {
				t.Errorf("native editor fullBSS differs: got%s want%s", got, f.BSSHash)
			}
			for _, span := range [][2]int{{0x37bc, 0x381e}, {0xab4e, 0xab4e + 2048}} {
				for at := span[0]; at < span[1]; at++ {
					if code[at] != expectedCode[at] {
						t.Fatalf("native editor CODE%#x differs: got%x want%x", at, code[at], expectedCode[at])
					}
				}
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(bitmap)); got != f.BitmapHash {
				t.Errorf("native editor actualbitmap differs: got%s want%s", got, f.BitmapHash)
			}
		})
	}
	if modal == 0 {
		t.Fatal("native editor modal callerprefix untested")
	}
}
