package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go-populous2/internal/amiga"
	"os"
	"reflect"
	"strings"
	"testing"
)

func nativeLiveRenderRulesTest(t *testing.T, exe *amiga.Executable) (NativeRenderFrameRules, error) {
	t.Helper()
	r, err := DecodeNativeRenderFrameRules(exe)
	if err != nil {
		return r, err
	}
	code, _ := nativeSharedCodeTestView(t)
	err = r.BindCode(code.Physical(), code.Logical().Read32)
	return r, err
}
func nativeLiveActorRulesTest(t *testing.T, exe *amiga.Executable) (NativeActorRenderRules, error) {
	r, err := nativeLiveRenderRulesTest(t, exe)
	return NativeActorRenderRules{Frames: r}, err
}
func TestNativeLiveSpriteBitmapsAgainstOriginalCPUAndDMA(t *testing.T) {
	data, err := os.ReadFile("testdata/native_sprite_bitmap_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []renderFrameFixture }
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 1041 {
		t.Fatalf("native render corpus incomplete: %v", err)
	}
	bundle := testBundle(t)
	rules, err := nativeLiveRenderRulesTest(t, bundle.Executable)
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

func TestNativeLiveActorRenderingFullFrameAndPixelsAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/render_frame_actor_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []renderActorFixture }
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 774 {
		t.Fatalf("native actor renderer corpus incomplete: %v", err)
	}
	bundle := testBundle(t)
	rules, err := nativeLiveActorRulesTest(t, bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	bank, err := DecodeNativeSpriteBitmapBank(bundle, 0)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	sprites, permissions := 0, 0
	for _, f := range catalog.Cases {
		counts[f.Input.Mode]++
		t.Run(f.Input.Name, func(t *testing.T) {
			if f.Error != "" {
				t.Fatal("original renderer did not finish", f.Error)
			}
			raw := renderActorInitial(f)
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			image := rules.Frames.Images.NewImageState()
			expectedImage := image
			for _, p := range f.BankChanges {
				if p.Address < 0 || p.Address >= len(expectedImage.AudioBank) || p.Width != 1 {
					t.Fatal("native image bank mutation malformed")
				}
				expectedImage.AudioBank[p.Address] = byte(p.Value)
			}
			bitmap := make([]byte, 32000)
			if f.Input.Pattern {
				for i := range bitmap {
					bitmap[i] = byte(i*7 + 13)
				}
			}
			cb := NativeRenderFrameCallbacks{Memory: commandNativeMemory(raw), Frame: &frame, Image: &image, Bitmap: bitmap, Sprite: bank.Paint}
			state := NativeActorRenderState{}
			var plan NativeRenderFramePlan
			if f.Input.Mode == "angle" {
				err = rules.Angle(&frame)
			} else if f.Input.Mode == "selected" {
				plan, err = rules.Frames.Selected(cb, NativeRenderFrameChildren{DrawActor: func(at int, c *NativeFrameRegisterContext) error {
					_, err := rules.Follower(at, cb, &state, NativeActorRenderChildren{})
					return err
				}})
			} else {
				plan, err = rules.Follower(0x76f4, cb, &state, NativeActorRenderChildren{})
			}
			if f.Child != 0 {
				if err == nil || f.Child != 0x314a || !strings.Contains(err.Error(), "314a") {
					t.Fatalf("native actor child boundary differs: %x/%v", f.Child, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if frame.D != f.D {
				t.Errorf("native actor all8D differ: got%x want%x", frame.D, f.D)
			}
			if image.LastY != f.LastY || image.AudioBank != expectedImage.AudioBank || state.TownHitHeight != f.TownHitHeight {
				t.Errorf("native actor mutable CODE differs: Y%x/%x town%x/%x", image.LastY, f.LastY, state.TownHitHeight, f.TownHitHeight)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != f.BSSHash {
				expected := renderActorInitial(f)
				for _, p := range f.Changes {
					renderFramePatch(commandNativeMemory(expected), p)
				}
				shown := 0
				for i := range raw {
					if raw[i] != expected[i] && shown < 10 {
						t.Logf("BSS%#x differs: got%x native%x", i, raw[i], expected[i])
						shown++
					}
				}
				t.Errorf("native actor full BSS differs: got%s want%s", got, f.BSSHash)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(bitmap)); got != f.BitmapHash {
				t.Errorf("native actor actual sprite/software bitmap differs: got%s want%s", got, f.BitmapHash)
			}
			if f.Input.Mode != "selected" {
				if len(plan.Sprites) != len(f.Sprites) {
					t.Fatalf("native actor sprite request count differs: got%d want%d", len(plan.Sprites), len(f.Sprites))
				}
				for i, s := range plan.Sprites {
					want := f.Sprites[i]
					if s.Sprite != want.Sprite || s.X != want.X || s.Y != want.Y || s.Height != int16(want.Height) || s.Routine != want.Routine {
						t.Errorf("native actor sprite request differs: got%+v want%+v", s, want)
					}
				}
			}
			sprites += len(f.Sprites)
			if raw[0xe76a+314+0x4b]&3 != 0 || raw[0xe76a+628+0x4b]&3 != 0 {
				permissions++
			}
		})
	}
	if counts["actor"] != 633 || counts["angle"] != 81 || counts["selected"] != 60 || sprites == 0 || permissions == 0 {
		t.Fatalf("native actor render coverage incomplete: modes%v sprites%d permissions%d", counts, sprites, permissions)
	}
}

func TestNativeLiveActorEffectsRenderingAgainstOriginalDMA(t *testing.T) {
	data, err := os.ReadFile("testdata/render_frame_effects_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []renderEffectsFixture }
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 835 {
		t.Fatalf("native effect render corpus incomplete: %v", err)
	}
	bundle := testBundle(t)
	rules, err := nativeLiveActorRulesTest(t, bundle.Executable)
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

func TestNativeLiveColumnAndWaterLayerPixelsAgainstOriginalDMA(t *testing.T) {
	data, err := os.ReadFile("testdata/render_frame_column_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []renderEffectsFixture }
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 189 {
		t.Fatalf("native column corpus incomplete: %v", err)
	}
	bundle := testBundle(t)
	rules, err := nativeLiveActorRulesTest(t, bundle.Executable)
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

func TestNativeLiveBeamRenderingAgainstOriginalCPU(t *testing.T) {
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

func TestNativeLiveMainRenderAgainstOriginalCPUAndDMA(t *testing.T) {
	data, e := os.ReadFile("testdata/render_frame_main_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct {
		Cases []struct {
			renderWorldFixture
			Input struct {
				Name         string
				Land         int
				Initial      []nativeHeroPatch
				D            [8]uint32
				Header, Tile uint8
				Clock        uint32
				View         uint16
			}
		}
	}
	if e := json.Unmarshal(data, &catalog); e != nil || len(catalog.Cases) != 727 {
		t.Fatalf("native main render corpus incomplete: %v", e)
	}
	bundle := testBundle(t)
	rules, e := nativeLiveActorRulesTest(t, bundle.Executable)
	if e != nil {
		t.Fatal(e)
	}
	sprites, e := DecodeNativeSpriteBitmapBank(bundle, 0)
	if e != nil {
		t.Fatal(e)
	}
	banks := [4]*NativeTileBitmapBank{}
	for i := range banks {
		banks[i], e = DecodeNativeTileBitmapBank(bundle.Raw[fmt.Sprintf("block%d.pak", i)])
		if e != nil {
			t.Fatal(e)
		}
	}
	adjacentCases := 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			if f.Error != "" {
				t.Fatal("native world capture failed", f.Error)
			}
			prefix := f.renderWorldFixture
			prefix.Input.Name, prefix.Input.Land, prefix.Input.Initial, prefix.Input.D, prefix.Input.Header, prefix.Input.Tile, prefix.Input.Clock = f.Input.Name, f.Input.Land, f.Input.Initial, f.Input.D, f.Input.Header, f.Input.Tile, f.Input.Clock
			raw := renderWorldInitial(prefix)
			m := commandNativeMemory(raw)
			_ = m.Write16(0xf0c, f.Input.View)
			_ = m.Write16(0xeb44, 2)
			_ = m.Write16(0xeb42, 1)
			_ = m.Write32(0xeb6a, 0x200000+0xeb56)
			_ = m.Write32(0xe8a4, 10000)
			_ = m.Write32(0xe9de, 15000)
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			input, e := NewNativeInputState(bundle.Executable)
			if e != nil {
				t.Fatal(e)
			}
			image := rules.Frames.Images.NewImageState()
			wantImage := image
			for _, p := range f.BankChanges {
				wantImage.AudioBank[p.Address] = byte(p.Value)
			}
			window := NativeBitmapWindow{Bytes: make([]byte, 33024), BitmapOffset: 256}
			for i := range window.Bytes {
				window.Bytes[i] = byte(i*29 + 43)
			}
			bitmap, background := window.Bytes[256:32256], make([]byte, 32000)
			for i := range bitmap {
				bitmap[i] = byte(i*7 + 13)
				background[i] = byte(i*53 + 17)
			}
			tiles := banks[f.Input.Land]
			cb := NativeWorldRenderCallbacks{Effects: NativeActorEffectsCallbacks{NativeRenderFrameCallbacks: NativeRenderFrameCallbacks{Memory: commandNativeMemory(raw), Frame: &frame, Image: &image, Bitmap: bitmap, Sprite: sprites.Paint, Input: &input}, Cropped: sprites.PaintCropped, Reinterpreted: sprites.PaintReinterpreted}, Tiles: tiles, Tile: func(r NativeTileChunkRequest, _ []byte) error { return tiles.PaintChunkWindow(r, window) }, Background: background}
			state := NativeMainRenderState{}
			e = rules.MainFrame(NativeMainRenderCallbacks{World: cb}, &state)
			if f.Input.Tile == 220 && f.Input.Header == 0xa8 && f.Input.Clock == 3 {
				adjacentCases++
			}
			if e != nil {
				t.Fatal(e)
			}
			if frame.D != f.D {
				t.Error("native whole world register wrapper differs")
			}
			if image.LastY != f.LastY || image.AudioBank != wantImage.AudioBank || state.Actor.TownHitHeight != f.TownHitHeight {
				t.Errorf("native world sharedCODE differs: Y%x/%x town%x/%x", image.LastY, f.LastY, state.Actor.TownHitHeight, f.TownHitHeight)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != f.BSSHash {
				t.Errorf("native world fullBSS differs: got%s want%s", got, f.BSSHash)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(bitmap)); got != f.BitmapHash {
				t.Errorf("native world actualframebuffer differs: got%s want%s", got, f.BitmapHash)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(window.Bytes)); got != f.WindowHash {
				t.Errorf("native actual adjacent bitmap RAM differs: got%s want%s", got, f.WindowHash)
			}
		})
	}
	if adjacentCases != 124 {
		t.Fatal("original adjacent chip RAM boundary coverage incomplete")
	}
}
