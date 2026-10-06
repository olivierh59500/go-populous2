package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	"os"
	"testing"

	"go-populous2/internal/engine"
	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func TestPrivatePointerFramesMatchOriginalAttachedSpritePixels(t *testing.T) {
	path := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if path == "" {
		t.Skip("set private original graphic directory")
	}
	source, err := populous2.LoadFS(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	if err := exportPointers(output, source); err != nil {
		t.Fatal(err)
	}
	art, err := visualassets.LoadPointers(os.DirFS(output))
	if err != nil {
		t.Fatal(err)
	}
	rules, err := populous2.DecodeNativeRenderFrameRules(source.Executable)
	if err != nil {
		t.Fatal(err)
	}
	raw := make([]byte, 0x11280)
	memory := populous2.FollowerCleanupMemory{
		Read8: func(at int) (uint8, error) { return raw[at], nil }, Read16: func(at int) (uint16, error) { return binary.BigEndian.Uint16(raw[at:]), nil }, Read32: func(at int) (uint32, error) { return binary.BigEndian.Uint32(raw[at:]), nil },
		Write8: func(at int, v uint8) error { raw[at] = v; return nil }, Write16: func(at int, v uint16) error { binary.BigEndian.PutUint16(raw[at:], v); return nil }, Write32: func(at int, v uint32) error { binary.BigEndian.PutUint32(raw[at:], v); return nil },
	}
	checked := 0
	roles := []struct {
		name     string
		selector uint16
	}{{"normal", 0}, {"forbidden", 0x360}, {"interface", 0x750}, {"inspect", 0x5a0}}
	for _, role := range roles {
		p, err := populous2.NewNativeFramePresentationState(source.Executable, 0x500000, 0x400000)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.Initialize(source.Executable, populous2.NativeMouseSample{}); err != nil {
			t.Fatal(err)
		}
		p.Input.Mouse.Image = role.selector
		p.Input.Mouse.PositionX, p.Input.Mouse.PositionY = 80, 60
		frame := populous2.NativeFrameRegisterContext{AddressBase: 0x200000}
		if _, err := p.VBlank(populous2.NativeMouseSample{}, memory, &frame); err != nil {
			t.Fatal(err)
		}
		background, composed := make([]byte, 320*200*4), make([]byte, 320*200*4)
		if err := p.WriteRGBA(background, false); err != nil {
			t.Fatal(err)
		}
		if err := p.WriteRGBA(composed, true); err != nil {
			t.Fatal(err)
		}
		got := image.NewRGBA(image.Rect(0, 0, 320, 200))
		copy(got.Pix, background)
		draw.Draw(got, image.Rect(40, 30, 56, 46), art.Frames[role.name][0].Image, image.Point{}, draw.Over)
		if !bytes.Equal(got.Pix, composed) {
			t.Fatal("original named pointer pixels differ", role.name)
		}
	}

	for _, power := range source.Spells {
		if power.ID == populous2.RaiseLower {
			continue
		}
		command := uint16(0)
		for _, a := range source.Actions {
			if a.Spell == power.ID {
				command = uint16(a.Command)
				break
			}
		}
		for phase := 0; phase < 4; phase++ {
			binary.BigEndian.PutUint16(raw[0xeb18:], command)
			binary.BigEndian.PutUint16(raw[0xf42:], uint16(phase*8))
			p, err := populous2.NewNativeFramePresentationState(source.Executable, 0x500000, 0x400000)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := p.Initialize(source.Executable, populous2.NativeMouseSample{}); err != nil {
				t.Fatal(err)
			}
			frame := populous2.NativeFrameRegisterContext{AddressBase: 0x200000}
			imageState := rules.Images.NewImageState()
			if _, err := rules.Cursor(populous2.NativeRenderFrameCallbacks{Memory: memory, Frame: &frame, Image: &imageState, Input: &p.Input}); err != nil {
				t.Fatal(err)
			}
			p.Input.Mouse.PositionX, p.Input.Mouse.PositionY = 80, 60
			if _, err := p.VBlank(populous2.NativeMouseSample{}, memory, &frame); err != nil {
				t.Fatal(err)
			}
			background, composed := make([]byte, 320*200*4), make([]byte, 320*200*4)
			if err := p.WriteRGBA(background, false); err != nil {
				t.Fatal(err)
			}
			if err := p.WriteRGBA(composed, true); err != nil {
				t.Fatal(err)
			}
			got := image.NewRGBA(image.Rect(0, 0, 320, 200))
			copy(got.Pix, background)
			name := fmt.Sprintf("power/%d", engine.PowerFromCostSlot(engine.PowerID(power.ID)))
			sprite := art.Frames[name][phase]
			draw.Draw(got, image.Rect(40, 30, 56, 46), sprite.Image, image.Point{}, draw.Over)
			if !bytes.Equal(got.Pix, composed) {
				t.Fatalf("attached pointer pixels/position differ power%d phase%d", power.ID, phase)
			}
			checked++
		}
	}
	if checked != 112 {
		t.Fatal("original pointer power/frame coverage incomplete", checked)
	}
}

func TestPrivateMapPointerGeometryMatchesOriginalNormalShapes(t *testing.T) {
	path := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if path == "" {
		t.Skip("set private original art directory")
	}
	source, err := populous2.LoadFS(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	rules, err := populous2.DecodeNativeRenderFrameRules(source.Executable)
	if err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	if err := exportPointers(output, source); err != nil {
		t.Fatal(err)
	}
	art, err := visualassets.LoadPointers(os.DirFS(output))
	if err != nil {
		t.Fatal(err)
	}
	for shape := 0; shape < 16; shape++ {
		raw := make([]byte, 0x11280)
		put := func(at int, v uint16) { binary.BigEndian.PutUint16(raw[at:], v) }
		put(0x5f44, 20)
		put(0x5f46, 20)
		put(0x5f4c, 24)
		put(0x5f4e, 24)
		put(0x5f48, 193)
		put(0x5f4a, 111)
		raw[0xf45+(24+24*64)*4] = byte(shape)
		memory := populous2.FollowerCleanupMemory{Read8: func(at int) (uint8, error) { return raw[at], nil }, Read16: func(at int) (uint16, error) { return binary.BigEndian.Uint16(raw[at:]), nil }, Read32: func(at int) (uint32, error) { return binary.BigEndian.Uint32(raw[at:]), nil }, Write8: func(at int, v uint8) error { raw[at] = v; return nil }, Write16: func(at int, v uint16) error { put(at, v); return nil }, Write32: func(at int, v uint32) error { binary.BigEndian.PutUint32(raw[at:], v); return nil }}
		frame := populous2.NativeFrameRegisterContext{AddressBase: 0x200000}
		imageState := rules.Images.NewImageState()
		plan, err := rules.MapCursor(populous2.NativeRenderFrameCallbacks{Memory: memory, Frame: &frame, Image: &imageState, Bitmap: make([]byte, 32000)})
		if err != nil {
			t.Fatal(err)
		}
		if len(plan.Sprites) != 1 || plan.Sprites[0].Sprite != art.MapMarkerSprite || plan.Sprites[0].X != 189 || plan.Sprites[0].Y != 112 {
			t.Fatal("original map marker sprite/location differs", plan.Sprites)
		}
		if len(plan.Pixels) != 4 {
			t.Fatal("source map pointer outline omitted vertices")
		}
		for vertex, pixel := range plan.Pixels {
			offset := art.MapOutlines[shape][vertex]
			if int(pixel.X) != 192+offset[0] || int(pixel.Y) != 114+offset[1] || pixel.Color != 5 {
				t.Fatalf("map pointer shape%d vertex%d differs", shape, vertex)
			}
		}
	}
}
