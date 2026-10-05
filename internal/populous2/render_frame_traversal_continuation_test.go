package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestNativeRetainedWorldDrawAgainstOriginalDMA(t *testing.T) {
	data, e := os.ReadFile("testdata/render_frame_world_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []renderWorldFixture }
	if e := json.Unmarshal(data, &catalog); e != nil || len(catalog.Cases) != 340 {
		t.Fatalf("native world draw corpus incomplete: %v", e)
	}
	bundle := testBundle(t)
	rules, e := DecodeNativeActorRenderRules(bundle.Executable)
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
			raw := renderWorldInitial(f)
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
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
			cb := NativeWorldRenderCallbacks{Effects: NativeActorEffectsCallbacks{NativeRenderFrameCallbacks: NativeRenderFrameCallbacks{Memory: commandNativeMemory(raw), Frame: &frame, Image: &image, Bitmap: bitmap, Sprite: sprites.Paint}, Cropped: sprites.PaintCropped, Reinterpreted: sprites.PaintReinterpreted}, Tiles: tiles, Tile: func(r NativeTileChunkRequest, _ []byte) error { return tiles.PaintChunkWindow(r, window) }, Background: background}
			state := NativeWorldRenderState{}
			continuation := NativeWorldRenderContinuation{}
			plan, done, e := rules.AdvanceWorldDraw(cb, &state, &continuation)
			if e == nil && !done {
				t.Fatal("ordinary source traversal unexpectedly waited")
			}
			if f.Input.Tile == 220 && f.Input.Header == 0xa8 && f.Input.Clock == 3 {
				adjacentCases++
			}
			if e != nil {
				t.Fatal(e)
			}
			if frame.D != f.D {
				t.Error("native whole world register wrapper differs")
			}
			refs := make([]uint16, len(plan.Actors))
			for i, r := range plan.Actors {
				refs[i] = uint16(r)
			}
			if !reflect.DeepEqual(refs, f.Actors) {
				t.Errorf("native raw tail-to-head draw order differs: got%x want%x", refs, f.Actors)
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
	if adjacentCases != 4 {
		t.Fatal("original adjacent chip RAM boundary coverage incomplete")
	}
}

func TestNativeRetainedAlternateWorldAgainstOriginalDMA(t *testing.T) {
	data, e := os.ReadFile("testdata/render_frame_alternate_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []renderAlternateFixture }
	if e := json.Unmarshal(data, &catalog); e != nil || len(catalog.Cases) != 391 {
		t.Fatalf("native alternate corpus incomplete: %v", e)
	}
	bundle := testBundle(t)
	rules, e := DecodeNativeActorRenderRules(bundle.Executable)
	if e != nil {
		t.Fatal(e)
	}
	sprites, e := DecodeNativeSpriteBitmapBank(bundle, 0)
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			if f.Error != "" {
				t.Fatal("original alternate failed", f.Error)
			}
			prefix := renderWorldFixture{}
			prefix.Input.Name, prefix.Input.Land, prefix.Input.Initial, prefix.Input.D, prefix.Input.Header, prefix.Input.Tile, prefix.Input.Clock = f.Input.Name, f.Input.Land, f.Input.Initial, f.Input.D, f.Input.Header, f.Input.Tile, f.Input.Clock
			raw := renderWorldInitial(prefix)
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			if f.Input.Mode != "clear" {
				frame.Word(0, f.Input.View)
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
			bitmap := window.Bytes[256:32256]
			background := make([]byte, 32000)
			for i := range bitmap {
				bitmap[i] = byte(i*7 + 13)
				background[i] = byte(i*53 + 17)
			}
			tiles, e := DecodeNativeTileBitmapBank(bundle.Raw[fmt.Sprintf("block%d.pak", f.Input.Land)])
			if e != nil {
				t.Fatal(e)
			}
			cb := NativeWorldRenderCallbacks{Effects: NativeActorEffectsCallbacks{NativeRenderFrameCallbacks: NativeRenderFrameCallbacks{Memory: commandNativeMemory(raw), Frame: &frame, Image: &image, Bitmap: bitmap, Sprite: sprites.Paint}, Cropped: sprites.PaintCropped, Reinterpreted: sprites.PaintReinterpreted}, Tiles: tiles, Tile: func(r NativeTileChunkRequest, _ []byte) error { return tiles.PaintChunkWindow(r, window) }, Background: background}
			state := NativeAlternateRenderState{}
			if f.Input.Mode == "clear" {
				e = rules.Frames.AlternateClear(cb.Effects.NativeRenderFrameCallbacks)
			} else {
				continuation := NativeWorldRenderContinuation{}
				var done bool
				_, done, e = rules.AdvanceAlternateDraw(cb, &state, &continuation)
				if e == nil && !done {
					t.Fatal("ordinary alternate traversal unexpectedly waited")
				}
			}
			if e != nil {
				t.Fatal(e)
			}
			if frame.D != f.D {
				t.Errorf("native alternate all8D differ: got%x want%x", frame.D, f.D)
			}
			if f.Input.Mode != "clear" && (state.Scratch != f.Scratch || [2]uint16{state.World.ProjectionX, state.World.ProjectionY} != f.Projection) {
				t.Errorf("native alternate mutableCODE differs: scratch%x/%x projection%x/%x", state.Scratch, f.Scratch, [2]uint16{state.World.ProjectionX, state.World.ProjectionY}, f.Projection)
			}
			if image.LastY != f.LastY || image.AudioBank != wantImage.AudioBank {
				t.Error("native alternate image bank differs")
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != f.BSSHash {
				t.Errorf("native alternate fullBSS differs: got%s want%s", got, f.BSSHash)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(bitmap)); got != f.BitmapHash {
				t.Errorf("native alternate framebuffer differs: got%s want%s", got, f.BitmapHash)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(window.Bytes)); got != f.WindowHash {
				t.Errorf("native alternate adjacentRAM differs: got%s want%s", got, f.WindowHash)
			}
		})
	}
}
