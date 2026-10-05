package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type spellHelpAnimationFixture struct {
	Input struct {
		Name                string
		Slot, Counter, X, Y uint16
		Terrain, Ticks      int
		Registers           [8]uint32
	}
	Snapshots []struct {
		Registers      [8]uint32
		Counter, LastY uint16
		Audio          []nativeHeroPatch
		Sprites        []struct {
			PC     uint32
			X, Y   int16
			Height uint16
		}
		Tiles []struct {
			Index      uint16
			ByteOffset int32
			SourceHigh uint16
		}
	}
}

func TestSpellHelpAnimationAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/spell_help_animation_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []spellHelpAnimationFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 600 {
		t.Fatal("native help preview corpus incomplete")
	}
	bundle := testBundle(t)
	r, err := DecodeNativeSpellHelpAnimationRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	slots := map[uint16]bool{}
	terrains := map[int]bool{}
	for _, f := range catalog.Cases {
		count += len(f.Snapshots)
		slots[f.Input.Slot] = true
		terrains[f.Input.Terrain] = true
		t.Run(f.Input.Name, func(t *testing.T) {
			if f.Input.Ticks == 0 || len(f.Snapshots) != f.Input.Ticks {
				t.Fatal("native help preview snapshots incomplete")
			}
			state := r.NewState()
			state.TableOffset, state.Frame, state.X, state.Y = f.Input.Slot, f.Input.Counter, f.Input.X, f.Input.Y
			initialAudio := state.Image.AudioBank
			registers := f.Input.Registers
			block := bundle.Raw[fmt.Sprintf("block%d.pak", f.Input.Terrain)]
			if len(block) == 0 {
				t.Fatal("native help descriptor backing missing")
			}
			for tick, want := range f.Snapshots {
				plan, err := r.Advance(&state, block, &registers)
				if err != nil {
					t.Fatal(err)
				}
				if registers != want.Registers {
					t.Fatalf("native help tick%d full register continuation differs: got%x want%x", tick, registers, want.Registers)
				}
				if state.Frame != want.Counter || state.Image.LastY != want.LastY {
					t.Fatalf("native help tick%d retained frame/Y differs: got%x/%x want%x/%x", tick, state.Frame, state.Image.LastY, want.Counter, want.LastY)
				}
				expectedAudio := initialAudio
				for _, p := range want.Audio {
					if p.Address < 0 || p.Address >= len(expectedAudio) || p.Width != 1 {
						t.Fatal("native help audio byte outside bank")
					}
					expectedAudio[p.Address] = byte(p.Value)
				}
				if state.Image.AudioBank != expectedAudio {
					t.Fatalf("native help tick%d raw audio bank differs", tick)
				}
				if len(plan.Sprites) != len(want.Sprites) || len(plan.Tiles) != len(want.Tiles) {
					t.Fatalf("native help tick%d primitive count differs: sprites%d/%d tiles%d/%d", tick, len(plan.Sprites), len(want.Sprites), len(plan.Tiles), len(want.Tiles))
				}
				for i, p := range plan.Sprites {
					w := want.Sprites[i]
					if p.Routine != w.PC || p.X != w.X || p.Y != w.Y || uint16(p.Height) != w.Height {
						t.Fatalf("native help tick%d sprite%d differs: got%+v want%+v", tick, i, p, w)
					}
				}
				for i, p := range plan.Tiles {
					w := want.Tiles[i]
					if p.Tile != w.Index || p.ByteOffset != w.ByteOffset || p.SourceHigh != w.SourceHigh {
						t.Fatalf("native help tick%d tile%d differs: got%+v want%+v", tick, i, p, w)
					}
				}
			}
		})
	}
	if count != 18624 || len(slots) != 36 || len(terrains) != 4 {
		t.Fatalf("native help dispatch/terrain/frame coverage differs: %d/%d/%d", len(slots), len(terrains), count)
	}
}

func TestSpellHelpAnimationRetainsNativeMaskOnlyAndOpeningBoundary(t *testing.T) {
	b := testBundle(t)
	r, err := DecodeNativeSpellHelpAnimationRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	state := r.NewState()
	state.Image.LastY = 123
	state.Image.AudioBank[0] = 77
	if err := r.Begin(&state, 64); err != nil {
		t.Fatal(err)
	}
	if state.Frame != 0xffff || state.TableOffset != 64 || state.Image.LastY != 123 || state.Image.AudioBank[0] != 77 {
		t.Fatal("help opening reset unrelated shared image state")
	}
	d := [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}
	for i := 0; i < 4; i++ {
		plan, err := r.Advance(&state, b.Raw["block0.pak"], &d)
		if err != nil {
			t.Fatal(err)
		}
		if state.Frame != 1 || len(plan.Tiles) != 1 || plan.Tiles[0].Tile != 144 {
			t.Fatal("original mask-only baptism preview was replaced with invented alternation")
		}
	}
	if err := r.Begin(&state, 71); err == nil {
		t.Fatal("unaligned help table offset accepted")
	}
	if _, err := DecodeNativeSpellHelpAnimationRules(nil); err == nil {
		t.Fatal("missing help executable accepted")
	}
}
