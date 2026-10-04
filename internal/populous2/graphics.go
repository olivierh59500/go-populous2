package populous2

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"go-populous2/internal/amiga"
)

const (
	TileWidth    = 32
	TileHeight   = 24
	TileCount    = 255
	TownStages   = 19
	LandDataSize = 556
)

type Landscape struct {
	ManaAdd           [TownStages]int
	PopulationAdd     [TownStages]int
	PopulationLimit   [TownStages]int
	EmigrationDivisor [TownStages]int
	Weapons           [TownStages]int
	WorkTicks         [TownStages]int
	Parameters        [3]int
	MapColor          [256]byte
	Palettes          [2][16]color.RGBA
	LastWord          uint16
}

// DecodeLandscape follows the tables read at 0x117de (town economy) and 0xd870
// (minimap). Populous II has 19 town stages; the first game's 114-byte header
// and 11-stage arrays cannot decode these 556-byte files.
func DecodeLandscape(data []byte) (Landscape, error) {
	var land Landscape
	if len(data) != LandDataSize {
		return land, fmt.Errorf("LANDn.DAT has %d bytes; expected %d", len(data), LandDataSize)
	}
	pos := 0
	for _, table := range []*[TownStages]int{&land.ManaAdd, &land.PopulationAdd, &land.PopulationLimit, &land.EmigrationDivisor, &land.Weapons, &land.WorkTicks} {
		for i := range table {
			table[i] = int(int16(binary.BigEndian.Uint16(data[pos : pos+2])))
			pos += 2
		}
	}
	for i := range land.Parameters {
		land.Parameters[i] = int(int16(binary.BigEndian.Uint16(data[pos : pos+2])))
		pos += 2
	}
	copy(land.MapColor[:], data[pos:pos+256])
	pos += 256
	for p := range land.Palettes {
		for i := range land.Palettes[p] {
			value := binary.BigEndian.Uint16(data[pos : pos+2])
			pos += 2
			if value > 0x0fff {
				return Landscape{}, fmt.Errorf("invalid Amiga color 0x%x", value)
			}
			land.Palettes[p][i] = AmigaColor(value)
		}
	}
	land.LastWord = binary.BigEndian.Uint16(data[pos:])
	return land, nil
}

func AmigaColor(value uint16) color.RGBA {
	r, g, b := uint8(value>>8&15), uint8(value>>4&15), uint8(value&15)
	return color.RGBA{R: r * 17, G: g * 17, B: b * 17, A: 255}
}

// DecodeTiles translates the three pairs of 16x8 chunks drawn at 0xbef4/0xbfac.
// Each 12-byte descriptor contains six big-endian chunk offsets. The source
// rows are mask-first and interleaved. The Amiga routine at 0x1a3f0 rearranges
// them in place for the blitter; we decode the original rows directly.
func DecodeTiles(data []byte, palette [16]color.RGBA) ([]*image.RGBA, error) {
	if len(data) < TileCount*12 {
		return nil, fmt.Errorf("block descriptor table is truncated")
	}
	result := make([]*image.RGBA, TileCount)
	for tile := range result {
		img := image.NewRGBA(image.Rect(0, 0, TileWidth, TileHeight))
		for chunk := 0; chunk < 6; chunk++ {
			p := int(binary.BigEndian.Uint16(data[tile*12+chunk*2:]))
			if p == 0 {
				continue
			}
			if p < TileCount*12 || p+80 > len(data) {
				return nil, fmt.Errorf("tile %d chunk %d offset %d is outside block data", tile, chunk, p)
			}
			part, err := DecodeInterleaved(data[p:p+80], 16, 8, palette)
			if err != nil {
				return nil, err
			}
			x, y := (chunk%2)*16, (chunk/2)*8
			draw.Draw(img, image.Rect(x, y, x+16, y+8), part, image.Point{}, draw.Src)
		}
		result[tile] = img
	}
	return result, nil
}

type Sprite struct {
	Image   *image.RGBA
	AnchorX int
	AnchorY int
	Hunk    int
	Offset  uint32
}

// DecodeSprites reads the original 12-byte descriptors at 0x21626, using Hunk
// relocations to distinguish embedded UI sprites (hunk 3) from loaded walking
// and effect sprites (hunk 5). Resource offsets alone do not identify a buffer.
func DecodeSprites(exe *amiga.Executable, s16, s32 []byte, palette [16]color.RGBA) ([]Sprite, error) {
	if exe == nil || len(exe.Hunks) < 6 {
		return nil, fmt.Errorf("sprite executable hunks missing")
	}
	code := exe.Hunks[0].Data
	const start = 0x21626
	const count = 830 // final descriptor at index 830 is the -99 terminator.
	if start+(count+1)*12 > len(code) || binary.BigEndian.Uint32(code[start+count*12:]) != 0xffffff9d {
		return nil, fmt.Errorf("unsupported sprite descriptor table")
	}
	targets := make(map[int]int)
	for _, group := range exe.Hunks[0].Relocations {
		for _, offset := range group.Offsets {
			targets[int(offset)] = int(group.Target)
		}
	}
	result := make([]Sprite, count)
	for i := 1; i < count; i++ {
		p := start + i*12
		offset := binary.BigEndian.Uint32(code[p:])
		anchorX := int(binary.BigEndian.Uint16(code[p+4:]))
		height := int(binary.BigEndian.Uint16(code[p+6:]))
		width := anchorX * 2
		if (width != 16 && width != 32) || height < 1 || height > 128 {
			return nil, fmt.Errorf("sprite %d has invalid dimensions %dx%d", i, width, height)
		}
		target, ok := targets[p]
		if !ok {
			return nil, fmt.Errorf("sprite %d has no data relocation", i)
		}
		var data []byte
		local := int(offset)
		switch target {
		case 3:
			data = exe.Hunks[3].Data
		case 5:
			if offset >= 0x118c8 {
				data = s16
				local -= 0x118c8
			} else {
				data = s32
			}
		default:
			return nil, fmt.Errorf("sprite %d uses unsupported data hunk %d", i, target)
		}
		length := width / 8 * 5 * height
		if local < 0 || local+length > len(data) {
			return nil, fmt.Errorf("sprite %d data extends beyond hunk %d resource", i, target)
		}
		img, err := DecodeInterleaved(data[local:local+length], width, height, palette)
		if err != nil {
			return nil, fmt.Errorf("sprite %d: %w", i, err)
		}
		result[i] = Sprite{Image: img, AnchorX: anchorX, AnchorY: height, Hunk: target, Offset: offset}
	}
	return result, nil
}

// DecodeInterleaved reads mask-first rows. A set mask bit preserves the Amiga
// background; it therefore becomes transparent in RGBA, unlike block chunks.
func DecodeInterleaved(data []byte, width, height int, palette [16]color.RGBA) (*image.RGBA, error) {
	if width < 16 || width > 64 || width%16 != 0 || height < 1 || height > 1024 {
		return nil, fmt.Errorf("invalid planar dimensions %dx%d", width, height)
	}
	row := width / 8
	if len(data) != row*5*height {
		return nil, fmt.Errorf("planar payload size %d does not match %dx%d", len(data), width, height)
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// 32-pixel rows contain two mask+four-plane 16-pixel groups.
			p, bit := (y*(width/16)+x/16)*10+(x%16)/8, uint(7-x%8)
			if data[p]>>bit&1 != 0 {
				continue
			}
			index := 0
			for plane := 0; plane < 4; plane++ {
				index |= int(data[p+(plane+1)*2]>>bit&1) << plane
			}
			img.SetRGBA(x, y, palette[index])
		}
	}
	return img, nil
}

func DecodeScreen(data []byte, palette [16]color.RGBA) (*image.RGBA, error) {
	const width, height, row = 320, 200, 40
	if len(data) != row*height*4 {
		return nil, fmt.Errorf("screen size %d, expected 32000", len(data))
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			index := 0
			for plane := 0; plane < 4; plane++ {
				index |= int(data[plane*row*height+y*row+x/8]>>uint(7-x%8)&1) << plane
			}
			img.SetRGBA(x, y, palette[index])
		}
	}
	return img, nil
}

func TileAtlas(tiles []*image.RGBA, columns int) *image.RGBA {
	columns = max(1, columns)
	atlas := image.NewRGBA(image.Rect(0, 0, columns*TileWidth, ((len(tiles)+columns-1)/columns)*TileHeight))
	for i, tile := range tiles {
		draw.Draw(atlas, image.Rect(i%columns*TileWidth, i/columns*TileHeight, i%columns*TileWidth+TileWidth, i/columns*TileHeight+TileHeight), tile, image.Point{}, draw.Src)
	}
	return atlas
}
