package app

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"go-populous2/internal/engine"
	"go-populous2/internal/populous2"
	"image"
	"image/color"
	"image/draw"
	"os"
	"testing"
)

func TestPrivateMixedDepthMatchesOriginalHighObjectPixels(t *testing.T) {
	original, portable := os.Getenv("POPULOUS2_EXPORT_TEST_DIR"), os.Getenv("POPULOUS2_ENDING_TEST_DIR")
	if original == "" || portable == "" {
		t.Skip("set private original and portable art directories")
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
	raw := make([]byte, 0x11280)
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
	const x, y = 24, 24
	const follower = 0x76f4
	const scenery = 0x6bd0
	grid := 0xf44 + (x+y*64)*4
	// Raised flat surface and centered objects provide actual sprite overlap.
	raw[grid], raw[grid+1] = 0xa3, 31
	raw[follower], raw[follower+12], raw[follower+22] = 2, 1, 4
	binary.BigEndian.PutUint16(raw[follower+6:], x*256+128)
	binary.BigEndian.PutUint16(raw[follower+8:], y*256+128)
	binary.BigEndian.PutUint16(raw[follower+16:], 0xffec)
	binary.BigEndian.PutUint32(raw[follower+26:], 1000)
	raw[scenery], raw[scenery+1], raw[scenery+12] = 22, 1, 3
	binary.BigEndian.PutUint16(raw[scenery+6:], x*256+128)
	binary.BigEndian.PutUint16(raw[scenery+8:], y*256+128)
	binary.BigEndian.PutUint16(raw[scenery+10:], uint16(source.Scenery.Trees.Animations[0]))
	world := &engine.World{}
	world.Tiles[x+y*64] = engine.Cell{BaseAltitude: 3, Shape: 15, Code: 31, Corners: [4]uint8{4, 4, 4, 4}}
	if err := world.EditorPlaceFollower(0, x, y, 1000); err != nil {
		t.Fatal(err)
	}
	world.Followers[1].Direction = 0
	world.Players[0].Leader = 0
	world.Nature.Scenery[0] = engine.SceneryActor{Kind: engine.SceneryTree, X: x, Y: y, Age: 1}
	followerRef := engine.ActorRef{Kind: engine.ActorFollower, Index: 1}
	sceneryRef := engine.ActorRef{Kind: engine.ActorScenery, Index: 0}
	// Insert the tree last: the prior family renderer incorrectly put it behind
	// every follower regardless of this real source chain membership.
	world.Actors.Link(followerRef, x*256+128, y*256+128)
	world.Actors.Link(sceneryRef, x*256+128, y*256+128)
	g := &Game{World: world, Assets: assets, CameraX: 20, CameraY: 20, framebuffer: image.NewRGBA(image.Rect(0, 0, 320, 200))}
	for _, command := range sourceRenderPlan(world, 20, 20, nil) {
		if command.Actor.Kind != engine.ActorNone {
			g.drawRegisteredActor(command.Actor, 0)
		}
	}
	want := image.NewRGBA(image.Rect(0, 0, 320, 200))
	bank, err := populous2.DecodeNativeSpriteBitmapBank(source, 0)
	if err != nil {
		t.Fatal(err)
	}
	bitmap := make([]byte, 32000)
	opaque := image.NewRGBA(g.framebuffer.Bounds())
	draw.Draw(opaque, opaque.Bounds(), image.NewUniform(assets.Visual.Palettes[0][0]), image.Point{}, draw.Src)
	draw.Draw(opaque, opaque.Bounds(), g.framebuffer, image.Point{}, draw.Over)
	g.framebuffer = opaque
	var requests []populous2.NativePresentationSprite
	frame := populous2.NativeFrameRegisterContext{AddressBase: 0x200000}
	state := populous2.NativeWorldRenderState{ProjectionX: 192, ProjectionY: 72}
	imageState := rules.Frames.Images.NewImageState()
	cb := populous2.NativeWorldRenderCallbacks{Effects: populous2.NativeActorEffectsCallbacks{NativeRenderFrameCallbacks: populous2.NativeRenderFrameCallbacks{Memory: memory, Frame: &frame, Image: &imageState, Bitmap: bitmap, Sprite: func(r populous2.NativePresentationSprite, b []byte) error {
		requests = append(requests, r)
		return bank.Paint(r, b)
	}}, Cropped: func(r populous2.NativeCroppedSpriteRequest, _ []byte) error {
		requests = append(requests, r.Sprite)
		return bank.PaintCropped(r, bitmap)
	}}}
	// ProjectActor restores the row and column supplied by its source parent.
	frame.D[6], frame.D[7] = 4, 4
	for _, at := range []int{follower, scenery} {
		if _, err := rules.ProjectActor(at, grid+4, cb, &state); err != nil {
			t.Fatal(err)
		}
	}
	want, err = populous2.DecodeScreen(bitmap, assets.Visual.Palettes[0])
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(g.framebuffer.Pix, want.Pix) {
		for y := 0; y < 200; y++ {
			for x := 0; x < 320; x++ {
				if g.framebuffer.RGBAAt(x, y) != want.RGBAAt(x, y) {
					t.Fatalf("mixed source high-object framebuffer differs at %d,%d got%v want%v requests%v anchor%v", x, y, g.framebuffer.RGBAAt(x, y), want.RGBAAt(x, y), requests, world.Followers[1])
				}
			}
		}
	}
	reversed := image.NewRGBA(want.Bounds())
	draw.Draw(reversed, reversed.Bounds(), image.NewUniform(color.RGBAModel.Convert(assets.Visual.Palettes[0][0])), image.Point{}, draw.Src)
	for i := len(requests) - 1; i >= 0; i-- {
		r := requests[i]
		sprite := assets.Visual.Sprites[0][r.Sprite].Image
		draw.Draw(reversed, image.Rect(int(r.X), int(r.Y), int(r.X)+sprite.Bounds().Dx(), int(r.Y)+int(r.Height)), sprite, image.Point{}, draw.Over)
	}
	if bytes.Equal(want.Pix, reversed.Pix) {
		t.Fatal("fixture does not exercise visible high-object overlap")
	}
}
