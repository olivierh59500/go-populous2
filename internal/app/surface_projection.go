package app

import "go-populous2/internal/engine"

// fractionalSurfaceOffset follows the tile's piecewise triangular surface.
// Saddle shapes deliberately differ from bilinear height interpolation.
func fractionalSurfaceOffset(shape uint8, fractionX, fractionY uint8) (int, int) {
	x, y := int(fractionX), int(fractionY)
	horizontal := (x - y) >> 4
	xPlane := func() int { return ((x>>1)+y)>>4 - 8 }
	yPlane := func() int { return ((y>>1)+x)>>4 - 8 }
	carry := x+y > 255
	vertical := 0
	switch shape {
	case 0:
		vertical = (x + y) >> 5
	case 1:
		if y < x {
			vertical = yPlane()
		} else {
			vertical = xPlane()
		}
	case 2:
		if carry {
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
		if carry {
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
		if carry {
			vertical = y >> 5
		} else {
			vertical = xPlane()
		}
	case 8:
		if carry {
			vertical = yPlane()
		} else {
			vertical = x >> 5
		}
	case 9:
		vertical = yPlane()
	case 10:
		if y < x {
			if carry {
				vertical = xPlane()
			} else {
				vertical = y >> 5
			}
		} else if carry {
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
		if carry {
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

func projectSurface(cell engine.Cell, fixedX, fixedY, cameraX, cameraY int) (int, int) {
	x, y := fixedX>>8, fixedY>>8
	ox, oy := fractionalSurfaceOffset(cell.Shape, uint8(fixedX), uint8(fixedY))
	return 192 + 16*(x-cameraX-y+cameraY) + ox, 72 + 8*(x-cameraX+y-cameraY) + oy - int(cell.BaseAltitude)*8
}

func (g *Game) projectActor(fixedX, fixedY int) (int, int) {
	if g.sceneProjection != nil {
		return mobileProjectSurface(g.World.Cell(fixedX>>8, fixedY>>8), fixedX, fixedY, *g.sceneProjection)
	}
	return projectSurface(g.World.Cell(fixedX>>8, fixedY>>8), fixedX, fixedY, g.CameraX, g.CameraY)
}

func (g *Game) followerRenderAnchor(f engine.Follower) (int, int) {
	x, y := f.Position()
	return g.projectActor(int(x*256), int(y*256))
}
