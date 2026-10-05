package populous2

import "fmt"

// Line is the original$e056 integer line producer. Packed endpoints are
// D6/D7(x high-word,y low-word); every pixel uses the exact checked$e17a child.
func (r *NativeRenderFrameRules) Line(cb NativeRenderFrameCallbacks) (NativeRenderFramePlan, error) {
	p := NativeRenderFramePlan{Drawn: true, Pixels: []NativeHUDPixel{}, Sprites: []NativePresentationSprite{}}
	if r == nil || cb.Frame == nil || len(cb.Bitmap) != 32000 {
		return p, fmt.Errorf("native line frame/bitmap missing")
	}
	c := cb.Frame
	c.Word(3, uint16(c.D[3])&15)
	if c.D[7] == c.D[6] {
		c.Word(1, uint16(c.D[6]))
		c.Swap(6)
		c.Word(0, uint16(c.D[6]))
		c.Word(2, uint16(c.D[3]))
		err := renderFramePixel(cb, &p, true)
		return p, err
	}
	if int16(c.D[7]) <= int16(c.D[6]) {
		c.D[6], c.D[7] = c.D[7], c.D[6]
	}
	c.D[7] -= c.D[6]
	c.D[5] = c.D[7]
	c.Swap(5)
	c.D[5] = uint32(int32(int16(c.D[5])))
	if int32(c.D[5]) < 0 {
		c.D[5] = -c.D[5]
	}
	yMajor := int16(c.D[7]) > int16(c.D[5])
	if !yMajor {
		c.Swap(6)
		c.Swap(7)
		if int16(c.D[7]) < 0 {
			c.D[6] += c.D[7]
			c.D[7] = -c.D[7]
		}
		c.D[5] = c.D[7]
		c.Swap(5)
		c.D[5] = uint32(int32(int16(c.D[5])))
		if int32(c.D[5]) < 0 {
			c.D[5] = -c.D[5]
		}
	}
	c.Swap(5)
	divisor := uint16(c.D[7])
	if divisor == 0 {
		return p, fmt.Errorf("native line DIVU zero")
	}
	if c.D[5]/uint32(divisor) > 0xffff {
		c.D[5] = 1
		c.Swap(5)
	} else {
		if err := frameDivide(c, 5, divisor); err != nil {
			return p, err
		}
		c.D[5] &= 0xffff
	}
	if int32(c.D[7]) < 0 {
		c.D[5] = -c.D[5]
	}
	c.Word(4, uint16(c.D[6]))
	c.Word(6, 0)
	for steps := 0; steps < 65536; steps++ {
		if yMajor {
			c.Word(1, uint16(c.D[4]))
			c.Swap(6)
			c.Word(0, uint16(c.D[6]))
		} else {
			c.Word(0, uint16(c.D[4]))
			c.Swap(6)
			c.Word(1, uint16(c.D[6]))
		}
		c.Word(2, uint16(c.D[3]))
		if err := renderFramePixel(cb, &p, true); err != nil {
			return p, err
		}
		c.Swap(6)
		c.D[6] += c.D[5]
		c.Word(4, uint16(c.D[4])+1)
		c.Word(7, uint16(c.D[7])-1)
		if uint16(c.D[7]) == 0 {
			return p, nil
		}
	}
	return p, fmt.Errorf("native line exceeded WORD count")
}
