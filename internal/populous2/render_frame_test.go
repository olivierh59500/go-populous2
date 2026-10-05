package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

// This original corpus executes all software pixel/stencil instructions and
// real sprite primitive registers. Hardware blit pixels remain an explicit
// pending boundary in both the CPU capture and this first renderer slice.
func TestNativeRenderHUDAllRegistersAndSoftwareBitmapAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/hud_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []hudNativeFixture }
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 336 {
		t.Fatalf("native render HUD corpus incomplete: %v", err)
	}
	r, err := DecodeNativeRenderFrameRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, f := range catalog.Cases {
		// These entries execute $1be8 instead of $1f5e. Their full output
		// belongs to the later cursor controller, not a skipped HUD call.
		if f.Input.NoRedraw {
			continue
		}
		count++
		t.Run(f.Input.Name, func(t *testing.T) {
			m := hudFixtureMemory(f.Input)
			frame := NativeFrameRegisterContext{D: f.Input.Registers}
			image := r.Images.NewImageState()
			bitmap := make([]byte, 32000)
			if f.Input.Pattern {
				for i := range bitmap {
					bitmap[i] = uint8(i*7 + 13)
				}
			}
			p, err := r.HUD(NativeRenderFrameCallbacks{Memory: m.callbacks(), Frame: &frame, Image: &image, Bitmap: bitmap}, true)
			if err != nil {
				t.Fatal(err)
			}
			if frame.D != f.Registers {
				t.Errorf("native HUD all8D differ: got%x want%x", frame.D, f.Registers)
			}
			if !reflect.DeepEqual(p.Pixels, f.Pixels) {
				t.Error("native HUD software pixel requests differ")
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(bitmap)); got != f.Hash {
				t.Errorf("native HUD software bitmap differs: got%s want%s", got, f.Hash)
			}
			if !p.Drawn || !p.HardwarePending || len(p.Sprites) == 0 {
				t.Fatal("native sprite hardware boundary silently omitted")
			}
		})
	}
	if count != 328 {
		t.Fatal("native HUD body coverage incomplete")
	}
}

type renderFrameFixture struct {
	Input struct {
		Name, Mode string
		Initial    []nativeHeroPatch
		D          [8]uint32
		Pointer    uint16
		Pattern    bool
		Sprite     int
	}
	D                    [8]uint32
	Changes, BankChanges []nativeHeroPatch
	Pointer, LastY       uint16
	Sprites              []struct {
		Sprite            int
		X, Y              int16
		HalfWidth, Height uint16
		Routine           uint32
		PreparedHash      string
	}
	BSSHash, BitmapHash, Error string
}

func renderFrameInitial(f renderFrameFixture) []byte {
	b := make([]byte, 0x11280)
	for i := 0; i < 4096; i++ {
		b[0xf44+i*4], b[0xf45+i*4] = 0xa8, 15
	}
	m := commandNativeMemory(b)
	_ = m.Write32(0x1e, 0xa10000)
	for _, p := range []nativeHeroPatch{{0xf0c, 2, 8}, {0xeb42, 2, 1}, {0xeb6a, 4, 0x200000 + 0xeb56}, {0x5f44, 2, 20}, {0x5f46, 2, 20}, {0x5f4c, 2, 24}, {0x5f4e, 2, 24}} {
		renderFramePatch(m, p)
	}
	for _, p := range f.Input.Initial {
		renderFramePatch(m, p)
	}
	return b
}

func renderFramePatch(m FollowerCleanupMemory, p nativeHeroPatch) {
	switch p.Width {
	case 1:
		_ = m.Write8(p.Address, uint8(p.Value))
	case 2:
		_ = m.Write16(p.Address, uint16(p.Value))
	case 4:
		_ = m.Write32(p.Address, p.Value)
	}
}

func renderFramePreparedSprite(b *Bundle, index int) ([]byte, error) {
	if index < 1 || index >= len(b.Sprites[0]) {
		return nil, fmt.Errorf("native prepared sprite outside actual bank")
	}
	s := b.Sprites[0][index]
	width, height := s.Image.Bounds().Dx(), s.Image.Bounds().Dy()
	length := width / 8 * 5 * height
	offset := int(s.Offset)
	var raw []byte
	if s.Hunk == 3 {
		raw = b.Executable.Hunks[3].Data
	} else if s.Hunk == 5 {
		if offset >= 0x118c8 {
			raw = b.Raw["s16-0.pak"]
			offset -= 0x118c8
		} else {
			raw = b.Raw["s32-0.pak"]
		}
	} else {
		return nil, fmt.Errorf("native prepared sprite source hunk unsupported")
	}
	if offset < 0 || offset+length > len(raw) {
		return nil, fmt.Errorf("native prepared sprite source truncated")
	}
	return PrepareNativeMaskedPlanes(raw[offset:offset+length], width, height)
}

func TestNativeRenderingFullFrameAgainstPreparedOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/render_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []renderFrameFixture }
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 1079 {
		t.Fatalf("native render corpus incomplete: %v", err)
	}
	bundle := testBundle(t)
	rules, err := DecodeNativeRenderFrameRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	hardware := 0
	for _, f := range catalog.Cases {
		counts[f.Input.Mode]++
		t.Run(f.Input.Name, func(t *testing.T) {
			if f.Error != "" {
				t.Fatal("original renderer did not finish", f.Error)
			}
			raw := renderFrameInitial(f)
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			state := rules.Images.NewImageState()
			expectedState := state
			for _, p := range f.BankChanges {
				if p.Address < 0 || p.Address >= len(expectedState.AudioBank) || p.Width != 1 {
					t.Fatal("native image bank delta malformed")
				}
				expectedState.AudioBank[p.Address] = byte(p.Value)
			}
			input, err := NewNativeInputState(bundle.Executable)
			if err != nil {
				t.Fatal(err)
			}
			input.Mouse.Image = f.Input.Pointer
			bitmap := make([]byte, 32000)
			if f.Input.Pattern {
				for i := range bitmap {
					bitmap[i] = uint8(i*7 + 13)
				}
			}
			cb := NativeRenderFrameCallbacks{Memory: commandNativeMemory(raw), Frame: &frame, Image: &state, Input: &input, Bitmap: bitmap}
			var plan NativeRenderFramePlan
			switch f.Input.Mode {
			case "hud":
				plan, err = rules.HUD(cb, true)
			case "map":
				plan, err = rules.MapCursor(cb)
			case "camera":
				plan, err = rules.CameraMarker(cb)
			case "cursor":
				plan, err = rules.Cursor(cb)
			case "height":
				err = rules.TerrainHeight(cb.Memory, &frame)
			case "admission":
				err = rules.TerrainAdmission(cb.Memory, &frame)
			case "descriptor":
				err = rules.descriptor(0x21626+f.Input.Sprite*12, cb, &plan)
			default:
				t.Fatal("unknown native renderer mode")
			}
			if err != nil {
				t.Fatal(err)
			}
			if frame.D != f.D {
				t.Errorf("native rendering all8D differ: got%x want%x", frame.D, f.D)
			}
			if input.Mouse.Image != f.Pointer || state.LastY != f.LastY || state.AudioBank != expectedState.AudioBank {
				t.Errorf("native rendering mutable CODE differs: pointer%x/%x Y%x/%x", input.Mouse.Image, f.Pointer, state.LastY, f.LastY)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != f.BSSHash {
				expected := renderFrameInitial(f)
				for _, p := range f.Changes {
					renderFramePatch(commandNativeMemory(expected), p)
				}
				shown := 0
				for i := range raw {
					if raw[i] != expected[i] && shown < 10 {
						t.Logf("BSS%#x differs: got%x want%x", i, raw[i], expected[i])
						shown++
					}
				}
				t.Errorf("native rendering full BSS differs: got%s want%s", got, f.BSSHash)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(bitmap)); got != f.BitmapHash {
				t.Errorf("native rendering actual software bitmap differs: got%s want%s", got, f.BitmapHash)
			}
			if len(plan.Sprites) != len(f.Sprites) {
				t.Fatalf("native sprite request count differs: got%d want%d", len(plan.Sprites), len(f.Sprites))
			}
			for i, s := range plan.Sprites {
				want := f.Sprites[i]
				hardware++
				if s.Sprite != want.Sprite || s.X != want.X || s.Y != want.Y || s.HalfWidth != int16(want.HalfWidth) || s.Height != int16(want.Height) || s.Routine != want.Routine {
					t.Errorf("native prepared sprite request differs: got%+v want%+v", s, want)
				}
				planes, err := renderFramePreparedSprite(bundle, s.Sprite)
				if err != nil {
					t.Fatal(err)
				}
				if got := fmt.Sprintf("%x", sha256.Sum256(planes)); got != want.PreparedHash {
					t.Errorf("actual1069C prepared sprite planes differ: got%s want%s", got, want.PreparedHash)
				}
			}
			if plan.HardwarePending != (len(f.Sprites) > 0) {
				t.Fatal("native hardware boundary silently reported complete")
			}
		})
	}
	if !reflect.DeepEqual(counts, map[string]int{"map": 181, "camera": 25, "height": 144, "admission": 48, "cursor": 221, "hud": 42, "descriptor": 418}) || hardware == 0 {
		t.Fatalf("native render body coverage incomplete: %v requests%d", counts, hardware)
	}
}

func TestNativeRenderHUDCallerSkipPreservesAllRegisters(t *testing.T) {
	frame := NativeFrameRegisterContext{D: [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}}
	before := frame.D
	var rules *NativeRenderFrameRules
	p, err := rules.HUD(NativeRenderFrameCallbacks{Frame: &frame}, false)
	if err != nil || frame.D != before || p.Drawn || p.HardwarePending || len(p.Pixels) != 0 || len(p.Sprites) != 0 {
		t.Fatal("skipped original HUD call read or assigned context")
	}
}
