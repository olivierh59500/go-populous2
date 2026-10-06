package app

import (
	"bytes"
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

func TestTerrainBackdropUsesBothEdgeCornersAndOriginalRectangles(t *testing.T) {
	w := &engine.World{}
	w.Heights[20+28*engine.CornerSize] = 8
	w.Heights[28+20*engine.CornerSize] = 1
	strips := terrainBackdropStrips(w, 20, 20)
	if len(strips) != 2 || strips[0].Destination != image.Rect(64, 72, 128, 135) || strips[0].Source != image.Pt(128, 104) || strips[1].Destination != image.Rect(256, 128, 320, 135) || strips[1].Source != image.Pt(192, 160) {
		t.Fatal("edge backdrop geometry differs", strips)
	}
	w.Heights[20+28*engine.CornerSize], w.Heights[28+20*engine.CornerSize] = 0, 0
	if len(terrainBackdropStrips(w, 20, 20)) != 0 {
		t.Fatal("water edge received a backdrop strip")
	}
	if len(terrainBackdropStrips(w, 64, 64)) != 0 {
		t.Fatal("outside camera read a map corner")
	}
}

func TestTerrainBackdropRetainsCorner64AndCopiesFromImmutableBackground(t *testing.T) {
	w := &engine.World{}
	w.Heights[56+64*engine.CornerSize] = 2
	w.Heights[64+56*engine.CornerSize] = 3
	background := image.NewRGBA(image.Rect(0, 0, 320, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			background.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y), 17, 255})
		}
	}
	before := append([]byte(nil), background.Pix...)
	g := &Game{World: w, CameraX: 56, CameraY: 56, Assets: &Assets{Visual: &visualassets.Bundle{Background: background}}, framebuffer: image.NewRGBA(background.Bounds())}
	g.drawTerrainBackdrop()
	if g.framebuffer.RGBAAt(64, 120) != (color.RGBA{128, 152, 17, 255}) || g.framebuffer.RGBAAt(256, 112) != (color.RGBA{192, 144, 17, 255}) {
		t.Fatal("map boundary strip did not use corner64")
	}
	if !bytes.Equal(background.Pix, before) {
		t.Fatal("strip restoration mutated the source background")
	}
}

func TestPrivateTerrainBackdropPixelsMatchOriginalWorldCopies(t *testing.T) {
	path := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if path == "" {
		t.Skip("set private original art directory")
	}
	source, err := populous2.LoadFS(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	rules, err := populous2.DecodeNativeActorRenderRules(source.Executable)
	if err != nil {
		t.Fatal(err)
	}
	tiles, err := populous2.DecodeNativeTileBitmapBank(source.Raw["block0.pak"])
	if err != nil {
		t.Fatal(err)
	}
	var palette [16]color.RGBA
	for i := range palette {
		palette[i] = color.RGBA{uint8(i * 17), uint8(i * 13), uint8(i * 7), 255}
	}
	// Distinct rows and columns expose wrong offsets and accidental overlapping
	// copies even when the shipped scenery happens to contain a flat color.
	background := make([]byte, 32000)
	for i := range background {
		background[i] = byte(i*53 + 17)
	}
	backgroundRGBA, err := populous2.DecodeScreen(background, palette)
	if err != nil {
		t.Fatal(err)
	}
	for _, camera := range [][2]int{{20, 20}, {56, 56}} {
		for left := 0; left <= 8; left++ {
			for right := 0; right <= 8; right++ {
				raw := make([]byte, 0x11280)
				binary.BigEndian.PutUint16(raw[0x5f44:], uint16(camera[0]))
				binary.BigEndian.PutUint16(raw[0x5f46:], uint16(camera[1]))
				w := &engine.World{}
				for side, corner := range [2][2]int{{camera[0], camera[1] + 8}, {camera[0] + 8, camera[1]}} {
					height := left
					if side == 1 {
						height = right
					}
					w.Heights[corner[0]+corner[1]*engine.CornerSize] = uint8(height)
					x, y := corner[0], corner[1]
					bit := uint8(1)
					if x == 64 {
						x--
						bit = 2
					}
					if y == 64 {
						y--
						bit = 8
					}
					at := 0xf44 + (x+y*64)*4
					if height == 8 {
						raw[at], raw[at+1] = 7, bit
					} else {
						raw[at] = uint8(height)
					}
				}
				span := func(at, n int) error {
					if at < 0 || at > len(raw)-n {
						return fmt.Errorf("private backdrop memory outside backing")
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
				bitmap := append([]byte(nil), background...)
				frame := populous2.NativeFrameRegisterContext{AddressBase: 0x200000}
				imageState := rules.Frames.Images.NewImageState()
				_, err := rules.WorldDraw(populous2.NativeWorldRenderCallbacks{Effects: populous2.NativeActorEffectsCallbacks{NativeRenderFrameCallbacks: populous2.NativeRenderFrameCallbacks{Memory: memory, Frame: &frame, Image: &imageState, Bitmap: bitmap}}, Tiles: tiles, Tile: func(populous2.NativeTileChunkRequest, []byte) error { return nil }, Background: background}, &populous2.NativeWorldRenderState{})
				if err != nil {
					t.Fatal(err)
				}
				expected, err := populous2.DecodeScreen(bitmap, palette)
				if err != nil {
					t.Fatal(err)
				}
				g := &Game{World: w, CameraX: camera[0], CameraY: camera[1], Assets: &Assets{Visual: &visualassets.Bundle{Background: backgroundRGBA}}, framebuffer: image.NewRGBA(expected.Bounds())}
				g.drawTerrainBackdrop()
				if !bytes.Equal(g.framebuffer.Pix, expected.Pix) {
					t.Fatalf("original high-edge copies differ camera%v heights%d/%d", camera, left, right)
				}
			}
		}
	}
}
