package populous2

import (
	"encoding/binary"
	"fmt"
	"go-populous2/internal/amiga"
	"slices"
)

// NativeAction preserves the command-to-power mapping used by CODE:$17500.
// A power can have several commands: raising/lowering and directed variants
// are distinct actions even though their affordability lookup uses one power.
type NativeAction struct {
	Command   uint8
	Spell     SpellID
	Handler   int
	SoundCues []int
}

func DecodeActions(exe *amiga.Executable) ([]NativeAction, error) {
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x210b0+86 {
		return nil, fmt.Errorf("native action tables missing")
	}
	code := exe.Hunks[0].Data
	handlers := []int{}
	for command := 2; command <= 84; command += 2 {
		at := 0x17524 + int(int16(binary.BigEndian.Uint16(code[0x17524+command:])))
		if at < 0x175a2 || at > 0x17e96 {
			return nil, fmt.Errorf("native action %d has invalid handler", command)
		}
		handlers = append(handlers, at)
	}
	slices.Sort(handlers)
	result := []NativeAction{}
	for command := 2; command <= 82; command += 2 {
		power := int(int16(binary.BigEndian.Uint16(code[0x210b0+command:])))
		if power < 0 {
			continue
		}
		if power%2 != 0 || power >= 72 {
			return nil, fmt.Errorf("native command %d has invalid power", command)
		}
		at := 0x17524 + int(int16(binary.BigEndian.Uint16(code[0x17524+command:])))
		end := 0x17e38
		for _, next := range handlers {
			if next > at {
				end = next
				break
			}
		}
		action := NativeAction{Command: uint8(command), Spell: SpellID(power / 2), Handler: at}
		// Each direct cue call has a word-immediate MOVE to D0 followed by
		// JSR $184f6. Restrict matching to the selected handler's code range.
		for pc := at; pc+10 <= end; pc += 2 {
			if binary.BigEndian.Uint16(code[pc:]) == 0x303c && binary.BigEndian.Uint16(code[pc+4:]) == 0x4eb9 && binary.BigEndian.Uint32(code[pc+6:]) == 0x184f6 {
				offset := int(binary.BigEndian.Uint16(code[pc+2:]))
				if offset%10 != 0 || offset/10 >= 133 {
					return nil, fmt.Errorf("native action %d has invalid audio cue", command)
				}
				action.SoundCues = append(action.SoundCues, offset/10)
			}
		}
		result = append(result, action)
	}
	return result, nil
}

func (b *Bundle) CastSoundCues(id SpellID, player int) []int {
	if id.IsHero() {
		// Hero sounds are selected in the shared conversion helper $142d4.
		heroCues := [6]int{78, 79, 80, 81, 35, 86}
		return []int{heroCues[heroIndex(id)]}
	}
	for _, action := range b.Actions {
		if action.Spell != id || len(action.SoundCues) == 0 {
			continue
		}
		if id == PapalMagnet && len(action.SoundCues) == 2 {
			return []int{action.SoundCues[clamp(player, 0, 1)]}
		}
		return action.SoundCues
	}
	return nil
}
