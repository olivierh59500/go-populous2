package game

import (
	"testing"

	"go-populous2/internal/populous2"
)

func TestNativePresentationPilotEmitsSingleKeyboardTransitions(t *testing.T) {
	p := NewNativePresentationPilot()
	p.x, p.y = 100, 100
	p.key(0x54)
	g := &NativeGame{}
	var wires []uint8
	for tick := 0; tick < 40 && len(p.queue) > 0; tick++ {
		in, err := p.advanceAction(g)
		if err != nil {
			t.Fatal(err)
		}
		wires = append(wires, in.Keys...)
	}
	down, _ := populous2.NativeKeyWire(0x54, true)
	up, _ := populous2.NativeKeyWire(0x54, false)
	if len(wires) != 2 || wires[0] != down || wires[1] != up {
		t.Fatal("presentation keys must use one original wire press and release", wires)
	}
	if len(p.queue) != 0 || p.Actions != 1 {
		t.Fatal("key action did not finish", p.Actions)
	}
}

func TestNativePresentationPilotMouseTravelsBeforeClicking(t *testing.T) {
	p := NewNativePresentationPilot()
	p.x, p.y = 40, 40
	p.click(240, 110, false, -1, 0)
	g := &NativeGame{}
	clicked := false
	for tick := 0; tick < 120 && len(p.queue) > 0; tick++ {
		beforeX, beforeY := p.x, p.y
		in, err := p.advanceAction(g)
		if err != nil {
			t.Fatal(err)
		}
		if abs(int(p.x-beforeX)) > 11 || abs(int(p.y-beforeY)) > 11 {
			t.Fatal("cursor teleported")
		}
		if in.Left {
			clicked = true
			if in.X != 240 || in.Y != 110 {
				t.Fatal("click occurred during cursor travel", in)
			}
		}
	}
	if !clicked || len(p.queue) != 0 {
		t.Fatal("mouse action did not finish")
	}
}

func TestNativePresentationPilotTerrainAtWorldEdge(t *testing.T) {
	p := NewNativePresentationPilot()
	p.Mana = 1000
	p.failed = make(map[int]int)
	b := nativePilotBoard{owner: 1, tool: 2, view: 8, cameraX: 56, cameraY: 56, projectionX: 192, projectionY: 72}
	b.actors = []nativePilotActor{{x: 61, y: 61, owner: 1, kind: 4, state: 6, stage: 10, population: 1000}}
	b.heights[61+61*65] = 1
	if !p.planLand(&NativeGame{}, b) || len(p.queue) == 0 {
		t.Fatal("edge settlement did not produce a legal plan")
	}
	for _, a := range p.queue {
		if a.vertex >= 65*65 {
			t.Fatal("terrain plan left the original height grid", a.vertex)
		}
	}
}
