package populous2

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type editorCursorFixture struct {
	Input struct {
		Name, Mode        string
		Frame, Edit, Tool uint16
		X, Y              int16
		WrapCounters      bool
		Repeat            int
		Registers         [8]uint32
	}
	Registers    [8]uint32
	LastY        uint16
	AudioChanges []nativeHeroPatch
	Sprites      []struct {
		PC     uint32
		X, Y   int16
		Height uint16
	}
	Error string
}

func TestNativeEditorCursorAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/editor_cursor_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []editorCursorFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 4105 {
		t.Fatal("native editor/image corpus incomplete")
	}
	r, err := DecodeNativeEditorCursorRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	frames := map[uint16]bool{}
	for _, f := range catalog.Cases {
		counts[f.Input.Mode]++
		if f.Input.Mode == "image" && f.Input.X == 100 && f.Input.Y == 100 && f.Input.Frame%4 == 0 && f.Input.Frame < 0x2c38 {
			frames[f.Input.Frame] = true
		}
		t.Run(f.Input.Name, func(t *testing.T) {
			if f.Error != "" {
				t.Fatal("original image CPU fixture did not finish", f.Error)
			}
			state := r.NewImageState()
			if f.Input.WrapCounters {
				for i := range state.AudioBank {
					state.AudioBank[i] = 0xff
				}
			}
			expected := state
			for _, p := range f.AudioChanges {
				if p.Address < 0 || p.Address >= len(expected.AudioBank) || p.Width != 1 {
					t.Fatal("native audio mutation outside retained bank")
				}
				expected.AudioBank[p.Address] = byte(p.Value)
			}
			registers := f.Input.Registers
			var sprites []NativePresentationSprite
			if f.Input.Mode == "preview" {
				m := &scenarioScriptMemory{}
				_ = m.write16(0xf0e, f.Input.Edit)
				_ = m.write16(0xf10, f.Input.Tool)
				_ = m.write16(0x138, uint16(f.Input.X))
				_ = m.write16(0x13a, uint16(f.Input.Y))
				sprites, err = r.Preview(m.callbacks(), &state, &registers)
			} else {
				registers[0] = hudWord(registers[0], uint16(f.Input.X))
				registers[1] = hudWord(registers[1], uint16(f.Input.Y))
				registers[2] = hudWord(registers[2], f.Input.Frame)
				repeat := f.Input.Repeat
				if repeat == 0 {
					repeat = 1
				}
				for i := 0; i < repeat; i++ {
					registers[2] = hudWord(registers[2], f.Input.Frame)
					var layers []NativePresentationSprite
					layers, err = r.DrawImage(&state, &registers)
					if err != nil {
						break
					}
					sprites = append(sprites, layers...)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if registers != f.Registers {
				t.Fatalf("original image full register continuation differs: got%x want%x", registers, f.Registers)
			}
			if state.LastY != f.LastY || state.AudioBank != expected.AudioBank {
				t.Fatalf("original image mutable CODE differs: Y%x want%x", state.LastY, f.LastY)
			}
			if len(sprites) != len(f.Sprites) {
				t.Fatalf("original image sprite count%d want%d", len(sprites), len(f.Sprites))
			}
			for i, p := range sprites {
				want := f.Sprites[i]
				if p.Routine != want.PC || p.X != want.X || p.Y != want.Y || uint16(p.Height) != want.Height {
					t.Fatalf("original image layer%d differs: got%+v want%+v", i, p, want)
				}
			}
		})
	}
	if !reflect.DeepEqual(counts, map[string]int{"image": 4033, "preview": 72}) || len(frames) != 2830 {
		t.Fatalf("native image table/prefix coverage differs: %v/%d", counts, len(frames))
	}
}

func TestNativeEditorCursorPreservesInactiveContextAndSharedSoundBank(t *testing.T) {
	r, err := DecodeNativeEditorCursorRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	m := &scenarioScriptMemory{}
	state := r.NewImageState()
	registers := [8]uint32{1, 2, 0x12345678, 4, 5, 6, 7, 8}
	before := registers
	if _, err := r.Preview(m.callbacks(), &state, &registers); err != nil {
		t.Fatal(err)
	}
	if registers != before {
		t.Fatal("inactive editor initialized arbitrary register values")
	}
	_ = m.write16(0xf0e, 1)
	if _, err := r.Preview(m.callbacks(), &state, &registers); err != nil {
		t.Fatal(err)
	}
	before[2] = 0x12340000
	if registers != before {
		t.Fatal("zero tool lost the original low-word assignment")
	}
	if _, err := DecodeNativeEditorCursorRules(nil); err == nil {
		t.Fatal("missing cursor executable accepted")
	}
}
