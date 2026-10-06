package populous2

import "fmt"

// NativeDesktopKeyBinding names a portable key and its original Amiga raw
// code. Printable codes are checked against the executable's $100f4 banks.
// F11 supplies Help, Home/End the keypad parentheses, and Meta the Amiga keys.
type NativeDesktopKeyBinding struct {
	Name string
	Raw  uint8
}

var nativeDesktopKeyBindings = [...]NativeDesktopKeyBinding{
	{"Escape", 0x45}, {"Space", 0x40}, {"Enter", 0x44}, {"Backspace", 0x41}, {"Delete", 0x46}, {"Tab", 0x42},
	{"ArrowUp", 0x4c}, {"ArrowDown", 0x4d}, {"ArrowLeft", 0x4f}, {"ArrowRight", 0x4e},
	{"ShiftLeft", 0x60}, {"ShiftRight", 0x61}, {"ControlLeft", 0x63}, {"ControlRight", 0x63}, {"AltLeft", 0x64}, {"AltRight", 0x65}, {"CapsLock", 0x62}, {"MetaLeft", 0x66}, {"MetaRight", 0x67},
	{"A", 0x20}, {"B", 0x35}, {"C", 0x33}, {"D", 0x22}, {"E", 0x12}, {"F", 0x23}, {"G", 0x24}, {"H", 0x25}, {"I", 0x17}, {"J", 0x26}, {"K", 0x27}, {"L", 0x28}, {"M", 0x37}, {"N", 0x36}, {"O", 0x18}, {"P", 0x19}, {"Q", 0x10}, {"R", 0x13}, {"S", 0x21}, {"T", 0x14}, {"U", 0x16}, {"V", 0x34}, {"W", 0x11}, {"X", 0x32}, {"Y", 0x15}, {"Z", 0x31},
	{"Digit1", 0x01}, {"Digit2", 0x02}, {"Digit3", 0x03}, {"Digit4", 0x04}, {"Digit5", 0x05}, {"Digit6", 0x06}, {"Digit7", 0x07}, {"Digit8", 0x08}, {"Digit9", 0x09}, {"Digit0", 0x0a},
	{"Backquote", 0x00}, {"Minus", 0x0b}, {"Equal", 0x0c}, {"Backslash", 0x0d}, {"BracketLeft", 0x1a}, {"BracketRight", 0x1b}, {"Semicolon", 0x29}, {"Apostrophe", 0x2a}, {"Comma", 0x38}, {"Period", 0x39}, {"Slash", 0x3a},
	{"F1", 0x50}, {"F2", 0x51}, {"F3", 0x52}, {"F4", 0x53}, {"F5", 0x54}, {"F6", 0x55}, {"F7", 0x56}, {"F8", 0x57}, {"F9", 0x58}, {"F10", 0x59}, {"F11", 0x5f},
	{"Numpad0", 0x0f}, {"Numpad1", 0x1d}, {"Numpad2", 0x1e}, {"Numpad3", 0x1f}, {"Numpad4", 0x2d}, {"Numpad5", 0x2e}, {"Numpad6", 0x2f}, {"Numpad7", 0x3d}, {"Numpad8", 0x3e}, {"Numpad9", 0x3f},
	{"NumpadDecimal", 0x3c}, {"NumpadEnter", 0x43}, {"NumpadSubtract", 0x4a}, {"NumpadDivide", 0x5c}, {"NumpadMultiply", 0x5d}, {"NumpadAdd", 0x5e},
	{"Home", 0x5a}, {"End", 0x5b},
}

// NativeDesktopKeyBindings returns a caller-owned mapping. The backend uses
// the same names tested by the headless source-input tests.
func NativeDesktopKeyBindings() []NativeDesktopKeyBinding {
	return append([]NativeDesktopKeyBinding(nil), nativeDesktopKeyBindings[:]...)
}

type NativeHostKeySample struct {
	Raw  uint8
	Down bool
}

// NativeHostKeyboardTransitions merges portable aliases before emitting $620
// wire events. Releasing one Control key while the other is held must not
// release the Amiga's single Control key. Ordering follows the current sample,
// so modifier changes precede printable keys in the configured mapping.
func NativeHostKeyboardTransitions(previous, current []NativeHostKeySample) ([]uint8, error) {
	var before, after, seen [128]bool
	for _, sample := range previous {
		if sample.Raw >= 128 {
			return nil, fmt.Errorf("native previous host key outside raw bank")
		}
		before[sample.Raw] = before[sample.Raw] || sample.Down
	}
	for _, sample := range current {
		if sample.Raw >= 128 {
			return nil, fmt.Errorf("native current host key outside raw bank")
		}
		after[sample.Raw] = after[sample.Raw] || sample.Down
	}
	var wires []uint8
	for _, samples := range [][]NativeHostKeySample{current, previous} {
		for _, sample := range samples {
			if seen[sample.Raw] {
				continue
			}
			seen[sample.Raw] = true
			if before[sample.Raw] == after[sample.Raw] {
				continue
			}
			wire, err := NativeKeyWire(sample.Raw, after[sample.Raw])
			if err != nil {
				return nil, err
			}
			wires = append(wires, wire)
		}
	}
	return wires, nil
}
