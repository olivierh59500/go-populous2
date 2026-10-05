package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeFollowerMode uint16

const (
	NativeFollowerSettle NativeFollowerMode = 14
	NativeFollowerMagnet NativeFollowerMode = 16
	NativeFollowerJoin   NativeFollowerMode = 18
	NativeFollowerFight  NativeFollowerMode = 20
)

type FollowerDecisionRules struct {
	SearchCounts   [19]int
	Preferred      [48][2]int
	Neighbors      [16][2]int
	RoadDirections [16][2]int8
	Properties     [256]uint16
	RoadBonus      uint8
	Motion         FollowerMotionRules
}

func DecodeFollowerDecisionRules(exe *amiga.Executable) (FollowerDecisionRules, error) {
	var rules FollowerDecisionRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33512 {
		return rules, fmt.Errorf("native follower decision tables missing")
	}
	code := exe.Hunks[0].Data
	var err error
	rules.Motion, err = DecodeFollowerMotionRules(exe)
	if err != nil {
		return rules, err
	}
	offset := func(at int) [2]int {
		n := int(int16(binary.BigEndian.Uint16(code[at:])))
		x := int(int8(byte(n)))
		return [2]int{x, (n - x) / 256}
	}
	for index := range rules.SearchCounts {
		rules.SearchCounts[index] = int(binary.BigEndian.Uint16(code[0x20bc0+index*2:])) + 1
		if rules.SearchCounts[index] < 1 || rules.SearchCounts[index] > len(rules.Preferred) {
			return FollowerDecisionRules{}, fmt.Errorf("invalid native follower search count")
		}
	}
	for index := range rules.Preferred {
		rules.Preferred[index] = offset(0x20be6 + index*2)
	}
	for index := range rules.Neighbors {
		rules.Neighbors[index] = offset(0x20c46 + index*2)
		at := 0x20c66 + index*2
		rules.RoadDirections[index] = [2]int8{int8(code[at+1]), int8(code[at])}
	}
	for index := range rules.Properties {
		rules.Properties[index] = binary.BigEndian.Uint16(code[0x33312+index*2:])
	}
	bonus := binary.BigEndian.Uint16(code[0x2075a:])
	if bonus > 255 {
		return FollowerDecisionRules{}, fmt.Errorf("invalid native road speed bonus")
	}
	rules.RoadBonus = uint8(bonus)
	return rules, nil
}

type FollowerDecisionRecord struct {
	Kind, Owner uint8 // Native owner bytes, including inactive owner zero.
	Next        NativeRecordReference
}

type FollowerDecisionCallbacks struct {
	Context *NativeFollowerRegisterContext
	// Leave Prepass nil when the motion dispatcher already ran $12c3c.
	// Attrition and nonmoving death states belong to that caller's dispatch.
	Prepass func(*FollowerMotionActor) bool
	// Record must retain inactive records and native linked-list order.
	Record func(NativeRecordReference) (FollowerDecisionRecord, bool)
	Random func() int
}

type FollowerDecisionKind uint8

const (
	FollowerNoTarget FollowerDecisionKind = iota
	FollowerPreferredTarget
	FollowerPressureTarget
	FollowerRoadTarget
	FollowerMagnetHandler
	FollowerHeroHandler
)

type FollowerDecision struct {
	Kind             FollowerDecisionKind
	TargetX, TargetY int16
	Fallthrough      bool
}

// Select translates $1134e-$1156c after caller-owned attrition. State 2 searches
// for a direction; it does not settle on its current cell. Settlement occurs
// later in committed entry at $1275a/$12bd8. Unknown raw deity modes follow the
// original default search; only 16, 18 and 20 have special comparisons here.
func (rules *FollowerDecisionRules) Select(actor *FollowerMotionActor, searchIndex uint8, mode NativeFollowerMode, cells *NativeOccupancyState, callbacks FollowerDecisionCallbacks) (FollowerDecision, error) {
	var result FollowerDecision
	if rules == nil || actor == nil || cells == nil || actor.Player > 1 || !inside(int(actor.X)>>8, int(actor.Y)>>8) || searchIndex > 36 || searchIndex%2 != 0 {
		return result, fmt.Errorf("invalid native follower decision input")
	}
	if callbacks.Prepass != nil && !callbacks.Prepass(actor) {
		return result, nil
	}
	if actor.Flags&2 != 0 {
		result.Kind = FollowerHeroHandler
		return result, nil
	}
	if mode == NativeFollowerMagnet {
		result.Kind = FollowerMagnetHandler
		return result, nil
	}
	if callbacks.Context != nil {
		callbacks.Context.Long4(0)
	}
	x, y := int(actor.X)>>8, int(actor.Y)>>8
	wanted := uint8(0)
	if mode == NativeFollowerJoin {
		wanted = actor.Player + 1
	}
	if mode == NativeFollowerFight {
		wanted = (actor.Player ^ 1) + 1
	}
	selected := 0
	for _, delta := range rules.Preferred[:rules.SearchCounts[searchIndex/2]] {
		xx, yy := x+delta[0], y+delta[1]
		if !inside(xx, yy) {
			continue
		}
		index := xx + yy*64
		cell := cells.Cells[index]
		blocked, matched, err := decisionChain(cell.Head, callbacks.Record, func(record FollowerDecisionRecord) (bool, bool) {
			if record.Kind == 24 {
				return true, false
			}
			if record.Owner != wanted {
				return false, false
			}
			if wanted == actor.Player+1 {
				return false, record.Kind == 2
			}
			return false, record.Kind == 2 || record.Kind == 4
		})
		if err != nil {
			return result, err
		}
		if blocked {
			continue
		}
		if matched {
			selected = index * 4
			if callbacks.Context != nil {
				callbacks.Context.Word4(uint16(selected))
			}
			break
		}
		if rules.Properties[cell.Tile]&1 != 0 {
			selected = index * 4
			if callbacks.Context != nil {
				callbacks.Context.Word4(uint16(selected))
			}
			if wanted == 0 {
				break
			}
		}
	}
	if selected != 0 {
		result.Kind = FollowerPreferredTarget
		return rules.beginDecision(actor, selected, false, result, callbacks.Context)
	}
	if callbacks.Random == nil {
		return result, fmt.Errorf("native follower decision RNG missing")
	}
	bits := uint16(callbacks.Random())
	if callbacks.Context != nil {
		callbacks.Context.Long4(uint32(uint8(-followerMotionSign(int(actor.VX)))))
		callbacks.Context.Long5(uint32(uint8(-followerMotionSign(int(actor.VY)))))
	}
	start := int(bits&14) / 2
	pressure := uint8(255)
	road := 0
	for offset := 0; offset < 8; offset++ {
		position := start + offset
		delta := rules.Neighbors[position]
		xx, yy := x+delta[0], y+delta[1]
		if !inside(xx, yy) {
			continue
		}
		index := xx + yy*64
		cell := cells.Cells[index]
		property := rules.Properties[cell.Tile]
		if property&8 != 0 {
			continue
		}
		direction := rules.RoadDirections[position]
		if property&64 != 0 && direction != [2]int8{} && direction != [2]int8{int8(-followerMotionSign(int(actor.VX))), int8(-followerMotionSign(int(actor.VY)))} {
			road = index * 4
			selected = road
			// Native priority occurs before the boulder scan. Even a zero
			// address jumps here; A5 zero then omits the road speed bonus.
			result.Kind = FollowerPressureTarget
			if road != 0 {
				result.Kind = FollowerRoadTarget
			}
			return rules.beginDecision(actor, selected, road != 0, result, callbacks.Context)
		}
		blocked, _, err := decisionChain(cell.Head, callbacks.Record, func(record FollowerDecisionRecord) (bool, bool) { return record.Kind == 24, false })
		if err != nil {
			return result, err
		}
		if blocked {
			continue
		}
		candidate := cell.Header & 248
		if candidate > pressure || candidate == pressure && bits&(1<<uint(7-offset)) == 0 {
			continue
		}
		pressure, selected = candidate, index*4
	}
	if selected == 0 {
		return result, nil
	}
	result.Kind = FollowerPressureTarget
	return rules.beginDecision(actor, selected, false, result, callbacks.Context)
}

func (rules *FollowerDecisionRules) beginDecision(actor *FollowerMotionActor, address int, road bool, result FollowerDecision, context *NativeFollowerRegisterContext) (FollowerDecision, error) {
	result.TargetX, result.TargetY = int16((address&255)<<6), int16(address&0xff00)
	if road {
		actor.Speed = uint8(min(255, int(actor.Speed)+int(rules.RoadBonus)))
		if context != nil {
			context.Long5(uint32(actor.Speed))
		}
	}
	err := rules.Motion.BeginLegWithContext(actor, result.TargetX, result.TargetY, context)
	if road {
		// $11556 subtracts the full bonus even after saturation. Starting
		// speed240..255 therefore becomes235 while velocity remains255.
		actor.Speed -= rules.RoadBonus
		if context != nil && err == nil {
			context.Word5(uint16(rules.RoadBonus))
		}
	}
	if err != nil {
		return result, err
	}
	if actor.VX == 0 && actor.VY == 0 {
		return result, fmt.Errorf("native stationary decision triggers illegal instruction")
	}
	actor.State, actor.ReturnState = 4, 2
	result.Fallthrough = true
	return result, nil
}

func decisionChain(head NativeRecordReference, lookup func(NativeRecordReference) (FollowerDecisionRecord, bool), visit func(FollowerDecisionRecord) (blocked, matched bool)) (bool, bool, error) {
	seen := make(map[NativeRecordReference]bool)
	for reference := head; reference != 0; {
		if lookup == nil || seen[reference] {
			return false, false, fmt.Errorf("missing or cyclic native decision record")
		}
		seen[reference] = true
		record, ok := lookup(reference)
		if !ok {
			return false, false, fmt.Errorf("unknown native decision record %04x", uint16(reference))
		}
		blocked, matched := visit(record)
		if blocked || matched {
			return blocked, matched, nil
		}
		reference = record.Next
	}
	return false, false, nil
}
