package app

import (
	"go-populous2/internal/engine"
	"testing"
)

func TestCornerPickingFollowsTheProjectedHeight(t *testing.T) {
	world := &engine.World{}
	for i := range world.Heights {
		world.Heights[i] = 1
	}
	g := &Game{World: world, CameraX: 8, CameraY: 8}
	x, y := 12, 12
	sx, sy := g.projectCorner(x, y)
	pickedX, pickedY, ok := g.pickCorner(sx, sy)
	if !ok || pickedX != x || pickedY != y {
		t.Fatal("flat surface picks wrong corner", pickedX, pickedY, ok)
	}
	world.Heights[x+y*engine.CornerSize] = 3
	sx, sy = g.projectCorner(x, y)
	pickedX, pickedY, ok = g.pickCorner(sx, sy)
	if !ok || pickedX != x || pickedY != y {
		t.Fatal("raised surface picks wrong corner", pickedX, pickedY, ok)
	}
	if _, _, ok := g.pickCorner(24, 190); ok {
		t.Fatal("interface area admitted terrain editing")
	}
}
