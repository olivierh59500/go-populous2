package populous2

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
)

// PrepareNativeMaskedPlanes translates $1069c's resource preparation. Raw
// disk/HUNK images interleave five words per sixteen pixels. The loader
// complements each mask word and writes five contiguous planes for $f3a0.
// It returns a separate buffer so shared raw resources are never transformed
// twice when several descriptors or palettes reuse the same source image.
func PrepareNativeMaskedPlanes(data []byte, width, height int) ([]byte, error) {
	if width <= 0 || width%16 != 0 || height <= 0 || width/8 > len(data)/5/height || len(data) != width/8*5*height {
		return nil, fmt.Errorf("native planar preparation size differs")
	}
	out := make([]byte, len(data))
	planeSize := width / 8 * height
	for word := range planeSize / 2 {
		for plane := range 5 {
			value := binary.BigEndian.Uint16(data[word*10+plane*2:])
			if plane == 0 {
				value = ^value
			}
			binary.BigEndian.PutUint16(out[plane*planeSize+word*2:], value)
		}
	}
	return out, nil
}

// DecodeNativeMaskedPlanes reads the five contiguous planes consumed by
// $f3a0 after resource preparation: one opaque-bit mask followed by four color
// planes. Unprepared disk/HUNK images retain interleaved transparent masks.
func DecodeNativeMaskedPlanes(data []byte, width, height int, palette [16]color.RGBA) (*image.RGBA, error) {
	if width <= 0 || width%8 != 0 || height <= 0 || width/8 > len(data)/5/height {
		return nil, fmt.Errorf("native masked planes truncated or dimensions invalid")
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	stride, planeSize := width/8, width/8*height
	for y := range height {
		for x := range width {
			at, shift := y*stride+x/8, uint(7-x%8)
			if data[at]>>shift&1 == 0 {
				continue
			}
			index := byte(0)
			for plane := range 4 {
				index |= (data[(plane+1)*planeSize+at] >> shift & 1) << uint(plane)
			}
			img.SetRGBA(x, y, palette[index])
		}
	}
	return img, nil
}
