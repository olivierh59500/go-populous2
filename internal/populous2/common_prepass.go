package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type CommonPrepassRules struct {
	Properties                 [256]uint16
	Swimming, Drowning, Fatal  [6]uint16
	Conversion, Swamp, Burning [6]uint16
	PlagueDamage               uint16
	Fungus                     FungusHazardRules
	AnimationWords             []uint16
}

func DecodeCommonPrepassRules(exe *amiga.Executable) (CommonPrepassRules, error) {
	var r CommonPrepassRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33612 {
		return r, fmt.Errorf("native common prepass tables missing")
	}
	code := exe.Hunks[0].Data
	for index := range r.Properties {
		r.Properties[index] = binary.BigEndian.Uint16(code[0x33312+index*2:])
	}
	for _, table := range []struct {
		start  int
		values *[6]uint16
	}{{0x20a90, &r.Swimming}, {0x20a60, &r.Drowning}, {0x20a54, &r.Fatal}, {0x20a78, &r.Conversion}, {0x20a3c, &r.Swamp}, {0x20a18, &r.Burning}} {
		for index := range table.values {
			table.values[index] = binary.BigEndian.Uint16(code[table.start+index*2:])
		}
	}
	r.PlagueDamage = binary.BigEndian.Uint16(code[0x20d56:])
	for at := 0x23d1a; at < 0x26956; at += 2 {
		r.AnimationWords = append(r.AnimationWords, binary.BigEndian.Uint16(code[at:]))
	}
	var err error
	r.Fungus, err = DecodeFungusHazardRules(exe)
	return r, err
}

type CommonPrepassCallbacks struct {
	Read        func(NativeRecordReference) (FollowerEntryActor, error)
	Write       func(NativeRecordReference, FollowerEntryActor) error
	Tile        func(NativePackedTile) (uint8, error)
	WriteTile   func(NativePackedTile, uint8) error
	Scenario    func(uint8) (uint16, error)
	ClearFarms  func(NativeRecordReference, uint8) error
	Cleanup     func(NativeRecordReference, uint16) error
	ClearLeader func(NativeRecordReference) error
	Unlink      func(NativeRecordReference) error
	Sound       func(uint16) error
}

type CommonPrepassHazard uint8

const (
	CommonPrepassNone CommonPrepassHazard = iota
	CommonPrepassSwimming
	CommonPrepassDrowning
	CommonPrepassFatal
	CommonPrepassConversion
	CommonPrepassSwamp
	CommonPrepassFungus
	CommonPrepassBurning
)

type CommonPrepassStep struct {
	OwnerSkipped, PlagueAdvanced, PlagueKilled bool
	Immune, Guarded, Removed                   bool
	Hazard                                     CommonPrepassHazard
}

func (r *CommonPrepassRules) imageWord(offset uint16) (int16, error) {
	if offset&1 != 0 || int(offset)/2 >= len(r.AnimationWords) {
		return 0, fmt.Errorf("native plague animation word outside bounded bank")
	}
	return int16(r.AnimationWords[int(offset)/2]), nil
}

func prepassHeroAnimation(a FollowerEntryActor, ordinary uint16, table [6]uint16) (uint16, error) {
	if a.Motion.Flags&2 == 0 {
		return ordinary, nil
	}
	if a.Hero40&1 != 0 || a.Hero40 > 10 {
		return 0, fmt.Errorf("native prepass hero selector outside six-entry table")
	}
	return table[a.Hero40/2], nil
}

// Tick translates $12c3c only. It preserves the native branch priority and
// re-reads after complete external cleanup/farm operations. Owner3 bypasses
// both the plague clock and terrain checks. No RNG or generic damage is used.
// The original caller still dispatches the resulting state after this returns,
// including a record whose owner was cleared by the corpse-removal branch.
func (r *CommonPrepassRules) Tick(ref NativeRecordReference, cb CommonPrepassCallbacks) (CommonPrepassStep, error) {
	var step CommonPrepassStep
	if r == nil || cb.Read == nil || cb.Write == nil || cb.Tile == nil {
		return step, fmt.Errorf("native common prepass callbacks/rules missing")
	}
	a, err := cb.Read(ref)
	if err != nil {
		return step, err
	}
	if a.Owner == 3 {
		step.OwnerSkipped = true
		return step, nil
	}
	if a.Motion.Flags&16 != 0 {
		next := uint16(a.Extra48 + 4)
		word, err := r.imageWord(next)
		if err != nil {
			return step, err
		}
		if word < 0 {
			next += uint16(word)
		}
		a.Extra48 = next
		if err := cb.Write(ref, a); err != nil {
			return step, err
		}
		step.PlagueAdvanced = true
		if a.Motion.State != 0x3e {
			previous := a.Motion.Population
			a.Motion.Population = int32(uint32(previous) - uint32(r.PlagueDamage))
			if err := cb.Write(ref, a); err != nil {
				return step, err
			}
			// Preserve signed SUB.L/BGT even when its stored result overflows.
			if int64(previous)-int64(r.PlagueDamage) <= 0 {
				a.Motion.State, a.Motion.Animation = 0x3e, 0x7dc
				if err := cb.Write(ref, a); err != nil {
					return step, err
				}
				if cb.Cleanup == nil {
					return step, fmt.Errorf("native plague cleanup missing")
				}
				if err := cb.Cleanup(ref, 1); err != nil {
					return step, err
				}
				step.PlagueKilled = true
				a, err = cb.Read(ref)
				if err != nil {
					return step, err
				}
			}
		}
	}
	packed := entryTile(a)
	tile, err := cb.Tile(packed)
	if err != nil {
		return step, err
	}
	properties := r.Properties[tile]
	hazards := properties & 0x0f98
	if hazards == 0 {
		return step, nil
	}
	switch a.Motion.Kind {
	case 6, 0x12:
		if a.Motion.Flags&1 != 0 {
			if cb.ClearLeader == nil {
				return step, fmt.Errorf("native corpse leader cleanup missing")
			}
			if err := cb.ClearLeader(ref); err != nil {
				return step, err
			}
			a, err = cb.Read(ref)
			if err != nil {
				return step, err
			}
		}
		a.Owner, a.Motion.Population = 0, 0
		if err := cb.Write(ref, a); err != nil {
			return step, err
		}
		if cb.Unlink == nil {
			return step, fmt.Errorf("native corpse unlink missing")
		}
		step.Removed = true
		return step, cb.Unlink(ref)
	case 8, 10, 12, 14, 16:
		return step, nil
	case 4:
		if cb.ClearFarms == nil {
			return step, fmt.Errorf("native town terrain cleanup missing")
		}
		if err := cb.ClearFarms(ref, 15); err != nil {
			return step, err
		}
		a, err = cb.Read(ref)
		if err != nil {
			return step, err
		}
	case 2:
	default:
		return step, fmt.Errorf("native common prepass kind outside audited dispatch table")
	}
	var animation uint16
	var rawSound uint16
	center, cleanup, restoreShallow := false, false, false
	switch {
	case hazards&0x800 != 0:
		step.Hazard = CommonPrepassSwimming
		if a.Motion.State == 0x2e {
			step.Guarded = true
			return step, nil
		}
		animation, err = prepassHeroAnimation(a, 0x196c, r.Swimming)
		if animation != 0 {
			a.Motion.State = 0x2e
		}
	case hazards&8 != 0:
		step.Hazard = CommonPrepassDrowning
		animation, err = prepassHeroAnimation(a, 0x7dc, r.Drowning)
		if animation != 0 {
			a.Motion.Kind, a.Motion.State, center = 0x0a, 0x16, true
		}
	case hazards&0x80 != 0:
		step.Hazard = CommonPrepassFatal
		if a.Motion.State == 0x3a {
			step.Guarded = true
			return step, nil
		}
		animation, err = prepassHeroAnimation(a, 0x7dc, r.Fatal)
		if animation != 0 {
			a.Motion.Kind, a.Motion.State, rawSound, center, cleanup = 0x0c, 0x32, 0x19a, true, true
		}
	case hazards&0x100 != 0:
		step.Hazard = CommonPrepassConversion
		ordinary := uint16(0xbd8)
		if a.Owner != 1 {
			ordinary = 0xc0c
		}
		animation, err = prepassHeroAnimation(a, ordinary, r.Conversion)
		if animation != 0 {
			a.Motion.Kind, a.Motion.State, rawSound, center = 0x0e, 0x36, 0xf0, true
		}
	case hazards&0x200 != 0:
		step.Hazard = CommonPrepassSwamp
		animation, err = prepassHeroAnimation(a, 0x7dc, r.Swamp)
		if animation != 0 {
			if cb.Scenario == nil {
				return step, fmt.Errorf("native shallow-swamp scenario callback missing")
			}
			scenario, err := cb.Scenario(a.Owner)
			if err != nil {
				return step, err
			}
			if scenario&0x200 != 0 {
				if cb.WriteTile == nil {
					return step, fmt.Errorf("native shallow-swamp tile callback missing")
				}
				restoreShallow = true
			}
			a.Motion.Kind, a.Motion.State, rawSound, center, cleanup = 0x10, 0x38, 0x10e, true, true
		}
	case hazards&0x10 != 0:
		step.Hazard = CommonPrepassFungus
		hero := -1
		if a.Motion.Flags&2 != 0 {
			if a.Hero40&1 != 0 || a.Hero40 > 10 {
				return step, fmt.Errorf("native fungus hero selector outside table")
			}
			hero = int(a.Hero40 / 2)
		}
		decision := r.Fungus.Enter(properties, a.Motion.Flags&2 != 0, hero)
		if !decision.Applies {
			step.Immune = decision.Immune
			return step, nil
		}
		animation = uint16(decision.Animation)
		a.Motion.Kind, a.Motion.State, rawSound, center, cleanup = decision.Kind, decision.State, decision.RawSoundArgument, true, true
	case hazards&0x400 != 0:
		step.Hazard = CommonPrepassBurning
		if a.Motion.State == 0x3c {
			step.Guarded = true
			return step, nil
		}
		if a.Motion.State == 0x3a {
			step.Guarded = true
			return step, nil
		}
		animation, err = prepassHeroAnimation(a, 0x564, r.Burning)
		if animation != 0 {
			a.Motion.State, rawSound = 0x3c, 0x64
		}
	}
	if err != nil {
		return step, err
	}
	if animation == 0 {
		step.Immune = true
		return step, nil
	}
	// Native writes animation before emitting sound and before kind/state/XY.
	original, err := cb.Read(ref)
	if err != nil {
		return step, err
	}
	original.Motion.Animation = int(animation)
	if err := cb.Write(ref, original); err != nil {
		return step, err
	}
	if restoreShallow {
		if err := cb.WriteTile(packed, 15); err != nil {
			return step, err
		}
	}
	if rawSound != 0 {
		if cb.Sound == nil {
			return step, fmt.Errorf("native terrain sound callback missing")
		}
		if err := cb.Sound(rawSound); err != nil {
			return step, err
		}
	}
	a.Motion.Animation = int(animation)
	if center {
		a.Motion.X = int16(uint16(a.Motion.X)&0xff00 | 128)
		a.Motion.Y = int16(uint16(a.Motion.Y)&0xff00 | 128)
	}
	if err := cb.Write(ref, a); err != nil {
		return step, err
	}
	if cleanup {
		if cb.Cleanup == nil {
			return step, fmt.Errorf("native terrain retained cleanup missing")
		}
		return step, cb.Cleanup(ref, 1)
	}
	return step, nil
}
