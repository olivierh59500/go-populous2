package engine

// PowerCostSlot names the original distinction between physical icon slots
// and actual command prices. Achilles and Volcano are the two crossed entries.
func PowerCostSlot(id PowerID) PowerID {
	switch id {
	case Achilles:
		return 27
	case Volcano:
		return 26
	}
	return id
}

// PanelPowerCost follows physical HUD slots for affordability markers and
// inspection admission. The actual creator is charged by PowerCost instead.
func (w *World) PanelPowerCost(owner int, id PowerID) int {
	switch id {
	case Achilles:
		return w.PowerCost(owner, Volcano)
	case Volcano:
		return w.PowerCost(owner, Achilles)
	}
	return w.PowerCost(owner, id)
}

// MigrateLegacyPowerID preserves the meaning of saved Go IDs from versions
// before physical slots were corrected. Native permission arrays never pass
// through this function: their indices have always been physical slots.
func MigrateLegacyPowerID(id PowerID) PowerID { return PowerFromCostSlot(id) }

// PowerFromCostSlot names a command-price entry as its canonical icon power.
func PowerFromCostSlot(id PowerID) PowerID {
	switch id {
	case 26:
		return Volcano
	case 27:
		return Achilles
	}
	return id
}

func migrateLegacyWorldPowerIDs(w *World) {
	for owner := range w.AI {
		ai := &w.AI[owner]
		ai.PreparedPower = MigrateLegacyPowerID(ai.PreparedPower)
		ai.Order.Power = MigrateLegacyPowerID(ai.Order.Power)
		for index := range ai.Choices {
			ai.Choices[index].Power = MigrateLegacyPowerID(ai.Choices[index].Power)
		}
	}
}
