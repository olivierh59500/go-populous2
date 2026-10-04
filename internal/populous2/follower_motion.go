package populous2

import (
	"encoding/binary"
	"errors"
	"fmt"

	"go-populous2/internal/amiga"
)

var ErrFollowerZeroSpeed = errors.New("native follower speed is zero")

// FollowerMotionActor retains motion and link fields of a native 52-byte
// follower record. Player uses Go sides 0/1; links remain native signed byte
// offsets from $76c0, rather than follower IDs. Variant is the even selector
// stored at record+$32. Nonmotion fields survive a motion update unchanged.
type FollowerMotionActor struct {
	Kind, Player, Flags     uint8
	Next, Previous, Variant uint16
	X, Y, VX, VY            int16
	Speed                   uint8
	Timer                   int16
	State, ReturnState      uint8
	Animation               int
	Population              int32
}

type FollowerMotionRules struct {
	FrameCount       int
	Bounce           [9][2]int8
	DirectionOffsets [8]int
	VariantBases     [2][8]int
	AngleShifts      [1024]uint8
	Angles           [1024]uint8
	Frames           map[int]AnimationFrame
}

func DecodeFollowerMotionRules(exe *amiga.Executable) (FollowerMotionRules, error) {
	var rules FollowerMotionRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x23d1a+0x169c {
		return rules, fmt.Errorf("native follower motion tables missing")
	}
	code := exe.Hunks[0].Data
	clock, err := DecodeAnimation(exe, 0)
	if err != nil {
		return rules, err
	}
	rules.FrameCount = len(clock)
	copy(rules.AngleShifts[:], code[0xf81a:0xfc1a])
	copy(rules.Angles[:], code[0xfc1a:0x1001a])
	for y := -1; y <= 1; y++ {
		for x := -1; x <= 1; x++ {
			at := 0x1171e + ((y+1)*4+x+1)*2
			// $116f8/$11706 use each lookup byte only for its sign. Two
			// bottom-row reads reach adjacent instruction bytes, whose signs
			// still select unit components; they are not vector magnitudes.
			rules.Bounce[(y+1)*3+x+1] = [2]int8{int8(followerMotionSign(int(int8(code[at])))), int8(followerMotionSign(int(int8(code[at+1]))))}
		}
	}
	for index := range rules.DirectionOffsets {
		rules.DirectionOffsets[index] = int(binary.BigEndian.Uint16(code[0x20d34+index*2:]))
	}
	rules.Frames = make(map[int]AnimationFrame)
	for player := range rules.VariantBases {
		for variant := range rules.VariantBases[player] {
			base := int(binary.BigEndian.Uint16(code[0x209e0+player*16+variant*2:]))
			rules.VariantBases[player][variant] = base
			for _, direction := range rules.DirectionOffsets {
				start := base + direction
				if _, exists := rules.Frames[start]; exists {
					continue
				}
				frames, err := DecodeAnimation(exe, start)
				if err != nil {
					return FollowerMotionRules{}, err
				}
				if len(frames) != rules.FrameCount {
					return FollowerMotionRules{}, fmt.Errorf("native walking animation has a different clock")
				}
				for index, frame := range frames {
					rules.Frames[start+index*4] = frame
				}
			}
		}
	}
	return rules, nil
}

// BeginLeg translates $13126. Different-cell targets use floor(256/speed);
// same-cell targets use the larger fractional distance. Native speed zero
// raises processor exception 5; Go reports it without changing the actor.
func (rules *FollowerMotionRules) BeginLeg(actor *FollowerMotionActor, targetX, targetY int16) error {
	if actor == nil {
		return fmt.Errorf("follower motion actor missing")
	}
	if actor.Speed == 0 {
		return ErrFollowerZeroSpeed
	}
	speed := int16(actor.Speed)
	if targetX>>8 == actor.X>>8 && targetY>>8 == actor.Y>>8 {
		x, y := int(uint8(targetX))-int(uint8(actor.X)), int(uint8(targetY))-int(uint8(actor.Y))
		actor.VX, actor.VY = int16(followerMotionSign(x))*speed, int16(followerMotionSign(y))*speed
		actor.Timer = int16(max(abs(x), abs(y)) / int(speed))
	} else {
		actor.VX = int16(followerMotionSign(int(targetX>>8)-int(actor.X>>8))) * speed
		actor.VY = int16(followerMotionSign(int(targetY>>8)-int(actor.Y>>8))) * speed
		actor.Timer = 256 / speed
	}
	return nil
}

type FollowerMotionCallbacks struct {
	// Prepass translates $12c3c before initial dispatch and timer-expiry
	// redispatch. A committed move does not run it on the new cell immediately.
	// Return false when the caller takes over a nonmotion hazard/death state.
	Prepass func(*FollowerMotionActor) bool
	// Decide translates the caller's state handler. Return true only when it
	// falls through into state 4 during this update; false ends the update.
	// For example, ordinary search falls through, while the magnet handler
	// sets state 4 but ends the update. Targeting, attrition, towns,
	// combat and hazards belong to those handlers, not this motion controller.
	Decide func(*FollowerMotionActor) bool
	// Admit runs only when the proposed position crosses a cell boundary.
	// Rejecting it applies the original bounce without committing the position.
	Admit func(*FollowerMotionActor, int16, int16) bool
	// Move runs after a committed cell crossing. The old coordinates let the
	// caller translate $12518 linked membership and subsequent entry/contact.
	Move func(*FollowerMotionActor, int16, int16)
}

type FollowerMotionStep struct {
	Moved, Crossed, Blocked, NeedDecision bool
}

// Tick translates ordinary state 4 at $1156c. Animation advances independently
// of speed. Timer expiry dispatches ReturnState immediately; a decision that
// starts another leg advances its animation and moves in the same update.
func (rules *FollowerMotionRules) Tick(actor *FollowerMotionActor, callbacks FollowerMotionCallbacks) FollowerMotionStep {
	var step FollowerMotionStep
	if rules == nil || actor == nil || rules.FrameCount <= 0 {
		return step
	}
	for {
		if callbacks.Prepass != nil && !callbacks.Prepass(actor) {
			return step
		}
		if actor.State != 4 {
			if callbacks.Decide == nil {
				step.NeedDecision = true
				return step
			}
			if !callbacks.Decide(actor) {
				return step
			}
			if actor.State != 4 {
				step.NeedDecision = true
				return step
			}
		}
		actor.Animation += 4
		if actor.Animation >= rules.FrameCount*4 {
			actor.Animation = 0
		}
		actor.Timer--
		if actor.Timer < 0 {
			actor.State = actor.ReturnState
			continue
		}
		x, y := actor.X+actor.VX, actor.Y+actor.VY
		crossed := x>>8 != actor.X>>8 || y>>8 != actor.Y>>8
		if crossed && (!inside(int(x)>>8, int(y)>>8) || callbacks.Admit != nil && !callbacks.Admit(actor, x, y)) {
			direction := rules.Bounce[(followerMotionSign(int(actor.VY))+1)*3+followerMotionSign(int(actor.VX))+1]
			actor.VX, actor.VY = int16(direction[0])*int16(actor.Speed), int16(direction[1])*int16(actor.Speed)
			step.Blocked = true
			return step
		}
		oldX, oldY := actor.X, actor.Y
		actor.X, actor.Y = x, y
		step.Moved, step.Crossed = true, crossed
		if crossed && callbacks.Move != nil {
			callbacks.Move(actor, oldX, oldY)
		}
		return step
	}
}

// Angle translates the original $f71c lookup, including its asymmetric shift
// registers. For high speeds, diagonal facing can differ from a sign-only
// direction lookup; drawing must follow the current velocity after a bounce.
func (rules *FollowerMotionRules) Angle(vx, vy int16) uint8 {
	x, y := int(vx), -int(vy)
	negativeX, negativeY := x < 0, y < 0
	x, y = abs(x), abs(y)
	if y > x {
		shift := uint(y >> 5)
		x, y = x>>shift, y>>shift
	} else {
		index := x >> 5
		if index >= len(rules.AngleShifts) {
			return 0
		}
		x, y = x>>rules.AngleShifts[index], y>>uint(index)
	}
	index := x*32 + y
	if index < 0 || index >= len(rules.Angles) {
		return 0
	}
	angle := int(rules.Angles[index])
	switch {
	case negativeX && negativeY:
		angle += 128
	case negativeX:
		angle = -angle
	case negativeY:
		angle = 128 - angle
	}
	return uint8(angle)
}

// Frame selects $e65c's ordinary-owner/variant bank and the current motion
// clock. Hero flags use a separate native image bank and are not interpreted
// as ordinary follower variants here.
func (rules *FollowerMotionRules) Frame(actor FollowerMotionActor) (AnimationFrame, int, bool) {
	if rules == nil || actor.Player > 1 || actor.Variant > 14 || actor.Variant%2 != 0 || actor.Flags&2 != 0 {
		return AnimationFrame{}, 0, false
	}
	direction := rules.Angle(actor.VX, actor.VY) >> 5
	animation := rules.VariantBases[actor.Player][actor.Variant/2] + rules.DirectionOffsets[direction] + actor.Animation
	frame, ok := rules.Frames[animation]
	return frame, animation, ok
}

func followerMotionSign(value int) int {
	if value < 0 {
		return -1
	}
	if value > 0 {
		return 1
	}
	return 0
}
