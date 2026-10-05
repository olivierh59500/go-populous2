package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestNativeSpriteBitmapsAgainstOriginalCPUAndDMA(t *testing.T) {
	data, err := os.ReadFile("testdata/native_sprite_bitmap_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []renderFrameFixture }
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 1041 {
		t.Fatalf("native render corpus incomplete: %v", err)
	}
	bundle := testBundle(t)
	rules, err := DecodeNativeRenderFrameRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	bank, err := DecodeNativeSpriteBitmapBank(bundle, 0)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	hardware := 0
	for _, f := range catalog.Cases {
		counts[f.Input.Mode]++
		t.Run(f.Input.Name, func(t *testing.T) {
			if f.Error != "" {
				t.Fatal("original renderer did not finish", f.Error)
			}
			raw := renderFrameInitial(f)
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			state := rules.Images.NewImageState()
			expectedState := state
			for _, p := range f.BankChanges {
				if p.Address < 0 || p.Address >= len(expectedState.AudioBank) || p.Width != 1 {
					t.Fatal("native image bank delta malformed")
				}
				expectedState.AudioBank[p.Address] = byte(p.Value)
			}
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
			cb := NativeRenderFrameCallbacks{Memory: commandNativeMemory(raw), Frame: &frame, Image: &state, Input: &input, Bitmap: bitmap, Sprite: bank.Paint}
			var plan NativeRenderFramePlan
			switch f.Input.Mode {
			case "hud":
				plan, err = rules.HUD(cb, true)
			case "map":
				plan, err = rules.MapCursor(cb)
			case "camera":
				plan, err = rules.CameraMarker(cb)
			case "cursor":
				plan, err = rules.Cursor(cb)
			case "height":
				err = rules.TerrainHeight(cb.Memory, &frame)
			case "admission":
				err = rules.TerrainAdmission(cb.Memory, &frame)
			case "descriptor":
				err = rules.descriptor(0x21626+f.Input.Sprite*12, cb, &plan)
			default:
				t.Fatal("unknown native renderer mode")
			}
			if err != nil {
				t.Fatal(err)
			}
			if frame.D != f.D {
				t.Errorf("native rendering all8D differ: got%x want%x", frame.D, f.D)
			}
			if input.Mouse.Image != f.Pointer || state.LastY != f.LastY || state.AudioBank != expectedState.AudioBank {
				t.Errorf("native rendering mutable CODE differs: pointer%x/%x Y%x/%x", input.Mouse.Image, f.Pointer, state.LastY, f.LastY)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != f.BSSHash {
				expected := renderFrameInitial(f)
				for _, p := range f.Changes {
					renderFramePatch(commandNativeMemory(expected), p)
				}
				shown := 0
				for i := range raw {
					if raw[i] != expected[i] && shown < 10 {
						t.Logf("BSS%#x differs: got%x want%x", i, raw[i], expected[i])
						shown++
					}
				}
				t.Errorf("native rendering full BSS differs: got%s want%s", got, f.BSSHash)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(bitmap)); got != f.BitmapHash {
				t.Errorf("native rendering DMA bitmap differs: got%s want%s", got, f.BitmapHash)
			}
			if len(plan.Sprites) != len(f.Sprites) {
				t.Fatalf("native sprite request count differs: got%d want%d", len(plan.Sprites), len(f.Sprites))
			}
			for i, s := range plan.Sprites {
				want := f.Sprites[i]
				hardware++
				if s.Sprite != want.Sprite || s.X != want.X || s.Y != want.Y || s.HalfWidth != int16(want.HalfWidth) || s.Height != int16(want.Height) || s.Routine != want.Routine {
					t.Errorf("native prepared sprite request differs: got%+v want%+v", s, want)
				}
				planes, err := renderFramePreparedSprite(bundle, s.Sprite)
				if err != nil {
					t.Fatal(err)
				}
				if got := fmt.Sprintf("%x", sha256.Sum256(planes)); got != want.PreparedHash {
					t.Errorf("actual1069C prepared sprite planes differ: got%s want%s", got, want.PreparedHash)
				}
			}
			if plan.HardwarePending {
				t.Fatal("native sprite pixel sink did not complete the blit")
			}
		})
	}
	if !reflect.DeepEqual(counts, map[string]int{"map": 181, "camera": 25, "height": 144, "admission": 48, "cursor": 221, "hud": 42, "descriptor": 380}) || hardware == 0 {
		t.Fatalf("native render body coverage incomplete: %v requests%d", counts, hardware)
	}
}
