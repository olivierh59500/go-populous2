package game

import (
	"fmt"
	"math"

	"go-populous2/internal/populous2"
)

// NativePresentationPilot plays through the ordinary mouse and keyboard
// adapters. It reads the current board to choose actions, but never writes
// terrain, resources, actors, commands or the outcome of a match.
type NativePresentationPilot struct {
	Stage                   string
	Population, Opponent    int64
	Mana                    uint32
	Towns, OpponentTowns    int
	Actions, TerrainActions int
	SpellActions, Heroes    int
	// TerrainEdit observes completed input actions for presentation review.
	// It cannot change the runtime or the planner's decisions.
	TerrainEdit func(NativePresentationTerrainEdit)

	queue          []nativePilotAction
	x, y           float64
	started        bool
	gameStarted    int
	nextPlan       int
	lastAttack     int
	failed         map[int]int
	rallied        bool
	marched        bool
	rallyTick      int
	castPending    bool
	castMana       uint32
	heroSent       bool
	finalBattle    bool
	fightAttempt   int
	expeditionDone bool
	terrain        nativePilotTerrainPlan
}

type nativePilotAction struct {
	x, y         int
	left, right  bool
	keys         []uint8
	ticks        int
	ready        bool
	vertex       int
	beforeHeight int
}

// NativePresentationTerrainEdit records a mouse action and its resulting
// vertex height. A failed click has equal Before and After heights.
type NativePresentationTerrainEdit struct {
	Tick, X, Y, Before, After int
	Lower                     bool
}

type nativePilotActor struct {
	x, y, kind, stage, state, owner int
	population                      int64
}

type nativePilotBoard struct {
	heights                                   [65 * 65]int
	actors                                    []nativePilotActor
	owner, cameraX, cameraY, view, tool, mode int
	projectionX, projectionY                  int
	god                                       int
}

// A town keeps its chosen altitude while it expands. Shared vertices retain
// their owner until that town disappears, so neighboring plateaus cannot
// repeatedly spend mana undoing each other's construction.
type nativePilotTerrainProject struct {
	x, y, altitude, radius, priority int
	standing                         bool
}

type nativePilotTerrainPlan struct {
	projects map[int]nativePilotTerrainProject
	owners   [65 * 65]int
	targets  [65 * 65]int
	claimed  [65 * 65]bool
	active   int
	started  bool
}

func NewNativePresentationPilot() *NativePresentationPilot { return &NativePresentationPilot{} }

func (p *NativePresentationPilot) Status(g *NativeGame) string {
	return fmt.Sprintf("%s; population %d/%d, towns %d/%d, mana %d, actions %d (%d successful terrain edits, %d spell debits, %d heroes)", p.Stage, p.Population, p.Opponent, p.Towns, p.OpponentTowns, p.Mana, p.Actions, p.TerrainActions, p.SpellActions, p.Heroes)
}

// Next returns one PAL tick of physical input. Mouse travel, click holds,
// releases and deliberation all consume normal game time.
func (p *NativePresentationPilot) Next(g *NativeGame) (NativeInput, error) {
	if g == nil || g.Host == nil {
		return NativeInput{}, fmt.Errorf("presentation pilot has no native game")
	}
	if !p.started {
		p.started = true
		p.x, p.y, p.Stage = 155, 95, "Choosing a conquest"
		p.failed = make(map[int]int)
	}
	if len(p.queue) != 0 {
		return p.advanceAction(g)
	}
	if g.Frame == nil {
		if g.Updates < 100 || g.Updates < p.nextPlan {
			return p.idle(), nil
		}
		action := 4 // Original Conquest action, not the demonstration mode.
		if g.Director.Selection != nil {
			p.Stage = "Reviewing the first conquest"
			action = 6 // Proceed in the original world chooser.
		}
		x, y, err := g.nativeActionPosition(action)
		if err != nil {
			// A palette or resource transition may not have a requester yet.
			p.nextPlan = g.Updates + 15
			return p.idle(), nil
		}
		p.click(x+3, y+3, false, -1, 0)
		p.nextPlan = g.Updates + 120
		return p.advanceAction(g)
	}
	if p.gameStarted == 0 {
		p.gameStarted = g.Updates
		p.Stage = "Building a strong settlement economy"
		p.nextPlan = g.Updates + 35
	}
	if g.Result.Result != nil {
		if p.Stage != "The conquest result" {
			if _, err := p.readBoard(g); err != nil {
				return p.idle(), err
			}
		}
		p.Stage = "The conquest result"
		return p.idle(), nil
	}
	if g.Updates < p.nextPlan || g.Host.Session.Phase != populous2.NativeFrameSessionIdle {
		return p.idle(), nil
	}
	b, err := p.readBoard(g)
	if err != nil {
		return p.idle(), err
	}
	if p.castPending {
		if p.Mana+400 < p.castMana {
			p.SpellActions++
		}
		p.castPending = false
	}
	if p.planExpedition(g, b) {
		return p.advanceAction(g)
	}
	if !p.rallied && b.mode != int(populous2.NativeFollowerSettle) && g.Updates-p.lastAttack > 800 {
		p.Stage = "Keeping the settlements productive"
		p.key(0x5a) // Native Home/keypad left parenthesis selects settle.
		return p.advanceAction(g)
	}
	if g.Updates-p.gameStarted > 3500 && g.Updates-p.lastAttack > 900 && p.Towns >= 3 && p.Mana > 1800 {
		if p.planAttack(g, b) {
			p.lastAttack = g.Updates
			return p.advanceAction(g)
		}
	}
	if p.planLand(g, b) {
		return p.advanceAction(g)
	}
	p.Stage = "Watching the population grow"
	p.nextPlan = g.Updates + 35
	return p.idle(), nil
}

func (p *NativePresentationPilot) idle() NativeInput {
	return NativeInput{X: int(math.Round(p.x)), Y: int(math.Round(p.y))}
}

func (p *NativePresentationPilot) click(x, y int, right bool, vertex, height int) {
	p.queue = append(p.queue, nativePilotAction{x: x, y: y, left: !right, right: right, ticks: 8, vertex: vertex, beforeHeight: height})
}

func (p *NativePresentationPilot) key(raw uint8) {
	p.queue = append(p.queue, nativePilotAction{x: int(p.x), y: int(p.y), keys: []uint8{raw}, ticks: 8, vertex: -1})
}

func (p *NativePresentationPilot) advanceAction(g *NativeGame) (NativeInput, error) {
	a := &p.queue[0]
	dx, dy := float64(a.x)-p.x, float64(a.y)-p.y
	distance := math.Hypot(dx, dy)
	if distance > 0.7 {
		step := math.Min(distance, math.Max(2.0, math.Min(10, distance*0.24)))
		p.x += dx * step / distance
		p.y += dy * step / distance
		return p.idle(), nil
	}
	p.x, p.y = float64(a.x), float64(a.y)
	if !a.ready {
		a.ready = true
		return p.idle(), nil
	}
	if a.ticks > 0 {
		var wires []uint8
		if a.ticks == 8 {
			for _, raw := range a.keys {
				wire, err := populous2.NativeKeyWire(raw, true)
				if err != nil {
					return p.idle(), err
				}
				wires = append(wires, wire)
			}
		}
		a.ticks--
		return NativeInput{X: a.x, Y: a.y, Left: a.left, Right: a.right, Keys: wires}, nil
	}
	if a.ticks > -8 {
		in := p.idle()
		if a.ticks == 0 {
			for _, raw := range a.keys {
				wire, err := populous2.NativeKeyWire(raw, false)
				if err != nil {
					return in, err
				}
				in.Keys = append(in.Keys, wire)
			}
		}
		a.ticks--
		return in, nil
	}
	p.Actions++
	if a.vertex >= 0 {
		h, err := p.height(g, a.vertex%65, a.vertex/65)
		if err != nil {
			return p.idle(), err
		}
		if h == a.beforeHeight {
			p.failed[a.vertex] = g.Updates + 450
		} else {
			p.TerrainActions++
		}
		if p.TerrainEdit != nil {
			p.TerrainEdit(NativePresentationTerrainEdit{Tick: g.Updates, X: a.vertex % 65, Y: a.vertex / 65, Before: a.beforeHeight, After: h, Lower: a.right})
		}
	}
	p.queue = p.queue[1:]
	p.nextPlan = g.Updates + 6
	return p.idle(), nil
}

func (p *NativePresentationPilot) height(g *NativeGame, x, y int) (int, error) {
	c := populous2.NativeFrameRegisterContext{}
	c.D[0], c.D[1] = uint32(x), uint32(y)
	if err := g.Rules.Selection.Render.TerrainHeight(g.Host.Memory.BSS, &c); err != nil {
		return 0, err
	}
	return int(int32(c.D[2])), nil
}

func (p *NativePresentationPilot) readBoard(g *NativeGame) (nativePilotBoard, error) {
	var b nativePilotBoard
	m := g.Host.Memory.BSS
	read := func(at int) int { v, _ := m.Read16(at); return int(int16(v)) }
	b.owner, b.cameraX, b.cameraY = read(0xeb42), read(0x5f44), read(0x5f46)
	b.view, b.tool = read(0xf0c), read(0xeb18)
	b.god = 0xe76a + 314*b.owner
	b.mode = read(b.god + 12)
	b.projectionX, _ = func() (int, error) { v, e := g.Host.Memory.Code.Read16(0xe458); return int(int16(v)), e }()
	b.projectionY, _ = func() (int, error) { v, e := g.Host.Memory.Code.Read16(0xe45a); return int(int16(v)), e }()
	p.Mana, _ = m.Read32(b.god)
	p.Population, p.Opponent, p.Towns, p.OpponentTowns, p.Heroes = 0, 0, 0, 0, 0
	for y := 0; y <= 64; y++ {
		for x := 0; x <= 64; x++ {
			h, err := p.height(g, x, y)
			if err != nil {
				return b, err
			}
			b.heights[x+y*65] = h
		}
	}
	for slot := 1; slot < 400; slot++ {
		at := 0x76c0 + slot*52
		owner, err := m.Read8(at + 12)
		if err != nil {
			return b, err
		}
		if owner < 1 || owner > 2 {
			continue
		}
		pop, err := m.Read32(at + 26)
		if err != nil {
			return b, err
		}
		if int32(pop) <= 0 {
			continue
		}
		x, _ := m.Read8(at + 6)
		y, _ := m.Read8(at + 8)
		kind, _ := m.Read8(at)
		stage, _ := m.Read8(at + 1)
		state, _ := m.Read8(at + 22)
		a := nativePilotActor{x: int(x), y: int(y), kind: int(kind), stage: int(stage), state: int(state), owner: int(owner), population: int64(pop)}
		b.actors = append(b.actors, a)
		if int(owner) == b.owner {
			p.Population += a.population
			flags, _ := m.Read8(at + 13)
			if flags&2 != 0 {
				p.Heroes++
			}
			if state == 6 {
				p.Towns++
			}
		} else {
			p.Opponent += a.population
			if state == 6 {
				p.OpponentTowns++
			}
		}
	}
	return b, nil
}

func (b *nativePilotBoard) project(x, y int) (int, int, bool) {
	dx, dy := x-b.cameraX, y-b.cameraY
	shift := b.view/2 - 4
	dx, dy = dx+shift, dy+shift
	sx := b.projectionX + 16*(dx-dy)
	sy := b.projectionY + 8*(dx+dy) - 8*b.heights[x+y*65] + 2
	return sx, sy, dx >= 1 && dy >= 1 && dx <= b.view-1 && dy <= b.view-1 && sx >= 120 && sx < 310 && sy >= 40 && sy < 177
}

func (p *NativePresentationPilot) center(x, y int) {
	// Right-clicking the overview chooses its native camera target directly.
	p.click(68+x-y, 4+(x+y)/2, true, -1, 0)
}

// sync assigns each vertex to one settlement project. Completed parcels remain
// reserved as long as their settlement exists; a new neighboring town inherits
// its existing borders instead of proposing the opposite edit there.
func (p *NativePresentationPilot) syncTerrainPlan(b nativePilotBoard) [65 * 65]bool {
	plan := &p.terrain
	if !plan.started {
		plan.started, plan.active = true, -1
		plan.projects = make(map[int]nativePilotTerrainProject)
	}
	live := make(map[int]nativePilotTerrainProject)
	var protected [65 * 65]bool
	for _, a := range b.actors {
		if a.owner != b.owner || a.x < 0 || a.y < 0 || a.x >= 64 || a.y >= 64 {
			continue
		}
		if a.state == 6 {
			for _, delta := range [4]int{0, 1, 65, 66} {
				protected[a.x+a.y*65+delta] = true
			}
		}
		if a.state != 6 && a.kind != 2 {
			continue
		}
		key := a.x + a.y*65
		project, existed := plan.projects[key]
		if !existed {
			project = nativePilotTerrainProject{x: a.x, y: a.y, altitude: max(1, b.heights[key])}
		}
		project.radius, project.standing, project.priority = 1, a.state == 6, 90
		if project.standing {
			project.radius, project.priority = 3, 115+min(a.stage, 12)*2
			// A disaster can move an entire standing town to another altitude.
			// Adopt that new support level, rather than trying to rebuild the old
			// level through the occupied parcel. Ordinary border edits never
			// change these globally protected four corners.
			alt := b.heights[key]
			flat := alt > 0
			for _, delta := range [4]int{0, 1, 65, 66} {
				protected[key+delta] = true
				flat = flat && b.heights[key+delta] == alt
			}
			if flat && project.altitude != alt {
				project.altitude = alt
				for v, owner := range plan.owners {
					if plan.claimed[v] && owner == key {
						plan.claimed[v] = false
					}
				}
			}
		}
		live[key] = project
	}
	plan.projects = live
	if _, ok := live[plan.active]; !ok {
		plan.active = -1
	}
	for v, owner := range plan.owners {
		project, exists := live[owner]
		x, y := v%65, v/65
		if !exists || x < project.x-project.radius || x > project.x+project.radius+1 || y < project.y-project.radius || y > project.y+project.radius+1 {
			plan.claimed[v] = false
		}
	}
	// Nearest support parcel wins new borders, with a coordinate tie-break.
	// Population changes and actor enumeration cannot reverse that decision.
	for y := 0; y <= 64; y++ {
		for x := 0; x <= 64; x++ {
			v := x + y*65
			if plan.claimed[v] || protected[v] {
				continue
			}
			best, distance := -1, 1000
			for key, project := range live {
				if x < project.x-project.radius || x > project.x+project.radius+1 || y < project.y-project.radius || y > project.y+project.radius+1 {
					continue
				}
				d := max(project.x-x, 0, x-project.x-1) + max(project.y-y, 0, y-project.y-1)
				if d < distance || d == distance && (best < 0 || key < best) {
					best, distance = key, d
				}
			}
			if best >= 0 {
				plan.claimed[v], plan.owners[v], plan.targets[v] = true, best, live[best].altitude
			}
		}
	}
	return protected
}

// terrainEdit predicts the eight-neighbor slope propagation of one native
// click. Every changed reserved vertex must move toward its own agreed target,
// and none of the standing friendly towns may lose a supporting corner. This
// prediction reads a copy: execution still goes through ordinary mouse input.
func (p *NativePresentationPilot) terrainEdit(b nativePilotBoard, protected [65 * 65]bool, vertex, target int) ([65 * 65]int, bool) {
	heights := b.heights
	direction := 1
	if heights[vertex] > target {
		direction = -1
	}
	var edit func(int) bool
	edit = func(v int) bool {
		before, after := heights[v], heights[v]+direction
		if after < 0 || after > 8 || protected[v] {
			return false
		}
		if p.terrain.claimed[v] && abs(after-p.terrain.targets[v]) >= abs(before-p.terrain.targets[v]) {
			return false
		}
		x, y := v%65, v/65
		for _, delta := range [8][2]int{{0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}} {
			xx, yy := x+delta[0], y+delta[1]
			if xx < 0 || yy < 0 || xx > 64 || yy > 64 {
				continue
			}
			n := xx + yy*65
			if direction*(after-heights[n]) > 1 && !edit(n) {
				return false
			}
		}
		heights[v] = after
		return true
	}
	safe := edit(vertex)
	return heights, safe
}

// terrainChoice finishes one town's useful parcels before moving to another.
// A blocked border is left as a terrace; it is reconsidered when terrain or
// settlement occupancy changes, not after an arbitrary retry delay.
func (p *NativePresentationPilot) terrainChoice(b nativePilotBoard, tick int) (int, int, bool) {
	protected := p.syncTerrainPlan(b)
	plan := &p.terrain
	bestVertex, bestScore, bestTarget, bestOwner := -1, -1, 0, -1
	choose := func(activeOnly bool) {
		for v, claimed := range plan.claimed {
			if !claimed || protected[v] || b.heights[v] == plan.targets[v] || p.failed[v] > tick || activeOnly && plan.owners[v] != plan.active {
				continue
			}
			x, y, target := v%65, v/65, plan.targets[v]
			project := plan.projects[plan.owners[v]]
			score := project.priority - (abs(x-project.x)+abs(y-project.y))*9 - abs(b.heights[v]-target)*3
			if _, _, visible := b.project(x, y); visible {
				score += 18
			}
			for yy := max(0, y-1); yy <= min(63, y); yy++ {
				for xx := max(0, x-1); xx <= min(63, x); xx++ {
					matches := 0
					for _, delta := range [4]int{0, 1, 65, 66} {
						corner := xx + yy*65 + delta
						if corner != v && b.heights[corner] == target {
							matches++
						}
					}
					if matches == 3 {
						score += 22
					}
				}
			}
			if score <= bestScore {
				continue
			}
			if _, safe := p.terrainEdit(b, protected, v, target); !safe {
				continue
			}
			bestVertex, bestScore, bestTarget, bestOwner = v, score, target, plan.owners[v]
		}
	}
	if plan.active >= 0 {
		choose(true)
	}
	if bestVertex < 0 {
		choose(false)
	}
	if bestVertex < 0 {
		plan.active = -1
		return 0, 0, false
	}
	plan.active = bestOwner
	return bestVertex, bestTarget, true
}

func (p *NativePresentationPilot) planLand(g *NativeGame, b nativePilotBoard) bool {
	if p.Mana < 30 {
		return false
	}
	vertex, target, ok := p.terrainChoice(b, g.Updates)
	if !ok {
		return false
	}
	x, y := vertex%65, vertex/65
	sx, sy, visible := b.project(x, y)
	if !visible {
		p.center(x, y)
		p.Stage = "Moving to the next settlement"
		return true
	}
	if b.tool != 2 {
		p.key(0x50)
		p.key(0x01)
		p.Stage = "Selecting terrain construction"
		return true
	}
	p.Stage = "Completing coherent settlement terraces"
	p.click(sx, sy, b.heights[vertex] > target, vertex, b.heights[vertex])
	return true
}

func (p *NativePresentationPilot) planAttack(g *NativeGame, b nativePilotBoard) bool {
	// Choose an unlocked offensive power and keep a reserve for construction.
	for _, slot := range []int{8, 18, 19, 14, 24, 25} {
		flag, err := g.Host.Memory.BSS.Read8(b.god + 0x70 + slot)
		if err != nil || int8(flag) <= 0 {
			continue
		}
		c := populous2.NativeCommandRegisterContext{}
		c.D[0], c.D[1] = uint32(slot), uint32(b.owner)
		if err := g.Rules.Selection.Render.Commands.Cost(&c, g.Host.Memory.BSS); err != nil || uint32(c.D[0])*4+600 > p.Mana {
			continue
		}
		best := nativePilotActor{}
		for _, a := range b.actors {
			if a.owner != b.owner && a.state == 6 && a.population > best.population {
				best = a
			}
		}
		if best.population == 0 {
			continue
		}
		p.Stage = "Spending earned mana on the opponent"
		p.center(best.x, best.y)
		p.key(uint8(0x50 + slot/6))
		p.key(uint8(1 + slot%6))
		// The camera change takes place through the queued overview click.
		view := b
		view.cameraX, view.cameraY = max(0, min(56, best.x-3)), max(0, min(56, best.y-3))
		sx, sy, visible := view.project(best.x, best.y)
		if visible {
			p.click(sx, sy, false, -1, 0)
			p.castPending = true
			p.castMana = p.Mana
		}
		return true
	}
	return false
}

func (p *NativePresentationPilot) planExpedition(g *NativeGame, b nativePilotBoard) bool {
	// A mature economy can send a strong force while its settlements keep
	// producing. The rally, march and release use the original magnet/modes.
	if !p.finalBattle && p.Population > p.Opponent*2 && p.Towns >= 10 {
		flag, _ := g.Host.Memory.BSS.Read8(b.god + 0x74)
		c := populous2.NativeCommandRegisterContext{}
		c.D[0], c.D[1] = 4, uint32(b.owner)
		if int8(flag) > 0 && g.Rules.Selection.Render.Commands.Cost(&c, g.Host.Memory.BSS) == nil && c.D[0]*4+500 < p.Mana {
			p.Stage = "Committing the stronger army to the final battle"
			p.key(0x50)
			p.key(0x05)
			p.finalBattle = true
			p.castPending = true
			p.castMana = p.Mana
			return true
		}
	}
	if p.rallied && !p.heroSent && g.Updates-p.rallyTick > 1200 {
		leader, _ := g.Host.Memory.BSS.Read16(b.god + 10)
		if leader != 0 {
			at := 0x76c0 + int(int16(leader))
			pop, _ := g.Host.Memory.BSS.Read32(at + 26)
			flag, _ := g.Host.Memory.BSS.Read8(b.god + 0x72)
			c := populous2.NativeCommandRegisterContext{}
			c.D[0], c.D[1] = 2, uint32(b.owner)
			if pop > 1200 && int8(flag) > 0 && g.Rules.Selection.Render.Commands.Cost(&c, g.Host.Memory.BSS) == nil && c.D[0]*4+500 < p.Mana {
				p.Stage = "Creating Perseus from the rallied leader"
				p.key(0x50)
				p.key(0x03)
				p.heroSent = true
				p.castPending = true
				p.castMana = p.Mana
				return true
			}
		}
	}
	if !p.rallied {
		if g.Updates-p.gameStarted < 6500 || p.Towns < 8 || p.Population < p.Opponent*3/2 {
			return false
		}
		leader, _ := g.Host.Memory.BSS.Read16(b.god + 10)
		if leader == 0 {
			return false
		}
		at := 0x76c0 + int(int16(leader))
		x, _ := g.Host.Memory.BSS.Read8(at + 6)
		y, _ := g.Host.Memory.BSS.Read8(at + 8)
		p.Stage = "Rallying a strong expedition"
		p.center(int(x), int(y))
		p.key(0x50)
		p.key(0x02)
		view := b
		view.cameraX, view.cameraY = max(0, min(56, int(x)-3)), max(0, min(56, int(y)-3))
		sx, sy, ok := view.project(int(x), int(y))
		if !ok {
			return false
		}
		p.click(sx, sy, false, -1, 0)
		p.key(0x5d) // Native keypad multiply selects magnet mode.
		p.rallied = true
		p.rallyTick = g.Updates
		return true
	}
	if !p.marched {
		if g.Updates-p.rallyTick < 1500 {
			return false
		}
		best := nativePilotActor{}
		for _, a := range b.actors {
			if a.owner != b.owner && a.population > best.population {
				best = a
			}
		}
		if best.population == 0 {
			return false
		}
		p.Stage = "Leading the expedition towards the opponent"
		p.center(best.x, best.y)
		p.key(0x50)
		p.key(0x02)
		view := b
		view.cameraX, view.cameraY = max(0, min(56, best.x-3)), max(0, min(56, best.y-3))
		sx, sy, ok := view.project(best.x, best.y)
		if !ok {
			return false
		}
		p.click(sx, sy, false, -1, 0)
		p.marched = true
		p.rallyTick = g.Updates
		return true
	}
	if !p.expeditionDone && g.Updates-p.rallyTick > 1500 && b.mode != int(populous2.NativeFollowerFight) && g.Updates-p.fightAttempt > 500 {
		p.Stage = "Engaging the opponent while towns keep producing"
		p.key(0x5c) // Native keypad divide selects fight mode.
		p.fightAttempt = g.Updates
		return true
	}
	if g.Updates-p.rallyTick > 3000 && p.heroSent && !p.expeditionDone {
		p.Stage = "Letting the hero fight while settlers resume building"
		p.key(0x5a)
		p.expeditionDone = true
		return true
	}
	return false
}
