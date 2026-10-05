package populous2

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
)

type serialFrameFixture struct {
	Input struct {
		Name           string
		Baud, Mode     uint16
		Incoming       []byte
		History, Modem string
		Cancelled      bool
		D              [8]uint32
		Events         []struct {
			Action int
			Keys   []byte
			VBlank bool
		}
	}
	Frames []struct {
		PC                                       int
		Waiting, Complete                        bool
		D                                        [8]uint32
		BSSHash, CodeHash, ChipHash, PointerHash string
		Selector, Patch, Copper                  uint32
		Sounds                                   []uint16
		Sent                                     []byte
		Handshake                                bool
		Calls                                    []struct {
			Routine                     int
			D, AfterD                   [8]uint32
			A0                          uint32
			Zero, Negative              bool
			BSSHash, CodeHash, ChipHash string
		}
	}
}

func TestNativeSerialFrameDoesNotCompleteMissingOrSuspendedTransport(t *testing.T) {
	bundle := testBundle(t)
	rules, err := DecodeNativeSerialFrameRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewNativeFramePresentationState(bundle.Executable, 0x500000, 0x400000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.Initialize(bundle.Executable, NativeMouseSample{}); err != nil {
		t.Fatal(err)
	}
	code := fileFrameRelocatedCode(t)
	m := p.Memory(commandFrameBacking(make([]byte, 0x11280)))
	_ = m.Write16(0x15a, 300)
	frame := NativeFrameRegisterContext{D: [8]uint32{0x11112222, 0x33334444, 0x55556666, 0x77778888, 0x9999aaaa, 0xbbbbcccc, 0xddddeeee, 0x12345678}, AddressBase: 0x200000}
	cb := NativeSerialFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Code: commandFrameBacking(code), Memory: m, CodeBase: 0x100000, Frame: &frame, Presentation: p, Bitmap: func(address uint32) ([]byte, error) {
		at := int(address - p.ChipBase)
		if at < 0 || at > len(p.Chip)-32000 {
			return nil, fmt.Errorf("actual screen unavailable")
		}
		return p.Chip[at : at+32000], nil
	}}}
	state := NativeSerialFrameState{}
	step, err := state.Advance(&rules, cb)
	if err == nil || step.Complete || state.PC != 0x49be {
		t.Fatal("missing configure transport completed native requester")
	}
	beforeCode, beforeChip := fileFrameHash(code), fileFrameHash(p.Chip)
	if step, again := state.Advance(&rules, cb); again != err || step.Complete || beforeCode != fileFrameHash(code) || beforeChip != fileFrameHash(p.Chip) {
		t.Fatal("failed transport replayed its native prefix")
	}
	state = NativeSerialFrameState{}
	calls := 0
	cb.Transport = func(call NativeSerialFrameCall, phase *uint32) (NativeSerialFrameChildResult, error) {
		calls++
		if call.Routine != 0xab4 || *phase != uint32(calls-1) {
			return NativeSerialFrameChildResult{}, fmt.Errorf("pending transport arguments changed")
		}
		*phase++
		return NativeSerialFrameChildResult{}, nil
	}
	step, err = state.Advance(&rules, cb)
	if err != nil || !step.Waiting || step.Complete || state.PC != 0x49be {
		t.Fatal("suspended configure did not retain its source call")
	}
	codeHash, chipHash, registers := fileFrameHash(code), fileFrameHash(p.Chip), frame.D
	step, err = state.Advance(&rules, cb)
	if err != nil || !step.Waiting || calls != 2 || frame.D != registers || codeHash != fileFrameHash(code) || chipHash != fileFrameHash(p.Chip) {
		t.Fatal("pending configure replayed requester setup")
	}
	failure := errors.New("actual transport failure")
	cb.Transport = func(NativeSerialFrameCall, *uint32) (NativeSerialFrameChildResult, error) {
		return NativeSerialFrameChildResult{}, failure
	}
	if step, err = state.Advance(&rules, cb); err != failure || step.Complete {
		t.Fatal("transport error fabricated a connection")
	}
	if _, err = state.Advance(&rules, cb); err != failure {
		t.Fatal("transport error was silently retried")
	}
}

func TestNativeSerialFrameKeepsOriginalSingleCRSuffix(t *testing.T) {
	data, err := os.ReadFile("testdata/serial_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []serialFrameFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	var actual []byte
	for _, fixture := range catalog.Cases {
		if fixture.Input.Name == "modem-AT" {
			for _, frame := range fixture.Frames {
				actual = append(actual, frame.Sent...)
			}
		}
	}
	if string(actual) != "AT\r" {
		t.Fatalf("original temporary WORD0D0D/length1 proof changed: %x", actual)
	}
	rules, err := DecodeNativeSerialRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := NewNativeSerialRequester(rules, 300, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = legacy.Action(6); err != nil {
		t.Fatal(err)
	}
	typed, err := legacy.FinishModemEdit([]byte("AT"))
	if err != nil {
		t.Fatal(err)
	}
	if string(typed.ModemBytes) != "AT\r\r" {
		t.Fatal("legacy adapter discrepancy changed; reassess the source controller binding")
	}
}

func TestNativeSerialFrameAgainstOriginalCPUAndIRQs(t *testing.T) {
	data, err := os.ReadFile("testdata/serial_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []serialFrameFixture }
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 63 {
		t.Fatalf("native serial corpus incomplete: %v", err)
	}
	bundle := testBundle(t)
	rules, err := DecodeNativeSerialFrameRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	frames, waiting, finished := 0, 0, 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			if len(f.Frames) == 0 {
				t.Fatal("empty original options trace")
			}
			p, err := NewNativeFramePresentationState(bundle.Executable, 0x500000, 0x400000)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0x408; i < len(p.Chip); i++ {
				p.Chip[i] = byte(i*17 + 3 + ((i-0x408)/32000)*91)
			}
			p.InterruptChain = false
			if _, err = p.Initialize(bundle.Executable, NativeMouseSample{}); err != nil {
				t.Fatal(err)
			}
			m := p.Memory(commandFrameBacking(make([]byte, 0x11280)))
			code := fileFrameRelocatedCode(t)
			cm := commandFrameBacking(code)
			_ = cm.Write16(0x3ea, 0)
			_ = cm.Write32(0x77a, p.CopperSelector)
			_ = cm.Write32(0x77e, p.SpritePatchPointer)
			for _, patch := range []nativeHeroPatch{{0x15a, 2, uint32(f.Input.Baud)}, {0xeb44, 2, uint32(f.Input.Mode)}, {0x15e, 2, 7}, {0x156, 2, uint32(len(f.Input.Incoming))}, {0xeb5e, 1, 6}, {0xeb68, 1, 6}, {0xeb72, 1, 6}} {
				renderFramePatch(m, patch)
			}
			for i, v := range f.Input.Incoming {
				_ = m.Write8(0x160+i, v)
			}
			copy(code[0x4b92:], append([]byte(f.Input.History), 0))
			copy(code[0x4b42:], append([]byte(f.Input.Modem), 0))
			if f.Input.Cancelled {
				_ = m.Write8(0xa7, 1)
				_ = m.Write8(0x6f, 1)
			}

			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			state := NativeSerialFrameState{}
			expected, sounds, calls := 0, 0, 0
			sent := []byte{}
			resolver := func(address uint32) ([]byte, error) {
				at := int(int64(address) - int64(p.ChipBase))
				if at < 0 || at > len(p.Chip)-32000 {
					return nil, fmt.Errorf("native serial target %#x unavailable", address)
				}
				return p.Chip[at : at+32000], nil
			}
			cancelled := func() bool {
				a, _ := m.Read8(0xa7)
				b, _ := m.Read8(0x6f)
				d, _ := m.Read8(0x71)
				return a != 0 && (b != 0 || d != 0)
			}
			disconnect := func() error {
				return DisconnectNativeSerial(NativeSerialCallbacks{Memory: m, Port: NativeSerialPort{Flush: func() error {
					if err := m.Write16(0x154, 0); err != nil {
						return err
					}
					return m.Write16(0x156, 0)
				}}})
			}
			cb := NativeSerialFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Code: cm, Memory: m, CodeBase: 0x100000, Frame: &frame, Presentation: p, Bitmap: resolver, Sound: func(cue uint16, c *NativeFrameRegisterContext) error {
				want := f.Frames[expected]
				if sounds >= len(want.Sounds) || cue != want.Sounds[sounds] {
					return fmt.Errorf("frame%d source sound differs", expected)
				}
				sounds++
				return nil
			}}, CallerStackPointer: 0xef0000}
			cb.Transport = func(call NativeSerialFrameCall, phase *uint32) (NativeSerialFrameChildResult, error) {
				if call.Routine == 0x17eec {
					return NativeSerialFrameChildResult{}, nil
				} // Genuine unexecuted handshake boundary.
				want := f.Frames[expected]
				if calls >= len(want.Calls) {
					return NativeSerialFrameChildResult{}, fmt.Errorf("unexpected child%x", call.Routine)
				}
				original := want.Calls[calls]
				if call.Routine != original.Routine || call.Frame.D != original.D {
					return NativeSerialFrameChildResult{}, fmt.Errorf("frame%d child%x input D differs", expected, call.Routine)
				}
				if call.Routine == 0xbbe || call.Routine == 0xc2a {
					if call.A[0].Address != original.A0 {
						return NativeSerialFrameChildResult{}, fmt.Errorf("native A0 differs: got%x want%x", call.A[0].Address, original.A0)
					}
				}
				// The independent CPU executes these complete hardware children.
				// This bounded host adapter applies their source semantics, not
				// recorded output registers or memory deltas.
				result := NativeSerialFrameChildResult{Complete: true}
				switch call.Routine {
				case 0xab4:
					baud, _ := m.Read16(0x15a)
					pal, _ := m.Read8(0x26)
					clock := uint32(0x369e99)
					if pal != 0 {
						clock = 0x361f0f
					}
					period := uint16(clock/uint32(baud)) - 1
					result.Zero, result.Negative = period == 0, int16(period) < 0
				case 0xae2:
					in, _ := m.Read16(0x154)
					out, _ := m.Read16(0x156)
					value := out - in
					if int16(value) < 0 {
						value = -value
					}
					if cancelled() {
						if err := disconnect(); err != nil {
							return result, err
						}
						value = 0xffff
					}
					call.Frame.Word(0, value)
					result.Zero, result.Negative = value == 0, int16(value) < 0
				case 0xbbe:
					if cancelled() {
						if err := disconnect(); err != nil {
							return result, err
						}
						result.Negative = true
					} else {
						index, _ := m.Read16(0x154)
						limit, _ := m.Read16(0x15e)
						value, _ := m.Read8(0x160 + int(int16(index)))
						if err := cm.Write8(0x4b40, value); err != nil {
							return result, err
						}
						if index == limit {
							index = 0xffff
						}
						if err := m.Write16(0x154, index+1); err != nil {
							return result, err
						}
						result.Zero = true
					}
				case 0xc2a:
					if cancelled() {
						if err := disconnect(); err != nil {
							return result, err
						}
					} else {
						for i := 0; i < int(uint16(call.Frame.D[0])); i++ {
							var value byte
							var err error
							if call.StackBytes != nil {
								if i >= len(call.StackBytes) {
									return result, fmt.Errorf("stack payload outside supplied bytes")
								}
								value = call.StackBytes[i]
							} else {
								value, err = cm.Read8(int(int64(call.A[0].Address)-0x100000) + i)
								if err != nil {
									return result, err
								}
							}
							sent = append(sent, value)
							if err := m.Write16(0x158, 1); err != nil {
								return result, err
							}
						}
					}
				default:
					return result, fmt.Errorf("unknown source child%x", call.Routine)
				}
				if call.Frame.D != original.AfterD || result.Zero != original.Zero || result.Negative != original.Negative {
					return result, fmt.Errorf("frame%d child%x raw return/CCR differs", expected, call.Routine)
				}
				calls++
				return result, nil
			}

			check := func() {
				step, err := state.Advance(&rules, cb)
				if err != nil {
					t.Fatalf("frame%d: %v", expected, err)
				}
				want := f.Frames[expected]
				if step.PC != want.PC || step.Waiting != want.Waiting || step.Complete != want.Complete || step.Idle != (!want.Waiting && !want.Complete) || frame.D != want.D {
					t.Fatalf("frame%d native poll/modal context differs: got%+v D%x wantPC%x wait%v done%v D%x", expected, step, frame.D, want.PC, want.Waiting, want.Complete, want.D)
				}
				values := []uint16{p.Input.Mouse.Image, p.Input.Mouse.CounterX, p.Input.Mouse.CounterY, p.Input.Mouse.PositionX, p.Input.Mouse.PositionY, p.Input.Mouse.MaximumY}
				for i, v := range values {
					binary.BigEndian.PutUint16(code[0xa2a+i*2:], v)
				}
				for _, pair := range []struct{ name, got, want string }{{"BSS", fileFrameHash(fileFrameMemoryBytes(t, m)), want.BSSHash}, {"CODE", fileFrameHash(code), want.CodeHash}, {"chip pixels/Copper", fileFrameHash(p.Chip), want.ChipHash}, {"pointer RAM", fileFrameHash(p.PointerData[:15260]), want.PointerHash}} {
					if pair.got != pair.want {
						t.Fatalf("frame%d native serial %s differs: got%s want%s", expected, pair.name, pair.got, pair.want)
					}
				}
				if p.CopperSelector != want.Selector || p.SpritePatchPointer != want.Patch || p.ActiveCopper != want.Copper || sounds != len(want.Sounds) || calls != len(want.Calls) || string(sent) != string(want.Sent) {
					t.Fatal("native serial retained video/sound metadata differs")
				}
				frames++
				if step.Waiting {
					waiting++
				}
				if step.Complete {
					finished++
				}
				sounds, calls = 0, 0
				sent = sent[:0]
			}
			check()
			irq := func(x, y uint8, left bool) {
				if _, err = p.VBlank(NativeMouseSample{CounterX: x, CounterY: y, Left: left}, m, &frame); err != nil {
					t.Fatal(err)
				}
			}
			find := func(action int) (int, int) {
				start := 0xab4e + int(int16(binary.BigEndian.Uint16(code[0xab4e:])))
				width := int(binary.BigEndian.Uint16(code[0xab54:]))
				count := 0
				for i := 0; code[start+i] != 0; i++ {
					v := code[start+i]
					if int8(v) > 0x5a && int8(code[0x4e92+int(v)-0x5b]) > 0 {
						count += 2
						if count == action {
							return (int(binary.BigEndian.Uint16(code[0xab50:])) + i%(width+1)) * 8, int(binary.BigEndian.Uint16(code[0xab52:])) + i/(width+1)*8
						}
					}
				}
				t.Fatalf("source action%d unavailable", action)
				return 0, 0
			}
			move := func(x, y int) {
				for int(p.Input.Mouse.PositionX) != x*2 || int(p.Input.Mouse.PositionY) != y*2 {
					dx, dy := max(-100, min(100, x*2-int(p.Input.Mouse.PositionX))), max(-100, min(100, y*2-int(p.Input.Mouse.PositionY)))
					irq(uint8(int(p.Input.Mouse.CounterX)+dx), uint8(int(p.Input.Mouse.CounterY)+dy), false)
				}
				irq(uint8(p.Input.Mouse.CounterX), uint8(p.Input.Mouse.CounterY), true)
			}
			for i, event := range f.Input.Events {
				expected = i + 1
				if event.Action > 0 {
					x, y := find(event.Action)
					move(x, y)
				} else if event.VBlank {
					irq(uint8(p.Input.Mouse.CounterX), uint8(p.Input.Mouse.CounterY), false)
				}
				for _, wire := range event.Keys {
					// The original IRQ chains to the genuine interrupted $786
					// instruction. Retain that mutable CODE pointer explicitly.
					if state.Modal.Waiting {
						if err := cm.Write32(0x68a, 0x100786); err != nil {
							t.Fatal(err)
						}
					}
					if err := p.Input.KeyboardInterrupt(wire); err != nil {
						t.Fatal(err)
					}
				}
				check()
			}
			if state.Finished {
				before, registers := fileFrameHash(p.Chip), frame.D
				step, err := state.Advance(&rules, cb)
				if err != nil || !step.Complete || before != fileFrameHash(p.Chip) || frame.D != registers {
					t.Fatal("completed options requester replayed its prefix")
				}
			}
		})
	}
	if frames != 479 || waiting != 18 || finished != 61 {
		t.Fatalf("native serial coverage changed: frames%d waits%d exits%d", frames, waiting, finished)
	}
}
