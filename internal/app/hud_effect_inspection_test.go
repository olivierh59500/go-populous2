package app

import (
	"go-populous2/internal/engine"
	"testing"
)

func TestEffectInspectionNewAndHeldGesturesKeepCursorOwnerAndSelection(t *testing.T) {
	g := browserGame(t)
	s := g.World.Snapshot()
	for _, id := range []int{0, 2, 249} {
		s.Reservations[id] = engine.EffectReservation{Kind: engine.EffectFireColumn, Owner: 0, InspectionClass: engine.InspectFireColumn}
		s.World.Fire.Columns[id] = engine.FireEffect{Active: true, Owner: 0, X: (20+id%30)*256 + 255, Y: (25+id%20)*256 + 123}
	}
	s.Reservations[1] = engine.EffectReservation{Kind: engine.EffectFireColumn, Owner: 1, InspectionClass: engine.InspectFireColumn}
	s.World.Fire.Columns[1] = engine.FireEffect{Active: true, Owner: 1, X: 60 * 256, Y: 60 * 256}
	for _, id := range []int{0, 1, 2, 249} {
		effect := s.World.Fire.Columns[id]
		s.World.Actors.Link(engine.ActorRef{Kind: engine.ActorEffect, Index: uint16(id)}, effect.X, effect.Y)
	}
	var err error
	g.World, err = s.Restore()
	if err != nil {
		t.Fatal(err)
	}
	before := g.World.Snapshot()
	selected := g.SelectedFollower
	g.inspectEffectIcon(engine.FireColumn, true)
	if g.effectScanCursor != 0 || g.CameraX != 16 || g.CameraY != 21 {
		t.Fatal("new secondary press advanced before checking current effect slot")
	}
	g.inspectEffectIcon(engine.FireColumn, false)
	if g.effectScanCursor != 2 {
		t.Fatal("held scan did not advance or selected an enemy effect")
	}
	g.effectScanCursor = 249
	g.inspectEffectIcon(engine.FireColumn, false)
	if g.effectScanCursor != 0 {
		t.Fatal("effect scan did not wrap through slot zero")
	}
	if g.World.Snapshot() != before || g.SelectedFollower != selected {
		t.Fatal("effect inspection cast, spent mana, or selected a follower")
	}
}
