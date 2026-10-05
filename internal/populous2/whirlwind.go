package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type WhirlwindRules struct {
	BaseLife, WhirlpoolChance int
	Speed                     uint8
	Neighbors                 [16][2]int
	Vectors                   [16][2]int16
	Geometry                  [256]uint8
	PickupAnimations          [6]int
	ReleaseOffsets            [8][2]int
	Frames                    map[int]AnimationFrame
	SequenceLengths           map[int]int
	LoopOffset                int
}

func DecodeWhirlwindRules(exe *amiga.Executable) (WhirlwindRules, error) {
	var rules WhirlwindRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33612 {
		return rules, fmt.Errorf("native whirlwind tables missing")
	}
	code := exe.Hunks[0].Data
	rules.BaseLife = int(binary.BigEndian.Uint16(code[0x20d48:]))
	rules.Speed = code[0x20d4b]
	rules.WhirlpoolChance = int(binary.BigEndian.Uint16(code[0x20d78:]))
	if rules.BaseLife < 1 || rules.BaseLife > 32000 || rules.Speed == 0 || rules.WhirlpoolChance < 1 {
		return WhirlwindRules{}, fmt.Errorf("invalid native whirlwind parameters")
	}
	offset := func(at int) [2]int {
		n := int(int16(binary.BigEndian.Uint16(code[at:])))
		x := int(int8(byte(n)))
		return [2]int{x, (n - x) / 256}
	}
	for index := range rules.Neighbors {
		rules.Neighbors[index] = offset(0x20dd6 + index*2)
		rules.Vectors[index] = [2]int16{int16(binary.BigEndian.Uint16(code[0x20df6+index*4:])), int16(binary.BigEndian.Uint16(code[0x20df6+index*4+2:]))}
	}
	copy(rules.Geometry[:], code[0x33512:0x33612])
	for index := range rules.PickupAnimations {
		rules.PickupAnimations[index] = int(binary.BigEndian.Uint16(code[0x20a6c+index*2:]))
	}
	for index := range rules.ReleaseOffsets {
		rules.ReleaseOffsets[index] = offset(0x20efa + index*2)
	}
	rules.Frames = make(map[int]AnimationFrame)
	rules.SequenceLengths = make(map[int]int)
	for _, start := range []int{0x4c8, 0x6cc} {
		frames, err := DecodeAnimation(exe, start)
		if err != nil {
			return WhirlwindRules{}, err
		}
		rules.SequenceLengths[start] = len(frames)
		for index, frame := range frames {
			rules.Frames[start+index*4] = frame
		}
	}
	rules.LoopOffset = int(int16(binary.BigEndian.Uint16(code[0x23d1a+0x4c8+rules.SequenceLengths[0x4c8]*4:])))
	return rules, nil
}

// Create translates $15c3e. It consumes no random numbers and preserves the
// velocity words of a reused slot until its first active reroute.
func (rules *WhirlwindRules) Create(pool *[NativeEffectCapacity]NativeEffectActor, player, x, y int, airExperience uint8) bool {
	if rules == nil || pool == nil || player < 0 || player > 1 || !inside(x, y) {
		return false
	}
	for index := range pool {
		actor := &pool[index]
		if actor.Active {
			continue
		}
		vx, vy := actor.VX, actor.VY
		*actor = NativeEffectActor{Active: true, Kind: 0x20, Player: uint8(player), X: int16(x*256 + 128), Y: int16(y*256 + 128), VX: vx, VY: vy, Speed: rules.Speed, Timer: 1, Life: int16(rules.BaseLife + int(airExperience)), State: 8, Animation: 0x4c8}
		return true
	}
	return false
}

type WhirlwindStep struct {
	Moved, SpawnWhirlpool, Finished bool
	X, Y                            int
}

// Tick translates phases $14a24/$14a50/$14c02 without flame damage. The caller
// owns original water-only whirlpool creation, actor lifting/town collapse,
// and release of captured followers when Finished is reported.
func (rules *WhirlwindRules) Tick(actor *NativeEffectActor, random func() int, terrain func(int, int) (int, uint8)) WhirlwindStep {
	var step WhirlwindStep
	if rules == nil || actor == nil || !actor.Active || random == nil || terrain == nil {
		return step
	}
	if actor.State == 8 {
		if actor.Animation+4 < 0x4c8+rules.SequenceLengths[0x4c8]*4 {
			actor.Animation += 4
			return step
		}
		actor.State, actor.Animation = 10, 0x4c8
	}
	if actor.State == 10 {
		actor.Life--
		if actor.Life <= 0 {
			actor.State, actor.Animation = 12, 0x6cc
		} else {
			actor.Animation += 4
			if actor.Animation >= 0x4c8+rules.SequenceLengths[0x4c8]*4 {
				actor.Animation += rules.LoopOffset
			}
			actor.Timer--
			if actor.Timer <= 0 {
				rules.route(actor, random, terrain)
			}
			x, y := actor.X+actor.VX, actor.Y+actor.VY
			if x < 0 || y < 0 || x >= 0x4000 || y >= 0x4000 {
				actor.Active = false
				step.Finished = true
				step.X, step.Y = int(actor.X)>>8, int(actor.Y)>>8
				return step
			}
			actor.X, actor.Y = x, y
			step.Moved = true
			step.X, step.Y = int(x)>>8, int(y)>>8
			step.SpawnWhirlpool = random()%rules.WhirlpoolChance == 0
			return step
		}
	}
	if actor.State == 12 {
		next := actor.Animation + 4
		if next >= 0x6cc+rules.SequenceLengths[0x6cc]*4 {
			actor.Active = false
			step.Finished = true
			step.X, step.Y = int(actor.X)>>8, int(actor.Y)>>8
		} else {
			actor.Animation = next
		}
	}
	return step
}

func (rules *WhirlwindRules) route(actor *NativeEffectActor, random func() int, terrain func(int, int) (int, uint8)) {
	x, y := int(actor.X)>>8, int(actor.Y)>>8
	height, _ := terrain(x, y)
	bits := uint16(random())
	start, selected := int(bits&0xe)/2, 0
	for offset := 0; offset < 8; offset++ {
		index := start + offset
		delta := rules.Neighbors[index]
		xx, yy := x+delta[0], y+delta[1]
		if !inside(xx, yy) {
			continue
		}
		neighbor, code := terrain(xx, yy)
		neighbor += int(rules.Geometry[code] & 1)
		if neighbor > height {
			continue
		}
		if neighbor == height {
			accept := bits&1 != 0
			bits >>= 1
			if !accept {
				continue
			}
		}
		height, selected = neighbor, index*4
	}
	if selected == 0 {
		selected = int(bits & 0x3c)
	}
	vector := rules.Vectors[selected/4]
	actor.VX, actor.VY = vector[0]*int16(actor.Speed), vector[1]*int16(actor.Speed)
	// $14b34 computes 255/speed, but $14b3c overwrites that result with RNG.
	actor.Timer = int16(random() & 0x78)
}
