package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

const WallCapacity = 200

type WallArt struct {
	Animation int
	Variant   uint8
}

// WallRules preserves the connection and animation tables used by $1626c.
// Walls occupy actors over the ground; they do not replace terrain tile codes.
type WallRules struct {
	Offsets         [4][2]int
	Art             [16]WallArt
	Properties      [256]uint16
	RoadConnections [20]uint8
	Frames          map[int]AnimationFrame
	BreakBase       int32
	ClimbBase       int32
	BreakAnimations [5]int
}

func DecodeWallRules(exe *amiga.Executable) (WallRules, error) {
	var rules WallRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33512 {
		return rules, fmt.Errorf("native wall tables missing")
	}
	code := exe.Hunks[0].Data
	rules.Frames = make(map[int]AnimationFrame)
	for index := range rules.Art {
		at := 0x332c8 + index*4
		art := WallArt{Animation: int(binary.BigEndian.Uint16(code[at:])), Variant: code[at+3]}
		if binary.BigEndian.Uint16(code[at+2:]) > 255 {
			return WallRules{}, fmt.Errorf("wall connection %d has invalid variant", index)
		}
		rules.Art[index] = art
		if err := rules.addAnimation(exe, art.Animation); err != nil {
			return WallRules{}, err
		}
	}
	for index := range rules.Offsets {
		n := int(int16(binary.BigEndian.Uint16(code[0x33308+index*2:])))
		x := int(int8(byte(n)))
		rules.Offsets[index] = [2]int{x, (n - x) / 256}
		if abs(x)+abs(rules.Offsets[index][1]) != 1 {
			return WallRules{}, fmt.Errorf("invalid wall neighbor offset")
		}
	}
	if binary.BigEndian.Uint16(code[0x33310:]) != 0xff9d {
		return WallRules{}, fmt.Errorf("wall neighbor table terminator missing")
	}
	for index := range rules.Properties {
		rules.Properties[index] = binary.BigEndian.Uint16(code[0x33312+index*2:])
	}
	copy(rules.RoadConnections[:], code[0x1691c:0x16930])
	rules.BreakBase = int32(binary.BigEndian.Uint32(code[0x20d60:]))
	rules.ClimbBase = int32(binary.BigEndian.Uint32(code[0x20d5c:]))
	for index := range rules.BreakAnimations {
		rules.BreakAnimations[index] = int(binary.BigEndian.Uint16(code[0x20f1e+index*2:]))
		if err := rules.addAnimation(exe, rules.BreakAnimations[index]); err != nil {
			return WallRules{}, err
		}
	}
	for _, animation := range []int{0xb44, 0xb54} {
		if err := rules.addAnimation(exe, animation); err != nil {
			return WallRules{}, err
		}
	}
	return rules, nil
}

func (rules *WallRules) addAnimation(exe *amiga.Executable, offset int) error {
	if _, exists := rules.Frames[offset]; exists {
		return nil
	}
	frames, err := DecodeAnimation(exe, offset)
	if err != nil {
		return fmt.Errorf("wall animation $%x: %w", offset, err)
	}
	for index, frame := range frames {
		rules.Frames[offset+index*4] = frame
	}
	return nil
}

// WallActor corresponds to one native sixteen-byte record in $5f50-$6bd0.
// Next and Heads use slot+1 references; zero means no record.
type WallActor struct {
	Active    bool
	Broken    bool
	Player    uint8
	X, Y      int
	Variant   uint8
	Animation int
	Next      uint16
}

type WallState struct {
	Actors [WallCapacity]WallActor
	Heads  [2]uint16
}

// Validate checks saved references and drawable actor state without following
// the lists. Native disconnected-cast heads can be inactive or self-linked;
// a later allocation by the other deity can also reuse that referenced slot.
func (state *WallState) Validate(rules *WallRules) error {
	if state == nil || rules == nil {
		return fmt.Errorf("native wall state or rules missing")
	}
	for player, head := range state.Heads {
		if head > WallCapacity {
			return fmt.Errorf("wall head for player %d exceeds actor pool", player)
		}
	}
	for index, actor := range state.Actors {
		if actor.Next > WallCapacity {
			return fmt.Errorf("wall %d has an out-of-pool reference", index)
		}
		if !actor.Active {
			continue
		}
		if actor.Player > 1 || !inside(actor.X, actor.Y) {
			return fmt.Errorf("wall %d has invalid ownership or coordinates", index)
		}
		if _, found := rules.Frames[actor.Animation]; !found {
			return fmt.Errorf("wall %d has an unknown native animation", index)
		}
	}
	return nil
}

// At returns an active wall's slot, or -1 when the cell contains no wall.
func (state *WallState) At(x, y int) int {
	for index, actor := range state.Actors {
		if actor.Active && !actor.Broken && actor.X == x && actor.Y == y {
			return index
		}
	}
	return -1
}

// Place follows $1626c, including the original pre-validation head update.
// A failed disconnected placement therefore leaves an inactive head record.
// The native first-wall exception is per deity, while neighboring walls can
// belong to either side. The caller owns affordability and mana debiting.
func (state *WallState) Place(rules *WallRules, player, x, y int, tileAt func(int, int) uint8) bool {
	if rules == nil || tileAt == nil || player < 0 || player > 1 || !inside(x, y) {
		return false
	}
	tile := tileAt(x, y)
	mask := uint16(0x43)
	if player == 1 {
		mask = 0x45
	}
	if rules.Properties[tile]&mask == 0 || state.At(x, y) >= 0 {
		return false
	}
	slot := -1
	for index, actor := range state.Actors {
		if !actor.Active {
			slot = index
			break
		}
	}
	if slot < 0 {
		return false
	}
	state.Actors[slot].Next = state.Heads[player]
	state.Heads[player] = uint16(slot + 1)
	connections := uint8(0)
	neighbors := [4]int{-1, -1, -1, -1}
	for index, offset := range rules.Offsets {
		connections <<= 1
		xx, yy := x+offset[0], y+offset[1]
		if !inside(xx, yy) {
			continue
		}
		neighbors[index] = state.At(xx, yy)
		if neighbors[index] >= 0 {
			connections |= 1
		}
	}
	if connections == 0 && state.Actors[slot].Next != 0 {
		return false
	}
	art := rules.Art[connections]
	state.Actors[slot] = WallActor{Active: true, Player: uint8(player), X: x, Y: y, Variant: art.Variant, Animation: art.Animation, Next: state.Actors[slot].Next}
	for _, index := range neighbors {
		if index < 0 || art.Variant == 0 {
			continue
		}
		neighbor := &state.Actors[index]
		switch neighbor.Variant {
		case 2:
			if art.Animation == 0x5cc {
				continue
			}
		case 4:
			if art.Animation == 0x5dc {
				continue
			}
		default:
			continue
		}
		neighbor.Animation, neighbor.Variant = 0x5bc, 8
	}
	// A road under the new actor selects an actual gate, after neighbor joins.
	if rules.Properties[tile]&0x40 != 0 {
		if tile < 197 || int(tile)-197 >= len(rules.RoadConnections) {
			return true
		}
		state.Actors[slot].Animation, state.Actors[slot].Variant = 0xb44, 8
		if rules.RoadConnections[int(tile)-197]&5 != 0 {
			state.Actors[slot].Animation, state.Actors[slot].Variant = 0xb54, 6
		}
	}
	return true
}

// Tick follows $161cc: unsuitable ground destroys an actor, and construction
// animations advance once then hold their terminal frame instead of looping.
func (state *WallState) Tick(rules *WallRules, tileAt func(int, int) uint8) {
	if rules == nil || tileAt == nil {
		return
	}
	for index := range state.Actors {
		actor := &state.Actors[index]
		if !actor.Active {
			continue
		}
		if !inside(actor.X, actor.Y) || rules.Properties[tileAt(actor.X, actor.Y)]&0x77 == 0 {
			if actor.Player < 2 && state.Heads[actor.Player] == uint16(index+1) {
				state.Heads[actor.Player] = actor.Next
			}
			actor.Active = false
			continue
		}
		if _, exists := rules.Frames[actor.Animation+4]; exists {
			actor.Animation += 4
		}
	}
}

func (rules *WallRules) Layers(actor WallActor) []SpriteLayer {
	if rules == nil || !actor.Active {
		return nil
	}
	return rules.Frames[actor.Animation].Layers
}

type WallCrossing uint8

const (
	WallBlocked WallCrossing = iota
	WallPass
	WallClimb
	WallBreak
)

type WallDecision struct {
	Crossing      WallCrossing
	HeroState     uint8
	HeroAnimation int
}

// DecideCrossing follows $141a2/$14226 without mutating candidate state.
// Experience belongs to the walker, not the wall's deity. MOVE.B preserves
// the upper bytes left by the native deity-index MULU #314; retaining this
// original register behavior produces side-dependent strength thresholds.
// Neither gate variants nor hero status bypass the enemy-wall comparison.
func (rules *WallRules) DecideCrossing(walkerPlayer, wallPlayer int, walkerEarthXP uint8, population int, hero bool) WallDecision {
	decision := WallDecision{Crossing: WallBlocked}
	if rules == nil || walkerPlayer < 0 || walkerPlayer > 1 || wallPlayer < 0 || wallPlayer > 1 || population <= 0 {
		return decision
	}
	if walkerPlayer == wallPlayer {
		decision.Crossing = WallPass
		return decision
	}
	register := (uint32((walkerPlayer+1)*314) & 0xffffff00) | uint32(walkerEarthXP)
	bonus := register << 7
	strength := int32(population)
	if strength > int32(bonus+uint32(rules.BreakBase)) {
		decision.Crossing = WallBreak
		if hero {
			decision.HeroState = 0x2a
			decision.HeroAnimation = 0x7cc
			if walkerPlayer == 1 {
				decision.HeroAnimation = 0x7d4
			}
		}
	} else if strength > int32(bonus+uint32(rules.ClimbBase)) {
		decision.Crossing = WallClimb
	}
	return decision
}

// Break starts the original variant-specific destruction art and removes only
// the intact-wall collision state. Native kind $1c retains its actor and head
// references; candidate movement checks must call DecideCrossing instead.
func (state *WallState) Break(rules *WallRules, index int) bool {
	if rules == nil || index < 0 || index >= len(state.Actors) {
		return false
	}
	actor := &state.Actors[index]
	if !actor.Active || actor.Broken || actor.Variant > 8 || actor.Variant%2 != 0 {
		return false
	}
	actor.Broken = true
	actor.Animation = rules.BreakAnimations[actor.Variant/2]
	return true
}
