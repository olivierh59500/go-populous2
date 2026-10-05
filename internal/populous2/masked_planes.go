package populous2

import (
	"fmt"
	"image"
	"image/color"
)

// DecodeNativeMaskedPlanes reads the five contiguous planes consumed by
// $f3a0: one opaque-bit mask followed by four color planes. Moving sprites
// use a different interleaved layout and a transparent-bit mask.
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
