package app

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"reflect"
	"testing"

	"go-populous2/internal/engine"
	"go-populous2/internal/populous2"
)

func TestPrivateCampaignScoreUsesTownRightsFromCurrentRenderedPass(t *testing.T) {
	path := os.Getenv("POPULOUS2_REFERENCE_TEST_DIR")
	if path == "" {
		t.Skip("set the private imported original resources directory")
	}
	bundle, err := populous2.LoadFS(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	rules, err := populous2.DecodeNativeActorRenderRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	sprites, err := populous2.DecodeNativeSpriteBitmapBank(bundle, 0)
	if err != nil {
		t.Fatal(err)
	}
	tiles, err := populous2.DecodeNativeTileBitmapBank(bundle.Raw["block0.pak"])
	if err != nil {
		t.Fatal(err)
	}
	raw := make([]byte, 0x11280)
	memory := populous2.FollowerCleanupMemory{
		Read8: func(at int) (uint8, error) { return raw[at], nil }, Read16: func(at int) (uint16, error) { return binary.BigEndian.Uint16(raw[at:]), nil }, Read32: func(at int) (uint32, error) { return binary.BigEndian.Uint32(raw[at:]), nil },
		Write8: func(at int, v uint8) error { raw[at] = v; return nil }, Write16: func(at int, v uint16) error { binary.BigEndian.PutUint16(raw[at:], v); return nil }, Write32: func(at int, v uint32) error { binary.BigEndian.PutUint32(raw[at:], v); return nil },
	}
	p, err := populous2.NewNativeFramePresentationState(bundle.Executable, 0x500000, 0x400000)
	if err != nil {
		t.Fatal(err)
	}
	m := p.Memory(memory)
	_ = m.Write16(0x5f44, 20)
	_ = m.Write16(0x5f46, 20)
	_ = m.Write16(0xeb18, 2)
	_ = m.Write16(0xeb42, 1)
	_ = m.Write16(0x3b8, 1)
	_ = m.Write32(0xf40, 100)
	_ = m.Write16(0xeb2c, 0)
	_ = m.Write16(0xeb2e, 0)
	_ = m.Write16(0xe8bc, 1)
	_ = m.Write16(0xe9f6, 2)
	_ = m.Write32(0xe8a8, 100)
	_ = m.Write32(0xe9e2, 100)
	_ = m.Write16(0x138, 100)
	for cell := 0; cell < 4096; cell++ {
		raw[0xf44+cell*4], raw[0xf45+cell*4] = 1, 245
	}
	for side := 0; side < 2; side++ {
		at := 0x76f4 + side*52
		x, y := 22+side, 22+side
		raw[at], raw[at+1], raw[at+12], raw[at+22] = 4, 1, byte(side+1), 6
		binary.BigEndian.PutUint16(raw[at+6:], uint16(x*256+128))
		binary.BigEndian.PutUint16(raw[at+8:], uint16(y*256+128))
		binary.BigEndian.PutUint32(raw[at+26:], 100)
		binary.BigEndian.PutUint16(raw[0xf46+(x+y*64)*4:], uint16((side+1)*52))
	}
	frame := populous2.NativeFrameRegisterContext{AddressBase: 0x200000}
	ready, err := p.AdvanceClock(&frame, populous2.NativeFrameClockCallbacks{Memory: m})
	if err != nil || !ready {
		t.Fatal("source clock did not enter its ordinary frame", ready, err)
	}
	for _, at := range []int{0xe8ee, 0xea28} {
		if value, _ := m.Read16(at); value != 0 {
			t.Fatal("clock did not initialize the static scenario word", value)
		}
	}
	imageState := rules.Frames.Images.NewImageState()
	bitmap := make([]byte, 32000)
	callbacks := populous2.NativeWorldRenderCallbacks{Effects: populous2.NativeActorEffectsCallbacks{NativeRenderFrameCallbacks: populous2.NativeRenderFrameCallbacks{Memory: m, Frame: &frame, Image: &imageState, Bitmap: bitmap, Sprite: sprites.Paint}}, Tiles: tiles, Tile: tiles.PaintChunk, Background: make([]byte, 32000)}
	plan, err := rules.WorldDraw(callbacks, &populous2.NativeWorldRenderState{})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Actors) != 2 {
		t.Fatal("normal viewport did not render both live towns", plan.Actors)
	}
	for side := 0; side < 2; side++ {
		value, _ := m.Read16(0xe8a4 + side*314 + 0x4a)
		if value != 1 {
			t.Fatalf("town rendering did not add full terrain rights for camp %d: %d", side, value)
		}
	}
	// The population pass follows drawing. Elimination does not erase the
	// rights already recorded by the town rendered in this same frame.
	_ = m.Write32(0xe9e2, 0)
	scoreRules, err := populous2.DecodeCampaignResultRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	var stats [3]populous2.CampaignResultStatistics
	for owner := 1; owner <= 2; owner++ {
		stats[owner], err = populous2.ReadCampaignResultStatistics(uint8(owner), m)
		if err != nil {
			t.Fatal(err)
		}
	}
	if eliminated, ok := scoreRules.Detect(2, stats); !ok || eliminated != 2 {
		t.Fatal("source did not detect the normal winning outcome", eliminated, ok)
	}
	clock, _ := m.Read32(0xf40)
	want, err := scoreRules.Score(clock, stats[1], stats[2])
	if err != nil {
		t.Fatal(err)
	}
	got, err := engine.ScoreCampaign(clock, engine.CampaignStatistics{ScenarioOptions: 1}, engine.CampaignStatistics{ScenarioOptions: 1})
	if err != nil || got.Value != want.Value || got.Ratio != want.RatioWord {
		t.Fatal("rendered town rights did not reproduce the source score", got, want, err)
	}
	t.Log(fmt.Sprintf("clock %d, rendered rights 1/1, score %d", clock, got.Value))
}

func TestPrivateCustomDefaultsMatchOriginalStructuredTemplate(t *testing.T) {
	path := os.Getenv("POPULOUS2_REFERENCE_TEST_DIR")
	if path == "" {
		t.Skip("set the private imported original resources directory")
	}
	bundle, err := populous2.LoadFS(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	record := bundle.Executable.Hunks[0].Data[0x20630:0x2072a]
	levels, err := engine.DecodeCampaign(bytes.Repeat(record, 200))
	if err != nil {
		t.Fatal(err)
	}
	want := levels[0]
	for side := range want.Players {
		if !want.Players[side].FixedMagnet {
			want.Players[side].MagnetX, want.Players[side].MagnetY = 0, 0
		}
	}
	got := defaultCustomLevel(0, 0)
	if !reflect.DeepEqual(got.Players, want.Players) || got.OpponentExperience != want.OpponentExperience || got.WorldParameters != want.WorldParameters {
		t.Fatal("named Go custom setup differs from the original data template", got.Players, want.Players)
	}
}
