package app

import (
	"image"
	"math"

	"go-populous2/internal/engine"
	"go-populous2/internal/mobileui"
)

// mobileProjectSurface keeps the exact saddle and triangular surface offsets
// used by desktop followers and effects; only the screen origin changes.
func mobileProjectSurface(cell engine.Cell, fixedX, fixedY int, view mobileui.Viewport) (int, int) {
	x, y := fixedX>>8, fixedY>>8
	ox, oy := fractionalSurfaceOffset(cell.Shape, uint8(fixedX), uint8(fixedY))
	originX, originY := view.Origin()
	return originX + 16*(x-y) + ox,
		originY + 8*(x+y) + oy - int(cell.BaseAltitude)*8
}

// mobileConstructionView records only ground parcels intersecting the playing
// area. The larger sprite culling rectangle is deliberately not used for this
// permission mask. A network command can carry this same checked geometry.
func (g *Game) mobileConstructionView(view mobileui.Viewport) engine.Viewport {
	if g.World == nil || view.Rect.Empty() {
		return engine.Viewport{}
	}
	displayed := g.displayedGame()
	bounds := view.Bounds(engine.MapSize, engine.MapSize, 8, 16)
	mask := make([]uint64, engine.MapSize)
	minX, minY, maxX, maxY := engine.MapSize, engine.MapSize, -1, -1
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			cell := displayed.World.Cell(x, y)
			var surface [4]mobileui.Point
			for i, corner := range [4][2]int{{x, y}, {x + 1, y}, {x + 1, y + 1}, {x, y + 1}} {
				px, py := view.Project(float64(corner[0]), float64(corner[1]), float64(cell.Corners[i]))
				surface[i] = mobileui.Point{X: px, Y: py}
			}
			if !view.PolygonVisible(surface) {
				continue
			}
			mask[y] |= uint64(1) << uint(x)
			minX, minY, maxX, maxY = min(minX, x), min(minY, y), max(maxX, x), max(maxY, y)
		}
	}
	if maxX < minX || maxY < minY {
		return engine.Viewport{}
	}
	return engine.Viewport{X: minX, Y: minY, Width: maxX - minX + 1, Height: maxY - minY + 1, Visible: mask}
}

// pickMobileCorner follows the original nearest-vertex/frontmost-tie rule on
// the immutable displayed world. The conservative rendering box is not an
// authorization box: only vertices actually inside the playfield can be hit.
func (g *Game) pickMobileCorner(view mobileui.Viewport, screenX, screenY int) (int, int, bool) {
	if g.World == nil || !image.Pt(screenX, screenY).In(view.Rect) {
		return 0, 0, false
	}
	displayed := g.displayedGame()
	w := displayed.World
	bounds := view.Bounds(engine.CornerSize, engine.CornerSize, 8, 16)
	bestDistance, bestX, bestY, found := 12*12, 0, 0, false
	for diagonal := bounds.Min.X + bounds.Min.Y; diagonal <= bounds.Max.X+bounds.Max.Y-2; diagonal++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			y := diagonal - x
			if y < bounds.Min.Y || y >= bounds.Max.Y {
				continue
			}
			height := float64(w.Heights[x+y*engine.CornerSize])
			if !view.CornerVisible(x, y, height) {
				continue
			}
			px, py := view.Project(float64(x), float64(y), height)
			dx, dy := int(math.Round(px))-screenX, int(math.Round(py))-screenY
			if distance := dx*dx + dy*dy; distance <= bestDistance {
				bestDistance, bestX, bestY, found = distance, x, y, true
			}
		}
	}
	return bestX, bestY, found
}

// pickMobileFollower uses the same walking/town eligibility and anchor hit
// dimensions as the original inspect tool, extended to the wider playing area.
func (g *Game) pickMobileFollower(view mobileui.Viewport, screenX, screenY int) int {
	if g.World == nil || !image.Pt(screenX, screenY).In(view.Rect) {
		return 0
	}
	displayed := g.displayedGame()
	displayed.sceneProjection = &view
	bounds := view.Bounds(engine.MapSize, engine.MapSize, 8, 32)
	var chain [engine.FollowerCapacity + engine.EffectCapacity + engine.SceneryCapacity + engine.WallCapacity + 2]engine.ActorRef
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			count := mobileParcelChain(displayed.World, x, y, chain[:])
			for i := count - 1; i >= 0; i-- {
				ref := chain[i]
				if ref.Kind != engine.ActorFollower || ref.Index < 1 || int(ref.Index) >= engine.FollowerCapacity {
					continue
				}
				f := displayed.World.Followers[ref.Index]
				if f.State == engine.Inactive {
					continue
				}
				width, height := 8, 18
				if f.State == engine.Town || f.BattleWasTown {
					width, height = 16, displayed.townInspectHeight(f, displayed.World.Level.Landscape)
				} else if f.State != engine.Walking || f.ContactWaiting {
					continue
				}
				ax, ay := displayed.followerRenderAnchor(f)
				if absInt(screenX-ax) <= width && screenY <= ay && ay-screenY <= height {
					return int(ref.Index)
				}
			}
		}
	}
	return 0
}
