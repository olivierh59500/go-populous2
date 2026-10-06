package visualassets

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"io/fs"
)

const (
	maxCatalogBytes = 8 << 20
	maxImageBytes   = 16 << 20
	maxImagePixels  = 4096 * 4096
	maxBundlePixels = 32 * 1024 * 1024
	maxTiles        = 1024
	maxSprites      = 4096
)

type imageLoader struct {
	files  fs.FS
	images map[string]image.Image
	pixels int
}

func readLimited(files fs.FS, path string, limit int64) ([]byte, error) {
	if !fs.ValidPath(path) || path == "." {
		return nil, fmt.Errorf("invalid visual asset path %q", path)
	}
	f, err := files.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("visual asset %q exceeds size limit", path)
	}
	return data, nil
}

func (l *imageLoader) image(path string) (image.Image, error) {
	if existing := l.images[path]; existing != nil {
		return existing, nil
	}
	data, err := readLimited(l.files, path, maxImageBytes)
	if err != nil {
		return nil, err
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("visual PNG %q: %w", path, err)
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > 4096 || config.Height > 4096 || config.Width > maxImagePixels/config.Height {
		return nil, fmt.Errorf("visual PNG %q has excessive dimensions", path)
	}
	pixels := config.Width * config.Height
	if pixels > maxBundlePixels-l.pixels {
		return nil, fmt.Errorf("visual bundle exceeds decoded pixel limit")
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("visual PNG %q: %w", path, err)
	}
	l.pixels += pixels
	l.images[path] = img
	return img, nil
}

func (l *imageLoader) rgba(path string) (*image.RGBA, error) {
	img, err := l.image(path)
	if err != nil {
		return nil, err
	}
	result := image.NewRGBA(img.Bounds())
	draw.Draw(result, result.Bounds(), img, img.Bounds().Min, draw.Src)
	return result, nil
}

func (l *imageLoader) region(region Region, optional bool) (*image.RGBA, error) {
	if optional && region == (Region{}) {
		return nil, nil
	}
	if region.Width <= 0 || region.Height <= 0 || region.Width > 320 || region.Height > 256 || region.X < 0 || region.Y < 0 || region.X > 4096-region.Width || region.Y > 4096-region.Height {
		return nil, fmt.Errorf("invalid artwork region %+v", region)
	}
	if region.AnchorX < -320 || region.AnchorX > 320 || region.AnchorY < -256 || region.AnchorY > 256 {
		return nil, fmt.Errorf("artwork anchor outside supported range")
	}
	atlas, err := l.image(region.Atlas)
	if err != nil {
		return nil, err
	}
	bounds := image.Rect(region.X, region.Y, region.X+region.Width, region.Y+region.Height)
	if !bounds.In(atlas.Bounds()) {
		return nil, fmt.Errorf("artwork region outside PNG %q", region.Atlas)
	}
	pixels := region.Width * region.Height
	if pixels > maxBundlePixels-l.pixels {
		return nil, fmt.Errorf("visual bundle exceeds extracted pixel limit")
	}
	l.pixels += pixels
	result := image.NewRGBA(image.Rect(0, 0, region.Width, region.Height))
	draw.Draw(result, result.Bounds(), atlas, bounds.Min, draw.Src)
	return result, nil
}

func loadFS(files fs.FS) (*Bundle, error) {
	if files == nil {
		return nil, fmt.Errorf("visual asset filesystem missing")
	}
	data, err := readLimited(files, CatalogName, maxCatalogBytes)
	if err != nil {
		return nil, fmt.Errorf("visual assets are missing: generate them with go run ./cmd/export-visual-assets: %w", err)
	}
	var catalog Catalog
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&catalog); err != nil {
		return nil, fmt.Errorf("visual catalog: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("visual catalog contains trailing JSON")
	}
	if catalog.Version != SchemaVersion {
		return nil, fmt.Errorf("unsupported visual catalog version %d", catalog.Version)
	}
	loader := imageLoader{files: files, images: make(map[string]image.Image)}
	b := &Bundle{Animations: catalog.Animations, StartupPalette: catalog.StartupPalette, TileRasters: catalog.TileRasters}
	for land, bank := range catalog.Landscapes {
		if len(bank.Tiles) == 0 || len(bank.Tiles) > maxTiles || len(bank.Sprites) == 0 || len(bank.Sprites) > maxSprites {
			return nil, fmt.Errorf("invalid landscape %d artwork count", land)
		}
		b.Palettes[land], b.MapColors[land] = bank.Palette, bank.MapColors
		b.Tiles[land] = make([]*image.RGBA, len(bank.Tiles))
		b.Sprites[land] = make([]Sprite, len(bank.Sprites))
		for index, region := range bank.Tiles {
			if b.Tiles[land][index], err = loader.region(region, false); err != nil {
				return nil, fmt.Errorf("landscape %d tile %d: %w", land, index, err)
			}
		}
		for index, region := range bank.Sprites {
			img, e := loader.region(region, true)
			if e != nil {
				return nil, fmt.Errorf("landscape %d sprite %d: %w", land, index, e)
			}
			b.Sprites[land][index] = Sprite{Image: img, AnchorX: region.AnchorX, AnchorY: region.AnchorY}
		}
	}
	if b.Background, err = loader.rgba(catalog.Background); err != nil {
		return nil, err
	}
	if b.Startup, err = loader.rgba(catalog.Startup); err != nil {
		return nil, err
	}
	for _, screen := range []*image.RGBA{b.Background, b.Startup} {
		if screen.Bounds().Dx() != 320 || screen.Bounds().Dy() != 200 {
			return nil, fmt.Errorf("visual screen must be 320x200")
		}
	}
	if catalog.Ending != "" {
		if b.Ending, err = loader.rgba(catalog.Ending); err != nil {
			return nil, err
		}
	}
	for land, entries := range catalog.StormStrikeArt {
		if len(entries) > 8 {
			return nil, fmt.Errorf("too many storm strike sprite families")
		}
		for _, entry := range entries {
			if entry.Sprite < 0 || entry.Sprite >= len(b.Sprites[land]) || len(entry.Heights) == 0 || len(entry.Heights) > 128 {
				return nil, fmt.Errorf("invalid shortened storm artwork")
			}
			art := ShortenedSpriteArt{Sprite: entry.Sprite, Heights: make([]*image.RGBA, len(entry.Heights))}
			for height, region := range entry.Heights {
				if region.Height != height+1 {
					return nil, fmt.Errorf("storm artwork height metadata differs")
				}
				if art.Heights[height], err = loader.region(region, false); err != nil {
					return nil, err
				}
			}
			b.StormStrikeArt[land] = append(b.StormStrikeArt[land], art)
		}
	}
	for part := range catalog.PortraitParts {
		for variant, region := range catalog.PortraitParts[part] {
			if b.PortraitParts[part][variant], err = loader.region(region, false); err != nil {
				return nil, err
			}
		}
	}
	if b.Font, err = loader.font(catalog.Font); err != nil {
		return nil, err
	}
	if b.EndingSequence, err = loader.ending(catalog.EndingSequence); err != nil {
		return nil, err
	}
	b.Towns = catalog.Towns
	if b.Towns != nil {
		if b.Towns.FlagHeight < 1 || b.Towns.FlagHeight > 128 {
			return nil, fmt.Errorf("invalid town flag height")
		}
		for stage, divisor := range b.Towns.PopulationDivisors {
			if divisor == 0 || len(b.Towns.Centers[stage].Layers) == 0 {
				return nil, fmt.Errorf("invalid town center metadata")
			}
		}
		validateLayers := func(frame Frame) error {
			if len(frame.Layers) > 32 {
				return fmt.Errorf("town artwork has excessive layers")
			}
			for _, layer := range frame.Layers {
				if layer.Sprite < 0 || layer.X < -512 || layer.X > 512 || layer.Y < -512 || layer.Y > 512 {
					return fmt.Errorf("invalid town artwork layer")
				}
				for _, bank := range b.Sprites {
					if layer.Sprite >= len(bank) {
						return fmt.Errorf("town sprite exceeds artwork bank")
					}
				}
			}
			return nil
		}
		for stage, center := range b.Towns.Centers {
			if err := validateLayers(center); err != nil {
				return nil, err
			}
			for _, frame := range b.Towns.Surroundings[stage] {
				if err := validateLayers(frame); err != nil {
					return nil, err
				}
			}
		}
		for _, offset := range b.Towns.Offsets {
			if offset[0] < -1 || offset[0] > 1 || offset[1] < -1 || offset[1] > 1 {
				return nil, fmt.Errorf("town adjacent offset exceeds its eight neighbours")
			}
		}
		for _, flags := range b.Towns.FlagSprites {
			for _, sprite := range flags {
				for _, bank := range b.Sprites {
					if sprite < 0 || sprite >= len(bank) {
						return nil, fmt.Errorf("town flag sprite missing")
					}
				}
			}
		}
	}
	if len(b.Animations) > 4096 {
		return nil, fmt.Errorf("too many visual animations")
	}
	for name, animation := range b.Animations {
		if name == "" || len(name) > 128 || len(animation.Frames) == 0 || len(animation.Frames) > 256 {
			return nil, fmt.Errorf("invalid visual animation %q", name)
		}
		for _, frame := range animation.Frames {
			if len(frame.Layers) == 0 || len(frame.Layers) > 256 || frame.SoundCue < 0 || frame.SoundCue >= 133 {
				return nil, fmt.Errorf("invalid visual frame in %q", name)
			}
			for _, layer := range frame.Layers {
				if layer.Sprite < 0 || layer.X < -512 || layer.X > 512 || layer.Y < -512 || layer.Y > 512 {
					return nil, fmt.Errorf("invalid visual layer in %q", name)
				}
				for _, bank := range b.Sprites {
					if layer.Sprite >= len(bank) {
						return nil, fmt.Errorf("visual sprite outside bank in %q", name)
					}
				}
			}
		}
	}
	b.FileCompatibility = catalog.FileCompatibility
	if b.FileCompatibility != nil {
		if len(b.FileCompatibility.AnimationTokens) > 65536 {
			return nil, fmt.Errorf("too many save animation aliases")
		}
		for _, entry := range b.FileCompatibility.AnimationTokens {
			animation, ok := b.Animations[entry.Animation]
			if !ok || entry.Frame < 0 || entry.Frame >= len(animation.Frames) {
				return nil, fmt.Errorf("save artwork token references an absent animation/frame")
			}
		}
	}
	return b, nil
}

func (l *imageLoader) font(desc FontDescriptor) (*Font, error) {
	if desc.FirstCode < 0 || desc.FirstCode > 255 || desc.Count <= 0 || desc.Count > 256-desc.FirstCode || desc.GlyphWidth <= 0 || desc.GlyphWidth > 32 || desc.GlyphHeight <= 0 || desc.GlyphHeight > 32 || desc.Columns <= 0 || desc.Columns > 256 {
		return nil, fmt.Errorf("invalid visual font geometry")
	}
	img, err := l.image(desc.Atlas)
	if err != nil {
		return nil, err
	}
	atlas, ok := img.(*image.Paletted)
	if !ok || len(atlas.Palette) > 16 {
		return nil, fmt.Errorf("visual font must be an indexed PNG with at most 16 colors")
	}
	width := desc.Columns * desc.GlyphWidth
	height := (desc.Count + desc.Columns - 1) / desc.Columns * desc.GlyphHeight
	if atlas.Bounds().Dx() != width || atlas.Bounds().Dy() != height {
		return nil, fmt.Errorf("visual font atlas dimensions differ")
	}
	font := &Font{FirstCode: desc.FirstCode, Width: desc.GlyphWidth, Height: desc.GlyphHeight, Glyphs: make([][]uint8, desc.Count)}
	for glyph := range font.Glyphs {
		pixels := make([]uint8, desc.GlyphWidth*desc.GlyphHeight)
		x, y := glyph%desc.Columns*desc.GlyphWidth, glyph/desc.Columns*desc.GlyphHeight
		for row := range desc.GlyphHeight {
			for col := range desc.GlyphWidth {
				index := atlas.ColorIndexAt(x+col, y+row)
				if index >= 16 {
					return nil, fmt.Errorf("visual font color outside palette")
				}
				pixels[row*desc.GlyphWidth+col] = index
			}
		}
		font.Glyphs[glyph] = pixels
	}
	return font, nil
}
