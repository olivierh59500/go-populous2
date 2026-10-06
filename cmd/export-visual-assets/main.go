// export-visual-assets converts private original graphics into a portable
// image catalog. The original program is consulted only by this offline tool.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"

	embedded "go-populous2/assets"
	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

var compass = [...]struct {
	name  string
	delta image.Point
}{
	{"north", image.Pt(0, -1)}, {"northeast", image.Pt(1, -1)},
	{"east", image.Pt(1, 0)}, {"southeast", image.Pt(1, 1)},
	{"south", image.Pt(0, 1)}, {"southwest", image.Pt(-1, 1)},
	{"west", image.Pt(-1, 0)}, {"northwest", image.Pt(-1, -1)},
}

func main() {
	input := flag.String("input", "", "private imported original files (defaults to POPULOUS2_DATA_DIR or embedded installation)")
	output := flag.String("output", "assets/generated", "portable artwork output directory")
	flag.Parse()
	var files fs.FS
	var err error
	if *input != "" {
		files = os.DirFS(*input)
	} else {
		files, err = embedded.DataFS()
	}
	if err == nil {
		err = export(files, *output)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Portable artwork, campaign and landscape data written to %s\n", *output)
}

func export(files fs.FS, output string) error {
	if output == "" {
		return fmt.Errorf("an output directory is required")
	}
	source, err := populous2.LoadFS(files)
	if err != nil {
		return err
	}
	presentation, err := populous2.DecodeNativePresentation(source.Executable)
	if err != nil {
		return err
	}
	catalog := visualassets.Catalog{Version: visualassets.SchemaVersion, Background: "background.png", Startup: "startup.png", Ending: "ending.png", Animations: make(map[string]visualassets.Animation)}
	catalog.StartupPalette = presentation.StartupPalette
	if err := os.MkdirAll(output, 0755); err != nil {
		return err
	}
	for land := range source.Tiles {
		name := fmt.Sprintf("tiles-%d.png", land)
		atlas, regions, err := pack(name, source.Tiles[land], nil)
		if err != nil {
			return err
		}
		if err := writePNG(output, name, atlas); err != nil {
			return err
		}
		catalog.Landscapes[land].Tiles = regions
		images := make([]*image.RGBA, len(source.Sprites[land]))
		anchors := make([]image.Point, len(images))
		for index, sprite := range source.Sprites[land] {
			images[index], anchors[index] = sprite.Image, image.Pt(sprite.AnchorX, sprite.AnchorY)
		}
		name = fmt.Sprintf("sprites-%d.png", land)
		atlas, regions, err = pack(name, images, anchors)
		if err != nil {
			return err
		}
		if err := writePNG(output, name, atlas); err != nil {
			return err
		}
		catalog.Landscapes[land].Sprites = regions
		catalog.Landscapes[land].Palette = source.Landscapes[land].Palettes[0]
		for code, index := range source.Landscapes[land].MapColor {
			if index >= 16 {
				return fmt.Errorf("landscape %d map color %d outside palette", land, index)
			}
			catalog.Landscapes[land].MapColors[code] = source.Landscapes[land].Palettes[0][index]
		}
		if err := os.WriteFile(filepath.Join(output, fmt.Sprintf("land%d.dat", land)), source.Raw[fmt.Sprintf("land%d.dat", land)], 0644); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(output, "campaign.dat"), source.Raw["conquest.pak"], 0644); err != nil {
		return err
	}
	if err := writePNG(output, catalog.Background, source.Background); err != nil {
		return err
	}
	startup := image.NewRGBA(image.Rect(0, 0, 320, 200))
	for y := range 200 {
		for x := range 320 {
			startup.SetRGBA(x, y, presentation.StartupPalette[presentation.StartupPixels[x+y*320]])
		}
	}
	if err := writePNG(output, catalog.Startup, startup); err != nil {
		return err
	}
	endingAnimation, err := populous2.NewNativeScreenAnimation(source.Raw["end.pak"])
	if err != nil {
		return err
	}
	ending, err := endingAnimation.Image()
	if err != nil {
		return err
	}
	if err := writePNG(output, catalog.Ending, ending); err != nil {
		return err
	}
	catalog.Font = visualassets.FontDescriptor{Atlas: "font.png", FirstCode: populous2.NativeGlyphFirst, Count: populous2.NativeGlyphCount, GlyphWidth: 8, GlyphHeight: 8, Columns: 16}
	if err := writePNG(output, catalog.Font.Atlas, presentation.Font.Atlas(presentation.StartupPalette)); err != nil {
		return err
	}
	portraits := make([]*image.RGBA, 0, 24)
	for _, parts := range source.DeityArt.Parts {
		portraits = append(portraits, parts[:]...)
	}
	atlas, regions, err := pack("portraits.png", portraits, nil)
	if err != nil {
		return err
	}
	if err := writePNG(output, "portraits.png", atlas); err != nil {
		return err
	}
	for part := range catalog.PortraitParts {
		copy(catalog.PortraitParts[part][:], regions[part*8:(part+1)*8])
	}
	for side := range source.FollowerMotion.VariantBases {
		for variant, base := range source.FollowerMotion.VariantBases[side] {
			for direction, offset := range source.FollowerMotion.DirectionOffsets {
				frames := make([]populous2.AnimationFrame, source.FollowerMotion.FrameCount)
				for index := range frames {
					frames[index] = source.FollowerMotion.Frames[base+offset+index*4]
				}
				catalog.Animations[fmt.Sprintf("follower/%d/%d/%d", side, variant, direction)] = animation(frames, true)
			}
			for _, direction := range compass {
				index := source.FollowerMotion.Angle(int16(direction.delta.X*20), int16(direction.delta.Y*20)) >> 5
				catalog.Animations[fmt.Sprintf("follower/%d/%d/%s", side, variant, direction.name)] = catalog.Animations[fmt.Sprintf("follower/%d/%d/%d", side, variant, index)]
			}
		}
	}
	heroes := [...]string{"perseus", "adonis", "heracles", "odysseus", "achilles", "helen"}
	for hero, art := range source.HeroRules.Art {
		for direction, sequence := range art.Directions {
			frames := make([]populous2.AnimationFrame, len(sequence))
			for index, layers := range sequence {
				frames[index].Layers = layers
			}
			catalog.Animations[fmt.Sprintf("hero/%s/%d", heroes[hero], direction)] = animation(frames, true)
		}
		for _, direction := range compass {
			index := source.FollowerMotion.Angle(int16(direction.delta.X*20), int16(direction.delta.Y*20)) >> 5
			catalog.Animations[fmt.Sprintf("hero/%s/%s", heroes[hero], direction.name)] = catalog.Animations[fmt.Sprintf("hero/%s/%d", heroes[hero], index)]
		}
	}
	for kind, rules := range map[string]populous2.SceneryRules{"tree": source.Scenery.Trees, "boulder": source.Scenery.Boulders} {
		for variant, offset := range rules.Animations {
			catalog.Animations[fmt.Sprintf("scenery/%s/%d", kind, variant)] = animation(source.Scenery.Frames[offset], true)
		}
	}
	catalog.Animations["scenery/removal-start"] = animation(source.Scenery.Frames[source.Scenery.RemovalStart], false)
	catalog.Animations["scenery/removal-end"] = animation(source.Scenery.Frames[source.Scenery.RemovalEnd], false)
	catalog.Animations["plague"] = animation(source.PlagueAnimation, true)
	for side := range 2 {
		for stage := range source.TownCenterArt.Frames {
			frame, ok := source.TownCenterArt.Frame(stage, uint8(side+1), 0, 0)
			if !ok {
				return fmt.Errorf("town artwork missing")
			}
			catalog.Animations[fmt.Sprintf("town/%d/%d", side, stage)] = animation([]populous2.AnimationFrame{frame}, false)
		}
		frames, err := populous2.DecodeAnimation(source.Executable, int(source.MagnetRules.Animations[side+1]))
		if err != nil {
			return err
		}
		catalog.Animations[fmt.Sprintf("magnet/%d", side)] = animation(frames, true)
	}
	data, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(output, visualassets.CatalogName), append(data, '\n'), 0644); err != nil {
		return err
	}
	// Re-load the emitted files through the independent runtime decoder.
	_, err = visualassets.LoadFS(os.DirFS(output))
	return err
}

func animation(frames []populous2.AnimationFrame, loop bool) visualassets.Animation {
	result := visualassets.Animation{Frames: make([]visualassets.Frame, len(frames)), Loop: loop}
	for index, frame := range frames {
		result.Frames[index].SoundCue = frame.SoundCue
		result.Frames[index].Layers = make([]visualassets.SpriteLayer, len(frame.Layers))
		for layer, sprite := range frame.Layers {
			result.Frames[index].Layers[layer] = visualassets.SpriteLayer{Sprite: sprite.Sprite, X: sprite.X, Y: sprite.Y}
		}
	}
	return result
}

func pack(name string, images []*image.RGBA, anchors []image.Point) (*image.RGBA, []visualassets.Region, error) {
	const width = 512
	regions := make([]visualassets.Region, len(images))
	x, y, rowHeight := 0, 0, 0
	for index, img := range images {
		if img == nil {
			continue
		}
		w, h := img.Bounds().Dx(), img.Bounds().Dy()
		if w <= 0 || w > width || h <= 0 || h > 256 {
			return nil, nil, fmt.Errorf("invalid artwork geometry")
		}
		if x+w > width {
			x, y, rowHeight = 0, y+rowHeight, 0
		}
		region := visualassets.Region{Atlas: name, X: x, Y: y, Width: w, Height: h}
		if anchors != nil {
			region.AnchorX, region.AnchorY = anchors[index].X, anchors[index].Y
		}
		regions[index] = region
		x += w
		rowHeight = max(rowHeight, h)
	}
	height := max(1, y+rowHeight)
	if height > 4096 {
		return nil, nil, fmt.Errorf("artwork atlas too tall")
	}
	atlas := image.NewRGBA(image.Rect(0, 0, width, height))
	for index, img := range images {
		if img == nil {
			continue
		}
		region := regions[index]
		draw.Draw(atlas, image.Rect(region.X, region.Y, region.X+region.Width, region.Y+region.Height), img, img.Bounds().Min, draw.Src)
	}
	return atlas, regions, nil
}

func writePNG(output, name string, img image.Image) error {
	f, err := os.Create(filepath.Join(output, name))
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
