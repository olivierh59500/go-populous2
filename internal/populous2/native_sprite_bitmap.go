package populous2

import (
	"encoding/binary"
	"fmt"
)

type NativePreparedSprite struct {
	Width, Height int
	Planes        []byte
}

// NativeSpriteBitmapBank keeps loader-prepared mask/color planes for one LAND
// resource bank. Preparing once avoids decoding RGBA and reallocating source
// buffers on every native hardware-sprite request.
type NativeSpriteBitmapBank struct{ Sprites []NativePreparedSprite }

func DecodeNativeSpriteBitmapBank(bundle *Bundle, landIndex int) (*NativeSpriteBitmapBank, error) {
	if bundle == nil || bundle.Executable == nil || landIndex < 0 || landIndex >= len(bundle.Sprites) {
		return nil, fmt.Errorf("native sprite bitmap resources missing")
	}
	bank := &NativeSpriteBitmapBank{Sprites: make([]NativePreparedSprite, len(bundle.Sprites[landIndex]))}
	for index, s := range bundle.Sprites[landIndex] {
		if s.Image == nil {
			continue
		}
		width, height := s.Image.Bounds().Dx(), s.Image.Bounds().Dy()
		offset := int(s.Offset)
		var raw []byte
		switch s.Hunk {
		case 3:
			raw = bundle.Executable.Hunks[3].Data
		case 5:
			if offset >= 0x118c8 {
				raw = bundle.Raw[fmt.Sprintf("s16-%d.pak", landIndex)]
				offset -= 0x118c8
			} else {
				raw = bundle.Raw[fmt.Sprintf("s32-%d.pak", landIndex)]
			}
		default:
			return nil, fmt.Errorf("native sprite%d source hunk%d unsupported", index, s.Hunk)
		}
		length := width / 8 * 5 * height
		if offset < 0 || length < 0 || offset > len(raw)-length {
			return nil, fmt.Errorf("native sprite%d source outside retained asset", index)
		}
		prepared, err := PrepareNativeMaskedPlanes(raw[offset:offset+length], width, height)
		if err != nil {
			return nil, err
		}
		bank.Sprites[index] = NativePreparedSprite{Width: width, Height: height, Planes: prepared}
	}
	return bank, nil
}

func (b *NativeSpriteBitmapBank) Paint(request NativePresentationSprite, bitmap []byte) error {
	if b == nil || request.Sprite < 0 || request.Sprite >= len(b.Sprites) {
		return fmt.Errorf("native sprite bitmap index outside bank")
	}
	return b.Sprites[request.Sprite].Paint(request, bitmap)
}

// Paint supplies $f0ee/$f3a0's masked four-plane pixel result. Source words
// are already complemented/transposed by $1069c. The producer owns the
// original register outputs; this sink only writes the actual32000-byte RAM.
func (s NativePreparedSprite) Paint(request NativePresentationSprite, bitmap []byte) error {
	width := 16
	if request.Routine == 0xf3a0 {
		width = 32
	} else if request.Routine != 0xf0ee {
		return fmt.Errorf("native sprite pixel routine%x unsupported", request.Routine)
	}
	if len(bitmap) != 32000 || s.Width != width || s.Height != int(request.Height) || s.Height <= 0 || len(s.Planes) != width/8*5*s.Height {
		return fmt.Errorf("native sprite pixel geometry/backing differs")
	}
	x, y := int(request.X), int(request.Y)
	// The source's NEG.W overflow at -32768 can address adjacent asset RAM;
	// a standalone prepared image cannot supply those corrupt-save aliases.
	if y == -32768 {
		return fmt.Errorf("native sprite vertical overflow needs adjacent source backing")
	}
	if x <= -width || x >= 320 || y >= 200 || y+s.Height <= 0 {
		return nil
	}
	stride := width / 8
	planeSize := stride * s.Height
	baseWord := x >> 4
	shift := uint(x & 15)
	rowBits := func(plane, row int) uint64 {
		at := plane*planeSize + row*stride
		if width == 16 {
			return uint64(binary.BigEndian.Uint16(s.Planes[at:])) << 32 >> shift
		}
		return uint64(binary.BigEndian.Uint32(s.Planes[at:])) << 16 >> shift
	}
	first, last := 0, s.Height
	if y < 0 {
		first = -y
	}
	if y+last > 200 {
		last = 200 - y
	}
	words := 2
	if width == 32 {
		words = 3
	}
	for row := first; row < last; row++ {
		maskBits := rowBits(0, row)
		for word := 0; word < words; word++ {
			destWord := baseWord + word
			if destWord < 0 || destWord >= 20 {
				continue
			}
			bit := uint(32 - word*16)
			mask := uint16(maskBits >> bit)
			if mask == 0 {
				continue
			}
			at := (y+row)*40 + destWord*2
			for plane := 0; plane < 4; plane++ {
				p := plane*8000 + at
				old := binary.BigEndian.Uint16(bitmap[p:])
				color := uint16(rowBits(plane+1, row) >> bit)
				binary.BigEndian.PutUint16(bitmap[p:], old&^mask|color&mask)
			}
		}
	}
	return nil
}
