package main

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"os"
	"strconv"
	"testing"

	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func TestPrivateConquestBriefingPixelsAndHitsMatchEnglishSource(t *testing.T) {
	input := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if input == "" {
		t.Skip("set the private imported artwork directory")
	}
	source, err := populous2.LoadFS(os.DirFS(input))
	if err != nil {
		t.Fatal(err)
	}
	source, err = interfaceSource(source)
	if err != nil {
		t.Fatal(err)
	}
	d, icons, err := decodeConquest(source)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := populous2.DecodeNativeInGameRequesterRules(source.Executable)
	if err != nil {
		t.Fatal(err)
	}
	p := rules.Presentation
	font := &visualassets.Font{FirstCode: populous2.NativeGlyphFirst, Width: 8, Height: 8, Glyphs: make([][]uint8, populous2.NativeGlyphCount)}
	for i := range font.Glyphs {
		font.Glyphs[i] = append([]uint8(nil), p.Font.Glyphs[i][:]...)
	}
	palette, err := populous2.NativeWorldPalette(source)
	if err != nil {
		t.Fatal(err)
	}
	art := &visualassets.ConquestArt{Descriptor: *d, FaceBackdrop: icons[36]}
	copy(art.Icons[:], icons[:36])
	optionNames := []string{"build-anywhere", "sea-level-only", "forbid-enemy-terrain", "forbid-raise", "forbid-lower", "fatal-water", "hide-enemy", "disable-emigration", "hide-disasters", "shallow-swamps"}
	for _, world := range []uint16{0, 32, 500, 992, 999} {
		for _, bits := range []uint16{0, 0x155, 0x3ff} {
			t.Run(fmt.Sprintf("world%d-options%x", world, bits), func(t *testing.T) {
				state := populous2.NativeWorldRequesterState{World: world, Landscape: world % 4, RuleBits: bits}
				enabled := [36]bool{}
				for i := range enabled {
					if i%6 != 5 && (i+int(world))%3 != 0 {
						enabled[i] = true
						state.PowerFlags[i] = 1
					}
				}
				original, err := rules.WorldScreen(source, state, make([]byte, 64000), nil)
				if err != nil {
					t.Fatal(err)
				}
				got := image.NewRGBA(image.Rect(0, 0, 320, 200))
				draw.Draw(got, got.Bounds(), image.NewUniform(palette[0]), image.Point{}, draw.Src)
				values := map[string]string{"world-code": string(rules.NativeWorldCode(world)), "world-number": strconv.Itoa(int(world)), "landscape": d.LandscapeNames[state.Landscape], "opponent": d.Opponents[world/32].Name}
				flags := map[string]bool{}
				for i, name := range optionNames {
					flags[name] = bits&(1<<i) != 0
				}
				d.Layout.Draw(got, font, values, flags)
				art.DrawIcons(got, enabled)
				if !bytes.Equal(got.Pix, original.Image.Pix) {
					reportFirstPixelDifference(t, got, original.Image)
				}
				for y := 0; y < 200; y++ {
					for x := 0; x < 320; x++ {
						clone := *original.Requester
						clone.Text = append([]byte(nil), original.Requester.Text...)
						want := map[int]string{2: "world-code", 4: "opponent", 6: "proceed", 8: "cancel"}[p.Requesters.Click(&clone, x, y)]
						if got := d.Layout.ActionAt(x, y); got != want {
							t.Fatalf("action at%d,%d =%s want%s", x, y, got, want)
						}
						wantSlot, ok := populous2.NativeWorldSpellHit(uint16(x), uint16(y))
						gotSlot, gotOK := art.PowerAt(x, y)
						if ok != gotOK || ok && gotSlot != int(wantSlot/2) {
							t.Fatalf("icon at%d,%d differs", x, y)
						}
					}
				}
			})
		}
	}
	output := t.TempDir()
	if err := exportConquest(source, output); err != nil {
		t.Fatal(err)
	}
	if _, err := visualassets.LoadConquestArt(os.DirFS(output)); err != nil {
		t.Fatal(err)
	}
	portraits, err := populous2.DecodeDeityArt(source.Executable, source.Raw["faces.pak"], palette)
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []struct{ world, reaction, aggression uint16 }{{0, 0, 0}, {500, 5, 19}, {992, 14, 35}, {999, 65535, 65535}} {
		t.Run(fmt.Sprintf("opponent%d-reaction%d-aggression%d", state.world, state.reaction, state.aggression), func(t *testing.T) {
			original, err := rules.OpponentScreen(source, state.world, state.reaction, state.aggression, make([]byte, 64000))
			if err != nil {
				t.Fatal(err)
			}
			got := image.NewRGBA(image.Rect(0, 0, 320, 200))
			draw.Draw(got, got.Bounds(), image.NewUniform(palette[0]), image.Point{}, draw.Src)
			d.OpponentLayout.Draw(got, font, art.OpponentValues(int(state.world), int(int16(state.reaction)), int(state.aggression)), nil)
			art.DrawOpponentFace(got, int(state.world/32), portraits.Parts)
			if !bytes.Equal(got.Pix, original.Image.Pix) {
				reportFirstPixelDifference(t, got, original.Image)
			}
		})
	}
}

func reportFirstPixelDifference(t *testing.T, got, want *image.RGBA) {
	t.Helper()
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			if got.RGBAAt(x, y) != want.RGBAAt(x, y) {
				t.Fatalf("pixel%d,%d got%v want%v", x, y, got.RGBAAt(x, y), want.RGBAAt(x, y))
			}
		}
	}
	t.Fatal("pixel buffers differ")
}
