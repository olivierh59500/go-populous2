package app

import (
	"image"
	"image/color"
	"image/draw"
	"testing"

	"go-populous2/internal/engine"
	"go-populous2/internal/mobileui"
	"go-populous2/internal/visualassets"
)

func TestMobileSurfaceUsesEveryOriginalSlopeOffset(t *testing.T) {
	view := mobileui.Viewport{Rect: image.Rect(32, 8, 352, 248), CenterX: 23.5, CenterY: 18.5}
	for shape := range 16 {
		for _, x := range []int{0, 23, 128, 231, 255} {
			for _, y := range []int{0, 19, 128, 237, 255} {
				cell := engine.Cell{Shape: uint8(shape), BaseAltitude: 4}
				fixedX, fixedY := 23*256+x, 17*256+y
				a, b := mobileProjectSurface(cell, fixedX, fixedY, view)
				wantA, wantB := projectSurface(cell, fixedX, fixedY, 20, 15)
				if a != wantA || b != wantB {
					t.Fatal("mobile slope changed original saddle/triangular arithmetic", shape, x, y, a, b, wantA, wantB)
				}
			}
		}
	}
}

func mobileSceneTestGame() *Game {
	w := &engine.World{}
	for i := range w.Heights {
		w.Heights[i] = 1
	}
	for i := range w.Tiles {
		w.Tiles[i] = engine.Cell{Corners: [4]uint8{1, 1, 1, 1}, BaseAltitude: 1, Shape: 15, Code: 31}
	}
	visual := &visualassets.Bundle{Animations: map[string]visualassets.Animation{}}
	visual.Palettes[0][0] = color.RGBA{A: 255}
	visual.Tiles[0] = make([]*image.RGBA, 32)
	for i := range visual.Tiles[0] {
		visual.Tiles[0][i] = image.NewRGBA(image.Rect(0, 0, 32, 24))
	}
	for _, ink := range []color.RGBA{{R: 255, A: 255}, {G: 255, A: 255}, {B: 255, A: 255}, {R: 255, G: 255, A: 255}, {G: 255, B: 255, A: 255}} {
		img := image.NewRGBA(image.Rect(0, 0, 9, 9))
		draw.Draw(img, img.Bounds(), &image.Uniform{C: ink}, image.Point{}, draw.Src)
		visual.Sprites[0] = append(visual.Sprites[0], visualassets.Sprite{Image: img, AnchorX: 4, AnchorY: 9})
	}
	return &Game{World: w, Screen: Playing, Assets: &Assets{Visual: visual}, CameraX: 20, CameraY: 20}
}

func TestMobileSceneKeepsMixedRegisteredActorsBeyondDesktopWidth(t *testing.T) {
	g := mobileSceneTestGame()
	x, y := 38, 25
	g.World.Followers[1] = engine.Follower{State: engine.Walking, X: uint8(x), Y: uint8(y), Owner: 0, Population: 100}
	g.World.Nature.Scenery[0] = engine.SceneryActor{Kind: engine.SceneryTree, X: uint8(x), Y: uint8(y)}
	g.World.Earth.Walls[0] = engine.WallActor{Active: true, X: x, Y: y}
	g.World.Fire.Columns[0] = engine.FireEffect{Active: true, X: x*256 + 128, Y: y*256 + 128, Phase: engine.FireMoving}
	refs := []engine.ActorRef{{Kind: engine.ActorFollower, Index: 1}, {Kind: engine.ActorScenery}, {Kind: engine.ActorWall}, {Kind: engine.ActorEffect}, {Kind: engine.ActorMagnet}}
	names := []string{"follower/0/0/north", "scenery/tree/0", "wall/connection/0", "fire-column/active", "magnet/0"}
	for i, name := range names {
		g.Assets.Visual.Animations[name] = visualassets.Animation{Loop: true, Frames: []visualassets.Frame{{Layers: []visualassets.SpriteLayer{{Sprite: i}}}}}
		g.World.Actors.Link(refs[i], x*256+128, y*256+128)
	}
	g.World.Magnets[0] = engine.MagnetActor{X: x*256 + 128, Y: y*256 + 128}
	view := mobileui.Viewport{Rect: image.Rect(0, 24, 540, 196), CenterX: 32, CenterY: 32}
	dst := image.NewRGBA(image.Rect(0, 0, 540, 240))
	const outside = 0xa7
	for i := range dst.Pix {
		dst.Pix[i] = outside
	}
	g.drawMobileScene(dst, view, 96)
	px, py := mobileProjectSurface(g.World.Cell(x, y), x*256+128, y*256+128, view)
	if px <= 320 || dst.RGBAAt(px, py-1) != (color.RGBA{G: 255, B: 255, A: 255}) {
		t.Fatal("wide mixed actor chain lost its final magnet layer", px, py, dst.RGBAAt(px, py-1))
	}
	if dst.RGBAAt(10, 10) != (color.RGBA{outside, outside, outside, outside}) {
		t.Fatal("scene drew over the mobile toolbar")
	}
	// The same typed chain is traversed oldest to newest by both renderers.
	var chain [8]engine.ActorRef
	if count := mobileParcelChain(g.World, x, y, chain[:]); count != len(refs) {
		t.Fatal("registered actor family omitted", count)
	} else {
		for i, ref := range refs {
			if chain[count-i-1] != ref {
				t.Fatal("registered actor painter order changed", chain)
			}
		}
	}
}

func TestMobileConstructionMaskAndPickingUsePresentedTerrain(t *testing.T) {
	g := mobileSceneTestGame()
	view := mobileui.Viewport{Rect: image.Rect(0, 24, 540, 196), CenterX: 32, CenterY: 32}
	g.presentation.Capture(g, false)
	g.World.Heights[32+32*engine.CornerSize] = 8
	px, py := view.Project(32, 32, 1)
	x, y, ok := g.pickMobileCorner(view, int(px), int(py))
	if !ok || x != 32 || y != 32 {
		t.Fatal("mobile target jumped to live post-physics terrain", x, y, ok)
	}
	mask := g.mobileConstructionView(view)
	if !mask.Valid() || mask.Width <= 8 || mask.Height <= 8 {
		t.Fatal("wide view retained the old eight-parcel permissions", mask)
	}
	for y := range engine.MapSize {
		for x := range engine.MapSize {
			if !mask.ContainsCell(x, y) {
				continue
			}
			px, py := view.Project(float64(x)+0.5, float64(y)+0.5, 1)
			if px+16 <= float64(view.Rect.Min.X) || px-16 >= float64(view.Rect.Max.X) || py+8 <= float64(view.Rect.Min.Y) || py-8 >= float64(view.Rect.Max.Y) {
				t.Fatal("hidden culling parcel became buildable", x, y)
			}
		}
	}
	if _, _, ok := g.pickMobileCorner(view, 10, 10); ok {
		t.Fatal("toolbar touch sculpted terrain")
	}
}

func TestMobileSceneCachePreservesMainCycleAndRefreshesCamera(t *testing.T) {
	g := mobileSceneTestGame()
	g.World.Magnets[0] = engine.MagnetActor{X: 32*256 + 128, Y: 32*256 + 128}
	g.World.Actors.Link(engine.ActorRef{Kind: engine.ActorMagnet}, g.World.Magnets[0].X, g.World.Magnets[0].Y)
	g.Assets.Visual.Animations["magnet/0"] = visualassets.Animation{Loop: true, Frames: []visualassets.Frame{{Layers: []visualassets.SpriteLayer{{Sprite: 0}}}}}
	view := mobileui.Viewport{Rect: image.Rect(0, 24, 540, 196), CenterX: 32, CenterY: 32}
	dst := image.NewRGBA(image.Rect(0, 0, 540, 240))
	var cache mobileSceneCache
	g.drawCachedMobileScene(dst, view, 96, &cache)
	px, py := mobileProjectSurface(g.World.Cell(32, 32), g.World.Magnets[0].X, g.World.Magnets[0].Y, view)
	if dst.RGBAAt(px, py-1).R != 255 {
		t.Fatal("first scene missing")
	}
	g.Assets.Visual.Animations["magnet/0"] = visualassets.Animation{Loop: true, Frames: []visualassets.Frame{{Layers: []visualassets.SpriteLayer{{Sprite: 1}}}}}
	g.Updates++
	g.drawCachedMobileScene(dst, view, 96, &cache)
	if dst.RGBAAt(px, py-1).R != 255 {
		t.Fatal("display update changed an original main-cycle frame")
	}
	view = view.Pan(32, 0)
	g.drawCachedMobileScene(dst, view, 96, &cache)
	if dst.RGBAAt(px+32, py-1).G != 255 || dst.RGBAAt(px, py-1).R != 0 {
		t.Fatal("camera drag did not immediately refresh cached terrain")
	}
}

func TestMobileAnimationVisibilityIncludesCroppedAndOffsetLayers(t *testing.T) {
	g := mobileSceneTestGame()
	g.framebuffer = image.NewRGBA(image.Rect(0, 24, 540, 196))
	g.Assets.Visual.Animations["test"] = visualassets.Animation{Frames: []visualassets.Frame{{Layers: []visualassets.SpriteLayer{{Sprite: 0, X: 100, Y: -70}}}}}
	if !g.mobileAnimationVisible("test", 0, 360, 190, 0, 0) {
		t.Fatal("visible upper spell layer beyond desktop width was culled")
	}
	if g.mobileAnimationVisible("test", 0, 640, 190, 0, 0) || g.mobileAnimationVisible("test", 0, 360, 190, 0, 20) {
		t.Fatal("clipped or fully cropped layer remained drawable")
	}
}
