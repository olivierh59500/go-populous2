package populous2

import (
	"fmt"

	"go-populous2/internal/amiga"
)

// ManaRules are the two lookup tables used by the Amiga cost routine at
// CODE:$14768. Experience is an unsigned byte for each of the six elements.
// Its upper three bits select a divisor; a zero divisor leaves the cost intact.
type ManaRules struct {
	Categories [36]Element
	Divisors   [8]uint8
}

func DecodeManaRules(exe *amiga.Executable) (ManaRules, error) {
	var rules ManaRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x21066 {
		return rules, fmt.Errorf("Populous II experience cost tables missing")
	}
	code := exe.Hunks[0].Data
	for i, category := range code[0x147bc : 0x147bc+36] {
		if category > uint8(Water) {
			return ManaRules{}, fmt.Errorf("invalid power category %d at slot %d", category, i)
		}
		rules.Categories[i] = Element(category)
	}
	copy(rules.Divisors[:], code[0x2105e:0x21066])
	return rules, nil
}

// Cost translates the unsigned division and subtraction in CODE:$14768.
// A reserved $ffff entry is preserved, as in the original routine.
func (rules ManaRules) Cost(id SpellID, base int, experience [6]uint8) int {
	if int(id) >= len(rules.Categories) || base < 0 || base >= 0xffff {
		return base
	}
	category := rules.Categories[id]
	if category > Water {
		return base
	}
	divisor := int(rules.Divisors[experience[category]>>5])
	if divisor == 0 {
		return base
	}
	return base - base/divisor
}

// ManaCost returns the payable price for this deity, rather than the base
// price shown in the executable's power table. Invalid powers return -1.
func (w *World) ManaCost(player int, id SpellID) int {
	if player < 0 || player >= len(w.Experience) {
		return -1
	}
	spell, ok := SpellByID(w.Spells, id)
	if !ok {
		return -1
	}
	// The cost lookup returns quarter-mana units. Both the affordability
	// check ($147e0) and the successful cast debit ($17e7e) multiply by four.
	return 4 * w.ManaRules.Cost(id, spell.Cost, w.Experience[player])
}
