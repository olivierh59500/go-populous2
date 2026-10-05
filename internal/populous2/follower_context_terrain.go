package populous2

import "fmt"

// NativeFollowerLowerContext carries the original unpriced $d7f0 register
// outputs. It works on a private raw-map copy because $d012's recursive order
// and raster writes determine $d838's bounds and the retained registers. It
// does not mutate the caller's terrain, actor graph or native command queue.
type NativeFollowerLowerContext struct {
	Cells                  NativeOccupancyState
	ChangedVertices        int
	MinX, MinY, MaxX, MaxY int
}

func ObserveNativeFollowerLower(context *NativeFollowerRegisterContext, cells NativeOccupancyState, raster [256]uint8, x, y int16) (NativeFollowerLowerContext, error) {
	result := NativeFollowerLowerContext{Cells: cells, MinX: 256, MinY: 256}
	if context == nil {
		return result, fmt.Errorf("native lowering register context missing")
	}
	height := func(x, y int) (int, bool) {
		if x < 0 || y < 0 || x > 64 || y > 64 {
			return 0, false
		}
		bit := uint8(0)
		if x == 64 {
			x--
			bit = 1
		}
		if y == 64 {
			y--
			if bit == 1 {
				bit = 2
			} else {
				bit = 3
			}
		}
		c := result.Cells.Cells[x+y*64]
		return int(c.Header&7) + int(raster[c.Tile]>>bit&1), true
	}
	var lower func(int, int) error
	lower = func(x, y int) error {
		h, valid := height(x, y)
		if !valid || h == 0 {
			context.Long4(0xffffffff)
			return nil
		}
		if result.ChangedVertices > 32767 {
			return fmt.Errorf("native lowering recursive work exceeds bounded map")
		}
		h--
		// $d034..$d0c0 visits these eight points in this exact order.
		for _, delta := range [8][2]int{{0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}} {
			n, inside := height(x+delta[0], y+delta[1])
			if inside && n-h > 1 {
				if err := lower(x+delta[0], y+delta[1]); err != nil {
					return err
				}
			}
		}
		for _, corner := range [4][3]int{{-1, -1, 4}, {0, -1, 8}, {-1, 0, 2}, {0, 0, 1}} {
			xx, yy := x+corner[0], y+corner[1]
			if !inside(xx, yy) {
				continue
			}
			result.MinX = min(result.MinX, xx)
			result.MinY = min(result.MinY, yy)
			result.MaxX = max(result.MaxX, xx)
			result.MaxY = max(result.MaxY, yy)
			cell := &result.Cells.Cells[xx+yy*64]
			shape, bit := raster[cell.Tile], uint8(corner[2])
			context.Long5(uint32(shape)) // MOVEQ followed by the two raster byte loads.
			decrement := false
			if shape&bit != 0 {
				shape -= bit
				context.Byte5(shape)
				context.Byte4(shape & 15)
				if shape&15 == 0 {
					context.Byte4(cell.Header & 7)
					if cell.Header&7 != 0 {
						shape |= 15
						context.Byte5(shape)
						decrement = true
					}
				}
			} else {
				shape &= ^bit
				context.Word5(uint16(shape))
				decrement = true
			}
			if decrement {
				cell.Header--
			}
			cell.Header &= 7
			cell.Tile = shape
		}
		context.Word4(uint16(h)) // $d29e restores the local desired height word.
		result.ChangedVertices++
		return nil
	}
	if err := lower(int(x), int(y)); err != nil {
		return result, err
	}
	if result.MinY != 256 {
		// $d838..$d8ca scans the entire inclusive rectangle, regardless of
		// whether a particular interior tile's value changed.
		context.Word4(uint16(result.MaxX + 1))
		context.Word5(uint16(result.MaxY + 1))
	}
	return result, nil
}
