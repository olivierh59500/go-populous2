package visualassets

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"io/fs"
	"math/bits"
)

const HUDFile = "hud.json"

type HUDBar struct {
	X, Y  int
	Color uint8
}
type HUDPopulationPoint struct{ XByte, Y, Variant int }

// HUDStencilRow is three rows of original image ink, one preserve mask and
// four color planes. These are graphics bytes, not controller instructions.
type HUDStencilRow struct {
	Preserve uint8
	Ink      [4]uint8
}
type HUDHighlight struct {
	X, Y int
	Mask [12][4]uint8
}
type HUDDescriptor struct {
	IconPalette        [16]color.RGBA
	Version            int
	Icons              [36]Region
	DisabledIcon       Region
	IconPositions      [5]image.Point
	Bars               [36]HUDBar
	Indicator          Frame
	Population         [2][74]HUDPopulationPoint
	Stencils           [2][4][4][3]HUDStencilRow
	CategoryHighlights [6]HUDHighlight
	ModeHighlights     [4]HUDHighlight // Settle, rally, join, fight.
	InspectHighlight   HUDHighlight
	Controls           [25]string
}
type HUDArt struct {
	HUDDescriptor
	IconImages     [36]*image.RGBA
	DisabledImage  *image.RGBA
	IconPixels     [36][]uint8
	DisabledPixels []uint8
}
type HUDState struct {
	Mana           uint32
	Population     [2]uint32
	Costs          [36]uint16 // Quarter-mana units, supplied by the Go game rules.
	Enabled        [36]bool
	Category, Mode int
	Inspecting     bool
	Tick           uint64
	MouseX, MouseY int
}

func LoadHUD(files fs.FS) (*HUDArt, error) {
	f, err := files.Open(HUDFile)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, 1<<20))
	decoder.DisallowUnknownFields()
	var d HUDDescriptor
	if err := decoder.Decode(&d); err != nil {
		return nil, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("HUD metadata has trailing data")
	}
	if d.Version != 1 || len(d.Indicator.Layers) > 16 {
		return nil, fmt.Errorf("unsupported HUD metadata")
	}
	for _, action := range d.Controls {
		if action != "" && action != "inspect" && action != "settle" && action != "rally" && action != "join" && action != "fight" && action != "menu" {
			return nil, fmt.Errorf("unknown HUD control action")
		}
	}
	for _, bar := range d.Bars {
		if bar.X < 0 || bar.X >= 320 || bar.Y < 0 || bar.Y >= 200 || bar.Color > 15 {
			return nil, fmt.Errorf("HUD mana marker outside display")
		}
	}
	for _, layer := range d.Indicator.Layers {
		if layer.Sprite < 0 || layer.Sprite >= 4096 || layer.X < -320 || layer.X > 320 || layer.Y < -200 || layer.Y > 200 {
			return nil, fmt.Errorf("HUD indicator artwork is invalid")
		}
	}
	for _, highlight := range append(append(append([]HUDHighlight{}, d.CategoryHighlights[:]...), d.ModeHighlights[:]...), d.InspectHighlight) {
		if highlight.X < 0 || highlight.X >= 320 || highlight.Y < 0 || highlight.Y > 199 {
			return nil, fmt.Errorf("HUD selection mask outside display")
		}
	}
	loader := imageLoader{files: files, images: make(map[string]image.Image)}
	result := &HUDArt{HUDDescriptor: d}
	if result.DisabledImage, err = loader.region(d.DisabledIcon, false); err != nil {
		return nil, err
	}
	for index, region := range d.Icons {
		if result.IconImages[index], err = loader.region(region, false); err != nil {
			return nil, err
		}
	}
	decodePixels := func(img *image.RGBA) []uint8 {
		pixels := make([]uint8, img.Bounds().Dx()*img.Bounds().Dy())
		for y := 0; y < img.Bounds().Dy(); y++ {
			for x := 0; x < img.Bounds().Dx(); x++ {
				c := img.RGBAAt(x, y)
				value := uint8(255)
				if c.A != 0 {
					value = rgbaIndex(c, d.IconPalette)
				}
				pixels[x+y*img.Bounds().Dx()] = value
			}
		}
		return pixels
	}
	result.DisabledPixels = decodePixels(result.DisabledImage)
	for id, img := range result.IconImages {
		result.IconPixels[id] = decodePixels(img)
	}
	for _, point := range d.IconPositions {
		if point.X < 0 || point.Y < 0 || point.X > 288 || point.Y > 199 {
			return nil, fmt.Errorf("HUD icon coordinate outside display")
		}
	}
	for _, bank := range d.Population {
		for _, point := range bank {
			if point.XByte < 0 || point.XByte >= 40 || point.Y < 0 || point.Y > 197 || point.Variant < 0 || point.Variant >= 4 {
				return nil, fmt.Errorf("HUD population stencil outside display")
			}
		}
	}
	return result, nil
}

func hudDivision(value uint32, divisor uint16) uint32 {
	if divisor == 0 {
		return value
	}
	q := value / uint32(divisor)
	if q > 65535 {
		return value
	}
	return value%uint32(divisor)<<16 | q
}

// DrawIcons restores the five original category icons. Disabled slots use
// their original blank icon, retaining the source's masked drawing order.
func (h *HUDArt) DrawIcons(dst *image.RGBA, s HUDState, palette [16]color.RGBA) {
	if h == nil || s.Category < 0 || s.Category >= 6 {
		return
	}
	for row, point := range h.IconPositions {
		id := s.Category*6 + row
		img, pixels := h.DisabledImage, h.DisabledPixels
		if s.Enabled[id] {
			img, pixels = h.IconImages[id], h.IconPixels[id]
		}
		if img != nil {
			for y := 0; y < img.Bounds().Dy(); y++ {
				for x := 0; x < img.Bounds().Dx(); x++ {
					value := pixels[x+y*img.Bounds().Dx()]
					if value != 255 {
						dst.SetRGBA(point.X+x, point.Y+y, palette[value])
					}
				}
			}
		}

	}
}

func rgbaIndex(c color.RGBA, palette [16]color.RGBA) uint8 {
	for index, p := range palette {
		if c == p {
			return uint8(index)
		}
	}
	return 0
}
func (h *HUDArt) DrawHighlights(dst *image.RGBA, s HUDState, palette [16]color.RGBA) {
	if h == nil {
		return
	}
	paint := func(highlight HUDHighlight) {
		for row, mask := range highlight.Mask {
			for group, b := range mask {
				for bit := 0; bit < 8; bit++ {
					if b&(1<<uint(7-bit)) == 0 {
						continue
					}
					x, y := highlight.X+group*8+bit, highlight.Y+row
					if image.Pt(x, y).In(dst.Bounds()) {
						dst.SetRGBA(x, y, palette[rgbaIndex(dst.RGBAAt(x, y), palette)^1])
					}
				}
			}
		}
	}
	if s.Inspecting {
		paint(h.InspectHighlight)
	}
	if s.Mode >= 0 && s.Mode < 4 {
		paint(h.ModeHighlights[s.Mode])
	}
	if s.Category >= 0 && s.Category < 6 {
		paint(h.CategoryHighlights[s.Category])
	}
}

// DrawIndicators reexpresses the original HUD's scalar display rules over
// decoded stencil artwork. It is independent of record layout and CPU state.
func (h *HUDArt) DrawIndicators(dst *image.RGBA, s HUDState, bank []Sprite, palette [16]color.RGBA) {
	if h == nil {
		return
	}
	for slot, bar := range h.Bars {
		if !s.Enabled[slot] {
			continue
		}
		count := int(int16(uint16(hudDivision(s.Mana>>2, s.Costs[slot]))))
		for row := 0; row < min(4, count); row++ {
			dst.SetRGBA(bar.X, bar.Y-row, palette[bar.Color&15])
		}
	}
	selected, available := 0, uint32(0)
	for row := 0; row < 6; row++ {
		slot := s.Category*6 + row
		if slot < 0 || slot >= 36 {
			break
		}
		if !s.Enabled[slot] {
			continue
		}
		price := uint32(s.Costs[slot]) * 4
		if int32(price) >= int32(s.Mana) {
			break
		}
		selected, available = row, price
	}
	x, y := 27, 166
	if available != 0 {
		fraction := min(7, int(uint16(hudDivision(s.Mana>>2, uint16(available>>2)))))
		x += selected*16 + fraction*2
		y += selected*8 + fraction
	}
	seed := uint8(0)
	for _, layer := range h.Indicator.Layers {
		if layer.Sprite < 0 || layer.Sprite >= len(bank) || bank[layer.Sprite].Image == nil {
			continue
		}
		sprite := bank[layer.Sprite]
		left, top := x+layer.X-sprite.AnchorX, y+layer.Y-sprite.AnchorY
		img := sprite.Image
		draw.Draw(dst, image.Rect(left, top, left+img.Bounds().Dx(), top+img.Bounds().Dy()), img, image.Point{}, draw.Over)
		visible := img.Bounds().Dy()
		if top < 0 {
			visible += top
		} else if top+visible > 200 {
			visible = 200 - top
		}
		if visible > 0 && top < 200 && left > -16 && left < 320 {
			words := img.Bounds().Dx()/16 + 1
			if left < 0 || left >= 304 {
				words--
			}
			seed = uint8(visible<<6 | words)
		} else {
			seed = uint8(img.Bounds().Dy())
		}
	}
	span := int(int16(uint16(hudDivision(s.Mana, 5000))))
	if span > 0 {
		span = min(35, span)
		seed = 0x80
		for column := 0; column < span*2; column++ {
			dst.SetRGBA(25+column, 164+column/2, palette[8])
		}
	}
	rotation := uint16(s.MouseX + s.MouseY + int(s.Tick))
	for owner, population := range s.Population {
		count := int(int16(uint16(hudDivision(population, 2048))))
		if population == 0 || count <= 0 {
			continue
		}
		count = min(73, count)
		for index := 0; index <= count; index++ {
			rotation = bits.RotateLeft16(rotation, 1)
			point := h.Population[owner][index]
			phase := int((uint16(seed)+rotation)&6) / 2
			for row := 0; row < 3; row++ {
				stencil := h.Stencils[owner][point.Variant][phase][row]
				var old [4]uint8
				for pixel := 0; pixel < 8; pixel++ {
					ink := rgbaIndex(dst.RGBAAt(point.XByte*8+pixel, point.Y+row), palette)
					for plane := 0; plane < 4; plane++ {
						old[plane] |= ((ink >> uint(plane)) & 1) << uint(7-pixel)
					}
				}
				var output [4]uint8
				for plane := 0; plane < 4; plane++ {
					output[plane] = old[plane]&stencil.Preserve | stencil.Ink[plane]
					seed = output[plane]
				}
				for pixel := 0; pixel < 8; pixel++ {
					ink := uint8(0)
					for plane := 0; plane < 4; plane++ {
						ink |= ((output[plane] >> uint(7-pixel)) & 1) << uint(plane)
					}
					dst.SetRGBA(point.XByte*8+pixel, point.Y+row, palette[ink])
				}
			}
		}
	}
}

// HUDHit preserves the two slanted six-cell source grids. Their cell shapes
// are arithmetic UI geometry, not rectangular substitute button rows.
func HUDHit(x, y int) (kind string, index int) {
	for grid, origin := range [2]image.Point{image.Pt(59, 137), image.Pt(42, 151)} {
		horizontal := (x - origin.X) >> 1
		vertical := y - origin.Y
		if (vertical-horizontal)>>4 == 0 {
			cell := (horizontal + vertical) >> 4
			if cell >= 0 && cell <= 5 {
				if grid == 0 {
					return "category", cell
				}
				return "power", cell
			}
		}
	}
	horizontal := (x - 317) >> 1
	vertical := y - 140
	a, b := horizontal+vertical, vertical-horizontal
	if a >= 0 && b >= 0 {
		index := (a >> 4) + ((b >> 4) << 2)
		if index >= 0 && index <= 24 {
			return "control", index
		}
	}
	return "", 0
}
