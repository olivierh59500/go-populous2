package mobileui

import (
	"image"
	"math"
)

// Viewport uses the original 32-by-16 terrain diamonds at their native logical
// pixel size. A wider display reveals more parcels instead of stretching them.
// The camera is expressed in map coordinates and survives layout changes.
type Viewport struct {
	Rect             image.Rectangle
	CenterX, CenterY float64
}

// LogicalSize retains a readable landscape interface on phones and tablets.
// The shorter axis contains 240 logical pixels; device pixels are handled by
// Ebitengine's ordinary nearest-neighbour screen scaling.
func LogicalSize(outsideWidth, outsideHeight int) (width, height int) {
	if outsideWidth <= 0 || outsideHeight <= 0 {
		return 320, 240
	}
	if outsideWidth >= outsideHeight {
		return max(320, int(math.Round(240*float64(outsideWidth)/float64(outsideHeight)))), 240
	}
	return 320, max(240, int(math.Round(320*float64(outsideHeight)/float64(outsideWidth))))
}

// Origin rounds the whole camera once. Every terrain vertex and actor offset
// then retains the source renderer's integer arithmetic without sprite jitter.
func (v Viewport) Origin() (x, y int) {
	return int(math.Round(float64(v.Rect.Min.X+v.Rect.Max.X)/2 - 16*(v.CenterX-v.CenterY))),
		int(math.Round(float64(v.Rect.Min.Y+v.Rect.Max.Y)/2 - 8*(v.CenterX+v.CenterY)))
}

func (v Viewport) Project(mapX, mapY, altitude float64) (screenX, screenY float64) {
	x, y := v.Origin()
	return float64(x) + 16*(mapX-mapY), float64(y) + 8*(mapX+mapY-altitude)
}

func (v Viewport) Unproject(screenX, screenY, altitude float64) (mapX, mapY float64) {
	x, y := v.Origin()
	difference := (screenX - float64(x)) / 16
	sum := (screenY-float64(y))/8 + altitude
	return (sum + difference) / 2, (sum - difference) / 2
}

// Pan follows the fingers. It does not create a terrain command, and therefore
// cannot accidentally raise or lower a parcel after a two-finger gesture.
func (v Viewport) Pan(screenDX, screenDY float64) Viewport {
	v.CenterX -= screenDX/32 + screenDY/16
	v.CenterY -= screenDY/16 - screenDX/32
	return v
}

func (v Viewport) Clamp(mapWidth, mapHeight int) Viewport {
	v.CenterX = max(0, min(float64(max(0, mapWidth-1)), v.CenterX))
	v.CenterY = max(0, min(float64(max(0, mapHeight-1)), v.CenterY))
	return v
}

// Bounds conservatively includes raised ground and sprites reaching into the
// view. Sprite padding is supplied by the caller because Populous II includes
// tall storms and meteors in addition to ordinary followers and towns.
func (v Viewport) Bounds(mapWidth, mapHeight, maxAltitude, spritePadding int) image.Rectangle {
	if v.Rect.Empty() || mapWidth <= 0 || mapHeight <= 0 {
		return image.Rectangle{}
	}
	padding := float64(max(16, spritePadding))
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, altitude := range [2]float64{0, float64(max(0, maxAltitude))} {
		for _, point := range [4][2]float64{
			{float64(v.Rect.Min.X) - padding, float64(v.Rect.Min.Y) - padding},
			{float64(v.Rect.Max.X) + padding, float64(v.Rect.Min.Y) - padding},
			{float64(v.Rect.Min.X) - padding, float64(v.Rect.Max.Y) + padding},
			{float64(v.Rect.Max.X) + padding, float64(v.Rect.Max.Y) + padding},
		} {
			x, y := v.Unproject(point[0], point[1], altitude)
			minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
		}
	}
	return image.Rect(int(math.Floor(minX)), int(math.Floor(minY)), int(math.Ceil(maxX))+1, int(math.Ceil(maxY))+1).
		Intersect(image.Rect(0, 0, mapWidth, mapHeight))
}

// ParcelVisible tests the ground diamond, not the conservative culling box.
// Hidden parcels must not grant construction permissions merely because a
// tall spell might extend from their vicinity into the playing area.
func (v Viewport) ParcelVisible(mapX, mapY int, altitude float64) bool {
	x, y := v.Project(float64(mapX)+0.5, float64(mapY)+0.5, altitude)
	return x+16 > float64(v.Rect.Min.X) && x-16 < float64(v.Rect.Max.X) &&
		y+8 > float64(v.Rect.Min.Y) && y-8 < float64(v.Rect.Max.Y)
}

// CornerVisible is used both by targeting and terrain command authorization.
func (v Viewport) CornerVisible(mapX, mapY int, altitude float64) bool {
	x, y := v.Project(float64(mapX), float64(mapY), altitude)
	return x >= float64(v.Rect.Min.X) && x < float64(v.Rect.Max.X) &&
		y >= float64(v.Rect.Min.Y) && y < float64(v.Rect.Max.Y)
}

// PolygonVisible clips a projected terrain surface against the playfield.
// Bounding rectangles alone would count invisible diamonds near its corners,
// and lower tile origins alone would miss raised sides crossing its top edge.
func (v Viewport) PolygonVisible(surface [4]Point) bool {
	if v.Rect.Empty() {
		return false
	}
	var a, b [12]Point
	copy(a[:], surface[:])
	count := len(surface)
	for side, boundary := range [4]float64{float64(v.Rect.Min.X), float64(v.Rect.Max.X), float64(v.Rect.Min.Y), float64(v.Rect.Max.Y)} {
		if count == 0 {
			return false
		}
		nextCount := 0
		inside := func(p Point) bool {
			if side == 0 {
				return p.X >= boundary
			}
			if side == 1 {
				return p.X <= boundary
			}
			if side == 2 {
				return p.Y >= boundary
			}
			return p.Y <= boundary
		}
		previous := a[count-1]
		previousInside := inside(previous)
		for i := 0; i < count; i++ {
			current := a[i]
			currentInside := inside(current)
			if currentInside != previousInside {
				fraction := 0.0
				if side < 2 {
					fraction = (boundary - previous.X) / (current.X - previous.X)
				} else {
					fraction = (boundary - previous.Y) / (current.Y - previous.Y)
				}
				b[nextCount] = Point{previous.X + fraction*(current.X-previous.X), previous.Y + fraction*(current.Y-previous.Y)}
				nextCount++
			}
			if currentInside {
				b[nextCount] = current
				nextCount++
			}
			previous, previousInside = current, currentInside
		}
		a, b, count = b, a, nextCount
	}
	area := 0.0
	for i := range count {
		p, q := a[i], a[(i+1)%count]
		area += p.X*q.Y - q.X*p.Y
	}
	return math.Abs(area) > 1e-6
}
