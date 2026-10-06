package mobileui

import (
	"image"
	"math"
	"testing"
)

func TestMobileViewportRetainsOriginalProjectionAcrossWideScreen(t *testing.T) {
	view := Viewport{Rect: image.Rect(32, 8, 352, 248), CenterX: 3.5, CenterY: 3.5}
	for y := range 8 {
		for x := range 8 {
			for height := range 9 {
				px, py := view.Project(float64(x), float64(y), float64(height))
				if px != float64(192+16*(x-y)) || py != float64(72+8*(x+y-height)) {
					t.Fatal("original projection changed", x, y, height, px, py)
				}
			}
		}
	}
	view = Viewport{Rect: image.Rect(0, 24, 539, 196), CenterX: 32.25, CenterY: 29.75}
	for _, p := range [][3]float64{{0, 0, 0}, {64, 64, 8}, {13.5, 22.25, 4}} {
		px, py := view.Project(p[0], p[1], p[2])
		x, y := view.Unproject(px, py, p[2])
		if math.Abs(x-p[0])+math.Abs(y-p[1]) > 1e-9 {
			t.Fatal("wide projection wrapped or changed altitude", p, x, y)
		}
	}
}

func TestMobileViewportPanAndBoundsIncludeRaisedTerrain(t *testing.T) {
	view := Viewport{Rect: image.Rect(0, 24, 540, 196), CenterX: 32, CenterY: 32}
	x0, y0 := view.Project(31, 33, 4)
	panned := view.Pan(32, -16)
	x1, y1 := panned.Project(31, 33, 4)
	if x1-x0 != 32 || y1-y0 != -16 {
		t.Fatal("map did not follow the fingers", x1-x0, y1-y0)
	}
	for _, center := range [][2]float64{{0, 0}, {32, 32}, {63, 63}, {0, 63}} {
		view.CenterX, view.CenterY = center[0], center[1]
		bounds := view.Bounds(64, 64, 8, 32)
		for y := range 64 {
			for x := range 64 {
				for height := range 9 {
					if view.ParcelVisible(x, y, float64(height)) && !image.Pt(x, y).In(bounds) {
						t.Fatal("visible raised parcel omitted by culling", x, y, height, bounds)
					}
				}
			}
		}
	}
}

func TestMobileLogicalSizePreservesAspectAndLandscapeSpace(t *testing.T) {
	for _, tc := range []struct{ w, h, wantW, wantH int }{
		{2424, 1080, 539, 240}, {1080, 2424, 320, 718}, {1920, 1080, 427, 240}, {0, 0, 320, 240},
	} {
		w, h := LogicalSize(tc.w, tc.h)
		if w != tc.wantW || h != tc.wantH {
			t.Fatal("logical dimensions differ", tc, w, h)
		}
	}
}

func TestMobileVisibilityUsesClippedSurfaceInsteadOfItsBoundingBox(t *testing.T) {
	view := Viewport{Rect: image.Rect(0, 0, 10, 10)}
	for _, tc := range []struct {
		name    string
		surface [4]Point
		visible bool
	}{
		{"inside", [4]Point{{5, 1}, {9, 5}, {5, 9}, {1, 5}}, true},
		{"covers entire view", [4]Point{{5, -30}, {40, 5}, {5, 40}, {-30, 5}}, true},
		{"clips edge", [4]Point{{-2, 1}, {2, 5}, {-2, 9}, {-6, 5}}, true},
		{"touches edge only", [4]Point{{-4, 1}, {0, 5}, {-4, 9}, {-8, 5}}, false},
		{"bounding box crosses corner", [4]Point{{-5, -1}, {1, -5}, {-5, -11}, {-11, -5}}, false},
	} {
		if got := view.PolygonVisible(tc.surface); got != tc.visible {
			t.Fatal(tc.name, got)
		}
	}
}
