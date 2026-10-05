package populous2

import "fmt"

// SwitchNativeProfile executes $111ae directly for requester continuations.
// Deferred multiplayer swaps still belong to command124 at the command stage.
func (w *World) SwitchNativeProfile(side uint16, context *NativeCommandRegisterContext) error {
	if w == nil || context == nil || side < 1 || side > 2 {
		return fmt.Errorf("native profile switch context missing")
	}
	commandWord(context, 0, side)
	err := w.runNativeFollowerCall(func() error {
		_, err := w.commandSwitchProfile(NativeCommandCall{Routine: 0x111ae, Context: context})
		return err
	})
	if err == nil {
		w.Deity.Experience = w.Experience[side-1]
	}
	return err
}

// NativeWorldMenuState reads the selected deity's live signed power flags and
// rules. It does not reconstruct them from the campaign defaults after edits.
func (w *World) NativeWorldMenuState() (NativeWorldRequesterState, error) {
	var state NativeWorldRequesterState
	if w == nil || w.NativeProfileSide < 1 || w.NativeProfileSide > 2 {
		return state, fmt.Errorf("native world menu profile missing")
	}
	m := w.nativeCleanupMemory()
	var err error
	state.World, err = m.Read16(0xeb46)
	if err != nil {
		return state, err
	}
	state.Landscape, err = m.Read16(0xeb22)
	if err != nil {
		return state, err
	}
	god, _ := NativeDeityAddress(w.NativeProfileSide)
	state.RuleBits, err = m.Read16(god + 0x66)
	if err != nil {
		return state, err
	}
	for slot := range state.PowerFlags {
		value, err := m.Read8(god + 0x70 + slot)
		if err != nil {
			return state, err
		}
		state.PowerFlags[slot] = int8(value)
	}
	return state, nil
}

func (w *World) NativeOpponentMenuState() (world, reaction, aggression uint16, err error) {
	if w == nil || w.NativeProfileSide < 1 || w.NativeProfileSide > 2 {
		err = fmt.Errorf("native opponent menu profile missing")
		return
	}
	m := w.nativeCleanupMemory()
	world, err = m.Read16(0xeb46)
	if err != nil {
		return
	}
	god, _ := NativeDeityAddress(3 - w.NativeProfileSide)
	reaction, err = m.Read16(god + 0x68)
	if err != nil {
		return
	}
	aggression, err = m.Read16(god + 0x6a)
	return
}

// LoadNativeWorldMenu follows $3e8a/$11044 on the retained session. It copies
// templates and resets the script cursor/RNG, preserving actors, command
// records, clock, saved seed and already compiled policy lists. Fresh terrain
// and scene allocation occur only after the requester returns Proceed.
func (w *World) LoadNativeWorldMenu(bundle *Bundle, number uint16, context *NativeCommandRegisterContext) error {
	if w == nil || bundle == nil || context == nil || int(number) >= len(bundle.Levels) {
		return fmt.Errorf("native requester world outside campaign")
	}
	level := bundle.Levels[number]
	m := w.nativeCleanupMemory()
	if err := m.Write16(0xeb46, number); err != nil {
		return err
	}
	experience, err := w.copyNativeAITemplates(&w.NativeAI, level, false)
	if err != nil {
		return err
	}
	if err := m.Write32(0xeb28, level.RandomSeed); err != nil {
		return err
	}
	w.Level = level
	w.Experience = experience
	for side := range w.Rules {
		w.Rules[side] = level.Players[side].ScenarioRules()
	}
	w.Deity.Experience = experience[w.NativeProfileSide-1]
	god, _ := NativeDeityAddress(w.NativeProfileSide)
	w.Deity.Bolts, err = m.Read16(god + 0x58)
	if err != nil {
		return err
	}
	context.D[0] = uint32(number%5) * 0x2d7
	context.D[1] = level.RandomSeed
	context.D[2] = uint32(number/5) * 250
	return nil
}
