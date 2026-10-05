package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type profilePanelFrameFixture struct {
	Input struct {
		Name, Mode string
		D          [8]uint32
		Initial    []nativeHeroPatch
		Pattern    bool
	}
	D                    [8]uint32
	A0, A1               uint32
	Changes, BankChanges []nativeHeroPatch
	LastY                uint16
	Sprites              []struct {
		Descriptor int
		X, Y       int16
		Height     uint16
		Routine    uint32
	}
	BSSHash, BitmapHash, BackgroundHash, Error string
}

func TestNativeProfileAndPanelFramesAgainstCompleteOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/profile_panel_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []profilePanelFrameFixture }
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 360 {
		t.Fatalf("native profile/panel corpus incomplete: %v", err)
	}
	bundle := testBundle(t)
	rules, err := DecodeNativeProfilePanelFrameRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	bank, err := DecodeNativeSpriteBitmapBank(bundle, 0)
	if err != nil {
		t.Fatal(err)
	}
	profiles, panels, ownershipCalls := 0, 0, 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			if f.Error != "" {
				t.Fatal("original native body capture failed", f.Error)
			}
			raw := make([]byte, 0x11280)
			if f.Input.Mode == "profile" {
				for i := range raw {
					raw[i] = byte(i*17 + 3)
				}
			}
			m := commandNativeMemory(raw)
			_ = m.Write32(0x1e, 0xa10000)
			_ = m.Write32(0x22, 0xa20000)
			for _, p := range f.Input.Initial {
				renderFramePatch(m, p)
			}
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			image := rules.Render.Images.NewImageState()
			wantImage := image
			for _, p := range f.BankChanges {
				wantImage.AudioBank[p.Address] = byte(p.Value)
			}
			bitmap, background := make([]byte, 32000), make([]byte, 32000)
			if f.Input.Pattern {
				for i := range bitmap {
					bitmap[i] = byte(i*7 + 13)
					background[i] = byte(i*11 + 23)
				}
			}
			if f.Input.Mode == "profile" {
				plan, err := rules.SwitchProfile(m, &frame)
				if err != nil {
					t.Fatal(err)
				}
				if plan.A0 != f.A0 || plan.A1 != f.A1 {
					t.Errorf("native profile surviving command pointers differ: got%x/%x want%x/%x", plan.A0, plan.A1, f.A0, f.A1)
				}
				profiles++
			} else {
				ownership := 0
				cb := NativeProfilePanelFrameCallbacks{Memory: m, Frame: &frame, Image: &image, Sprite: bank.Paint, Bitmap: func(address uint32) ([]byte, error) {
					switch address {
					case 0xa10000:
						return bitmap, nil
					case 0xa20000:
						return background, nil
					}
					return nil, fmt.Errorf("unknown physical native target %#x", address)
				}, Ownership: func(owned bool, c *NativeFrameRegisterContext) error {
					if owned != (ownership%2 == 0) {
						return fmt.Errorf("source handoff order changed")
					}
					ownership++
					for i := range c.D {
						c.D[i] = uint32(i + 1)
					}
					return nil
				}}
				plan, err := rules.RestorePanel(cb)
				if err != nil {
					t.Fatal(err)
				}
				if ownership != 10 {
					t.Fatalf("native panel handoff count changed: %d", ownership)
				}
				if len(plan.Descriptors) != 5 || len(plan.Sprites) != 5 {
					t.Fatal("native five-icon panel incomplete")
				}
				for i, s := range plan.Sprites {
					want := f.Sprites[i]
					if plan.Descriptors[i] != want.Descriptor || s.X != want.X || s.Y != want.Y || s.Height != int16(want.Height) || s.Routine != want.Routine {
						t.Errorf("native panel descriptor%d differs: got%#x %+v want%+v", i, plan.Descriptors[i], s, want)
					}
				}
				panels++
				ownershipCalls += ownership
			}
			if frame.D != f.D {
				t.Errorf("native all8D differ: got%x want%x", frame.D, f.D)
			}
			if fileFrameHash(raw) != f.BSSHash {
				t.Errorf("native full BSS differs: got%s want%s", fileFrameHash(raw), f.BSSHash)
			}
			if fileFrameHash(bitmap) != f.BitmapHash || fileFrameHash(background) != f.BackgroundHash {
				t.Error("native distinct panel/HUD physical pixels differ")
			}
			if image.AudioBank != wantImage.AudioBank || image.LastY != f.LastY {
				t.Error("native shared image/audio CODE state differs")
			}
		})
	}
	if profiles != 144 || panels != 216 || ownershipCalls != 2160 {
		t.Fatalf("native source coverage incomplete: %d/%d/%d", profiles, panels, ownershipCalls)
	}
}
