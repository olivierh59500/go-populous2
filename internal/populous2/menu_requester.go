package populous2

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"

	"go-populous2/internal/amiga"
)

type NativeRequester struct {
	Column, Row, Width, Height int
	Text                       []byte
}

type NativeRequesterRules struct {
	Markers [37]int8 // Native classification window for byte codes 91..127.
}

func DecodeNativeRequesterRules(exe *amiga.Executable) (NativeRequesterRules, error) {
	var rules NativeRequesterRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x4eb7 {
		return rules, fmt.Errorf("native requester rules missing")
	}
	code := exe.Hunks[0].Data
	for index := range rules.Markers {
		rules.Markers[index] = int8(code[0x4e92+index])
	}
	return rules, nil
}

// EnableOptionMarkers reproduces the word changed by $471c/$481a while
// entering/leaving the game-options requester. Other windows keep the table.
func (rules *NativeRequesterRules) EnableOptionMarkers(enabled bool) {
	value := int8(0)
	if enabled {
		value = 1
	}
	rules.Markers['y'-91], rules.Markers['z'-91] = value, value
}

func NativeRequesterTemplate(exe *amiga.Executable, offset int) ([]byte, error) {
	if exe == nil || len(exe.Hunks) == 0 || offset < 0 || offset+4 >= len(exe.Hunks[0].Data) {
		return nil, fmt.Errorf("native requester template missing")
	}
	code := exe.Hunks[0].Data
	end := bytes.IndexByte(code[offset+4:], 0)
	if end < 0 || end > 4096 {
		return nil, fmt.Errorf("invalid native requester terminator")
	}
	return append([]byte(nil), code[offset:offset+4+end+1]...), nil
}

// Startup applies the main two-disk game's visible-window override. The
// definition also contains two extra rows; $3b98/$3bac hide those at startup.
func (rules NativeRequesterRules) Startup(exe *amiga.Executable) (*NativeRequester, error) {
	definition, err := NativeRequesterTemplate(exe, 0x8854)
	if err != nil {
		return nil, err
	}
	requester, err := rules.Compile(definition, nil)
	if err != nil {
		return nil, err
	}
	code := exe.Hunks[0].Data
	if binary.BigEndian.Uint16(code[0x3b98:]) != 0x33fc || binary.BigEndian.Uint16(code[0x3bac:]) != 0x43e9 {
		return nil, fmt.Errorf("unsupported native startup window override")
	}
	height := int(binary.BigEndian.Uint16(code[0x3b9a:]))
	end := int(binary.BigEndian.Uint16(code[0x3bae:]))
	if end > len(requester.Text) || height < 8 || height > 200 {
		return nil, fmt.Errorf("invalid native startup window bounds")
	}
	requester.Height, requester.Text = height, requester.Text[:end]
	return requester, nil
}

func NativeStartupPalette(exe *amiga.Executable) ([16]color.RGBA, error) {
	var palette [16]color.RGBA
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x3c548 {
		return palette, fmt.Errorf("native startup palette missing")
	}
	for index := range palette {
		word := binary.BigEndian.Uint16(exe.Hunks[0].Data[0x3c528+index*2:])
		if word > 0x0fff {
			return palette, fmt.Errorf("invalid native startup color")
		}
		palette[index] = AmigaColor(word)
	}
	return palette, nil
}

// Image prepares the native text planes as an opaque 320x200 indexed image.
// The caller supplies the active display palette and background composition.
func (requester *NativeRequester) Image(font *NativeMenuFont, palette [16]color.RGBA) (*image.Paletted, error) {
	if requester == nil {
		return nil, fmt.Errorf("native requester missing")
	}
	colors := make(color.Palette, len(palette))
	for index, entry := range palette {
		colors[index] = entry
	}
	img := image.NewPaletted(image.Rect(0, 0, NativeMenuWidth, NativeMenuHeight), colors)
	if err := font.DrawIndices(img.Pix, string(requester.Text), requester.Column, requester.Row); err != nil {
		return nil, err
	}
	return img, nil
}

// Compile translates $4eb6. Encoded high-bit bytes lose $25; substitution
// consumes positions in the definition, while text fields retain their tail
// and use native glyph 'k' as padding. Width is the final native line counter,
// not a recomputed maximum. Parameters are byte strings, not Unicode glyphs.
func (rules NativeRequesterRules) Compile(definition []byte, parameters [][]byte) (*NativeRequester, error) {
	if len(definition) < 5 || len(definition) > 4101 {
		return nil, fmt.Errorf("invalid native requester definition")
	}
	result := &NativeRequester{Column: int(binary.BigEndian.Uint16(definition)), Row: int(binary.BigEndian.Uint16(definition[2:]))}
	if result.Column >= 40 || result.Row >= 200 {
		return nil, fmt.Errorf("native requester origin outside display")
	}
	position, parameter, width, height := 4, 0, 0, 0
	argument := func(advance bool) []byte {
		var value []byte
		if parameter < len(parameters) {
			value = parameters[parameter]
		}
		if end := bytes.IndexByte(value, 0); end >= 0 {
			value = value[:end]
		}
		if advance {
			parameter++
		}
		return value
	}
	for position < len(definition) {
		width++
		code := definition[position]
		position++
		if code == 0 {
			result.Width, result.Height = width-1, height+8
			return result, nil
		}
		if code&0x80 != 0 {
			code -= 0x25
		}
		if code == '{' {
			value := argument(true)
			if len(value) == 0 {
				result.Text = append(result.Text, ' ')
			} else {
				position += len(value) - 1
				result.Text = append(result.Text, value...)
			}
		} else {
			result.Text = append(result.Text, code)
			if code == 'v' && len(argument(false)) > 0 {
				field := position
				for field < len(definition) && definition[field] != 'w' && definition[field] != 0x9c {
					if definition[field] == 0 {
						return nil, fmt.Errorf("unterminated native requester field")
					}
					field++
				}
				if field == len(definition) {
					return nil, fmt.Errorf("unterminated native requester field")
				}
				length := field - position
				value := argument(true)
				if len(value) > length {
					value = value[len(value)-length:]
				}
				result.Text = append(result.Text, value...)
				for padding := len(value); padding < length; padding++ {
					result.Text = append(result.Text, 'k')
				}
				position = field
			} else if code == '\n' {
				height += 8
				width = 0
			}
		}
		if len(result.Text) > 2048 {
			return nil, fmt.Errorf("native requester output exceeds buffer")
		}
	}
	return nil, fmt.Errorf("native requester definition has no reachable terminator")
}

func (rules NativeRequesterRules) marker(code byte) int8 {
	if code < 91 || code > 127 {
		return 0
	}
	return rules.Markers[code-91]
}

// Click follows $4dac. Positive marker bytes number actions in increments of
// two; negative marker bytes clear the active region. Radio glyphs c/d toggle
// in the prepared text. Out-of-buffer probes return no action rather than
// reading adjacent original scratch memory at the native bottom boundary.
func (rules NativeRequesterRules) Click(requester *NativeRequester, mouseX, mouseY int) int {
	if requester == nil || mouseX < 0 || mouseY < 0 {
		return 0
	}
	x, y := mouseX/8-requester.Column, mouseY-requester.Row
	if x < 0 || x >= requester.Width || y < 0 || y > requester.Height {
		return 0
	}
	row := y / 8 * (requester.Width + 1)
	index := row + x
	if row < 0 || index >= len(requester.Text) {
		return 0
	}
	active := false
	for at := row; at <= index; at++ {
		value := rules.marker(requester.Text[at])
		if value > 0 {
			active = true
		} else if value < 0 {
			active = false
		}
	}
	if !active {
		return 0
	}
	command, selected := 0, -1
	for at := 0; at <= index; at++ {
		if rules.marker(requester.Text[at]) > 0 {
			command += 2
			selected = at
		}
	}
	if selected >= 0 {
		if requester.Text[selected] == 'c' {
			requester.Text[selected] = 'd'
		} else if requester.Text[selected] == 'd' {
			requester.Text[selected] = 'c'
		}
	}
	return command
}
