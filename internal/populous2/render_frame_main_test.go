package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
)

func TestNativeMainRenderAgainstOriginalCPUAndDMA(t *testing.T) {
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

func TestNativeMainRenderBeginPreservesSharedState(t *testing.T) {
	state := NativeMainRenderState{Step: 11, View: 8}
	state.Actor.TownHitHeight = 37
	state.World.ProjectionX, state.Alternate.World.ProjectionY = 19, 23
	if err := state.Begin(); err != nil {
		t.Fatal(err)
	}
	if state.Step != 0 || state.View != 0 || state.Actor.TownHitHeight != 37 || state.World.ProjectionX != 19 || state.Alternate.World.ProjectionY != 23 {
		t.Fatal("frame restart cleared shared native rendering state")
	}
	state.Step = 4
	if err := state.Begin(); err == nil || state.Step != 4 {
		t.Fatal("incomplete selected actor rendering was silently restarted")
	}
}

func TestNativeMainEditorContinuationDoesNotRereadAdmission(t *testing.T) {
	raw := make([]byte, 0x11280)
	memory := commandFrameBacking(raw)
	_ = memory.Write16(0xf0e, 1)
	frame := NativeFrameRegisterContext{}
	state := NativeMainRenderState{Step: 4, View: 8}
	rules := NativeActorRenderRules{}
	want := errors.New("retained source editor failure")
	calls := 0
	cb := NativeMainRenderCallbacks{World: NativeWorldRenderCallbacks{Effects: NativeActorEffectsCallbacks{NativeRenderFrameCallbacks: NativeRenderFrameCallbacks{Memory: memory, Frame: &frame}}}, PaintingAdvance: func(*NativeFrameRegisterContext) (bool, error) {
		calls++
		if calls == 1 {
			_ = memory.Write16(0xf0e, 0)
			return false, nil
		}
		return false, want
	}}
	if done, err := rules.AdvanceMain(cb, &state); err != nil || done || !state.painting || state.Step != 4 {
		t.Fatal("source editor call did not suspend", done, err)
	}
	if done, err := rules.AdvanceMain(cb, &state); done || !errors.Is(err, want) || calls != 2 {
		t.Fatal("changed editor mode lost the retained source call", done, err, calls)
	}
	if err := state.Begin(); !errors.Is(err, want) {
		t.Fatal("failed native prefix was silently restarted", err)
	}
}
