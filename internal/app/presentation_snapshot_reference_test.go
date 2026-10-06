package app

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/draw"
	"os"
	"testing"

	"go-populous2/internal/engine"
	"go-populous2/internal/populous2"
)

// Thirty-six 50 Hz host updates contain nine original main-frame captures.
// The source actor moves after each captured image; intermediate host updates
// must retain that image's phase instead of exposing the following physics.
func TestPrivatePresentationPreservesOriginalSelectedPhaseFor36Updates(t *testing.T) {
	original, portable := os.Getenv("POPULOUS2_EXPORT_TEST_DIR"), os.Getenv("POPULOUS2_ENDING_TEST_DIR")
	if original == "" || portable == "" {
		t.Skip("set original and portable artwork directories")
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
	bank, err := populous2.DecodeNativeSpriteBitmapBank(source, 0)
	if err != nil {
		t.Fatal(err)
	}
	land, err := engine.DecodeLandscape(source.Raw["land0.dat"])
	if err != nil {
		t.Fatal(err)
	}
	w := &engine.World{Editor: true, Landscape: land}
	var heights [engine.CornerSize * engine.CornerSize]uint8
	for i := range heights {
		heights[i] = 1
	}
	if err := w.EditorSetTerrain(heights); err != nil {
		t.Fatal(err)
	}
	if err := w.EditorPlaceFollower(0, 32, 32, 0x123); err != nil {
		t.Fatal(err)
	}
	w.Players[0].Mode = engine.Rally
	w.Players[0].Leader = 0
	w.Followers[1].Direction = 2
	w.Followers[1].MovementSpeed = 20
	w.Followers[1].Weapons = 7
	s := w.Snapshot()
	s.Motion[1] = engine.FollowerMotionSnapshot{PositionX: 32*256 + 128, PositionY: 32*256 + 128, VelocityX: 20, LegRemaining: 255, PositionSet: true, Moving: true}
	w, err = s.Restore()
	if err != nil {
		t.Fatal(err)
	}
	g := &Game{World: w, SelectedFollower: 1, Assets: assets, framebuffer: image.NewRGBA(image.Rect(0, 0, 320, 200))}
	var p PlayingPresentation
	actor := populous2.FollowerMotionActor{Kind: 2, Player: 0, X: 32*256 + 128, Y: 32*256 + 128, VX: 20, Speed: 20, Timer: 255, State: 4, ReturnState: 18, Population: 0x123}
	raw := make([]byte, 0x11280)
	const record = 0x76f4
	raw[record], raw[record+12], raw[record+22], raw[record+25] = 2, 1, 4, 7
	binary.BigEndian.PutUint32(raw[0xf36:], 0x200000+record)
	binary.BigEndian.PutUint16(raw[0x3b8:], 1)
	binary.BigEndian.PutUint32(raw[record+26:], 0x123)
	memory := selectedPanelReferenceMemory(raw)
	var expected *image.RGBA
	for update := 0; update < 36; update++ {
		if update%4 == 0 {
			p.Capture(g, true)
			binary.BigEndian.PutUint16(raw[record+10:], uint16(actor.Animation))
			binary.BigEndian.PutUint16(raw[record+14:], uint16(actor.VX))
			binary.BigEndian.PutUint16(raw[record+16:], uint16(actor.VY))
			binary.BigEndian.PutUint16(raw[0xf42:], uint16(update/4+1))
			frame := populous2.NativeFrameRegisterContext{AddressBase: 0x200000}
			imageState := rules.Frames.Images.NewImageState()
			bitmap := make([]byte, 32000)
			cb := populous2.NativeRenderFrameCallbacks{Memory: memory, Frame: &frame, Image: &imageState, Bitmap: bitmap, Sprite: bank.Paint}
			_, err := rules.Frames.Selected(cb, populous2.NativeRenderFrameChildren{DrawActor: func(at int, _ *populous2.NativeFrameRegisterContext) error {
				_, e := rules.Actor(at, populous2.NativeActorEffectsCallbacks{NativeRenderFrameCallbacks: cb}, &populous2.NativeActorRenderState{}, populous2.NativeActorRenderChildren{})
				return e
			}})
			if err != nil {
				t.Fatal(err)
			}
			expected, err = populous2.DecodeScreen(bitmap, assets.Visual.Palettes[0])
			if err != nil {
				t.Fatal(err)
			}
			// Both physics controllers now advance the live actor. The selected
			// image above remains authoritative until the next main capture.
			source.FollowerMotion.Tick(&actor, populous2.FollowerMotionCallbacks{Admit: func(*populous2.FollowerMotionActor, int16, int16) bool { return true }})
			w.Step()
		}
		view := p.Renderer(g)
		draw.Draw(view.framebuffer, view.framebuffer.Bounds(), image.NewUniform(assets.Visual.Palettes[0][0]), image.Point{}, draw.Src)
		view.drawSelectionPanel()
		if !bytes.Equal(view.framebuffer.Pix, expected.Pix) {
			t.Fatalf("host update %d exposed a different selected animation phase than the source main image", update)
		}
	}
}
