package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNativeColumnAndWaterLayerPixelsAgainstOriginalDMA(t *testing.T) {
	data, err := os.ReadFile("testdata/render_frame_column_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []renderEffectsFixture }
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 189 {
		t.Fatalf("native column corpus incomplete: %v", err)
	}
	bundle := testBundle(t)
	rules, err := DecodeNativeActorRenderRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	bank, err := DecodeNativeSpriteBitmapBank(bundle, 0)
	if err != nil {
		t.Fatal(err)
	}
	shortened := 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			if f.Error != "" {
				t.Fatal("native column capture failed", f.Error)
			}
			raw := renderActorInitial(f.renderActorFixture)
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
			calls := 0
			check := func(s NativePresentationSprite) {
				t.Helper()
				if calls >= len(f.Sprites) {
					t.Fatal("unexpected column sprite")
				}
				want := f.Sprites[calls]
				calls++
				if s.Sprite != want.Sprite || s.X != want.X || s.Y != want.Y || s.Height != int16(want.Height) || s.Routine != want.Routine {
					t.Errorf("native column request differs: got%+v want%+v", s, want)
				}
			}
			cb := NativeActorEffectsCallbacks{NativeRenderFrameCallbacks: NativeRenderFrameCallbacks{Memory: commandNativeMemory(raw), Frame: &frame, Image: &image, Bitmap: bitmap, Sprite: func(s NativePresentationSprite, b []byte) error { check(s); return bank.Paint(s, b) }}, Reinterpreted: func(r NativeReinterpretedSpriteRequest, b []byte) error {
				if r.Sprite.Height < int16(bank.Sprites[r.Sprite.Sprite].Height) {
					shortened++
				}
				check(r.Sprite)
				return bank.PaintReinterpreted(r, b)
			}}
			_, err := rules.Actor(0x76f4, cb, &NativeActorRenderState{}, NativeActorRenderChildren{})
			if err != nil {
				t.Fatal(err)
			}
			if calls != len(f.Sprites) {
				t.Errorf("native column request count differs: got%d want%d", calls, len(f.Sprites))
			}
			if frame.D != f.D {
				t.Errorf("native column all8D differ: got%x want%x", frame.D, f.D)
			}
			if image.LastY != f.LastY || image.AudioBank != wantImage.AudioBank {
				t.Error("native column sharedCODE/audio differs")
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != f.BSSHash {
				t.Errorf("native column fullBSS differs: got%s want%s", got, f.BSSHash)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(bitmap)); got != f.BitmapHash {
				t.Errorf("native column actualbitmap differs: got%s want%s", got, f.BitmapHash)
			}
		})
	}
	if shortened == 0 {
		t.Fatal("native shortened source plane alias untested")
	}
}
