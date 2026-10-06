package game

import "testing"

func nativePilotTerrainTestBoard() nativePilotBoard {
	b := nativePilotBoard{owner: 1, tool: 2, view: 8, cameraX: 7, cameraY: 7, projectionX: 192, projectionY: 72}
	for v := range b.heights {
		b.heights[v] = 1
	}
	return b
}

func nativePilotTerrainTestRun(t *testing.T, p *NativePresentationPilot, b *nativePilotBoard, limit int) (int, map[int]int) {
	t.Helper()
	changes := 0
	directions := make(map[int]int)
	for tick := 0; tick < limit; tick++ {
		vertex, target, ok := p.terrainChoice(*b, tick)
		if !ok {
			return changes, directions
		}
		protected := p.syncTerrainPlan(*b)
		heights, safe := p.terrainEdit(*b, protected, vertex, target)
		if !safe || heights[vertex] == b.heights[vertex] {
			t.Fatalf("chosen edit does not make safe progress at vertex %d", vertex)
		}
		for v, h := range heights {
			if h == b.heights[v] {
				continue
			}
			if protected[v] {
				t.Fatalf("standing town support changed at vertex %d", v)
			}
			direction := 1
			if h < b.heights[v] {
				direction = -1
			}
			if earlier, edited := directions[v]; edited && earlier != direction {
				t.Fatalf("vertex %d reverses its completed construction (%d to %d)", v, earlier, direction)
			}
			directions[v] = direction
			changes++
		}
		b.heights = heights
	}
	t.Fatalf("terrain planner did not finish within %d ordinary edits", limit)
	return 0, nil
}

func TestNativePresentationPilotNeighboringTerracesDoNotAlternate(t *testing.T) {
	p := NewNativePresentationPilot()
	b := nativePilotTerrainTestBoard()
	// This is a legal one-level ramp. Both towns want the same border for
	// different plateaus; the completed lower plateau must remain useful.
	for y := 0; y <= 64; y++ {
		for x := 13; x <= 64; x++ {
			b.heights[x+y*65] = 2
		}
	}
	b.actors = []nativePilotActor{
		{x: 10, y: 10, owner: 1, kind: 4, state: 6, stage: 10},
		{x: 13, y: 10, owner: 1, kind: 4, state: 6, stage: 10},
	}
	// A depression and a small ridge require real, opposite kinds of work.
	b.heights[9+9*65] = 0
	b.heights[15+9*65] = 3
	changes, directions := nativePilotTerrainTestRun(t, p, &b, 300)
	if changes < 2 || directions[9+9*65] != 1 || directions[15+9*65] != -1 {
		t.Fatalf("town development did not repair both terraces: %d, %v", changes, directions)
	}
	beforeTargets := p.terrain.targets
	// Rendering/actor order, population and town growth must not change the
	// ownership of a completed border or cause another mana-consuming click.
	b.actors[0], b.actors[1] = b.actors[1], b.actors[0]
	b.actors[0].stage++
	for tick := 300; tick < 450; tick++ {
		if v, target, ok := p.terrainChoice(b, tick); ok {
			t.Fatalf("completed neighboring towns started another edit: %d -> %d", v, target)
		}
	}
	if beforeTargets != p.terrain.targets {
		t.Fatal("completed shared-border targets changed with actor order")
	}
}

func TestNativePresentationPilotProtectsEveryTownAndRecursiveSlope(t *testing.T) {
	p := NewNativePresentationPilot()
	b := nativePilotTerrainTestBoard()
	b.actors = []nativePilotActor{
		{x: 10, y: 10, owner: 1, kind: 4, state: 6},
		{x: 13, y: 10, owner: 1, kind: 4, state: 6},
		{x: 0, y: 0, owner: 1, kind: 4, state: 6},
		{x: 63, y: 63, owner: 1, kind: 4, state: 6},
	}
	protected := p.syncTerrainPlan(b)
	for _, a := range b.actors {
		for _, delta := range [4]int{0, 1, 65, 66} {
			v := a.x + a.y*65 + delta
			if _, safe := p.terrainEdit(b, protected, v, 2); safe {
				t.Fatalf("town %d,%d support %d can be directly edited", a.x, a.y, v)
			}
		}
	}
	// A height-3 click outside either town would recursively raise their
	// height-1 corners. Reject it even though the clicked vertex is unoccupied.
	v := 12 + 10*65
	b.heights[v] = 2
	p.terrain.targets[v] = 3
	if _, safe := p.terrainEdit(b, protected, v, 3); safe {
		t.Fatal("recursive terrain propagation can destroy a neighboring town")
	}
}

func TestNativePresentationPilotRemovedTownReleasesItsReservations(t *testing.T) {
	p := NewNativePresentationPilot()
	b := nativePilotTerrainTestBoard()
	b.actors = []nativePilotActor{{x: 10, y: 10, owner: 1, kind: 4, state: 6}}
	p.syncTerrainPlan(b)
	vertex := 12 + 10*65
	if !p.terrain.claimed[vertex] || p.terrain.targets[vertex] != 1 {
		t.Fatal("first town did not reserve its border")
	}
	// Once that town is genuinely gone, another altitude can be developed
	// there. Old completed work is not permanently made unusable.
	b.actors = []nativePilotActor{{x: 13, y: 10, owner: 1, kind: 4, state: 6}}
	for _, delta := range [4]int{0, 1, 65, 66} {
		b.heights[13+10*65+delta] = 2
	}
	changes, _ := nativePilotTerrainTestRun(t, p, &b, 300)
	if changes == 0 || p.terrain.targets[vertex] != 2 || b.heights[vertex] != 2 {
		t.Fatal("a removed town permanently blocks a surviving town's expansion")
	}
}

func TestNativePresentationPilotNewTownAndDisasterResumeUsefulWork(t *testing.T) {
	p := NewNativePresentationPilot()
	b := nativePilotTerrainTestBoard()
	b.actors = []nativePilotActor{{x: 10, y: 10, owner: 1, kind: 4, state: 6}}
	b.heights[9+9*65] = 0
	if changes, _ := nativePilotTerrainTestRun(t, p, &b, 100); changes == 0 {
		t.Fatal("first project never completes terrain construction")
	}
	// A fresh disturbance of completed land should be repaired to the same
	// altitude, without adopting the attack's terrain as a new goal.
	b.heights[9+9*65] = 0
	if changes, _ := nativePilotTerrainTestRun(t, p, &b, 100); changes == 0 || b.heights[9+9*65] != 1 {
		t.Fatal("external terrain damage did not trigger a useful repair")
	}
	// A second town must still start a project after the first one is done.
	b.actors = append(b.actors, nativePilotActor{x: 22, y: 10, owner: 1, kind: 4, state: 6})
	b.heights[21+9*65] = 0
	if changes, _ := nativePilotTerrainTestRun(t, p, &b, 100); changes == 0 || b.heights[21+9*65] != 1 {
		t.Fatal("new settlement cannot develop after a completed project")
	}
	// An actual relocated standing footprint is different from a disputed
	// border. The town's newly flat support level becomes its new altitude.
	for _, delta := range [4]int{0, 1, 65, 66} {
		b.heights[22+10*65+delta] = 2
	}
	changes, _ := nativePilotTerrainTestRun(t, p, &b, 300)
	if changes == 0 || p.terrain.projects[22+10*65].altitude != 2 {
		t.Fatal("a standing town's new altitude was not adopted after external change")
	}
}
