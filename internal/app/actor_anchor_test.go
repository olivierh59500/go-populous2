package app

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"os"
	"testing"

	"go-populous2/internal/engine"
	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

// The source renderer is an optional comparison tool. Its callbacks capture
// final sprite coordinates; the production app uses only named imported art.
func TestPrivateActorCallerAnchorsMatchOriginalRenderer(t *testing.T) {
	original, portable := os.Getenv("POPULOUS2_EXPORT_TEST_DIR"), os.Getenv("POPULOUS2_ENDING_TEST_DIR")
	if original == "" || portable == "" {
		t.Skip("set private original and portable asset directories")
	}
	source, err := populous2.LoadFS(os.DirFS(original))
	if err != nil {
		t.Fatal(err)
	}
	assets, err := LoadAssets(os.DirFS(portable))
	if err != nil {
		t.Fatal(err)
	}
	rules, err := populous2.DecodeNativeActorRenderRules(source.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name        string
		kind, state uint8
		token       uint16
		animation   string
		frame       int
		town        bool
	}{
		{"walking", 2, 4, 0, "follower/0/0/north", 0, false},
		{"town", 4, 6, 0, "", 0, true},
		{"retained death", 2, 8, 0x1e74, "combat/death", 0, false},
		{"combat", 2, 14, 0x1c8, "combat/attack", 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw := make([]byte, 0x11280)
			const actor = 0x76f4
			raw[actor], raw[actor+12], raw[actor+22], raw[actor+1] = test.kind, 1, test.state, 18
			binary.BigEndian.PutUint16(raw[actor+10:], test.token)
			binary.BigEndian.PutUint32(raw[actor+26:], 1000)
			binary.BigEndian.PutUint16(raw[actor+16:], uint16(0xffec))
			binary.BigEndian.PutUint16(raw[0x3b8:], 1)
			binary.BigEndian.PutUint16(raw[0x3b0:], 1)
			span := func(at, n int) error {
				if at < 0 || at > len(raw)-n {
					return fmt.Errorf("private actor read outside backing")
				}
				return nil
			}
			memory := populous2.FollowerCleanupMemory{
				Read8: func(at int) (uint8, error) {
					if e := span(at, 1); e != nil {
						return 0, e
					}
					return raw[at], nil
				},
				Read16: func(at int) (uint16, error) {
					if e := span(at, 2); e != nil {
						return 0, e
					}
					return binary.BigEndian.Uint16(raw[at:]), nil
				},
				Read32: func(at int) (uint32, error) {
					if e := span(at, 4); e != nil {
						return 0, e
					}
					return binary.BigEndian.Uint32(raw[at:]), nil
				},
				Write8: func(at int, v uint8) error {
					if e := span(at, 1); e != nil {
						return e
					}
					raw[at] = v
					return nil
				},
				Write16: func(at int, v uint16) error {
					if e := span(at, 2); e != nil {
						return e
					}
					binary.BigEndian.PutUint16(raw[at:], v)
					return nil
				},
				Write32: func(at int, v uint32) error {
					if e := span(at, 4); e != nil {
						return e
					}
					binary.BigEndian.PutUint32(raw[at:], v)
					return nil
				},
			}
			world := &engine.World{}
			world.Followers[1] = engine.Follower{State: engine.Walking, Owner: 0, X: 12, Y: 12, Direction: 0, Population: 1000, Stage: 18}
			world.Tiles[12+12*64] = engine.Cell{Shape: 15, BaseAltitude: 2}
			g := &Game{World: world, CameraX: 8, CameraY: 8, Assets: assets, framebuffer: image.NewRGBA(image.Rect(0, 0, 320, 200))}
			ax, ay := g.followerRenderAnchor(world.Followers[1])
			frame := populous2.NativeFrameRegisterContext{AddressBase: 0x200000}
			frame.D[0], frame.D[1] = uint32(ax), uint32(ay)
			imageState := rules.Frames.Images.NewImageState()
			var requests []populous2.NativePresentationSprite
			_, err := rules.Follower(actor, populous2.NativeRenderFrameCallbacks{Memory: memory, Frame: &frame, Image: &imageState, Sprite: func(s populous2.NativePresentationSprite, _ []byte) error { requests = append(requests, s); return nil }}, &populous2.NativeActorRenderState{}, populous2.NativeActorRenderChildren{})
			if err != nil {
				t.Fatal(err)
			}
			var layers []visualassets.SpriteLayer
			if test.town {
				layers = assets.Visual.Towns.CenterLayers(18, 0, 1000, 0)
				g.drawTownCenter(world.Followers[1], ax, ay, 0)
			} else {
				layers = assets.Visual.Animations[test.animation].Frames[test.frame].Layers
				g.animation(test.animation, test.frame, ax, ay, 0)
			}
			if len(requests) != len(layers) {
				t.Fatal("actor caller produced different layer count", len(requests), len(layers))
			}
			for i, layer := range layers {
				sprite := assets.Visual.Sprites[0][layer.Sprite]
				px, py := ax+layer.X-sprite.AnchorX, ay+layer.Y-sprite.AnchorY
				expected := requests[i]
				if layer.Sprite != expected.Sprite || int16(px) != expected.X || int16(py) != expected.Y {
					t.Fatal("app/source final coordinates differ", i, px, py, expected)
				}
			}
			// Composing the source's final requests catches any extra caller Y offset.
			expectedPixels := image.NewRGBA(image.Rect(0, 0, 320, 200))
			for _, request := range requests {
				sprite := assets.Visual.Sprites[0][request.Sprite]
				for y := range sprite.Image.Bounds().Dy() {
					for x := range sprite.Image.Bounds().Dx() {
						p := sprite.Image.RGBAAt(x, y)
						if p.A == 255 {
							expectedPixels.SetRGBA(int(request.X)+x, int(request.Y)+y, p)
						}
					}
				}
			}
			for y := range 200 {
				for x := range 320 {
					if g.framebuffer.RGBAAt(x, y) != (color.RGBAModel.Convert(expectedPixels.At(x, y)).(color.RGBA)) {
						t.Fatal("app/source actor framebuffer placement differs", x, y)
					}
				}
			}
		})
	}
}
