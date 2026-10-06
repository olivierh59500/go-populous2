package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type startupDeityPrimitiveFixture struct {
	Input struct {
		Name    string
		Entry   int
		D       [8]uint32
		A       [7]uint32
		Payload [8]uint8
		Letters [16]uint8
		XP      [6]uint8
		Owner   uint16
	}
	D                             [8]uint32
	A                             [7]uint32
	CCR                           uint16
	BSSHash, CodeHash, BitmapHash string
}

func TestNativeStartupDeityPrimitivesAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/startup_menu_children_primitives_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []startupDeityPrimitiveFixture
	}
	if err = json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 454 {
		t.Fatalf("native deity primitive corpus changed:%d", len(catalog.Cases))
	}
	base := resourceFrameInitialRAM(t)
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			ram := append([]byte(nil), base...)
			raw, code, bitmap := ram[0x200000:0x211280], ram[0x100000:0x13fa2c], ram[0xa10000:0xa17d00]
			for i := range raw {
				raw[i] = byte(i*13 + (i >> 4) + 23)
			}
			for i := range bitmap {
				bitmap[i] = byte(i*7 + 13)
			}
			m := commandFrameBacking(raw)
			_ = m.Write32(0x1e, 0xa10000)
			_ = m.Write16(0xeb42, f.Input.Owner)
			for i, v := range f.Input.XP {
				_ = m.Write8(0xe76a+int(int16(uint32(f.Input.Owner)*314))+0x52+i, v)
			}
			copy(code[0xbaa0:], f.Input.Payload[:])
			copy(code[0xba60:], f.Input.Letters[:])
			c := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			a := [7]NativeRequesterAddress{}
			for i, v := range f.Input.A {
				a[i] = NativeRequesterAddress{Address: v, Absolute: true}
			}
			cb := NativeStartupResetFrameCallbacks{Frame: &c, Code: commandFrameBacking(code), Memory: m, RAM: commandFrameBacking(ram), CodeBase: 0x100000}
			if f.Input.Entry == 0xbb3a {
				err = DrawNativeDeityExperienceFrame(cb, &a)
			} else {
				err = RunNativeDeityPasswordFrame(f.Input.Entry, cb, &a)
			}
			if err != nil {
				t.Fatal(err)
			}
			if c.D != f.D {
				t.Errorf("deity primitive D differs:%08x/%08x", c.D, f.D)
			}
			for i, v := range a {
				if v.Address != f.A[i] {
					t.Errorf("deity primitive A%d differs:%x/%x", i, v.Address, f.A[i])
				}
			}
			if fileFrameHash(raw) != f.BSSHash || fileFrameHash(code) != f.CodeHash || fileFrameHash(bitmap) != f.BitmapHash {
				t.Fatal(fmt.Sprintf("complete deity primitive memory differs for%x", f.Input.Entry))
			}
		})
	}
}
