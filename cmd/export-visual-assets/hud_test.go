package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	"os"
	"testing"

	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func TestPrivateHUDMatchesOriginalArtworkIndicatorsAndPopulation(t *testing.T) {
	path := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if path == "" {
		t.Skip("set private original art directory")
	}
	source, err := populous2.LoadFS(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	if err := exportHUD(source, output); err != nil {
		t.Fatal(err)
	}
	art, err := visualassets.LoadHUD(os.DirFS(output))
	if err != nil {
		t.Fatal(err)
	}
	panel, err := populous2.DecodeNativeProfilePanelFrameRules(source.Executable)
	if err != nil {
		t.Fatal(err)
	}
	hud, err := populous2.DecodeNativeHUDRules(source.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for land := 0; land < 4; land++ {
		sprites, err := populous2.DecodeNativeSpriteBitmapBank(source, land)
		if err != nil {
			t.Fatal(err)
		}
		palette := source.Landscapes[land].Palettes[0]
		bank := make([]visualassets.Sprite, len(source.Sprites[land]))
		for id, s := range source.Sprites[land] {
			bank[id] = visualassets.Sprite{Image: s.Image, AnchorX: s.AnchorX, AnchorY: s.AnchorY}
		}
		for _, test := range []struct {
			mana       uint32
			population [2]uint32
			category   int
			clock      uint16
			mouse      image.Point
		}{{1000, [2]uint32{100, 500}, 0, 0, image.Pt(192, 100)}, {200000, [2]uint32{20000, 100000}, 4, 24, image.Pt(50, 180)}} {
			raw := make([]byte, 0x11280)
			put16 := func(at int, v uint16) { binary.BigEndian.PutUint16(raw[at:], v) }
			put32 := func(at int, v uint32) { binary.BigEndian.PutUint32(raw[at:], v) }
			put16(0xeb42, 1)
			put16(0xf3a, uint16(test.category*2))
			put16(0xf42, test.clock)
			put16(0x138, uint16(test.mouse.X))
			put16(0x13a, uint16(test.mouse.Y))
			put32(0x1e, 0xa10000)
			put32(0x22, 0xa20000)
			put32(0xe8a4, test.mana)
			put32(0xe8a4+4, test.population[0])
			put32(0xe9de+4, test.population[1])
			put16(0xe8a4+12, 14)
			state := visualassets.HUDState{Mana: test.mana, Population: test.population, Category: test.category, Tick: uint64(test.clock), MouseX: test.mouse.X, MouseY: test.mouse.Y}
			for id := range state.Costs {
				state.Costs[id] = uint16(hud.Mana.Cost(populous2.SpellID(id), int(hud.Costs[id]), [6]uint8{}))
			}
			for _, power := range source.Spells {
				state.Enabled[power.ID] = true
				raw[0xe8a4+0x70+int(power.ID)] = 1
			}
			span := func(at, n int) error {
				if at < 0 || at > len(raw)-n {
					return fmt.Errorf("private HUD backing outside memory")
				}
				return nil
			}
			memory := populous2.FollowerCleanupMemory{Read8: func(at int) (uint8, error) {
				if e := span(at, 1); e != nil {
					return 0, e
				}
				return raw[at], nil
			}, Read16: func(at int) (uint16, error) {
				if e := span(at, 2); e != nil {
					return 0, e
				}
				return binary.BigEndian.Uint16(raw[at:]), nil
			}, Read32: func(at int) (uint32, error) {
				if e := span(at, 4); e != nil {
					return 0, e
				}
				return binary.BigEndian.Uint32(raw[at:]), nil
			}, Write8: func(at int, v uint8) error { raw[at] = v; return nil }, Write16: func(at int, v uint16) error { put16(at, v); return nil }, Write32: func(at int, v uint32) error { put32(at, v); return nil }}
			bitmap := append([]byte(nil), source.Raw["qaz.pak"]...)
			scratch := append([]byte(nil), bitmap...)
			if len(bitmap) != 32000 {
				t.Fatal("original gameplay backdrop missing", len(bitmap))
			}
			frame := populous2.NativeFrameRegisterContext{AddressBase: 0x200000}
			imageState := panel.Render.Images.NewImageState()
			_, err := panel.RestorePanel(populous2.NativeProfilePanelFrameCallbacks{Memory: memory, Frame: &frame, Image: &imageState, Sprite: sprites.Paint, Bitmap: func(address uint32) ([]byte, error) {
				if address == 0xa20000 {
					return bitmap, nil
				}
				return scratch, nil
			}, Ownership: func(bool, *populous2.NativeFrameRegisterContext) error { return nil }})
			if err != nil {
				t.Fatal(err)
			}
			cb := populous2.NativeRenderFrameCallbacks{Memory: memory, Frame: &frame, Image: &imageState, Bitmap: bitmap, Sprite: sprites.Paint}
			if err := panel.Render.Highlights(cb); err != nil {
				t.Fatal(err)
			}
			if _, err := panel.Render.HUD(cb, true); err != nil {
				t.Fatal(err)
			}
			expected, err := populous2.DecodeScreen(bitmap, palette)
			if err != nil {
				t.Fatal(err)
			}
			got := image.NewRGBA(expected.Bounds())
			base, err := populous2.DecodeScreen(source.Raw["qaz.pak"], palette)
			if err != nil {
				t.Fatal(err)
			}
			draw.Draw(got, got.Bounds(), base, image.Point{}, draw.Src)
			art.DrawIcons(got, state, palette)
			art.DrawHighlights(got, state, palette)
			art.DrawIndicators(got, state, bank, palette)
			if !bytes.Equal(got.Pix, expected.Pix) {
				for y := 0; y < 200; y++ {
					for x := 0; x < 320; x++ {
						if got.RGBAAt(x, y) != expected.RGBAAt(x, y) {
							t.Fatalf("HUD pixels differ mana%d category%d at%d,%d got%v want%v", test.mana, test.category, x, y, got.RGBAAt(x, y), expected.RGBAAt(x, y))
						}
					}
				}
			}
		}
	}
}

func TestPrivateHUDHitGridsMatchOriginalCategoryAndPowerScan(t *testing.T) {
	path := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if path == "" {
		t.Skip("set private original UI directory")
	}
	source, err := populous2.LoadFS(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	rules, err := populous2.DecodeNativeGameplayHUDInputRules(source.Executable)
	if err != nil {
		t.Fatal(err)
	}
	code := source.Executable.Hunks[0].Data
	raw := make([]byte, 0x11280)
	memory := populous2.FollowerCleanupMemory{Read8: func(at int) (uint8, error) { return raw[at], nil }, Read16: func(at int) (uint16, error) { return binary.BigEndian.Uint16(raw[at:]), nil }, Read32: func(at int) (uint32, error) { return binary.BigEndian.Uint32(raw[at:]), nil }, Write8: func(at int, v uint8) error { raw[at] = v; return nil }, Write16: func(at int, v uint16) error { binary.BigEndian.PutUint16(raw[at:], v); return nil }, Write32: func(at int, v uint32) error { binary.BigEndian.PutUint32(raw[at:], v); return nil }}
	codeMemory := populous2.FollowerCleanupMemory{Read8: func(at int) (uint8, error) { return code[at], nil }, Read16: func(at int) (uint16, error) { return binary.BigEndian.Uint16(code[at:]), nil }, Read32: func(at int) (uint32, error) { return binary.BigEndian.Uint32(code[at:]), nil }}
	codeMemory.Write8 = func(int, uint8) error { return nil }
	codeMemory.Write16 = func(int, uint16) error { return nil }
	codeMemory.Write32 = func(int, uint32) error { return nil }
	ram := memory
	ram.Read8 = func(at int) (uint8, error) { return raw[at-0x200000], nil }
	ram.Read16 = func(at int) (uint16, error) { return binary.BigEndian.Uint16(raw[at-0x200000:]), nil }
	ram.Read32 = func(at int) (uint32, error) { return binary.BigEndian.Uint32(raw[at-0x200000:]), nil }
	ram.Write8 = func(at int, v uint8) error { raw[at-0x200000] = v; return nil }
	ram.Write16 = func(at int, v uint16) error { binary.BigEndian.PutUint16(raw[at-0x200000:], v); return nil }
	ram.Write32 = func(at int, v uint32) error { binary.BigEndian.PutUint32(raw[at-0x200000:], v); return nil }
	for y := 100; y < 200; y++ {
		for x := 0; x < 160; x++ {
			binary.BigEndian.PutUint16(raw[0xeb42:], 1)
			binary.BigEndian.PutUint16(raw[0xf3a:], 65535)
			frame := populous2.NativeFrameRegisterContext{AddressBase: 0x200000}
			frame.D[6], frame.D[7] = uint32(x), uint32(y)
			state := populous2.NativeGameplayHUDInputState{}
			cb := populous2.NativeGameplayHUDInputCallbacks{NativeStartupResetFrameCallbacks: populous2.NativeStartupResetFrameCallbacks{Memory: memory, Code: codeMemory, RAM: ram, Frame: &frame, CodeBase: 0x100000, Call: func(populous2.NativeStartupResetFrameCall, *uint32) (populous2.NativeCommandFrameResult, error) {
				return populous2.NativeCommandFrameResult{Complete: true}, nil
			}}}
			step, err := state.Advance(&rules, cb)
			if err != nil {
				t.Fatal(err)
			}
			kind, index := visualassets.HUDHit(x, y)
			got := kind == "category" || kind == "power"
			if got == step.Zero {
				t.Fatalf("source HUD grid hit differs at%d,%d kind%s", x, y, kind)
			}
			if kind == "category" && binary.BigEndian.Uint16(raw[0xf3a:]) != uint16(index*2) {
				t.Fatal("source category grid index differs")
			}
		}
	}
}
