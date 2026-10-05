package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type startupPanelFixture struct {
	Input struct {
		Name, Mode string
		Entry      int
		D          [8]uint32
		A          [7]uint32
		Initial    []nativeHeroPatch
		Pattern    bool
	}
	D                                             [8]uint32
	A                                             [7]uint32
	CCR                                           uint16
	BSSHash, CodeHash, BitmapHash, BackgroundHash string
	Ownership                                     []struct {
		Routine                                       int
		D                                             [8]uint32
		A                                             [7]uint32
		BSSHash, CodeHash, BitmapHash, BackgroundHash string
	}
}

func TestNativeStartupPanelAllRegistersAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/startup_panel_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []startupPanelFixture }
	if err = json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 792 {
		t.Fatalf("native panel corpus changed: %d", len(catalog.Cases))
	}
	bundle := testBundle(t)
	rules, err := DecodeNativeStartupPanelFrameRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	base := resourceFrameInitialRAM(t)
	copy(base[0x600000:], bundle.Raw["s32-0.pak"])
	copy(base[0x6118c8:], bundle.Raw["s16-0.pak"])
	if err = PrepareNativeResourceFramePlanes(commandFrameBacking(base), 0x1214b2, 0x2003be); err != nil {
		t.Fatal(err)
	}
	if err = PrepareNativeResourceFramePlanes(commandFrameBacking(base), 0x121632, 0x2003be); err != nil {
		t.Fatal(err)
	}
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			ram := append([]byte(nil), base...)
			raw, code := ram[0x200000:0x211280], ram[0x100000:0x13fa2c]
			clear(raw)
			m, cm := commandFrameBacking(raw), commandFrameBacking(code)
			_ = m.Write32(0x1e, 0xa10000)
			_ = m.Write32(0x22, 0xa20000)
			_ = cm.Write32(0x6f2, 0xd10000)
			for _, patch := range f.Input.Initial {
				renderFramePatch(m, patch)
			}
			bitmap, background := ram[0xa10000:0xa17d00], ram[0xa20000:0xa27d00]
			if f.Input.Pattern {
				for i := range bitmap {
					bitmap[i] = byte(i*7 + 13)
					background[i] = byte(i*11 + 23)
				}
			}
			shared, err := NewNativeSharedCode(bundle.Executable, ram[0x100000:0x100000+int(bundle.Executable.Hunks[0].AllocatedBytes)], []uint32{0x100000, 0x200000, 0x300000, 0x400000, 0x500000, 0x600000})
			if err != nil {
				t.Fatal(err)
			}
			c := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			a := [7]NativeRequesterAddress{}
			for i, v := range f.Input.A {
				a[i] = NativeRequesterAddress{Address: v, Absolute: true}
			}
			image := rules.Render.Images.NewImageState()
			ownership := 0
			cb := NativeStartupPanelFrameCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Frame: &c, Code: cm, Memory: m, RAM: commandFrameBacking(ram), CodeBase: 0x100000}, Logical: shared.Logical(), Image: &image, Bitmap: func(address uint32) ([]byte, error) {
				switch address {
				case 0xa10000:
					return bitmap, nil
				case 0xa20000:
					return background, nil
				}
				return nil, fmt.Errorf("native panel bitmap%x unavailable", address)
			}}
			cb.Ownership = func(released bool, frame *NativeFrameRegisterContext, addresses *[7]NativeRequesterAddress) error {
				if ownership >= len(f.Ownership) {
					return fmt.Errorf("unexpected native ownership operation")
				}
				want := f.Ownership[ownership]
				routine := 0xe4c
				if released {
					routine = 0xe28
				}
				if routine != want.Routine || frame.D != want.D {
					return fmt.Errorf("ownership%d routine/D differs:%08x/%08x", ownership, frame.D, want.D)
				}
				for i, v := range *addresses {
					if v.Address != want.A[i] {
						return fmt.Errorf("ownership%d A%d differs:%x/%x", ownership, i, v.Address, want.A[i])
					}
				}
				if fileFrameHash(raw) != want.BSSHash || fileFrameHash(code) != want.CodeHash || fileFrameHash(bitmap) != want.BitmapHash || fileFrameHash(background) != want.BackgroundHash {
					return fmt.Errorf("ownership%d full source memory differs", ownership)
				}
				for i := range frame.D {
					frame.D[i] ^= 0x12345678 * uint32(i+1)
				}
				for i := range *addresses {
					(*addresses)[i] = NativeRequesterAddress{Address: 0x730000 + uint32(i)*0x113, Absolute: true}
				}
				ownership++
				return nil
			}
			switch f.Input.Entry {
			case 0x1da0:
				_, err = rules.RestorePanel(cb, &a)
			case 0x1f5e:
				_, err = rules.HUD(cb, &a)
			default:
				t.Fatal("unexpected native panel entry")
			}
			if err != nil {
				t.Fatal(err)
			}
			if ownership != len(f.Ownership) {
				t.Fatal("native ownership operation missing")
			}
			if c.D != f.D {
				t.Errorf("full panel D differs:%08x expected%08x", c.D, f.D)
			}
			for i, v := range a {
				if v.Address != f.A[i] {
					t.Errorf("panel A%d differs:%x expected%x", i, v.Address, f.A[i])
				}
			}
			if fileFrameHash(raw) != f.BSSHash || fileFrameHash(code) != f.CodeHash || fileFrameHash(bitmap) != f.BitmapHash || fileFrameHash(background) != f.BackgroundHash {
				t.Fatal("native panel complete BSS/CODE/pixels differ")
			}
		})
	}
}
