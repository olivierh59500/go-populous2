package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type renderEffectsSprite struct {
	Sprite               int
	X, Y                 int16
	Height, SourceHeight uint16
	Routine              uint32
}
type renderEffectsFixture struct {
	renderActorFixture
	Sprites []renderEffectsSprite
}

func TestNativeActorEffectsRenderingAgainstOriginalDMA(t *testing.T) {
	data, err := os.ReadFile("testdata/render_frame_effects_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []renderEffectsFixture }
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 835 {
		t.Fatalf("native effect render corpus incomplete: %v", err)
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
	crops, requests := 0, 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			if f.Error != "" {
				t.Fatal("native effects capture failed", f.Error)
			}
			raw := renderActorInitial(f.renderActorFixture)
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			image := rules.Frames.Images.NewImageState()
			expectedImage := image
			for _, p := range f.BankChanges {
				if p.Address < 0 || p.Address >= len(expectedImage.AudioBank) || p.Width != 1 {
					t.Fatal("native sound delta malformed")
				}
				expectedImage.AudioBank[p.Address] = byte(p.Value)
			}
			bitmap := make([]byte, 32000)
			if f.Input.Pattern {
				for i := range bitmap {
					bitmap[i] = byte(i*7 + 13)
				}
			}
			calls := 0
			check := func(s NativePresentationSprite, sourceHeight int16) {
				t.Helper()
				if calls >= len(f.Sprites) {
					t.Fatal("unexpected effect sprite request")
				}
				want := f.Sprites[calls]
				calls++
				requests++
				if s.Sprite != want.Sprite || s.X != want.X || s.Y != want.Y || s.Height != int16(want.Height) || sourceHeight != int16(want.SourceHeight) || s.Routine != want.Routine {
					t.Errorf("native effect sprite request differs: got%+v stride%d want%+v", s, sourceHeight, want)
				}
			}
			cb := NativeActorEffectsCallbacks{NativeRenderFrameCallbacks: NativeRenderFrameCallbacks{Memory: commandNativeMemory(raw), Frame: &frame, Image: &image, Bitmap: bitmap, Sprite: func(s NativePresentationSprite, b []byte) error { check(s, s.Height); return bank.Paint(s, b) }}, Cropped: func(s NativeCroppedSpriteRequest, b []byte) error {
				crops++
				check(s.Sprite, s.SourceHeight)
				return bank.PaintCropped(s, b)
			}}
			state := NativeActorRenderState{}
			_, err := rules.Actor(0x76f4, cb, &state, NativeActorRenderChildren{})
			if err != nil {
				t.Fatal(err)
			}
			if calls != len(f.Sprites) {
				t.Errorf("native effect request count differs: got%d want%d", calls, len(f.Sprites))
			}
			if frame.D != f.D {
				t.Errorf("native effect all8D differ: got%x want%x", frame.D, f.D)
			}
			if image.LastY != f.LastY || image.AudioBank != expectedImage.AudioBank || state.TownHitHeight != f.TownHitHeight {
				t.Error("native effect sharedCODE differs")
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != f.BSSHash {
				t.Errorf("native effect full BSS differs: got%s want%s", got, f.BSSHash)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(bitmap)); got != f.BitmapHash {
				t.Errorf("native effect actualbitmap differs: got%s want%s", got, f.BitmapHash)
			}
		})
	}
	if crops == 0 || requests == 0 {
		t.Fatal("native full/cropped rendering branches untested")
	}
}
