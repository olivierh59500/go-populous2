package populous2

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type mainTraversalFixture struct {
	Input struct {
		protectionFrameInput
		Mixed                            bool
		Kind, Stage, State, Owner, Flags uint8
		Alternate                        bool
		View                             uint16
		Land                             int
	}
	CallerA      [7]uint32
	ChildEntered bool
	ChildEntryD  [8]uint32
	ChildEntryA  [7]uint32
	Frames       []struct {
		PC                                                 int
		D                                                  [8]uint32
		BSSHash, ChipHash, PointerHash, CodeHash, BankHash string
		Selector, Patch, Copper                            uint32
		LastY, TownHitHeight                               uint16
		Calls                                              []struct {
			Routine   int
			D, AfterD [8]uint32
		}
		Sounds []uint16
	}
}

func TestNativeMainTraversalThroughOriginalProtectionAndIRQs(t *testing.T) {
	data, err := os.ReadFile("testdata/render_frame_main_traversal_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []mainTraversalFixture
	}
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 68 {
		t.Fatalf("native retained actor corpus incomplete: %v", err)
	}
	bundle := testBundle(t)
	rules, err := DecodeNativeActorRenderRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	protectionRules, err := DecodeNativeProtectionFrameRules(bundle.Executable, bundle.Raw["faces.pak"])
	if err != nil {
		t.Fatal(err)
	}
	sprites, err := DecodeNativeSpriteBitmapBank(bundle, 0)
	if err != nil {
		t.Fatal(err)
	}
	frames := 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			if !f.ChildEntered || len(f.Frames) < 2 || f.Frames[len(f.Frames)-1].PC != 0 {
				t.Fatal("original actor/modal did not complete")
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
			_ = cm.Write32(0xbe48, 0x900000)
			_ = cm.Write32(0xc380, 0x900000)
			_ = cm.Write16(0xa2a, f.Input.Cursor)
			p.Input.Mouse.Image = f.Input.Cursor
			_ = cm.Write32(0x77a, p.CopperSelector)
			_ = cm.Write32(0x77e, p.SpritePatchPointer)
			_ = m.Write16(0x3b0, f.Input.Gate)
			_ = m.Write16(0xe, f.Input.Counter)
			_ = m.Write32(0x3ac, 0x3ffffff)
			at := 0x76f4
			_ = m.Write8(at, f.Input.Kind)
			_ = m.Write8(at+1, f.Input.Stage)
			_ = m.Write8(at+12, f.Input.Owner)
			_ = m.Write8(at+13, f.Input.Flags)
			_ = m.Write8(at+22, f.Input.State)
			_ = m.Write32(at+26, 1000)
			_ = m.Write16(at+6, 20*256+0xa0)
			_ = m.Write16(at+8, 20*256+0x68)
			if f.Input.Mixed {
				_ = m.Write16(at+2, 0x68)
				z := 0x7728
				_ = m.Write16(z+4, 0x34)
				_ = m.Write8(z, 2)
				_ = m.Write8(z+12, 2)
				_ = m.Write8(z+22, 2)
				_ = m.Write16(z+6, 20*256+0x90)
				_ = m.Write16(z+8, 20*256+0x70)
				_ = m.Write32(z+26, 500)
				z = 0x775c
				_ = m.Write8(z, 2)
				_ = m.Write8(z+22, 2)
				_ = m.Write8(z+12, 1)
				_ = m.Write32(z+26, 700)
				_ = m.Write16(z+6, 21*256+0x80)
				_ = m.Write16(z+8, 20*256+0x80)
			}
			_ = m.Write16(0x5f44, 20)
			_ = m.Write16(0x5f46, 20)
			view := uint16(8)
			if f.Input.Alternate {
				view = f.Input.View
			}
			for _, patch := range []nativeHeroPatch{{0xf0c, 2, uint32(view)}, {0xeb42, 2, 1}, {0xeb44, 2, 2}, {0xeb6a, 4, 0x200000 + 0xeb56}, {0xe8a4, 4, 10000}, {0xe9de, 4, 15000}} {
				renderFramePatch(m, patch)
			}
			front, _ := m.Read32(0x1a)
			_ = m.Write32(0x22, front)
			for i := 0; i < 4096; i++ {
				_ = m.Write8(0xf44+i*4, 0)
				_ = m.Write8(0xf45+i*4, 0)
			}
			_ = m.Write16(0xf44+(20+20*64)*4+2, 0x34)
			if f.Input.Mixed {
				_ = m.Write16(0xf44+(21+20*64)*4+2, 0x9c)
			}
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}

			image := rules.Frames.Images.NewImageState()
			main := NativeMainRenderState{}
			protection := NativeProtectionFrameState{}
			for i, a := range f.ChildEntryA {
				protection.A[i] = NativeRequesterAddress{Address: a, Absolute: true}
			}
			resolve := func(address uint32) ([]byte, error) {
				offset := int(int64(address) - int64(p.ChipBase))
				if offset < 0 || offset > len(p.Chip)-32000 {
					return nil, fmt.Errorf("native chip target %#x unavailable", address)
				}
				return p.Chip[offset : offset+32000], nil
			}
			bitmap, err := p.BackBuffer()
			if err != nil {
				t.Fatal(err)
			}
			kindReads, stateReads := 0, 0
			actorMemory := m
			actorMemory.Read8 = func(address int) (uint8, error) {
				if address == at {
					kindReads++
				}
				if address == at+22 {
					stateReads++
				}
				return m.Read8(address)
			}
			actorCB := NativeActorEffectsCallbacks{NativeRenderFrameCallbacks: NativeRenderFrameCallbacks{Memory: actorMemory, Frame: &frame, Image: &image, Bitmap: bitmap, Sprite: sprites.Paint, Input: &p.Input}, Cropped: sprites.PaintCropped, Reinterpreted: sprites.PaintReinterpreted}
			expected, calls, sounds, refreshes, entries := 0, 0, 0, 0, 0
			callCheck := func(routine int, c *NativeFrameRegisterContext) error {
				want := f.Frames[expected]
				if calls >= len(want.Calls) || routine != want.Calls[calls].Routine || c.D != want.Calls[calls].D {
					return fmt.Errorf("frame%d native modal child%x differs", expected, routine)
				}
				calls++
				return nil
			}
			modalCB := NativeProtectionFrameCallbacks{Code: cm, Memory: m, CodeBase: 0x100000, Frame: &frame, Presentation: p, Bitmap: resolve, Beam: func() (uint16, error) { return f.Input.Beam, nil },
				Ownership: func(owned bool, c *NativeFrameRegisterContext) error {
					routine := 0xe28
					if owned {
						routine = 0xe4c
					}
					return callCheck(routine, c)
				},
				Sound: func(cue uint16, c *NativeFrameRegisterContext) error {
					want := f.Frames[expected]
					if sounds >= len(want.Sounds) || cue != want.Sounds[sounds] {
						return fmt.Errorf("frame%d native modal sound differs", expected)
					}
					sounds++
					return nil
				},
				Call: func(call NativeFileFrameCall, _ *uint32) (NativeCommandFrameResult, error) {
					if err := callCheck(call.Routine, call.Frame); err != nil {
						return NativeCommandFrameResult{}, err
					}
					if call.Routine != 0x19cd0 && call.Routine != 0x1a32a {
						return NativeCommandFrameResult{}, fmt.Errorf("unexpected cached resource child%x", call.Routine)
					}
					cursor, err := cm.Read16(0xa2a)
					if err != nil {
						return NativeCommandFrameResult{}, err
					}
					if err := m.Write16(0x3ba, cursor); err != nil {
						return NativeCommandFrameResult{}, err
					}
					call.Frame.D[0] = 2
					if call.Frame.D != f.Frames[expected].Calls[calls-1].AfterD {
						return NativeCommandFrameResult{}, fmt.Errorf("actual cached-resource body output differs")
					}
					return NativeCommandFrameResult{Complete: true}, nil
				},
			}
			children := NativeActorRenderChildren{
				TownInfoAdvance: func(gotAt int, c *NativeFrameRegisterContext) (bool, error) {
					if gotAt != at {
						return false, fmt.Errorf("actor changed across native modal")
					}
					if !protection.Started {
						entries++
						if c.D != f.ChildEntryD {
							return false, fmt.Errorf("genuine314A entry D differs")
						}
					}
					step, err := protection.Advance(&protectionRules, modalCB)
					return step.Complete, err
				},
				RefreshTargets: func(target *NativeActorEffectsCallbacks) error {
					refreshes++
					address, err := m.Read32(0x1e)
					if err != nil {
						return err
					}
					target.Bitmap, err = resolve(address)
					return err
				},
			}
			syncCode := func() {
				values := []uint16{p.Input.Mouse.Image, p.Input.Mouse.CounterX, p.Input.Mouse.CounterY, p.Input.Mouse.PositionX, p.Input.Mouse.PositionY, p.Input.Mouse.MaximumY}
				for i, v := range values {
					binary.BigEndian.PutUint16(code[0xa2a+i*2:], v)
				}
				copy(code[0x185a8:0x18ada], image.AudioBank[:])
				binary.BigEndian.PutUint16(code[0xeee0:], image.LastY)
				binary.BigEndian.PutUint16(code[0xe8ce:], main.Actor.TownHitHeight)
			}
			tiles, err := DecodeNativeTileBitmapBank(bundle.Raw[fmt.Sprintf("block%d.pak", f.Input.Land)])
			if err != nil {
				t.Fatal(err)
			}
			advance := func() (bool, error) {
				target, err := m.Read32(0x1e)
				if err != nil {
					return false, err
				}
				current, err := resolve(target)
				if err != nil {
					return false, err
				}
				effects := actorCB
				effects.Bitmap = current
				background, err := resolve(front)
				if err != nil {
					return false, err
				}
				tileAddress := target
				window := NativeBitmapWindow{Bytes: p.Chip, BitmapOffset: int(tileAddress - p.ChipBase)}
				world := NativeWorldRenderCallbacks{Effects: effects, Tiles: tiles, Background: background, Children: children,
					Tile: func(q NativeTileChunkRequest, bitmap []byte) error {
						if main.Traversal.TileTargetAddress != 0 && main.Traversal.TileTargetAddress != tileAddress {
							tileAddress = main.Traversal.TileTargetAddress
							window = NativeBitmapWindow{Bytes: p.Chip, BitmapOffset: int(tileAddress - p.ChipBase)}
						}
						if &bitmap[0] != &window.Bytes[window.BitmapOffset] {
							return fmt.Errorf("wrong frozen tile target")
						}
						return tiles.PaintChunkWindow(q, window)
					},
					ResolveTargets: func(w *NativeWorldRenderCallbacks) error {
						target, err := m.Read32(0x1e)
						if err != nil {
							return err
						}
						w.Effects.Bitmap, err = resolve(target)
						return err
					},
				}
				return rules.AdvanceMain(NativeMainRenderCallbacks{World: world, Code: cm, RefreshTargets: func(w *NativeWorldRenderCallbacks) error {
					target, err := m.Read32(0x1e)
					if err != nil {
						return err
					}
					w.Effects.Bitmap, err = resolve(target)
					return err
				}}, &main)
			}
			check := func() {
				done, err := advance()
				if err != nil {
					t.Fatalf("frame%d: %v", expected, err)
				}
				want := f.Frames[expected]
				if frame.D != want.D {
					t.Fatalf("frame%d all8D differ: got%x want%x", expected, frame.D, want.D)
				}
				if done != (want.PC == 0) || !done && main.Traversal.Projection.Actor.PC != 0x314a {
					t.Fatal("native actor wait/return differs")
				}

				if !done && protection.PC != want.PC {
					t.Fatalf("frame%d child PC got%x want%x", expected, protection.PC, want.PC)
				}
				syncCode()
				for _, check := range []struct{ name, got, want string }{{"BSS", fileFrameHash(fileFrameMemoryBytes(t, m)), want.BSSHash}, {"chip screens/Copper", fileFrameHash(p.Chip), want.ChipHash}, {"pointer RAM", fileFrameHash(p.PointerData[:15260]), want.PointerHash}, {"CODE", fileFrameHash(code), want.CodeHash}, {"image bank", fileFrameHash(image.AudioBank[:]), want.BankHash}} {
					if check.got != check.want {
						t.Fatalf("frame%d native %s differs: got%s want%s", expected, check.name, check.got, check.want)
					}
				}
				if image.LastY != want.LastY || main.Actor.TownHitHeight != want.TownHitHeight || p.CopperSelector != want.Selector || p.SpritePatchPointer != want.Patch || p.ActiveCopper != want.Copper {
					t.Fatal("native retained renderer metadata differs")
				}

				if calls != len(want.Calls) || sounds != len(want.Sounds) {
					t.Fatal("native modal callback order/count differs")
				}
				calls, sounds = 0, 0
				frames++
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
				t.Fatalf("source action%d not found", action)
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
				check()
			}
			if entries != 1 || refreshes != 1 {
				t.Fatalf("modal prefix or screen target refresh repeated: %d/%d", entries, refreshes)
			}
			wantKindReads := 1
			if f.Input.State == 0x10 {
				wantKindReads++ // The original $e506 CMPI.B kind4 town branch.
			}
			if kindReads != wantKindReads || stateReads != 1 {
				t.Fatalf("actor dispatch replayed across modal wait: kind/state %d/%d", kindReads, stateReads)
			}
			before := fileFrameHash(p.Chip)
			done, err := advance()
			if err != nil || !done || before != fileFrameHash(p.Chip) {
				t.Fatal("completed main traversal was replayed", done, err)
			}

		})
	}
	if frames != 3944 {
		t.Fatalf("native modal snapshot coverage incomplete: %d", frames)
	}
}
