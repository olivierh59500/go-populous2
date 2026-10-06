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

func (p *NativePresentationPilot) planLand(g *NativeGame, b nativePilotBoard) bool {
	if p.Mana < 30 {
		return false
	}
	bestScore, bestX, bestY, bestTarget := -1, 0, 0, 0
	for _, a := range b.actors {
		if a.owner != b.owner || a.x < 2 || a.y < 2 || a.x > 61 || a.y > 61 {
			continue
		}
		if a.state != 6 && a.kind != 2 {
			continue
		}
		alt := b.heights[a.x+a.y*65]
		if alt == 0 {
			alt = 1
		}
		radius := 3
		if a.state != 6 {
			radius = 1
		}
		for yy := max(0, a.y-radius); yy <= min(64, a.y+radius+1); yy++ {
			for xx := max(0, a.x-radius); xx <= min(64, a.x+radius+1); xx++ {
				v := xx + yy*65
				if b.heights[v] == alt || p.failed[v] > g.Updates {
					continue
				}
				// Preserve the four corners underneath a standing settlement.
				if a.state == 6 && xx >= a.x && xx <= a.x+1 && yy >= a.y && yy <= a.y+1 {
					continue
				}
				dist := abs(xx-a.x) + abs(yy-a.y)
				score := 90 - dist*9 - abs(b.heights[v]-alt)*3
				if a.state == 6 {
					score += 25 + min(a.stage, 12)*2
				}
				if sx, sy, visible := b.project(xx, yy); visible && sx > 0 && sy > 0 {
					score += 18
				}
				// Finishing flat adjacent parcels produces food immediately.
				for ty := yy - 1; ty <= yy; ty++ {
					for tx := xx - 1; tx <= xx; tx++ {
						if tx < 0 || ty < 0 || tx >= 64 || ty >= 64 {
							continue
						}
						matches := 0
						for _, dv := range []int{0, 1, 65, 66} {
							if tx+ty*65+dv != v && b.heights[tx+ty*65+dv] == alt {
								matches++
							}
						}
						if matches == 3 {
							score += 22
						}
					}
				}
				if score > bestScore {
					bestScore, bestX, bestY, bestTarget = score, xx, yy, alt
				}
			}
		}
	}
	if bestScore < 0 {
		return false
	}
	sx, sy, visible := b.project(bestX, bestY)
	if !visible {
		p.center(bestX, bestY)
		p.Stage = "Moving to the next settlement"
		return true
	}
	if b.tool != 2 {
		p.key(0x50)
		p.key(0x01)
		p.Stage = "Selecting terrain construction"
		return true
	}
	p.Stage = "Flattening land for larger settlements"
	p.click(sx, sy, b.heights[bestX+bestY*65] > bestTarget, bestX+bestY*65, b.heights[bestX+bestY*65])
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
