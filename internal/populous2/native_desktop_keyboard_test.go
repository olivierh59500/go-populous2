package populous2

import (
	"reflect"
	"testing"
)

func nativeDesktopBinding(t *testing.T, name string) NativeDesktopKeyBinding {
	t.Helper()
	for _, binding := range NativeDesktopKeyBindings() {
		if binding.Name == name {
			return binding
		}
	}
	t.Fatalf("desktop key %s is not accessible", name)
	return NativeDesktopKeyBinding{}
}

func TestNativeDesktopKeyboardCharacters(t *testing.T) {
	rules, err := DecodeNativeInputRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	// These are filenames, path separators and editing controls consumed by
	// actual $10096/$4bba, rather than assertions about the mapping's spelling.
	for _, f := range []struct {
		name           string
		plain, shifted byte
	}{
		{"Backquote", '`', '~'}, {"Minus", '-', '_'}, {"Equal", '=', '+'}, {"Backslash", '\\', '|'},
		{"BracketLeft", '[', '{'}, {"BracketRight", ']', '}'}, {"Semicolon", ';', ':'}, {"Apostrophe", '\'', '"'},
		{"Comma", ',', '<'}, {"Period", '.', '>'}, {"Slash", '/', '?'},
		{"Numpad0", '0', '0'}, {"NumpadDecimal", '.', '.'}, {"NumpadSubtract", '-', '-'},
		{"NumpadDivide", '/', '/'}, {"NumpadMultiply", '*', '*'}, {"NumpadAdd", '+', '+'},
		{"NumpadEnter", 13, 13}, {"Enter", 13, 13}, {"Delete", 8, 8}, {"Backspace", 8, 8},
		{"Home", '(', '('}, {"End", ')', ')'},
	} {
		for _, shift := range []string{"", "ShiftLeft", "ShiftRight"} {
			t.Run(f.name+shift, func(t *testing.T) {
				var state NativeInputState
				current := []NativeHostKeySample{}
				if shift != "" {
					current = append(current, NativeHostKeySample{Raw: nativeDesktopBinding(t, shift).Raw, Down: true})
				}
				current = append(current, NativeHostKeySample{Raw: nativeDesktopBinding(t, f.name).Raw, Down: true})
				wires, err := NativeHostKeyboardTransitions(nil, current)
				if err != nil {
					t.Fatal(err)
				}
				for _, wire := range wires {
					if err := state.KeyboardInterrupt(wire); err != nil {
						t.Fatal(err)
					}
				}
				want := f.plain
				if shift != "" {
					want = f.shifted
				}
				got, err := rules.Character(&state, nil)
				if err != nil || got != want {
					t.Fatalf("original character: got%q want%q err%v", got, want, err)
				}
			})
		}
	}
}

func TestNativeDesktopKeyboardAliases(t *testing.T) {
	left, right := nativeDesktopBinding(t, "ControlLeft"), nativeDesktopBinding(t, "ControlRight")
	if left.Raw != right.Raw {
		t.Fatal("portable Controls must share the original Control key")
	}
	var state NativeInputState
	var previous []NativeHostKeySample
	for _, f := range []struct {
		left, right bool
		events      int
		held        bool
	}{
		{true, true, 1, true}, {false, true, 0, true}, {true, false, 0, true}, {false, false, 1, false},
	} {
		current := []NativeHostKeySample{{left.Raw, f.left}, {right.Raw, f.right}}
		wires, err := NativeHostKeyboardTransitions(previous, current)
		if err != nil || len(wires) != f.events {
			t.Fatal("alias transition differs", wires, err)
		}
		for _, wire := range wires {
			if err := state.KeyboardInterrupt(wire); err != nil {
				t.Fatal(err)
			}
		}
		wire, _ := NativeKeyWire(left.Raw, true)
		count := uint16(0)
		if f.held {
			count = 1
		}
		if (state.Low[0x32+int(wire)] != 0) != f.held || state.word(0x132) != count {
			t.Fatal("source Control latch/count differs")
		}
		previous = current
	}
	press, _ := NativeKeyWire(left.Raw, true)
	release, _ := NativeKeyWire(left.Raw, false)
	wires, err := NativeHostKeyboardTransitions([]NativeHostKeySample{{left.Raw, true}}, nil)
	if err != nil || !reflect.DeepEqual(wires, []byte{release}) || press == release {
		t.Fatal("removed portable key must release its native latch", wires, err)
	}
	if wires, err := NativeHostKeyboardTransitions(nil, []NativeHostKeySample{{128, true}}); err == nil || len(wires) != 0 {
		t.Fatal("invalid host sample must not produce partial input")
	}
}

func TestNativeDesktopHelpReachesOriginalSpellHelp(t *testing.T) {
	exe := testBundle(t).Executable
	host, err := NewNativeHunkMemory(exe, []uint32{0x100000, 0x200000, 0x300000, 0x400000, 0x500000, 0x600000})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := host.Span(0x200000, 0x11280)
	code, _ := host.Span(0x100000, 0x3fa2c)
	var input NativeInputState
	memory := input.Memory(commandFrameBacking(raw))
	for _, p := range []nativeHeroPatch{{0xeb42, 2, 1}, {0xf3a, 2, 0}, {0xe914, 1, 1}} {
		renderFramePatch(memory, p)
	}
	key := nativeDesktopBinding(t, "F11")
	wire, err := NativeKeyWire(key.Raw, true)
	if err != nil || key.Raw != 0x5f {
		t.Fatal("desktop Help must be the source dedicated Help key", key, err)
	}
	if err := input.KeyboardInterrupt(wire); err != nil {
		t.Fatal(err)
	}
	rules, err := DecodeNativeGameplayHUDInputRules(exe)
	if err != nil {
		t.Fatal(err)
	}
	frame := NativeFrameRegisterContext{AddressBase: 0x200000}
	state := NativeGameplayHUDInputState{Entry: 0x2472}
	calls := 0
	step, err := state.Advance(&rules, NativeGameplayHUDInputCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{
		Frame: &frame, Memory: memory, Code: commandFrameBacking(code), RAM: host.Memory(), CodeBase: 0x100000,
		Call: func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
			if call.Routine != 0x517a || call.Frame.D[1] != 0 {
				t.Fatalf("desktop Help reached unexpected source child%x D1%x", call.Routine, call.Frame.D[1])
			}
			calls++
			return NativeCommandFrameResult{Complete: true}, nil
		},
	}})
	if err != nil || !step.Complete || calls != 1 {
		t.Fatal("desktop Help did not reach the original animated-help boundary", step, calls, err)
	}
}

func TestNativeDesktopParenthesesProduceOriginalModeCommands(t *testing.T) {
	keys, err := DecodeNativeInputRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	render, err := DecodeNativeRenderFrameRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []struct {
		name    string
		command byte
	}{{"Home", 14}, {"End", 18}} {
		t.Run(f.name, func(t *testing.T) {
			host := nativeHostTestMemory(t)
			ram := host.Memory()
			var input NativeInputState
			m := input.Memory(nativeOffsetMemory(ram, 0x200000))
			physical := nativeByteAddressMemory(func(at int) (uint8, error) {
				if at >= 0x200000 && at < 0x211280 {
					return m.Read8(at - 0x200000)
				}
				return ram.Read8(at)
			}, func(at int, value uint8) error {
				if at >= 0x200000 && at < 0x211280 {
					return m.Write8(at-0x200000, value)
				}
				return ram.Write8(at, value)
			})
			for _, p := range []nativeHeroPatch{{0xeb6a, 4, 0x20eb56}, {0xf0c, 2, 23}, {0x138, 2, 319}, {0x13a, 2, 199}, {0x5f44, 2, 24}, {0x5f46, 2, 25}, {0xeb18, 2, 2}, {0xeb42, 2, 1}} {
				renderFramePatch(m, p)
			}
			code := nativeOffsetMemory(ram, 0x100000)
			if err := code.Write32(0xe458, 0x00c00048); err != nil {
				t.Fatal(err)
			}
			binding := nativeDesktopBinding(t, f.name)
			wires, err := NativeHostKeyboardTransitions(nil, []NativeHostKeySample{{binding.Raw, true}})
			if err != nil || len(wires) != 1 {
				t.Fatal("desktop mode key transition missing", wires, err)
			}
			if err := input.KeyboardInterrupt(wires[0]); err != nil {
				t.Fatal(err)
			}
			frame := NativeFrameRegisterContext{AddressBase: 0x200000}
			calls := 0
			cb := NativeGameplayInputCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{
				Memory: m, Code: code, RAM: physical, Frame: &frame, CodeBase: 0x100000,
				Call: func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
					if call.Routine == 0xd2b4 {
						err := render.TerrainHeight(m, call.Frame)
						return NativeCommandFrameResult{Complete: err == nil}, err
					}
					// The controller's genuine sound boundary is explicit; this
					// test proves keyboard admission and packet production.
					if call.Routine != 0x184f6 || uint16(call.Frame.D[0]) != 0x1cc {
						t.Fatalf("unexpected mode-key child%x D0%x", call.Routine, call.Frame.D[0])
					}
					calls++
					return NativeCommandFrameResult{Complete: true}, nil
				},
			}, Input: &input, Keys: keys}
			state := NativeGameplayInputState{}
			step, err := state.Advance(cb)
			command, readErr := m.Read8(0xeb57)
			if err != nil || readErr != nil || !step.Complete || command != f.command || calls != 1 {
				t.Fatal("original mode command differs", command, f.command, calls, step, err, readErr)
			}
		})
	}
}
