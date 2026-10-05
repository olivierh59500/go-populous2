package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNativeBeamRenderingAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/render_frame_beam_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []renderActorFixture }
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 126 {
		t.Fatalf("native beam corpus incomplete: %v", err)
	}
	rules, err := DecodeNativeActorRenderRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			if f.Error != "" {
				t.Fatal("original beam capture failed", f.Error)
			}
			raw := renderActorInitial(f)
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			image := rules.Frames.Images.NewImageState()
			wantImage := image
			for _, p := range f.BankChanges {
				wantImage.AudioBank[p.Address] = byte(p.Value)
			}
			bitmap := make([]byte, 32000)
			for i := range bitmap {
				bitmap[i] = byte(i*7 + 13)
			}
			cb := NativeActorEffectsCallbacks{NativeRenderFrameCallbacks: NativeRenderFrameCallbacks{Memory: commandNativeMemory(raw), Frame: &frame, Image: &image, Bitmap: bitmap}, GridCursorAddress: f.Input.GridCursor}
			_, err := rules.Actor(0x76f4, cb, &NativeActorRenderState{}, NativeActorRenderChildren{})
			if err != nil {
				t.Fatal(err)
			}
			if frame.D != f.D {
				t.Errorf("native beam all8D differ: got%x want%x", frame.D, f.D)
			}
			if image.AudioBank != wantImage.AudioBank || image.LastY != f.LastY {
				t.Error("native beam retainedCODE/audio differs")
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != f.BSSHash {
				t.Errorf("native beam fullBSS differs: got%s want%s", got, f.BSSHash)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(bitmap)); got != f.BitmapHash {
				t.Errorf("native beam actualpixels differ: got%s want%s", got, f.BitmapHash)
			}
		})
	}
}
