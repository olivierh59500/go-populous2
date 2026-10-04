package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

const BasaltActorKind uint8 = 0x3a

type BasaltRules struct {
	BaseLife, DelayModulus, DelayBias int
	Directions                        [4]uint16
	Geometry                          [256]uint8
	Frames                            map[int]AnimationFrame
	SequenceLength, LoopOffset        int
}

// BasaltState preserves the native direction word at effect-record offset
// $1a. Values 0/2/4/6 are byte offsets into the north/east/south/west table.
type BasaltState struct {
	Directions [NativeEffectCapacity]uint16
}

func DecodeBasaltRules(exe *amiga.Executable) (BasaltRules, error) {
	var rules BasaltRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33612 {
		return rules, fmt.Errorf("native basalt tables missing")
	}
	code := exe.Hunks[0].Data
	rules.BaseLife = int(binary.BigEndian.Uint16(code[0x172a2:]))
	rules.DelayModulus = int(binary.BigEndian.Uint16(code[0x172a0:]))
	rules.DelayBias = int(binary.BigEndian.Uint16(code[0x1727e:]))
	if rules.BaseLife < 1 || rules.BaseLife > 32000 || rules.DelayModulus < 1 || rules.DelayModulus > 32767 || rules.DelayBias < 1 || rules.DelayBias+rules.DelayModulus > 32767 {
		return BasaltRules{}, fmt.Errorf("invalid native basalt lifetime or delay")
	}
	for index := range rules.Directions {
		rules.Directions[index] = binary.BigEndian.Uint16(code[0x17298+index*2:])
	}
	copy(rules.Geometry[:], code[0x33512:0x33612])
	frames, err := DecodeAnimation(exe, 0x5ec)
	if err != nil {
		return BasaltRules{}, err
	}
	rules.SequenceLength = len(frames)
	rules.Frames = make(map[int]AnimationFrame, len(frames))
	for index, frame := range frames {
		rules.Frames[0x5ec+index*4] = frame
	}
	rules.LoopOffset = int(int16(binary.BigEndian.Uint16(code[0x23d1a+0x5ec+len(frames)*4:])))
	if rules.LoopOffset != -4*len(frames) {
		return BasaltRules{}, fmt.Errorf("invalid native basalt animation loop")
	}
	return rules, nil
}

type BasaltCallbacks struct {
	// ReadGeometry returns the entire byte from the native $33512 table for
	// the current tile code. A zero shape nibble alone does not admit basalt.
	ReadGeometry func(int, int) uint8
	WriteTile    func(int, int, uint8)
	Random       func() int
	// Link translates $125a0: insert this effect at the cell's chain head,
	// preserving existing occupants and the raw terrain header byte.
	Link func(int)
	// Unlink translates $125da after the actor's owner byte becomes zero.
	Unlink func(int)
}

// Create translates $171ea. User casts supply BaseLife+WaterXP; propagation
// supplies the parent's remaining life without adding experience again.
// Failure does not consume randomness, change terrain, or mutate the pool.
func (rules BasaltRules) Create(pool *[NativeEffectCapacity]NativeEffectActor, state *BasaltState, player, x, y int, life int16, direction uint16, callbacks BasaltCallbacks) (int, bool) {
	if pool == nil || state == nil || player < 0 || player > 1 || !inside(x, y) || direction&1 != 0 || direction > 6 || callbacks.ReadGeometry == nil || callbacks.WriteTile == nil || callbacks.Random == nil || callbacks.Link == nil || rules.DelayModulus < 1 {
		return -1, false
	}
	if callbacks.ReadGeometry(x, y) != 0 {
		return -1, false
	}
	for index := range pool {
		actor := &pool[index]
		if actor.Active {
			continue
		}
		callbacks.WriteTile(x, y, 0xe0)
		// Native creation leaves speed and velocity bytes of a reused slot
		// intact; they do not drive this stationary propagation controller.
		vx, vy, speed := actor.VX, actor.VY, actor.Speed
		*actor = NativeEffectActor{Active: true, Kind: BasaltActorKind, Player: uint8(player), X: int16(x*256 + 128), Y: int16(y*256 + 128), VX: vx, VY: vy, Speed: speed, Life: life, Animation: 0x5ec, State: 0x38}
		state.Directions[index] = direction
		actor.Timer = int16(callbacks.Random()%rules.DelayModulus + rules.DelayBias)
		callbacks.Link(index)
		return index, true
	}
	return -1, false
}

type BasaltStep struct {
	Finished     bool
	ChildCreated bool
	ChildIndex   int
}

// Tick translates $158a6. A child is allocated while the parent is still
// active. The caller iterates shared slots in their original ascending order,
// so a child in a later slot can run again during the same simulation pass.
func (rules BasaltRules) Tick(pool *[NativeEffectCapacity]NativeEffectActor, state *BasaltState, index int, callbacks BasaltCallbacks) (BasaltStep, error) {
	step := BasaltStep{ChildIndex: -1}
	if pool == nil || state == nil || index < 0 || index >= len(pool) {
		return step, fmt.Errorf("invalid native basalt slot")
	}
	actor := &pool[index]
	if !actor.Active {
		return step, nil
	}
	direction := state.Directions[index]
	if actor.Kind != BasaltActorKind || (actor.State != 0x38 && actor.State != 0x3a) || direction&1 != 0 || direction > 6 || callbacks.Unlink == nil || callbacks.ReadGeometry == nil || callbacks.WriteTile == nil || callbacks.Random == nil || callbacks.Link == nil || rules.SequenceLength < 1 {
		return step, fmt.Errorf("invalid native basalt actor or callbacks")
	}
	finish := func() {
		actor.State = 0x3a
		actor.Active = false
		callbacks.Unlink(index)
		step.Finished = true
	}
	if actor.State == 0x3a {
		finish()
		return step, nil
	}
	previousLife := actor.Life
	actor.Life--
	// Compare the original signed word to preserve SUBI.W/BGT overflow:
	// -32768 stores 32767 but still takes the native termination branch.
	if previousLife <= 1 {
		finish()
		return step, nil
	}
	actor.Animation += 4
	if actor.Animation >= 0x5ec+rules.SequenceLength*4 {
		actor.Animation += rules.LoopOffset
	}
	previousTimer := actor.Timer
	actor.Timer--
	if previousTimer > 1 {
		return step, nil
	}
	origin := uint16(actor.Y)&0xff00 | uint16(uint8(uint16(actor.X)>>8))
	child := origin + rules.Directions[direction/2]
	step.ChildIndex, step.ChildCreated = rules.Create(pool, state, int(actor.Player), int(uint8(child)), int(uint8(child>>8)), actor.Life, direction, callbacks)
	// $15902 always reaches $158ae/$15908, including rejected children.
	// Persistent $e0 terrain is not cleared when this actor is unlinked.
	finish()
	return step, nil
}
