package populous2

// ActorOffset translates the sixteen slope cases at CODE:$e392-$e422 for
// unsigned fractional coordinates. These piecewise triangular slopes match
// the original tile geometry; bilinear interpolation changes the saddles.
func (cell TerrainCell) ActorOffset(fractionX, fractionY uint8) (int, int) {
	x, y := int(fractionX), int(fractionY)
	horizontal := (x - y) >> 4
	xPlane := func() int { return ((x>>1)+y)>>4 - 8 }
	yPlane := func() int { return ((y>>1)+x)>>4 - 8 }
	sumCarry := x+y > 255
	vertical := 0
	switch cell.Shape {
	case 0:
		vertical = (x + y) >> 5
	case 1:
		if y < x {
			vertical = yPlane()
		} else {
			vertical = xPlane()
		}
	case 2:
		if sumCarry {
			vertical = xPlane()
		} else {
			vertical = y >> 5
		}
	case 3:
		vertical = xPlane()
	case 4:
		if y < x {
			vertical = x >> 5
		} else {
			vertical = y >> 5
		}
	case 5:
		if sumCarry {
			if y < x {
				vertical = x >> 5
			} else {
				vertical = y >> 5
			}
		} else if y < x {
			vertical = yPlane()
		} else {
			vertical = xPlane()
		}
	case 6:
		vertical = y >> 5
	case 7:
		if sumCarry {
			vertical = y >> 5
		} else {
			vertical = xPlane()
		}
	case 8:
		if sumCarry {
			vertical = yPlane()
		} else {
			vertical = x >> 5
		}
	case 9:
		vertical = yPlane()
	case 10:
		if y < x {
			if sumCarry {
				vertical = xPlane()
			} else {
				vertical = y >> 5
			}
		} else if sumCarry {
			vertical = yPlane()
		} else {
			vertical = x >> 5
		}
	case 11:
		if y < x {
			vertical = xPlane()
		} else {
			vertical = yPlane()
		}
	case 12:
		vertical = x >> 5
	case 13:
		if sumCarry {
			vertical = x >> 5
		} else {
			vertical = yPlane()
		}
	case 14:
		if y < x {
			vertical = y >> 5
		} else {
			vertical = x >> 5
		}
	case 15:
		vertical = (x+y)>>5 - 8
	}
	return horizontal, vertical
}
