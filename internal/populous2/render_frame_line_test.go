package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNativeRenderLineAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/render_frame_line_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var c struct {
		Cases []struct {
			Input struct {
				Name string
				D    [8]uint32
			}
			D           [8]uint32
			Hash, Error string
		}
	}
	if err := json.Unmarshal(data, &c); err != nil || len(c.Cases) != 363 {
		t.Fatalf("native line corpus incomplete: %v", err)
	}
	rules, err := DecodeNativeRenderFrameRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range c.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			if f.Error != "" {
				t.Fatal("native line failed", f.Error)
			}
			bitmap := make([]byte, 32000)
			for i := range bitmap {
				bitmap[i] = byte(i*7 + 13)
			}
			frame := NativeFrameRegisterContext{D: f.Input.D}
			_, err := rules.Line(NativeRenderFrameCallbacks{Frame: &frame, Bitmap: bitmap})
			if err != nil {
				t.Fatal(err)
			}
			if frame.D != f.D {
				t.Errorf("native line all8D differ: got%x want%x", frame.D, f.D)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(bitmap)); got != f.Hash {
				t.Errorf("native line bitmap differs: got%s want%s", got, f.Hash)
			}
		})
	}
}
