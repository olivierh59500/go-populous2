package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

type renderFrameInputFixture struct {
	renderFrameFixture
	Child uint32
	Copy  [6]uint32 // Original hardware BLTDPT, BLTAPT, control,size and modulos.
}

func TestNativeRenderInputFullFrameAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/render_frame_input_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []renderFrameInputFixture }
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 432 {
		t.Fatalf("native render input corpus incomplete: %v", err)
	}
	bundle := testBundle(t)
	rules, err := DecodeNativeRenderFrameRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	children := map[uint32]int{}
	for _, f := range catalog.Cases {
		counts[f.Input.Mode]++
		children[f.Child]++
		t.Run(f.Input.Name, func(t *testing.T) {
			if f.Error != "" {
				t.Fatal("native input render capture failed", f.Error)
			}
			raw := renderFrameInitial(f.renderFrameFixture)
			memory := commandNativeMemory(raw)
			_ = memory.Write32(0x22, 0xa20000)
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			image := rules.Images.NewImageState()
			input, err := NewNativeInputState(bundle.Executable)
			if err != nil {
				t.Fatal(err)
			}
			input.Mouse.Image = f.Input.Pointer
			bitmap := make([]byte, 32000)
			if f.Input.Pattern {
				for i := range bitmap {
					bitmap[i] = uint8(i*7 + 13)
				}
			}
			cb := NativeRenderFrameCallbacks{Memory: memory, Frame: &frame, Image: &image, Input: &input, Bitmap: bitmap}
			switch f.Input.Mode {
			case "copy":
				var p NativePresentationCopy
				p, err = rules.BackgroundCopy(memory, &frame)
				if err == nil && [6]uint32{p.Destination, p.Source, uint32(p.Control), uint32(p.Size), 0, 0} != f.Copy {
					t.Errorf("native background hardware direction/control differs: %+v native%x", p, f.Copy)
				}
			case "highlight":
				err = rules.Highlights(cb)
			case "countdown":
				err = rules.Countdown(cb)
			case "clamp":
				err = ClampNativeFrameCamera(memory, &frame)
			case "selected-hit":
				err = rules.SelectedHit(cb)
			case "selected":
				_, err = rules.Selected(cb, NativeRenderFrameChildren{SelectedHit: func(*NativeFrameRegisterContext) error { return rules.SelectedHit(cb) }})
			default:
				t.Fatal("unknown native render input stage")
			}
			if f.Child != 0 {
				if err == nil || (f.Child == 0xe45c && !strings.Contains(err.Error(), "e45c")) || (f.Child == 0x346a && !strings.Contains(err.Error(), "346a")) {
					t.Fatalf("native required child boundary changed: %x/%v", f.Child, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if frame.D != f.D {
				t.Errorf("native input render all8D differ: got%x want%x", frame.D, f.D)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != f.BSSHash {
				expected := renderFrameInitial(f.renderFrameFixture)
				_ = commandNativeMemory(expected).Write32(0x22, 0xa20000)
				for _, p := range f.Changes {
					renderFramePatch(commandNativeMemory(expected), p)
				}
				shown := 0
				for i := range raw {
					if raw[i] != expected[i] && shown < 12 {
						t.Logf("BSS%#x differs: got%x want%x", i, raw[i], expected[i])
						shown++
					}
				}
				t.Errorf("native input render full BSS differs: got%s want%s", got, f.BSSHash)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(bitmap)); got != f.BitmapHash {
				t.Errorf("native input render software pixels differ: got%s want%s", got, f.BitmapHash)
			}
		})
	}
	if counts["copy"] != 1 || counts["highlight"] != 120 || counts["countdown"] != 20 || counts["clamp"] != 100 || counts["selected-hit"] != 128 || counts["selected"] != 63 || children[0xe45c] == 0 || children[0x346a] != 1 {
		t.Fatalf("native render input stage coverage incomplete: %v children%v", counts, children)
	}
}
