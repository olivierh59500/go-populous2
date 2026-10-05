package populous2

import (
	"encoding/json"
	"os"
	"testing"
)

func TestNativeResourcePreparationFullAddressABI(t *testing.T) {
	data, err := os.ReadFile("testdata/resource_frame_preparation_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Table                                    uint32
			BeforeD, D                               [8]uint32
			BeforeA, A                               [7]uint32
			BSSHash, PointerHash, SpritesHash, Error string
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 12 {
		t.Fatalf("native direct preparation corpus incomplete: %v", err)
	}
	base := resourceFrameInitialRAM(t)
	bundle := testBundle(t)
	copy(base[0x600000:], bundle.Raw["s32-0.pak"])
	copy(base[0x600000+0x118c8:], bundle.Raw["s16-0.pak"])
	for _, f := range catalog.Cases {
		if f.Error != "" {
			t.Fatal("native preparation source failed", f.Error)
		}
		ram := append([]byte(nil), base...)
		m := commandFrameBacking(ram)
		frame := NativeFrameRegisterContext{D: f.BeforeD, AddressBase: 0x200000}
		address := f.BeforeA
		if err := PrepareNativeResourceFramePlanesWithRegisters(m, 0x100000+f.Table, 0x2003be, &frame, &address); err != nil {
			t.Fatal(err)
		}
		if frame.D != f.D || address != f.A {
			t.Fatalf("native1069C table%x allD/A differ: Dgot%x want%x Agot%x want%x", f.Table, frame.D, f.D, address, f.A)
		}
		if fileFrameHash(ram[0x200000:0x211280]) != f.BSSHash || fileFrameHash(ram[0x400000:0x400000+15260]) != f.PointerHash || fileFrameHash(ram[0x600000:0x600000+0x263d8]) != f.SpritesHash {
			t.Fatalf("native preparation scratch/source banks differ at%x", f.Table)
		}
	}
}
