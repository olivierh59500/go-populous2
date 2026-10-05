package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeMenuWidgetRules struct {
	Bars          [24][16 * 32]byte
	FaceBackdrop  [48 * 64]byte
	FacePositions [3][2]int16
	FaceHeights   [3][8]uint16
}

type NativeFaceLayer struct {
	Part, Variant       int
	X, Y, Width, Height int
}

func DecodeNativeMenuWidgetRules(exe *amiga.Executable) (NativeMenuWidgetRules, error) {
	var r NativeMenuWidgetRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x3ddf8+1536 {
		return r, fmt.Errorf("native menu widget data missing")
	}
	code := exe.Hunks[0].Data
	decode := func(data []byte, width, height int, dest []byte) {
		rowBytes := width / 8
		for y := range height {
			for x := range width {
				for plane := range 4 {
					dest[y*width+x] |= (data[y*rowBytes*4+plane*rowBytes+x/8] >> uint(7-x%8) & 1) << uint(plane)
				}
			}
		}
	}
	for index := range r.Bars {
		decode(code[0x3c568+index*256:0x3c568+(index+1)*256], 16, 32, r.Bars[index][:])
	}
	decode(code[0x3ddf8:0x3ddf8+1536], 48, 64, r.FaceBackdrop[:])
	for part := range 3 {
		offset := 0xba8e + (2-part)*6
		if binary.BigEndian.Uint16(code[offset:]) != uint16(part*8) {
			return NativeMenuWidgetRules{}, fmt.Errorf("native face widget bank differs")
		}
		r.FacePositions[part] = [2]int16{int16(binary.BigEndian.Uint16(code[offset+2:])), int16(binary.BigEndian.Uint16(code[offset+4:]))}
		for variant := range 8 {
			r.FaceHeights[part][variant] = binary.BigEndian.Uint16(code[0x212ba+(part*8+variant)*12+6:])
		}
	}
	return r, nil
}

// PaintDeity translates $baa8's opaque face backing and $bb3a's twelve
// original sixteen-pixel XP strips. Each nibble fills two rows per unit;
// unfilled rows use that strip's corresponding grey bank twelve slots later.
func (r NativeMenuWidgetRules) PaintDeity(pixels []byte, experience [6]uint8) error {
	if len(pixels) != 320*200 {
		return fmt.Errorf("native deity widget screen size differs")
	}
	for y := range 64 {
		copy(pixels[(40+y)*320+240:], r.FaceBackdrop[y*48:(y+1)*48])
	}
	for category, value := range experience {
		baseX, baseY := 32+(category%3)*48, 32+(category/3)*40
		for half, height := range []int{int(value>>4) * 2, int(value&15) * 2} {
			for y := range 32 {
				bank := category*2 + half + 12
				if y >= 32-height {
					bank = category*2 + half
				}
				copy(pixels[(baseY+y)*320+baseX+half*16:], r.Bars[bank][y*16:(y+1)*16])
			}
		}
	}
	return nil
}

// FaceLayers supplies the direct blitter's top-left coordinates at $baee.
// Draw the original DeityArt.Parts image without sprite-anchor subtraction.
func (r NativeMenuWidgetRules) FaceLayers(parts [3]uint8) ([]NativeFaceLayer, error) {
	layers := []NativeFaceLayer{}
	for part := 2; part >= 0; part-- {
		if parts[part] > 7 {
			return nil, fmt.Errorf("native face variant outside bank")
		}
		height := int(r.FaceHeights[part][parts[part]])
		layers = append(layers, NativeFaceLayer{Part: part, Variant: int(parts[part]), X: int(r.FacePositions[part][0]), Y: int(r.FacePositions[part][1]) + max(0, 16-height), Width: 32, Height: height})
	}
	return layers, nil
}

type NativeOptionsPresentation struct {
	Requester *NativeRequester
	Rules     NativeRequesterRules
}

// Options translates $471c..$47ec's ten low-bit checkbox markers and speed
// glyph. The signed speed comparison clamps values >=16 only for display;
// bounded negative aliases inside the prepared text retain their native write.
func (p *NativePresentation) Options(side, ruleWord, speed uint16, specialCodes []byte) (NativeOptionsPresentation, error) {
	var out NativeOptionsPresentation
	if p == nil || side < 1 || side > 2 {
		return out, fmt.Errorf("native options side outside 1/2")
	}
	r, err := p.Compile(NativeMenuOptions, [][]byte{p.winners[side-1], specialCodes})
	if err != nil {
		return out, err
	}
	out.Rules = p.Requesters
	out.Rules.EnableOptionMarkers(true)
	// $4958 always points to its input buffer, even when the first byte
	// is NUL. The generic template API also supports absent pointers;
	// this caller specifically requires opaque native 'k' field padding.
	if len(specialCodes) == 0 || specialCodes[0] == 0 {
		for index, code := range r.Text {
			if code != 'v' {
				continue
			}
			for field := index + 1; field < len(r.Text) && r.Text[field] != 'w'; field++ {
				r.Text[field] = 'k'
			}
		}
	}
	for index := 0; index < len(r.Text); index++ {
		switch r.Text[index] {
		case 'h':
			offset := int(int16(speed))
			if offset >= 16 {
				offset = 15
			}
			at := index + 1 + offset
			if at < 0 || at >= len(r.Text) {
				return out, fmt.Errorf("native speed glyph alias outside prepared text")
			}
			r.Text[at] = 'g'
		case 'y', 'z':
			r.Text[index] = 'z'
			if ruleWord&1 != 0 {
				r.Text[index] = 'y'
			}
			ruleWord >>= 1
		}
	}
	out.Requester = r
	return out, nil
}
