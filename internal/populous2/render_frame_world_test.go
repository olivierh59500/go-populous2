package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type renderWorldFixture struct {
	Input struct {
		Name         string
		Land         int
		Initial      []nativeHeroPatch
		D            [8]uint32
		Header, Tile uint8
		Clock        uint32
	}
	D                                      [8]uint32
	Changes, BankChanges                   []nativeHeroPatch
	Actors                                 []uint16
	LastY, TownHitHeight                   uint16
	BSSHash, BitmapHash, WindowHash, Error string
}

func renderWorldInitial(f renderWorldFixture) []byte {
	b := make([]byte, 0x11280)
	m := commandNativeMemory(b)
	_ = m.Write32(0x1e, 0xa10000)
	_ = m.Write32(0x22, 0xa20000)
	_ = m.Write16(0x5f44, 20)
	_ = m.Write16(0x5f46, 20)
	_ = m.Write32(0xf40, f.Input.Clock)
	for i := 0; i < 4096; i++ {
		b[0xf44+i*4], b[0xf45+i*4] = f.Input.Header, f.Input.Tile
	}
	for _, p := range f.Input.Initial {
		renderFramePatch(m, p)
	}
	return b
}
func TestNativeWorldDrawTraversalAndPixelsAgainstOriginalDMA(t *testing.T) {
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
			plan, e := rules.WorldDraw(cb, &state)
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
