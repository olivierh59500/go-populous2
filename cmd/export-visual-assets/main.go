// export-visual-assets converts private original graphics into a portable
// image catalog. The original program is consulted only by this offline tool.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	catalog.TileRasters = source.RenewNative.Raster
	catalog.Towns = &visualassets.TownArt{PopulationDivisors: source.TownCenterArt.PopulationDivisors, FlagSprites: [2][2]int{{89, 90}, {91, 92}}, FlagHeight: 24}
	for i, offset := range source.TownEvaluator.OverlayOffsets {
		x := int(int8(uint8(offset)))
		catalog.Towns.Offsets[i] = [2]int{x, (int(int16(offset)) - x) / 256}
	}
	for stage, center := range source.TownCenterArt.Frames {
		catalog.Towns.Centers[stage] = animation([]populous2.AnimationFrame{center}, false).Frames[0]
		for layer := range catalog.Towns.Centers[stage].Layers {
			catalog.Towns.Centers[stage].Layers[layer].Y += 8
		}
		for neighbor, code := range source.TownEvaluator.StructureTiles[stage] {
			if code == 0 {
				continue
			}
			catalog.Towns.Surroundings[stage][neighbor] = animation([]populous2.AnimationFrame{source.TownEvaluator.OverlayFrames[code]}, false).Frames[0]
		}
	}
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
	catalog.EndingSequence, err = exportEndingFrames(output, source.Raw["end.pak"], string(presentation.EndingText))
	if err != nil {
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
	for name, start := range map[string]int{
		"death/swamp":          source.FungusHazards.OrdinaryAnimation,
		"death/fungus":         source.FungusHazards.OrdinaryAnimation,
		"death/fire":           0x178,
		"death/burning":        0x564,
		"lightning/appearing":  0x6e0,
		"lightning/active":     0x6f8,
		"lightning/ending":     0x720,
		"lightning/hit":        0x738,
		"lightning/town-hit":   0x744,
		"lightning/recovery":   0x750,
		"whirlwind/appearing":  0x4c8,
		"whirlwind/active":     0x4c8,
		"whirlwind/ending":     0x6cc,
		"airborne/landing":     0x68c,
		"scenery/burning-tree": 0xf10,
		"fire-column/emerging": 0x1a0,
		"fire-column/active":   0x4b8,
		"fire-column/ending":   0x660,
		"meteor/falling":       0x81c,
		"fire-impact/land":     0x49c,
		"fire-impact/water":    0x5ec,
		"lava/north":           0xec8,
		"lava/east":            0xed4,
		"lava/south":           0xec8,
		"lava/west":            0xed4,
		"lava/slope-3":         0xeec,
		"lava/slope-6":         0xef8,
		"lava/slope-9":         0xee0,
		"lava/slope-12":        0xf04,
	} {
		frames, err := populous2.DecodeAnimation(source.Executable, start)
		if err != nil {
			return err
		}
		catalog.Animations[name] = animation(frames, name == "fire-column/active" || name == "death/burning" || name == "lightning/active" || name == "lightning/hit" || name == "lightning/town-hit" || name == "whirlwind/active" || strings.HasPrefix(name, "lava/"))
	}
	airborneRules, err := populous2.DecodeWhirlwindFollowerRules(source.Executable)
	if err != nil {
		return err
	}
	for role, start := range map[string]int{"airborne/follower": 0x4d4, "airborne/perseus": source.Whirlwinds.PickupAnimations[0], "airborne/adonis": source.Whirlwinds.PickupAnimations[1], "airborne/heracles": source.Whirlwinds.PickupAnimations[2], "airborne/achilles": source.Whirlwinds.PickupAnimations[4], "airborne/helen": source.Whirlwinds.PickupAnimations[5]} {
		frames := make([]populous2.AnimationFrame, airborneRules.SequenceLengths[start])
		for i := range frames {
			frames[i] = airborneRules.Frames[start+i*4]
		}
		catalog.Animations[role] = animation(frames, true)
	}
	for hero, name := range heroes {
		for _, entry := range []struct {
			name  string
			start int
			loop  bool
		}{{"lightning/hit/", source.LightningRules.HeroHit[hero], true}, {"lightning/recovery/", source.LightningRules.HeroRecovery[hero], false}} {
			if entry.start == 0 {
				continue
			}
			frames, err := populous2.DecodeAnimation(source.Executable, entry.start)
			if err != nil {
				return err
			}
			catalog.Animations[entry.name+name] = animation(frames, entry.loop)
		}
		if start := source.FireColumns.HeroDeath[hero]; start != 0 {
			frames, err := populous2.DecodeAnimation(source.Executable, start)
			if err != nil {
				return err
			}
			catalog.Animations["death/fire/"+name] = animation(frames, false)
		}
		if start := source.LavaRules.Burning[hero]; start != 0 {
			frames, err := populous2.DecodeAnimation(source.Executable, int(start))
			if err != nil {
				return err
			}
			catalog.Animations["death/burning/"+name] = animation(frames, true)
		}
		if start := source.FungusHazards.HeroDeath[hero]; start != 0 {
			frames, err := populous2.DecodeAnimation(source.Executable, start)
			if err != nil {
				return err
			}
			catalog.Animations["death/fungus/"+name] = animation(frames, false)
		}
		if start := source.CommonPrepass.Swamp[hero]; start != 0 {
			frames, err := populous2.DecodeAnimation(source.Executable, int(start))
			if err != nil {
				return err
			}
			catalog.Animations["death/swamp/"+name] = animation(frames, false)
		}
	}
	for stage, start := range source.FireColumns.TownDeath {
		frames, err := populous2.DecodeAnimation(source.Executable, start)
		if err != nil {
			return err
		}
		catalog.Animations[fmt.Sprintf("ruin/town/%d", stage)] = animation(frames, false)
	}
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
	for direction, name := range [...]string{"north", "east", "south", "west"} {
		frames, err := populous2.DecodeAnimation(source.Executable, int(source.TsunamiRules.Animations[direction]))
		if err != nil {
			return err
		}
		catalog.Animations["tidal/"+name] = animation(frames, true)
	}
	for connection, art := range source.WallRules.Art {
		frames, err := populous2.DecodeAnimation(source.Executable, art.Animation)
		if err != nil {
			return err
		}
		catalog.Animations[fmt.Sprintf("wall/connection/%d", connection)] = animation(frames, false)
	}
	for name, start := range map[string]int{"wall/post": 0x5bc, "wall/gate-horizontal": 0xb44, "wall/gate-vertical": 0xb54, "wall/attack-blue": 0x7cc, "wall/attack-red": 0x7d4} {
		frames, err := populous2.DecodeAnimation(source.Executable, start)
		if err != nil {
			return err
		}
		catalog.Animations[name] = animation(frames, false)
	}
	for variant, start := range source.WallRules.BreakAnimations {
		frames, err := populous2.DecodeAnimation(source.Executable, start)
		if err != nil {
			return err
		}
		catalog.Animations[fmt.Sprintf("wall/broken/%d", variant*2)] = animation(frames, false)
	}
	for name, start := range map[string]int{"conversion/blue": 0xbd8, "conversion/red": 0xc0c, "conversion/hero": 0xc0c} {
		frames, err := populous2.DecodeAnimation(source.Executable, start)
		if err != nil {
			return err
		}
		catalog.Animations[name] = animation(frames, false)
	}
	for name, start := range map[string]int{
		"neutral/road-maker": 0x2cc, "neutral/land-lowerer": 0x53c, "neutral/land-lowerer-paired": 0x550,
		"neutral/whirlwind-maker": 0xa98, "neutral/tree-planter": 0xab4, "neutral/fire-maker": 0x2bfc, "neutral/monster": 0x2c18, "neutral/monster-victim": 0x2c34,
	} {
		frames, err := populous2.DecodeAnimation(source.Executable, start)
		if err != nil {
			return err
		}
		catalog.Animations[name] = animation(frames, true)
	}
	for name, start := range map[string]int{"combat/attack": 0x1c8, "combat/death": 0x1e74, "combat/hero-death": 0x9d4, "combat/victory-blue": 0x1b2c, "combat/victory-red": 0x1cd0, "death/water": 0x196c, "death/fatal": 0x7dc} {
		frames, err := populous2.DecodeAnimation(source.Executable, start)
		if err != nil {
			return err
		}
		catalog.Animations[name] = animation(frames, name == "combat/attack")
	}
	for hero, name := range heroes {
		for role, table := range map[string][6]uint16{"death/water": source.CommonPrepass.Swimming, "death/fatal": source.CommonPrepass.Fatal} {
			start := table[hero]
			if start == 0 {
				continue
			}
			frames, err := populous2.DecodeAnimation(source.Executable, int(start))
			if err != nil {
				return err
			}
			catalog.Animations[role+"/"+name] = animation(frames, false)
		}
	}
	for name, start := range map[string]int{"swimming/follower": 0x7dc} {
		frames, err := populous2.DecodeAnimation(source.Executable, start)
		if err != nil {
			return err
		}
		catalog.Animations[name] = animation(frames, true)
	}
	for hero, name := range heroes {
		start := source.CommonPrepass.Drowning[hero]
		if start == 0 {
			continue
		}
		frames, err := populous2.DecodeAnimation(source.Executable, int(start))
		if err != nil {
			return err
		}
		catalog.Animations["swimming/"+name] = animation(frames, true)
	}
	catalog.FileCompatibility = exportSaveCompatibility(source, catalog)
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

func exportSaveCompatibility(source *populous2.Bundle, catalog visualassets.Catalog) *visualassets.FileCompatibility {
	result := &visualassets.FileCompatibility{}
	add := func(name string, start int) {
		for frame := range catalog.Animations[name].Frames {
			result.AnimationTokens = append(result.AnimationTokens, visualassets.SaveAnimationToken{Token: uint16(start + frame*4), Animation: name, Frame: frame})
		}
	}
	for connection, art := range source.WallRules.Art {
		add(fmt.Sprintf("wall/connection/%d", connection), art.Animation)
	}
	for variant, start := range source.WallRules.BreakAnimations {
		add(fmt.Sprintf("wall/broken/%d", variant*2), start)
	}
	add("wall/post", 0x5bc)
	add("wall/gate-horizontal", 0xb44)
	add("wall/gate-vertical", 0xb54)
	for kind, rules := range map[string]populous2.SceneryRules{"tree": source.Scenery.Trees, "boulder": source.Scenery.Boulders} {
		for variant, start := range rules.Animations {
			add(fmt.Sprintf("scenery/%s/%d", kind, variant), start)
		}
	}
	add("scenery/burning-tree", 0xf10)
	add("scenery/removal-start", source.Scenery.RemovalStart)
	add("scenery/removal-end", source.Scenery.RemovalEnd)
	for side, variants := range source.FollowerMotion.VariantBases {
		for variant, start := range variants {
			for direction, offset := range source.FollowerMotion.DirectionOffsets {
				add(fmt.Sprintf("follower/%d/%d/%d", side, variant, direction), start+offset)
			}
		}
	}
	heroes := [6]string{"perseus", "adonis", "heracles", "odysseus", "achilles", "helen"}
	for hero, start := range source.HeroRules.Variants {
		offset := int(start)
		for direction, frames := range source.HeroRules.Art[hero].Directions {
			add(fmt.Sprintf("hero/%s/%d", heroes[hero], direction), offset)
			offset += (len(frames) + 1) * 4
		}
	}
	for name, start := range map[string]int{"neutral/road-maker": 0x2cc, "neutral/land-lowerer": 0x53c, "neutral/land-lowerer-paired": 0x550, "neutral/whirlwind-maker": 0xa98, "neutral/tree-planter": 0xab4, "neutral/fire-maker": 0x2bfc, "neutral/monster": 0x2c18, "neutral/monster-victim": 0x2c34, "conversion/blue": 0xbd8, "conversion/red": 0xc0c, "conversion/hero": 0xc0c, "death/swamp": 0x7dc, "death/fungus": 0x7dc, "plague": 0xddc} {
		add(name, start)
	}
	sort.Slice(result.AnimationTokens, func(i, j int) bool {
		a, b := result.AnimationTokens[i], result.AnimationTokens[j]
		if a.Token != b.Token {
			return a.Token < b.Token
		}
		if a.Animation != b.Animation {
			return a.Animation < b.Animation
		}
		return a.Frame < b.Frame
	})
	return result
}

func exportEndingFrames(output string, art []byte, text string) (*visualassets.EndingDescriptor, error) {
	// The ending caller prepares its two drawing buffers before the first
	// visible update. Export that caller's exact artwork, not the standalone
	// animation resource reader's different initial swap convention.
	presentation := &populous2.NativePresentation{Font: &populous2.NativeMenuFont{}, EndingText: []byte{' '}}
	ending, err := populous2.NewNativeEnding(art, presentation, 0)
	if err != nil {
		return nil, err
	}
	desc := &visualassets.EndingDescriptor{Text: text, IntroWait: 1, FrameWait: 4, TextStepFrames: 2}
	seen := make(map[string]int)
	if err := os.MkdirAll(filepath.Join(output, "ending"), 0755); err != nil {
		return nil, err
	}
	for frame := 0; frame < 256; frame++ {
		a := ending.Animation
		// State is used only to find the artwork's complete repeating cycle;
		// exported metadata contains frame indices and ordinary PNG paths.
		key := fmt.Sprintf("%d/%d/%x", a.Code, a.Cursor, sha256.Sum256(a.Planes[:]))
		if start, exists := seen[key]; exists {
			desc.LoopStart = start
			return desc, nil
		}
		seen[key] = frame
		img, err := a.Image()
		if err != nil {
			return nil, err
		}
		name := fmt.Sprintf("ending/frame-%03d.png", frame)
		if err := writePNG(output, name, img); err != nil {
			return nil, err
		}
		desc.Frames = append(desc.Frames, name)
		if err := ending.Advance(); err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("ending artwork does not loop within supported frame limit")
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
