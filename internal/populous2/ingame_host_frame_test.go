package populous2

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
)

func TestNativeInGameHostComposedAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/ingame_host_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Input  ingameFrameInput
			Frames []struct {
				PC                                       int
				D                                        [8]uint32
				BSSHash, ChipHash, PointerHash, BankHash string
				LastY                                    uint16
				ScratchHash                              string
				Window, Mouse, Options                   []byte
				Selector, Patch, Copper                  uint32
				Calls                                    []struct {
					Routine int
					D       [8]uint32
					Hash    string
				}
				Sounds []uint16
			}
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 24 {
		t.Fatal("composed menu corpus incomplete", err)
	}
	bundle := testBundle(t)
	rules, err := DecodeNativeInGameHostRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	sprites, err := DecodeNativeSpriteBitmapBank(bundle, 0)
	if err != nil {
		t.Fatal(err)
	}
	snapshots := 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			presentation, err := NewNativeFramePresentationState(bundle.Executable, 0x500000, 0x400000)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0x408; i < len(presentation.Chip); i++ {
				presentation.Chip[i] = byte(i*17 + 3 + ((i-0x408)/32000)*91)
			}
			presentation.InterruptChain = false
			// The source reference enters446A after genuine1069C icon prep.
			// Supply that same initialized shared HUNK3, independently of menu
			// execution; RestorePanel uses its cached prepared icon sink.
			for descriptor, icon := range rules.ProfilePanel.Icons {
				offset := int(binary.BigEndian.Uint32(bundle.Executable.Hunks[0].Data[descriptor:]))
				copy(presentation.PointerData[offset:], icon.Planes)
			}
			if _, err := presentation.Initialize(bundle.Executable, NativeMouseSample{}); err != nil {
				t.Fatal(err)
			}
			memory := presentation.Memory(commandFrameBacking(make([]byte, 0x11280)))
			front, _ := memory.Read32(0x1a)
			for _, patch := range []nativeHeroPatch{{0x3b0, 2, uint32(f.Input.DialogFlag)}, {0xf40, 4, 10}, {0xf32, 4, 0xc930}, {0xeb42, 2, uint32(f.Input.Profile)}, {0xeb44, 2, uint32(f.Input.GameMode)}, {0xf0e, 2, uint32(f.Input.PaintFlag)}, {0xe76a + int(int16(f.Input.Profile*314)) + 0x1a, 2, uint32(f.Input.ControlMode)}, {0xeb6a, 4, 0x200000 + 0xeb56}, {0x22, 4, front}, {0xe8a4, 4, 10000}, {0xe9de, 4, 15000}} {
				renderFramePatch(memory, patch)
			}
			code := fileFrameRelocatedCode(t)
			cm := commandFrameBacking(code)
			_ = cm.Write16(0x3ea, 0)
			_ = cm.Write16(0x4468, 0)
			if f.Input.Save {
				_ = cm.Write16(0x4468, 1)
			}
			_ = cm.Write16(0x3f90, f.Input.Alternate)
			copy(code[0x4440:], f.Input.Drawer)
			code[0x4440+len(f.Input.Drawer)] = 0
			copy(code[0x4416:], f.Input.Filename)
			code[0x4416+len(f.Input.Filename)] = 0
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			image := rules.ProfilePanel.Render.Images.NewImageState()
			host := NativeInGameHostState{}
			resolver := func(address uint32) ([]byte, error) {
				at, err := presentation.chipAt(address, 32000)
				if err != nil {
					return nil, err
				}
				return presentation.Chip[at : at+32000], nil
			}
			expected, calls, sounds := 0, 0, 0
			cb := NativeInGameHostCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Code: cm, Memory: memory, CodeBase: 0x100000, Frame: &frame, Presentation: presentation, Bitmap: resolver,
				ReadAbsolute: func(address uint32) (uint8, error) {
					if int(address) >= len(f.Input.Absolute) {
						return 0, fmt.Errorf("physical byte%x unavailable", address)
					}
					return f.Input.Absolute[address], nil
				},
				Sound: func(cue uint16, c *NativeFrameRegisterContext) error {
					want := f.Frames[expected]
					if sounds >= len(want.Sounds) || cue != want.Sounds[sounds] {
						return fmt.Errorf("native menu click sound differs")
					}
					sounds++
					c.Word(0, cue)
					_, err := rules.Audio.Run(0x184f6, NativeAudioControlFrameCallbacks{Memory: memory, Frame: c, CodeBase: 0x100000})
					return err
				},
				Call: func(call NativeFileFrameCall, _ *uint32) (NativeCommandFrameResult, error) {
					want := f.Frames[expected]
					if call.Routine != 0x181c0 || calls >= len(want.Calls) || want.Calls[calls].Routine != call.Routine || call.Frame.D != want.Calls[calls].D || fileFrameHash(fileFrameMemoryBytes(t, memory)) != want.Calls[calls].Hash {
						return NativeCommandFrameResult{}, fmt.Errorf("native resume boundary differs")
					}
					calls++
					return NativeCommandFrameResult{Complete: true, Zero: f.Input.ResumeZero}, nil
				},
			}, Image: &image, Sprite: sprites.Paint, Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }}
			session := NativeFrameSession{Presentation: presentation}
			menuPhase := uint32(0)
			menu := session.MenuFrame(&rules, &host, cb)
			check := func() {
				done, err := menu(memory, &frame, &image, &menuPhase)
				if err != nil {
					t.Fatalf("frame%d: %v", expected, err)
				}
				want := f.Frames[expected]
				pc := host.Menu.PC
				if done {
					pc = 0
				} else if host.ChildRoutine == 0x471c {
					pc = host.Options.PC
					if host.Options.Modal.Active {
						pc = 0x786
					}
				}
				if pc != want.PC || frame.D != want.D {
					t.Fatalf("native composed menu frame%d PC/D differs: %x/%x D%x/%x", expected, pc, want.PC, frame.D, want.D)
				}
				mouse := []uint16{presentation.Input.Mouse.Image, presentation.Input.Mouse.CounterX, presentation.Input.Mouse.CounterY, presentation.Input.Mouse.PositionX, presentation.Input.Mouse.PositionY, presentation.Input.Mouse.MaximumY}
				for i, value := range mouse {
					binary.BigEndian.PutUint16(code[0xa2a+i*2:], value)
				}
				for _, hash := range []struct{ name, got, want string }{{"BSS", fileFrameHash(fileFrameMemoryBytes(t, memory)), want.BSSHash}, {"chip", fileFrameHash(presentation.Chip), want.ChipHash}, {"pointer", fileFrameHash(presentation.PointerData[:15260]), want.PointerHash}, {"image bank", fileFrameHash(image.AudioBank[:]), want.BankHash}} {
					if hash.got != hash.want {
						t.Fatalf("native composed menu frame%d %s differs: %s/%s", expected, hash.name, hash.got, hash.want)
					}
				}
				if !bytes.Equal(code[0x4710:0x471c], want.Window) || fileFrameHash(code[0xab4e:0xab4e+2048]) != want.ScratchHash || !bytes.Equal(code[0xa2a:0xa36], want.Mouse) || !bytes.Equal(code[0x4952:0x4978], want.Options) {
					t.Fatalf("native composed menu frame%d CODE differs", expected)
				}
				if image.LastY != want.LastY || presentation.CopperSelector != want.Selector || presentation.SpritePatchPointer != want.Patch || presentation.ActiveCopper != want.Copper || calls != len(want.Calls) || sounds != len(want.Sounds) {
					t.Fatal("native menu metadata/call sequence differs")
				}
				calls, sounds = 0, 0
				snapshots++
			}
			check()
			irq := func(x, y uint8, left bool) {
				if _, err := presentation.VBlank(NativeMouseSample{CounterX: x, CounterY: y, Left: left}, memory, &frame); err != nil {
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
				t.Fatalf("native menu action%d unavailable", action)
				return 0, 0
			}
			for i, event := range f.Input.Events {
				expected = i + 1
				if event.Action > 0 {
					x, y := find(event.Action)
					for int(presentation.Input.Mouse.PositionX) != x*2 || int(presentation.Input.Mouse.PositionY) != y*2 {
						dx := max(-100, min(100, x*2-int(presentation.Input.Mouse.PositionX)))
						dy := max(-100, min(100, y*2-int(presentation.Input.Mouse.PositionY)))
						irq(uint8(int(presentation.Input.Mouse.CounterX)+dx), uint8(int(presentation.Input.Mouse.CounterY)+dy), false)
					}
					irq(uint8(presentation.Input.Mouse.CounterX), uint8(presentation.Input.Mouse.CounterY), true)
				} else if event.VBlank {
					irq(uint8(presentation.Input.Mouse.CounterX), uint8(presentation.Input.Mouse.CounterY), false)
				}
				for _, wire := range event.Keys {
					if err := presentation.Input.KeyboardInterrupt(wire); err != nil {
						t.Fatal(err)
					}
				}
				check()
			}
			if !host.Menu.Finished {
				t.Fatal("original composed menu did not return")
			}
			if menuPhase != 2 {
				t.Fatal("concrete session menu did not finish its retained invocation")
			}
		})
	}
	if snapshots != 78 {
		t.Fatal("composed menu snapshot coverage incomplete", snapshots)
	}
}

func TestNativeInGameHostExternalWaitAndFailureAreRetained(t *testing.T) {
	rules, err := DecodeNativeInGameHostRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	frame := NativeFrameRegisterContext{D: [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}}
	host := NativeInGameHostState{}
	want := errors.New("native transport failure")
	calls := 0
	cb := NativeInGameHostCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Call: func(call NativeFileFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
		calls++
		if *phase == 0 {
			*phase = 1
			call.Frame.D[4] = 0xabcdef01
			return NativeCommandFrameResult{}, nil
		}
		if call.Frame.D[4] != 0xabcdef01 {
			t.Fatal("external child registers lost")
		}
		return NativeCommandFrameResult{}, want
	}}}
	phase := uint32(0)
	call := NativeFileFrameCall{Routine: 0x181c0, Frame: &frame}
	if result, err := host.AdvanceChild(&rules, cb, call, &phase); err != nil || result.Complete || host.ChildRoutine != call.Routine {
		t.Fatal("real external wait was acknowledged", result, err)
	}
	if result, err := host.AdvanceChild(&rules, cb, call, &phase); result.Complete || !errors.Is(err, want) || calls != 2 {
		t.Fatal("native child failure changed", result, err, calls)
	}
	if _, err := host.AdvanceChild(&rules, cb, call, &phase); !errors.Is(err, want) || calls != 2 {
		t.Fatal("failed native child was replayed", err, calls)
	}
}
