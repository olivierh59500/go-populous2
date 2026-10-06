package main

import (
	"encoding/binary"
	"image"
	"os"
	"reflect"
	"testing"

	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func TestPrivateSelectedPanelCoordinatesAndWeaponArtworkMatchSource(t *testing.T) {
	input := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if input == "" {
		t.Skip("set private imported artwork directory")
	}
	source, err := populous2.LoadFS(os.DirFS(input))
	if err != nil {
		t.Fatal(err)
	}
	p, err := decodeSelectedPanel(source)
	if err != nil {
		t.Fatal(err)
	}
	if p.ActorX != 282 || p.ActorY != 42 || p.WeaponX != 254 || p.WeaponY != 47 || p.HitRect() != image.Rect(245, 0, 315, 46) || p.PopulationSprite != 177 {
		t.Fatal("selected panel source geometry changed")
	}
	code := source.Executable.Hunks[0].Data
	for stage, frame := range source.TownCenterArt.Frames {
		height := 0
		for _, layer := range frame.Layers {
			if layer.Sprite != 89 {
				height = int(binary.BigEndian.Uint16(code[0x21626+layer.Sprite*12+6:]))
			}
		}
		if p.TownHitHeight(stage) != height {
			t.Fatalf("town stage%d hit height differs", stage)
		}
	}
	for weapon, frame := range p.Weapons {
		start := int(binary.BigEndian.Uint16(code[0x20b60+weapon*2:])) + 0x140
		frames, err := populous2.DecodeAnimation(source.Executable, start)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]visualassets.SpriteLayer, len(frames[0].Layers))
		for i, layer := range frames[0].Layers {
			want[i] = visualassets.SpriteLayer{Sprite: layer.Sprite, X: layer.X, Y: layer.Y}
		}
		if !reflect.DeepEqual(frame.Layers, want) {
			t.Fatalf("weapon%d composition differs", weapon)
		}
	}
	rules, err := populous2.DecodeNativePresentationContextRules(source.Executable)
	if err != nil {
		t.Fatal(err)
	}
	memory := make([]byte, 0x11280)
	selected := 0x76f4
	memory[selected], memory[selected+12], memory[selected+18], memory[selected+22], memory[selected+25] = 2, 1, 20, 2, 7
	binary.BigEndian.PutUint32(memory[0xf36:], uint32(selected))
	binary.BigEndian.PutUint32(memory[selected+26:], 0x123)
	read8 := func(at int) (uint8, error) { return memory[at], nil }
	read16 := func(at int) (uint16, error) { return binary.BigEndian.Uint16(memory[at:]), nil }
	read32 := func(at int) (uint32, error) { return binary.BigEndian.Uint32(memory[at:]), nil }
	write8 := func(at int, v uint8) error { memory[at] = v; return nil }
	write16 := func(at int, v uint16) error { binary.BigEndian.PutUint16(memory[at:], v); return nil }
	write32 := func(at int, v uint32) error { binary.BigEndian.PutUint32(memory[at:], v); return nil }
	plan, err := rules.Selected(populous2.NativePresentationContextCallbacks{Memory: populous2.FollowerCleanupMemory{Read8: read8, Read16: read16, Read32: read32, Write8: write8, Write16: write16, Write32: write32}}, &populous2.NativeHUDRegisters{})
	if err != nil {
		t.Fatal(err)
	}
	var sourcePoints []image.Point
	for _, sprite := range plan.Sprites {
		if sprite.Sprite == p.PopulationSprite {
			sourcePoints = append(sourcePoints, image.Pt(int(sprite.X), int(sprite.Y)))
		}
	}
	if !reflect.DeepEqual(sourcePoints, p.PopulationIndicators(0x123)) {
		t.Fatal("runtime population stamp positions differ from original selected renderer")
	}
	output := t.TempDir()
	if err := exportSelectedPanel(source, output); err != nil {
		t.Fatal(err)
	}
	loaded, err := visualassets.LoadSelectionPanel(os.DirFS(output))
	if err != nil || !reflect.DeepEqual(p, loaded) {
		t.Fatal("exported panel changed during loading")
	}
}
