package populous2

import "fmt"

func (w *World) NativeInGameMenuState() (NativeInGameState, error) {
	var state NativeInGameState
	if w == nil {
		return state, fmt.Errorf("native in-game World missing")
	}
	m := w.nativeCleanupMemory()
	var err error
	state.Profile, err = m.Read16(0xeb42)
	if err != nil {
		return state, err
	}
	if state.Profile < 1 || state.Profile > 2 {
		return state, fmt.Errorf("native in-game profile outside1/2")
	}
	state.GameMode, err = m.Read16(0xeb44)
	if err != nil {
		return state, err
	}
	god, _ := NativeDeityAddress(uint8(state.Profile))
	state.ControlMode, err = m.Read16(god + 0x1a)
	if err != nil {
		return state, err
	}
	state.PaintFlag, err = m.Read16(0xf0e)
	return state, err
}

// ApplyNativeInGameMenu follows $45b0 on the actual retained World. Save,
// load, restart, quit and multiplayer profile switching change only the
// selected command byte; their bodies wait for the later $1744c stage.
func (w *World) ApplyNativeInGameMenu(rules *NativeInGameRequesterRules, action int, context *NativeCommandRegisterContext) (NativeInGameAction, error) {
	var step NativeInGameAction
	if rules == nil || context == nil {
		return step, fmt.Errorf("native in-game rules/context missing")
	}
	state, err := w.NativeInGameMenuState()
	if err != nil {
		return step, err
	}
	step, err = rules.Action(&state, action)
	if err != nil {
		return step, err
	}
	m := w.nativeCleanupMemory()
	god, _ := NativeDeityAddress(uint8(state.Profile))
	if err := m.Write16(god+0x1a, state.ControlMode); err != nil {
		return step, err
	}
	if err := m.Write16(0xf0e, state.PaintFlag); err != nil {
		return step, err
	}
	if step.SwitchProfile != 0 {
		if err := w.SwitchNativeProfile(step.SwitchProfile, context); err != nil {
			return step, err
		}
	}
	if step.DeferredCommand != 0 {
		pointer, err := m.Read32(0xeb6a)
		if err != nil {
			return step, err
		}
		if err := m.Write8(int(pointer)+1, step.DeferredCommand); err != nil {
			return step, err
		}
	}
	return step, nil
}

// ResumeNativeInGameMenu runs the real $181c0 gate. Solo control records need
// no network callbacks; multiplayer records must supply an actual port. A
// pending transfer retains its controller until it completes or reports the
// original distinct failed-return path.
func (w *World) ResumeNativeInGameMenu(resume *NativeSerialResume, port NativeSerialPort, context *NativeCommandRegisterContext) (NativeSerialResumeStep, error) {
	if w == nil || resume == nil || context == nil {
		return NativeSerialResumeStep{}, fmt.Errorf("native in-game resume context missing")
	}
	step, err := resume.Advance(NativeSerialCallbacks{Memory: w.nativeCleanupMemory(), Port: port, SwitchProfile: func(side uint16) error {
		return w.SwitchNativeProfile(side, context)
	}})
	if err != nil {
		return step, err
	}
	m := w.nativeCleanupMemory()
	w.NativeGameMode, err = m.Read16(0xeb44)
	if err != nil {
		return step, err
	}
	profile, err := m.Read16(0xeb42)
	if err != nil {
		return step, err
	}
	w.NativeProfileSide = uint8(profile)
	return step, nil
}
