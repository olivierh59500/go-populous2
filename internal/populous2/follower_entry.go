package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

// FollowerEntryActor retains the fields touched by native entry, merging and
// battle preparation. Owner is the native byte, independently of Motion.Player.
// Names ending in a number retain original record offsets without guessing at
// fields whose broader state-machine meaning is not yet translated.
type FollowerEntryActor struct {
	Motion                                      FollowerMotionActor
	Owner, Byte1, Byte19, Weapon                uint8
	Contact30, Association34, AssociationBack36 uint16
	Hero40, Captive42, CaptiveBack44, Founded46 uint16
	Extra48                                     uint16
}

type FollowerEntryNode struct {
	Kind, Owner uint8
	Next        NativeRecordReference
}

type FollowerEntryRules struct {
	Properties      [256]uint16
	ReplyAnimations [6]int
	WaitingFrames   map[int]AnimationFrame
	WaitingLoops    map[int]int
	WaitingCues     map[int]uint16
}

func DecodeFollowerEntryRules(exe *amiga.Executable) (FollowerEntryRules, error) {
	var rules FollowerEntryRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33512 {
		return rules, fmt.Errorf("native follower entry tables missing")
	}
	code := exe.Hunks[0].Data
	for index := range rules.Properties {
		rules.Properties[index] = binary.BigEndian.Uint16(code[0x33312+index*2:])
	}
	for index := range rules.ReplyAnimations {
		rules.ReplyAnimations[index] = int(binary.BigEndian.Uint16(code[0x20a0c+index*2:]))
	}
	rules.WaitingFrames, rules.WaitingLoops, rules.WaitingCues = make(map[int]AnimationFrame), make(map[int]int), make(map[int]uint16)
	// Waiting expiry without an eligible enemy can leave state10 with
	// animation0. Its next pass reads the ordinary image bank at0.
	for _, start := range append([]int{0, 0xccc}, rules.ReplyAnimations[:]...) {
		terminated := false
		for index := range 256 {
			offset := start + index*4
			at := 0x23d1a + offset
			if at+4 > len(code) {
				return FollowerEntryRules{}, fmt.Errorf("native contact animation exceeds CODE")
			}
			image := int16(binary.BigEndian.Uint16(code[at:]))
			if image < 0 {
				loop := int(image)
				if index == 0 || offset+loop < start || offset+loop >= offset || loop%4 != 0 {
					return FollowerEntryRules{}, fmt.Errorf("invalid native contact animation loop")
				}
				rules.WaitingLoops[offset] = loop
				terminated = true
				break
			}
			layers, err := decodeImageLayers(code, uint16(image))
			if err != nil {
				return FollowerEntryRules{}, err
			}
			// Normal waiting's $cd0 frame contains raw cue $be0. Preserve it
			// even though it exceeds the generic animation cue catalog; entry
			// progression reads only image/loop words and emits no frame sound.
			cue := binary.BigEndian.Uint16(code[at+2:])
			rules.WaitingCues[offset] = cue
			rules.WaitingFrames[offset] = AnimationFrame{Layers: layers, SoundCue: int(cue) / 10}
		}
		if !terminated {
			return FollowerEntryRules{}, fmt.Errorf("unterminated native contact animation")
		}
	}
	return rules, nil
}

// FollowerEntryCallbacks preserve external native operations at their original
// call boundaries. EvaluateTown translates $13352, including farms and other
// affected town records, and returns the native stage0..18. ClearFarms is $135ca
// and ClearHeroLinks is $14654; supplying no-op versions is not a full port.
//
// ReadLong reads the original offset even for a14-byte scenery or16-byte wall
// record: population+$1a can alias a neighboring record. ReadWord/WriteWord also
// support bounded native hero-link aliases. Errors bound invalid memory rather
// than normalizing references. Write must preserve graph links except where
// Unlink explicitly removes the source; it synchronizes same-cell fractions.
type FollowerEntryCallbacks struct {
	Context        *NativeFollowerRegisterContext
	Head           func(NativePackedTile) (NativeRecordReference, error)
	Node           func(NativeRecordReference) (FollowerEntryNode, error)
	Read           func(NativeRecordReference) (FollowerEntryActor, error)
	Write          func(NativeRecordReference, FollowerEntryActor) error
	ReadWord       func(NativeRecordReference, uint16) (uint16, error)
	ReadLong       func(NativeRecordReference, uint16) (uint32, error)
	WriteWord      func(NativeRecordReference, uint16, uint16) error
	Tile           func(NativePackedTile) (uint8, error)
	GodMode        func(uint8) (uint16, error)
	Tick           func() uint16
	SetLeader      func(uint8, NativeRecordReference) error
	Selected       func() NativeRecordReference
	Select         func(NativeRecordReference) error
	Founded        func(uint8) error
	Unlink         func(NativeRecordReference) error
	EvaluateTown   func(NativeRecordReference) (int, error)
	ClearFarms     func(NativeRecordReference, uint8) error
	ClearHeroLinks func(NativeRecordReference) error
	Sound          func(uint16) error
}

type FollowerEntryOutcome uint8

const (
	FollowerEntryNone FollowerEntryOutcome = iota
	FollowerEntryHoming
	FollowerEntryMerged
	FollowerEntryBattle
	FollowerEntrySettlement
	FollowerEntryWaiting
)

type FollowerEntryStep struct {
	Outcome                          FollowerEntryOutcome
	Target                           NativeRecordReference
	WallObserved, SettlementCreated  bool
	NextFollower, ContinueTownUpdate bool
	RedispatchSearch, RedispatchHero bool
}

func (callbacks FollowerEntryCallbacks) validate() error {
	if callbacks.Head == nil || callbacks.Node == nil || callbacks.Read == nil || callbacks.Write == nil || callbacks.ReadWord == nil || callbacks.ReadLong == nil || callbacks.WriteWord == nil || callbacks.Tile == nil || callbacks.GodMode == nil || callbacks.Tick == nil || callbacks.SetLeader == nil || callbacks.Selected == nil || callbacks.Select == nil || callbacks.Founded == nil || callbacks.Unlink == nil || callbacks.EvaluateTown == nil || callbacks.ClearFarms == nil || callbacks.ClearHeroLinks == nil || callbacks.Sound == nil {
		return fmt.Errorf("incomplete native follower entry callbacks")
	}
	return nil
}

func entryTile(actor FollowerEntryActor) NativePackedTile {
	return NativePackedTile(uint16(actor.Motion.Y)&0xff00 | uint16(actor.Motion.X)>>8)
}

// Enter follows $1275a after committed movement. It does not replace homing
// contacts with an immediate merge/battle. Graph order determines tie-breaking.
func (rules FollowerEntryRules) Enter(reference NativeRecordReference, callbacks FollowerEntryCallbacks) (FollowerEntryStep, error) {
	var step FollowerEntryStep
	if err := callbacks.validate(); err != nil {
		return step, err
	}
	actor, err := callbacks.Read(reference)
	if err != nil || actor.Motion.Flags&8 != 0 {
		return step, err
	}
	head, err := callbacks.Head(entryTile(actor))
	if err != nil {
		return step, err
	}
	if head != 0 && callbacks.Context != nil {
		callbacks.Context.EntryScan()
	}
	priority := 0
	seen := make(map[NativeRecordReference]bool)
	for current := head; current != 0; {
		if seen[current] {
			return step, fmt.Errorf("cyclic native entry chain")
		}
		seen[current] = true
		node, err := callbacks.Node(current)
		if err != nil {
			return step, err
		}
		if current != reference && int8(node.Owner) > 0 {
			population, err := callbacks.ReadLong(current, 26)
			if err != nil {
				return step, err
			}
			if int32(population) > 0 {
				if node.Kind == 0x1a {
					step.WallObserved = true
				}
				if node.Kind == 4 && node.Owner == actor.Owner && actor.Motion.Flags&2 == 0 {
					step.Target, step.Outcome, step.NextFollower = current, FollowerEntryMerged, true
					return step, rules.merge(reference, current, callbacks)
				}
				candidate := 0
				if node.Kind == 4 && node.Owner != actor.Owner {
					candidate = 4
				} else if node.Kind == 2 {
					candidate = 2
					if node.Owner == actor.Owner {
						candidate = 6
					}
				}
				if candidate > priority {
					priority, step.Target = candidate, current
					if callbacks.Context != nil {
						callbacks.Context.EntryCandidate(callbacks.Context.RecordAddress(current), uint16(candidate))
					}
				}
			}
		}
		current = node.Next
	}
	if callbacks.Context != nil {
		callbacks.Context.EntryDispatch(uint16(priority))
	}
	if priority == 4 {
		step.Outcome = FollowerEntryBattle
		return step, rules.prepareBattle(reference, step.Target, callbacks)
	}
	if priority == 2 || priority == 6 {
		target, err := callbacks.Read(step.Target)
		if err != nil {
			return step, err
		}
		if priority == 6 && (actor.Motion.Flags|target.Motion.Flags)&2 != 0 {
			return step, nil
		}
		actor.Motion.ReturnState = 12
		motion := FollowerMotionRules{}
		if err := motion.BeginLegWithContext(&actor.Motion, target.Motion.X, target.Motion.Y, callbacks.Context); err != nil {
			return step, err
		}
		if err := callbacks.Write(reference, actor); err != nil {
			return step, err
		}
		if target.Motion.State == 4 {
			target.Motion.Timer, target.Motion.State, target.Motion.Animation = actor.Motion.Timer+1, 10, 0xccc
			if target.Motion.Flags&2 != 0 {
				if target.Hero40&1 != 0 || target.Hero40 > 10 {
					return step, fmt.Errorf("native contact hero index outside table")
				}
				target.Motion.Animation = rules.ReplyAnimations[target.Hero40/2]
			}
			if err := callbacks.Write(step.Target, target); err != nil {
				return step, err
			}
		}
		step.Outcome = FollowerEntryHoming
		return step, nil
	}
	mode, err := callbacks.GodMode(actor.Owner)
	if err != nil || mode == 16 || actor.Motion.Flags&2 != 0 || step.WallObserved {
		return step, err
	}
	tile, err := callbacks.Tile(entryTile(actor))
	if err != nil || rules.Properties[tile]&1 == 0 {
		return step, err
	}
	step.Outcome, step.ContinueTownUpdate = FollowerEntrySettlement, true
	stage, err := rules.settle(reference, callbacks)
	if err != nil {
		return step, err
	}
	step.SettlementCreated = stage > 0
	return step, callbacks.Founded(actor.Owner) // Increment even when support fails.
}

// CompleteContact follows state12 at $119f6. Unlike Enter, it retains the last
// friendly/enemy kind<=4 and does not filter inactive owner or population.
func (rules FollowerEntryRules) CompleteContact(reference NativeRecordReference, callbacks FollowerEntryCallbacks) (FollowerEntryStep, error) {
	var step FollowerEntryStep
	if err := callbacks.validate(); err != nil {
		return step, err
	}
	actor, err := callbacks.Read(reference)
	if err != nil {
		return step, err
	}
	head, err := callbacks.Head(entryTile(actor))
	if err != nil {
		return step, err
	}
	var friendly, enemy NativeRecordReference
	seen := make(map[NativeRecordReference]bool)
	for current := head; current != 0; {
		if seen[current] {
			return step, fmt.Errorf("cyclic native completed-contact chain")
		}
		seen[current] = true
		node, err := callbacks.Node(current)
		if err != nil {
			return step, err
		}
		if current != reference && int8(node.Kind) <= 4 {
			if node.Owner == actor.Owner {
				friendly = current
			} else {
				enemy = current
			}
		}
		current = node.Next
	}
	if friendly != 0 {
		step.Outcome, step.Target, step.NextFollower = FollowerEntryMerged, friendly, true
		if err := rules.merge(reference, friendly, callbacks); err != nil {
			return step, err
		}
		target, err := callbacks.Read(friendly)
		if err != nil {
			return step, err
		}
		if target.Motion.State == 10 {
			target.Motion.Animation, target.Motion.State = 0, 2
			err = callbacks.Write(friendly, target)
		}
		return step, err
	}
	if enemy != 0 {
		step.Outcome, step.Target = FollowerEntryBattle, enemy
		return step, rules.prepareBattle(reference, enemy, callbacks)
	}
	step.RedispatchHero = actor.Motion.Flags&2 != 0
	step.RedispatchSearch = !step.RedispatchHero
	return step, nil
}

// TickWaiting follows $119c8. Timer expiry clears animation and branches to
// ordinary search directly, without setting state2 or running a second prepass.
func (rules FollowerEntryRules) TickWaiting(reference NativeRecordReference, callbacks FollowerEntryCallbacks) (FollowerEntryStep, error) {
	step := FollowerEntryStep{Outcome: FollowerEntryWaiting}
	if callbacks.Read == nil || callbacks.Write == nil {
		return step, fmt.Errorf("native waiting record callbacks missing")
	}
	actor, err := callbacks.Read(reference)
	if err != nil {
		return step, err
	}
	next := actor.Motion.Animation + 4
	if loop, ok := rules.WaitingLoops[next]; ok {
		next += loop
	}
	if _, ok := rules.WaitingFrames[next]; !ok {
		return step, fmt.Errorf("native waiting animation outside decoded sequences")
	}
	actor.Motion.Animation = next
	previous := actor.Motion.Timer
	actor.Motion.Timer--
	if previous <= 1 {
		actor.Motion.Animation = 0
		step.RedispatchSearch = true
	}
	return step, callbacks.Write(reference, actor)
}

func (rules FollowerEntryRules) settle(reference NativeRecordReference, callbacks FollowerEntryCallbacks) (int, error) {
	if callbacks.Context != nil {
		saved := *callbacks.Context
		defer func() { *callbacks.Context = saved }()
	}
	actor, err := callbacks.Read(reference)
	if err != nil {
		return 0, err
	}
	actor.Motion.State, actor.Motion.Kind = 6, 4
	actor.Motion.X = int16(uint16(actor.Motion.X)&0xff00 | 128)
	actor.Motion.Y = int16(uint16(actor.Motion.Y)&0xff00 | 128)
	actor.Byte19, actor.Byte1, actor.Founded46 = 0, 0, callbacks.Tick()
	if err := callbacks.Write(reference, actor); err != nil {
		return 0, err
	}
	stage, err := callbacks.EvaluateTown(reference)
	if err != nil {
		return 0, err
	}
	if stage < 0 || stage >= TownStages {
		return 0, fmt.Errorf("native settlement support stage outside table")
	}
	actor, err = callbacks.Read(reference)
	if err != nil {
		return 0, err
	}
	if stage > 0 {
		actor.Byte1 = uint8(stage)
		return stage, callbacks.Write(reference, actor)
	}
	if err := callbacks.ClearFarms(reference, 15); err != nil {
		return 0, err
	}
	actor, err = callbacks.Read(reference)
	if err != nil {
		return 0, err
	}
	actor.Motion.State, actor.Motion.Kind, actor.Motion.Animation = 2, 2, 0
	return 0, callbacks.Write(reference, actor)
}

func (rules FollowerEntryRules) merge(source, target NativeRecordReference, callbacks FollowerEntryCallbacks) error {
	from, err := callbacks.Read(source)
	if err != nil {
		return err
	}
	to, err := callbacks.Read(target)
	if err != nil {
		return err
	}
	if from.Motion.Flags&2 != 0 {
		to.Hero40, to.Motion.Variant = from.Hero40, from.Motion.Variant
		if err := callbacks.Write(target, to); err != nil {
			return err
		}
	}
	if err := callbacks.ClearHeroLinks(source); err != nil {
		return err
	}
	from, err = callbacks.Read(source)
	if err != nil {
		return err
	}
	to, err = callbacks.Read(target)
	if err != nil {
		return err
	}
	to.Motion.Flags |= from.Motion.Flags
	if from.Motion.Flags&16 != 0 {
		to.Extra48 = from.Extra48
	}
	if err := callbacks.Write(target, to); err != nil {
		return err
	}
	if from.Motion.Flags&1 != 0 {
		if err := callbacks.SetLeader(from.Owner, target); err != nil {
			return err
		}
	}
	if int8(from.Weapon) > int8(to.Weapon) {
		to.Weapon = from.Weapon
	}
	to.Motion.Population = int32(uint32(to.Motion.Population) + uint32(from.Motion.Population))
	if err := callbacks.Write(target, to); err != nil {
		return err
	}
	if callbacks.Selected() == source {
		if err := callbacks.Select(target); err != nil {
			return err
		}
	}
	from.Owner, from.Motion.Population = 0, 0
	if err := callbacks.Write(source, from); err != nil {
		return err
	}
	return callbacks.Unlink(source)
}

func (rules FollowerEntryRules) prepareBattle(source, target NativeRecordReference, callbacks FollowerEntryCallbacks) error {
	from, err := callbacks.Read(source)
	if err != nil {
		return err
	}
	to, err := callbacks.Read(target)
	if err != nil {
		return err
	}
	if from.Motion.Flags&2 == 0 && to.Motion.Flags&2 != 0 {
		source, target, from, to = target, source, to, from
	}
	if from.Motion.Flags&2 != 0 {
		if from.Association34 != 0 {
			associated := NativeRecordReference(from.Association34)
			back, err := callbacks.ReadWord(associated, 36)
			if err != nil {
				return err
			}
			from.Association34 = 0
			if err := callbacks.Write(source, from); err != nil {
				return err
			}
			if NativeRecordReference(back) == source {
				if err := callbacks.WriteWord(associated, 36, 0); err != nil {
					return err
				}
			}
			from, err = callbacks.Read(source)
			if err != nil {
				return err
			}
			to, err = callbacks.Read(target)
			if err != nil {
				return err
			}
		}
		if from.Hero40 == 10 {
			to.Motion.Flags |= 8
			if err := callbacks.Write(target, to); err != nil {
				return err
			}
			if to.Motion.Kind == 4 {
				if err := callbacks.ClearFarms(target, 15); err != nil {
					return err
				}
			}
			if from.Captive42 != 0 {
				if err := callbacks.WriteWord(NativeRecordReference(from.Captive42), 42, uint16(target)); err != nil {
					return err
				}
			}
			from.Captive42 = uint16(target)
			if err := callbacks.Write(source, from); err != nil {
				return err
			}
			to, err = callbacks.Read(target)
			if err != nil {
				return err
			}
			to.CaptiveBack44 = uint16(source)
			if err := callbacks.Write(target, to); err != nil {
				return err
			}
			if err := callbacks.Sound(0x35c); err != nil {
				return err
			}
			// $184f6 preserves D0. The next native MOVE.W stores the sound
			// argument in the victim's word42; retain that original value.
			to.Captive42, to.Motion.State, to.Motion.Kind, to.Motion.Animation, to.AssociationBack36 = 0x35c, 0x34, 2, 0, 0
			if err := callbacks.Write(target, to); err != nil {
				return err
			}
			from.Motion.State, from.Association34 = 0x24, 0
			return callbacks.Write(source, from)
		}
	}
	from.Contact30 = uint16(target)
	if err := callbacks.Write(source, from); err != nil {
		return err
	}
	to.Contact30, to.Motion.State = uint16(source), 0x10
	if err := callbacks.Write(target, to); err != nil {
		return err
	}
	from.Motion.State, from.Motion.Animation = 0x0e, 0x1c8
	if from.Motion.Kind != 4 {
		from.Motion.X = int16(uint16(from.Motion.X)&0xff00 | 128)
		from.Motion.Y = int16(uint16(from.Motion.Y)&0xff00 | 128)
	}
	return callbacks.Write(source, from)
}
