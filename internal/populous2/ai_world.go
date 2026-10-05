package populous2

import "fmt"

// nativeAICallbacks borrows authoritative World bytes. Policy calls only write
// the original ten-byte command records; their execution has its later stage.
func (w *World) nativeAICallbacks(context *NativeAIRegisterContext) NativeAICallbacks {
	return NativeAICallbacks{Memory: w.nativeCleanupMemory(), Random: func() uint16 { return uint16(w.random()) }, Context: context}
}

// tickNativeAI consumes explicit caller register words, including an empty
// follower pass's preserved input. It does not reset or infer D4/D5, refresh
// raw actor bytes from typed views, or execute the newly queued commands.
func (w *World) tickNativeAI(rules *NativeAIRules, context *NativeAIRegisterContext) (NativeAIStep, error) {
	if w == nil || w.Core == nil || rules == nil || context == nil {
		return NativeAIStep{}, fmt.Errorf("native World AI rules/context missing")
	}
	return rules.TickComplete(context, w.nativeAICallbacks(context))
}

// initializeNativeAIControls executes the control-only suffix $10f9a..$11042.
// The earlier $10f1a whole-BSS reset belongs before terrain/actor creation and
// must not be replayed on a populated World. Existing command payload survives.
func (w *World) initializeNativeAIControls() error {
	if w == nil || w.Core == nil {
		return fmt.Errorf("native World AI control memory missing")
	}
	m := w.nativeCleanupMemory()
	selected, err := m.Read16(0xeb42)
	if err != nil {
		return err
	}
	if selected == 0 {
		for _, v := range []struct {
			address int
			value   uint8
		}{{0xeb56, 1}, {0xeb60, 2}, {0xeb5e, 2}, {0xeb68, 4}} {
			if err := m.Write8(v.address, v.value); err != nil {
				return err
			}
		}
		if err := m.Write16(0xeb42, 1); err != nil {
			return err
		}
		// Native pointer convention is BSS-relative here, never a Go pointer.
		if err := m.Write32(0xeb6a, 0xeb56); err != nil {
			return err
		}
		selected = 1
	}
	transport, err := m.Read8(0xeb5e)
	if err != nil {
		return err
	}
	computer := uint16(4)
	if int8(transport) >= 6 {
		computer = 2
	}
	controls := [2]uint16{2, computer}
	if selected != 1 {
		controls = [2]uint16{computer, 2}
	}
	for side := 0; side < 2; side++ {
		god := 0xe8a4 + side*314
		if err := m.Write16(god+0x1a, controls[side]); err != nil {
			return err
		}
		if err := m.Write16(god+0x18, uint16(side+1)); err != nil {
			return err
		}
		if err := m.Write16(god+0x0c, 14); err != nil {
			return err
		}
	}
	if err := m.Write16(0xeb18, 2); err != nil {
		return err
	}
	if err := m.Write16(0xeb6e, 0); err != nil {
		return err
	}
	return m.Write16(0xf0c, 8)
}

// loadNativeAITemplates is $10df2 followed by $10e90. Its profile copy is
// eight raw bytes116..123 to God+$52..$59, including the final bolt word.
// The returned actual XP bytes+$52..$57 let the caller hydrate its views before
// any bridge runs; raw bytes122/123 also change the original bolt word58/59.
func (w *World) loadNativeAITemplates(rules *NativeAIRules, level Level) ([2][6]uint8, error) {
	return w.copyNativeAITemplates(rules, level, true)
}

// copyNativeAITemplates keeps $10df2 separate from $10e90. Scene startup
// compiles policy lists afterward; the $11044 world requester does not.
func (w *World) copyNativeAITemplates(rules *NativeAIRules, level Level, compile bool) ([2][6]uint8, error) {
	var experience [2][6]uint8
	if w == nil || w.Core == nil || rules == nil {
		return experience, fmt.Errorf("native World AI template rules missing")
	}
	m := w.nativeCleanupMemory()
	mode, err := m.Read16(0xeb44)
	if err != nil {
		return experience, err
	}
	for side := 0; side < 2; side++ {
		god := 0xe8a4 + side*314
		for i, value := range level.Raw[side*58 : (side+1)*58] {
			if err := m.Write8(god+0x5a+i, value); err != nil {
				return experience, err
			}
		}
		control, err := m.Read16(god + 0x1a)
		if err != nil {
			return experience, err
		}
		if mode != 6 && control == 4 {
			for i, value := range level.Raw[116:124] {
				if err := m.Write8(god+0x52+i, value); err != nil {
					return experience, err
				}
			}
		}
	}
	for i, value := range level.Raw[122:182] {
		if err := m.Write8(0xdde+i, value); err != nil {
			return experience, err
		}
	}
	if err := m.Write16(0xf0a, 0); err != nil {
		return experience, err
	}
	if err := m.Write16(0xeb22, uint16(level.Raw[182])<<8|uint16(level.Raw[183])); err != nil {
		return experience, err
	}
	for side := 0; side < 2; side++ {
		god := 0xe8a4 + side*314
		if compile {
			if err := rules.CompileChoices(god, m); err != nil {
				return experience, err
			}
		}
		for i := range experience[side] {
			value, err := m.Read8(god + 0x52 + i)
			if err != nil {
				return experience, err
			}
			experience[side][i] = value
		}
	}
	return experience, nil
}

// NativeCommandRegisterContext retains the explicit data-register input to
// $1744c/$17500. Side1's command can change the context consumed by side2;
// the enclosing $1744c MOVEM restores the caller's complete values afterward.
type NativeCommandRegisterContext struct{ D [8]uint32 }
type NativeDeferredCommandCallbacks struct {
	// Execute is the actual normal-player $17500 body. Script execution and
	// World.Cast have different admission/debit behavior and cannot replace it.
	Execute func(int, *NativeCommandRegisterContext) error
	// Transport owns native modes6/8's replay/network prelude. The callback is
	// explicit because those original I/O/UI operations have not been replaced
	// by a silent success or guessed RNG resynchronization.
	Transport func(int, uint8, *NativeCommandRegisterContext) error
}
type NativeDeferredCommandStep struct {
	Visited, Executed int
	Addresses         []int
}

// executeNativeDeferredCommands is the $1744c record scheduler. It runs side1
// then side2, uses the raw byte8 transport dispatch, and clears only command
// byte1 and XY word2 after each actual body. It belongs after FX/wall/scenery
// and scenario-script stages; policy selection never calls this method itself.
func (w *World) executeNativeDeferredCommands(input NativeCommandRegisterContext, cb NativeDeferredCommandCallbacks) (NativeDeferredCommandStep, error) {
	step := NativeDeferredCommandStep{Addresses: []int{}}
	if w == nil || w.Core == nil {
		return step, fmt.Errorf("native deferred command World missing")
	}
	m := w.nativeCleanupMemory()
	context := input
	for side := 0; side < 2; side++ {
		address := 0xeb56 + side*10
		step.Visited++
		mode, err := m.Read8(address + 8)
		if err != nil {
			return step, err
		}
		dispatch := [6]uint16{0x0c, 0x0e, 0x0e, 0x16, 0x3e, 0x80}
		if mode&1 != 0 || int(mode/2) >= len(dispatch) {
			return step, fmt.Errorf("native command transport byte%d outside original dispatch", mode)
		}
		context.D[0] = uint32(dispatch[mode/2])
		switch mode {
		case 0, 10:
		case 2, 4, 6, 8:
			if cb.Execute == nil {
				return step, fmt.Errorf("native deferred17500 executor missing")
			}
			if mode == 6 || mode == 8 {
				if cb.Transport == nil {
					return step, fmt.Errorf("native command transport mode%d callback missing", mode)
				}
				if err := cb.Transport(address, mode, &context); err != nil {
					return step, err
				}
			}
			if err := cb.Execute(address, &context); err != nil {
				return step, err
			}
			step.Executed++
			step.Addresses = append(step.Addresses, address)
		default:
			return step, fmt.Errorf("native command transport byte%d outside original dispatch", mode)
		}
		if err := m.Write8(address+1, 0); err != nil {
			return step, err
		}
		if err := m.Write16(address+2, 0); err != nil {
			return step, err
		}
	}
	return step, nil
}
