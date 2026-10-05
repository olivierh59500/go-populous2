package populous2

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type presentationContextFixture struct {
	Input struct {
		Name, Mode string
		Initial    []nativeHeroPatch
		Registers  [8]uint32
		Buffer     int
	}
	Registers [8]uint32
	State     []nativeHeroPatch
	Sprites   []struct {
		PC             uint32
		X, Y           int16
		Height, Stride uint16
	}
}

func presentationContextMemory(f presentationContextFixture) *scenarioScriptMemory {
	m := &scenarioScriptMemory{}
	_ = m.write16(0xeb42, 1)
	_ = m.write16(0xf42, 123)
	_ = m.write16(0x5f44, 32)
	_ = m.write16(0x5f46, 32)
	for i := range 4096 {
		m[0xf45+i*4] = 15
	}
	a := 0x76f4
	m[a], m[a+12], m[a+18], m[a+22], m[a+25] = 2, 1, 20, 4, 7
	_ = m.write16(a+6, 0x2080)
	_ = m.write16(a+8, 0x2080)
	_ = m.write16(a+14, 20)
	_ = m.write32(a+26, 1000)
	for _, p := range f.Input.Initial {
		switch p.Width {
		case 1:
			_ = m.write8(p.Address, uint8(p.Value))
		case 2:
			_ = m.write16(p.Address, uint16(p.Value))
		case 4:
			_ = m.write32(p.Address, p.Value)
		}
	}
	return m
}

func TestPostHUDPresentationContextAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/presentation_context_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []presentationContextFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 358 {
		t.Fatal("post-HUD original corpus incomplete")
	}
	r, err := DecodeNativePresentationContextRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, f := range catalog.Cases {
		counts[f.Input.Mode]++
		t.Run(f.Input.Name, func(t *testing.T) {
			m := presentationContextMemory(f)
			c := NativeHUDRegisters{D4: f.Input.Registers[4], D5: f.Input.Registers[5], D7: f.Input.Registers[7]}
			var plan NativeSelectedPresentation
			switch f.Input.Mode {
			case "selected":
				plan, err = r.Selected(NativePresentationContextCallbacks{Memory: m.callbacks()}, &c)
			case "alt-clear":
				r.AlternateClear(&c)
			case "alt-view":
				r.AlternateView(&c)
			case "text":
				r.CountdownText(&c)
			case "portrait":
				r.Portrait(&c)
			case "marker":
				plan.Sprites, err = r.CameraMarker(m.callbacks(), &c)
				if err == nil {
					err = r.CursorNonEdit(m.callbacks(), &c)
				}
			case "composed":
				_, err = r.NormalFrameInput(NativePresentationContextCallbacks{Memory: m.callbacks()}, &c)
			case "pre-cursor":
				plan.Sprites, err = r.MapCursor(m.callbacks(), &c)
				if err == nil {
					var camera []NativePresentationSprite
					camera, err = r.CameraMarker(m.callbacks(), &c)
					plan.Sprites = append(plan.Sprites, camera...)
				}
				if err == nil {
					err = r.CursorNonEdit(m.callbacks(), &c)
				}
			default:
				t.Fatal("unknown post-HUD source stage")
			}
			if err != nil {
				t.Fatal(err)
			}
			if c != (NativeHUDRegisters{D4: f.Registers[4], D5: f.Registers[5], D7: f.Registers[7]}) {
				t.Fatalf("source presentation register continuation differs: got%+v want%x/%x/%x", c, f.Registers[4], f.Registers[5], f.Registers[7])
			}
			for _, state := range f.State {
				var value uint32
				if state.Width == 2 {
					v, e := m.read16(state.Address)
					if e != nil {
						t.Fatal(e)
					}
					value = uint32(v)
				} else {
					value, err = m.read32(state.Address)
					if err != nil {
						t.Fatal(err)
					}
				}
				if value != state.Value {
					t.Fatalf("selected raw state%x got%x want%x", state.Address, value, state.Value)
				}
			}
			if f.Input.Mode == "selected" || f.Input.Mode == "marker" || f.Input.Mode == "pre-cursor" {
				if len(plan.Sprites) != len(f.Sprites) {
					t.Fatalf("native selected sprite plan length%d want%d", len(plan.Sprites), len(f.Sprites))
				}
				for i, p := range plan.Sprites {
					want := f.Sprites[i]
					if p.X != want.X || p.Y != want.Y || uint16(p.Height) != want.Height || p.Routine != want.PC {
						t.Fatalf("native selected sprite%d differs: %+v want%+v", i, p, want)
					}
				}
			}
		})
	}
	if !reflect.DeepEqual(counts, map[string]int{"selected": 214, "alt-clear": 8, "alt-view": 12, "text": 8, "portrait": 8, "marker": 36, "composed": 48, "pre-cursor": 24}) {
		t.Fatalf("native post-HUD scopes differ: %v", counts)
	}
}
