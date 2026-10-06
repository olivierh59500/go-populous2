package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func TestAtlasPackingPreservesPixelsAndBlankSlots(t *testing.T) {
	first := image.NewRGBA(image.Rect(3, 4, 35, 6))
	first.SetRGBA(3, 4, color.RGBA{R: 255, A: 255})
	last := image.NewRGBA(image.Rect(0, 0, 32, 20))
	last.SetRGBA(31, 19, color.RGBA{B: 255, A: 255})
	images := make([]*image.RGBA, 18)
	for i := range images {
		images[i] = first
	}
	images[1] = nil
	images[17] = last
	anchors := make([]image.Point, len(images))
	anchors[17] = image.Pt(16, 20)
	atlas, regions, err := pack("atlas.png", images, anchors)
	if err != nil {
		t.Fatal(err)
	}
	if regions[1].Atlas != "" || regions[1].Width != 0 {
		t.Fatal("blank sprite slot acquired artwork")
	}
	region := regions[17]
	if region.Y != 2 || region.AnchorX != 16 || region.AnchorY != 20 {
		t.Fatalf("row packing or anchors changed: %+v", region)
	}
	if atlas.RGBAAt(region.X+31, region.Y+19).B != 255 || atlas.RGBAAt(0, 0).R != 255 {
		t.Fatal("atlas lost pixels or mishandled nonzero image origin")
	}
}

// Original data are optional and excluded from the source repository.
func TestPrivateArtworkExportRoundTrip(t *testing.T) {
	input := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if input == "" {
		t.Skip("set POPULOUS2_EXPORT_TEST_DIR for private original-art comparison")
	}
	files := os.DirFS(input)
	source, err := populous2.LoadFS(files)
	if err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	if err := export(files, output); err != nil {
		t.Fatal(err)
	}
	portable, err := visualassets.LoadFS(os.DirFS(output))
	if err != nil {
		t.Fatal(err)
	}
	for bank := range source.Tiles {
		for index, want := range source.Tiles[bank] {
			got := portable.Tiles[bank][index]
			if got.Bounds() != want.Bounds() || !bytes.Equal(got.Pix, want.Pix) {
				t.Fatalf("tile %d/%d changed", bank, index)
			}
		}
		for index, want := range source.Sprites[bank] {
			got := portable.Sprites[bank][index]
			if want.Image == nil {
				if got.Image != nil {
					t.Fatalf("empty sprite %d/%d changed", bank, index)
				}
				continue
			}
			if got.AnchorX != want.AnchorX || got.AnchorY != want.AnchorY || got.Image.Bounds() != want.Image.Bounds() || !bytes.Equal(got.Image.Pix, want.Image.Pix) {
				t.Fatalf("sprite %d/%d changed", bank, index)
			}
		}
	}
	font, err := populous2.DecodeNativeMenuFont(source.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for index, glyph := range font.Glyphs {
		if !bytes.Equal(portable.Font.Glyphs[index], glyph[:]) {
			t.Fatalf("font glyph %d changed", index)
		}
	}
	for part, variants := range source.DeityArt.Parts {
		for variant, want := range variants {
			if got := portable.PortraitParts[part][variant]; got.Bounds() != want.Bounds() || !bytes.Equal(got.Pix, want.Pix) {
				t.Fatalf("portrait strip %d/%d changed", part, variant)
			}
		}
	}
	for side := range source.FollowerMotion.VariantBases {
		for variant := range source.FollowerMotion.VariantBases[side] {
			for _, direction := range compass {
				index := source.FollowerMotion.Angle(int16(direction.delta.X*20), int16(direction.delta.Y*20)) >> 5
				name := fmt.Sprintf("follower/%d/%d/%s", side, variant, direction.name)
				want := portable.Animations[fmt.Sprintf("follower/%d/%d/%d", side, variant, index)]
				if got := portable.Animations[name]; !reflect.DeepEqual(got, want) || len(got.Frames) == 0 {
					t.Fatalf("wrong named compass animation %s", name)
				}
			}
		}
	}
	for name, start := range map[string]int{"death/fungus": source.FungusHazards.OrdinaryAnimation, "death/swamp": source.FungusHazards.OrdinaryAnimation, "death/fire": 0x178, "death/burning": 0x564, "fire-column/emerging": 0x1a0, "fire-column/active": 0x4b8, "fire-column/ending": 0x660, "lightning/appearing": 0x6e0, "lightning/active": 0x6f8, "lightning/ending": 0x720, "lightning/hit": 0x738, "lightning/town-hit": 0x744, "lightning/recovery": 0x750} {
		frames, err := populous2.DecodeAnimation(source.Executable, start)
		if err != nil {
			t.Fatal(err)
		}
		want := animation(frames, name == "fire-column/active" || name == "death/burning" || name == "lightning/active" || name == "lightning/hit" || name == "lightning/town-hit")
		if !reflect.DeepEqual(portable.Animations[name], want) {
			t.Fatalf("semantic phase artwork changed: %s", name)
		}
	}
	for stage, start := range source.FireColumns.TownDeath {
		frames, err := populous2.DecodeAnimation(source.Executable, start)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(portable.Animations[fmt.Sprintf("ruin/town/%d", stage)], animation(frames, false)) {
			t.Fatalf("town ruin artwork changed at stage %d", stage)
		}
	}
	for connection, art := range source.WallRules.Art {
		frames, err := populous2.DecodeAnimation(source.Executable, art.Animation)
		if err != nil {
			t.Fatal(err)
		}
		name := fmt.Sprintf("wall/connection/%d", connection)
		if !reflect.DeepEqual(portable.Animations[name], animation(frames, false)) {
			t.Fatal("wall artwork differs", name)
		}
	}
	for name, start := range map[string]int{"combat/attack": 0x1c8, "combat/death": 0x1e74, "combat/hero-death": 0x9d4, "combat/victory-blue": 0x1b2c, "combat/victory-red": 0x1cd0, "death/water": 0x196c, "death/fatal": 0x7dc} {
		frames, err := populous2.DecodeAnimation(source.Executable, start)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(portable.Animations[name], animation(frames, name == "combat/attack")) {
			t.Fatal("combat/terrain artwork changed", name)
		}
	}
	if portable.TileRasters != source.RenewNative.Raster {
		t.Fatal("tile surface metadata changed during import")
	}
	if portable.FileCompatibility == nil || len(portable.FileCompatibility.AnimationTokens) == 0 {
		t.Fatal("save artwork compatibility aliases missing")
	}
	for _, entry := range portable.FileCompatibility.AnimationTokens {
		// Each token is a record value from the historical save-file format.
		// Its referenced image/cue entry is art metadata, not executable code.
		if strings.HasPrefix(entry.Animation, "hero/") {
			// Hero walking uses its own validated artwork decoder; a shipped
			// cue in this bank lies beyond the generic effect cue catalogue.
			parts := strings.Split(entry.Animation, "/")
			direction := 0
			if _, err := fmt.Sscan(parts[2], &direction); err != nil {
				t.Fatal(err)
			}
			heroNames := [6]string{"perseus", "adonis", "heracles", "odysseus", "achilles", "helen"}
			hero := -1
			for i, name := range heroNames {
				if name == parts[1] {
					hero = i
				}
			}
			if hero < 0 || direction < 0 || direction > 7 {
				t.Fatal("bad hero file alias")
			}
			want := animation([]populous2.AnimationFrame{{Layers: source.HeroRules.Art[hero].Directions[direction][entry.Frame]}}, false).Frames[0]
			if !reflect.DeepEqual(portable.Animations[entry.Animation].Frames[entry.Frame], want) {
				t.Fatal("hero save artwork alias differs", entry)
			}
			continue
		}
		frames, err := populous2.DecodeAnimation(source.Executable, int(entry.Token)-entry.Frame*4)
		if err != nil || entry.Frame >= len(frames) {
			t.Fatal("save artwork token outside original sequence", entry)
		}
		want := animation(frames, false).Frames[entry.Frame]
		if !reflect.DeepEqual(portable.Animations[entry.Animation].Frames[entry.Frame], want) {
			t.Fatal("save artwork alias differs from source frame", entry)
		}
	}
	for name, start := range map[string]int{"neutral/road-maker": 0x2cc, "neutral/land-lowerer": 0x53c, "neutral/land-lowerer-paired": 0x550, "neutral/whirlwind-maker": 0xa98, "neutral/tree-planter": 0xab4, "neutral/fire-maker": 0x2bfc, "neutral/monster": 0x2c18, "neutral/monster-victim": 0x2c34} {
		frames, err := populous2.DecodeAnimation(source.Executable, start)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(portable.Animations[name], animation(frames, true)) {
			t.Fatal("neutral artwork import changed", name)
		}
	}
	for stage := range source.TownCenterArt.Frames {
		for owner := range 2 {
			for _, population := range []uint32{0, 1, 100, 4000, 1 << 24} {
				for tick := range uint16(2) {
					want, ok := source.TownCenterArt.Frame(stage, uint8(owner+1), population, tick)
					if !ok || !reflect.DeepEqual(portable.Towns.CenterLayers(stage, owner, population, uint64(tick)), animation([]populous2.AnimationFrame{want}, false).Frames[0].Layers) {
						t.Fatal("town population artwork differs", stage, owner, population, tick, portable.Towns.CenterLayers(stage, owner, population, uint64(tick)), want.Layers)
					}
				}
			}
		}
		for neighbor, code := range source.TownEvaluator.StructureTiles[stage] {
			if code == 0 {
				continue
			}
			want := animation([]populous2.AnimationFrame{source.TownEvaluator.OverlayFrames[code]}, false).Frames[0]
			if !reflect.DeepEqual(portable.Towns.Surroundings[stage][neighbor], want) {
				t.Fatal("town adjacent artwork differs", stage, neighbor)
			}
		}
	}
	if portable.EndingSequence == nil || len(portable.EndingSequence.Frames) != 28 || portable.EndingSequence.LoopStart != 12 {
		t.Fatal("complete ending artwork sequence missing")
	}
	ending, err := populous2.NewNativeEnding(source.Raw["end.pak"], &populous2.NativePresentation{Font: &populous2.NativeMenuFont{}, EndingText: []byte{' '}}, 0)
	if err != nil {
		t.Fatal(err)
	}
	for update := 0; update < 50; update++ {
		frame := update
		if frame >= 28 {
			frame = 12 + (frame-12)%16
		}
		original, err := ending.Image()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(original.Pix, portable.EndingSequence.Frames[frame].Pix) {
			t.Fatal("ending frame pixels differ", update)
		}
		if err := ending.Advance(); err != nil {
			t.Fatal(err)
		}
	}
	for land := range 4 {
		bank, err := populous2.DecodeNativeSpriteBitmapBank(source, land)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []int{396, 397} {
			sprite := bank.Sprites[id]
			for height := 1; height <= sprite.Height; height++ {
				want, err := populous2.DecodeNativeMaskedPlanes(sprite.Planes[:sprite.Width/8*5*height], sprite.Width, height, source.Landscapes[land].Palettes[0])
				if err != nil {
					t.Fatal(err)
				}
				got := portable.StormStrikeImage(land, id, height)
				if got == nil || !bytes.Equal(got.Pix, want.Pix) {
					t.Fatal("shortened storm artwork pixels differ", land, id, height)
				}
			}
		}
	}
	for name, start := range map[string]int{"storm/cloud": 0xce4, "storm/strike": 0xd30} {
		frames, err := populous2.DecodeAnimation(source.Executable, start)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(portable.Animations[name], animation(frames, true)) {
			t.Fatal("storm artwork differs", name)
		}
	}
	for _, path := range []string{"populous.ii", "POPULOUS.II", "code.bin"} {
		if _, err := os.Stat(filepath.Join(output, path)); !os.IsNotExist(err) {
			t.Fatalf("unexpected game program in portable installation: %s", path)
		}
	}
}
		if strings.HasPrefix(entry.Animation,"contact/")||entry.Animation=="follower/waiting"{
			want:=source.FollowerEntry.WaitingFrames[int(entry.Token)];want.SoundCue=0
			if !reflect.DeepEqual(portable.Animations[entry.Animation].Frames[entry.Frame],animation([]populous2.AnimationFrame{want},false).Frames[0]){t.Fatal("silent contact artwork alias differs",entry)}
			continue
		}
