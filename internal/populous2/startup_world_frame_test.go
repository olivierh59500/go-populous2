package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type startupWorldFixture struct {
	Input struct {
		Name                                                                      string
		Entry                                                                     int
		Seed                                                                      uint32
		D                                                                         [8]uint32
		Height, Tile, Free, Groups, Owner, Count, X, Y, Counter, Pattern, Profile int
		Table, Fixed                                                              bool
	}
	InitialD, D                     [8]uint32
	InitialA, A                     [7]uint32
	CCR                             uint16
	BSSHash, CodeHash, AdjacentHash string
	Stages                          []struct {
		Entry   int
		D       [8]uint32
		A       [7]uint32
		BSSHash string
	}
	Steps int
}

func startupWorldFixtureMemory(raw, code []byte, f startupWorldFixture) {
	m, cm := commandFrameBacking(raw), commandFrameBacking(code)
	for i := 0; i < 0x11280; i++ {
		raw[i] = byte(i*13 + (i >> 4) + 23)
	}
	for at := 0xf44; at < 0x4f44; at += 4 {
		_ = m.Write8(at, uint8(f.Input.Height))
		_ = m.Write8(at+1, uint8(f.Input.Tile))
		_ = m.Write16(at+2, 0)
		if f.Input.Pattern == 1 && ((at-0xf44)/4)%7 == 0 {
			_ = m.Write8(at+1, 0)
		}
	}
	for at := 0x5f50; at < 0x6b90; at += 32 {
		_ = m.Write8(at+12, 0)
	}
	for i, at := 0, 0x6bd0; at < 0x76c0; i, at = i+1, at+14 {
		owner := uint8(1)
		if i < f.Input.Free {
			owner = 0
		}
		_ = m.Write8(at+12, owner)
	}
	for i, at := 0, 0x76f4; at < 0xc800; i, at = i+1, at+52 {
		owner := uint8(1)
		if i < f.Input.Free {
			owner = 0
		}
		_ = m.Write8(at+12, owner)
	}
	for side := 0; side < 2; side++ {
		god := 0xe8a4 + side*314
		for _, v := range [][2]int{{0x5a, f.Input.Groups}, {0x5c, 100 + side*71}, {0x5e, 32 + side}, {0x60, 2 + side}, {0x62, 700 + side}, {0x64, 3 + side}, {0x66, side + 1}, {0x6e, 0xffff}} {
			_ = m.Write16(god+v[0], uint16(v[1]))
		}
		_ = m.Write8(god+0x53, 47+uint8(side)*53)
		if f.Input.Fixed {
			_ = m.Write16(god+0x6e, uint16(0x0102+side*0x3d3b))
		}
	}
	_ = m.Write32(0xeb28, f.Input.Seed)
	_ = m.Write16(0xeb42, 1)
	if f.Input.Profile != 0 {
		_ = m.Write16(0xeb42, uint16(f.Input.Profile))
	}
	_ = m.Write16(0xdd2, uint16(f.Input.Counter))
	_ = m.Write16(0xf2e, 0)
	if f.Input.Count >= 0 {
		_ = cm.Write16(0x20f40, uint16(f.Input.Count))
		_ = cm.Write16(0xddd0, uint16(f.Input.Count))
		_ = cm.Write16(0x20f3e, 3)
		_ = cm.Write16(0xddce, 3)
	}
	if f.Input.Table {
		for i, v := range []uint16{1, 31, 1, 31, 0xff9d, 0xff9d, 0xff9d, 0xff9d} {
			_ = cm.Write16(0x2072a+i*2, v)
		}
	}
	if f.Input.Entry == 0x13fb6 || f.Input.Entry == 0x125a0 {
		_ = m.Write8(0x76f4+12, uint8(f.Input.Owner))
		_ = m.Write8(0x76f4+6, 31)
		_ = m.Write8(0x76f4+8, 32)
		_ = m.Write8(0x76f4+9, 128)
	}
}

func TestNativeStartupWorldAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/startup_world_frame_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []startupWorldFixture }
	if e = json.Unmarshal(data, &catalog); e != nil {
		t.Fatal(e)
	}
	if len(catalog.Cases) != 688 {
		t.Fatalf("native startup-world corpus changed: %d", len(catalog.Cases))
	}
	base := fileFrameRelocatedCode(t)
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			raw, code := make([]byte, 0x18000), append([]byte(nil), base...)
			startupWorldFixtureMemory(raw, code, f)
			c := NativeFrameRegisterContext{D: f.InitialD, AddressBase: 0x200000}
			a := [7]NativeRequesterAddress{}
			for i, v := range f.InitialA {
				a[i] = NativeRequesterAddress{Address: v, Absolute: true}
			}
			entries := []int{f.Input.Entry}
			if f.Input.Entry == 0 {
				entries = []int{0xcd22, 0xd9d8, 0xdbd4, 0x10b38}
			}
			for i, entry := range entries {
				step, e := RunNativeStartupWorldFrame(entry, NativeStartupResetFrameCallbacks{Frame: &c, Memory: commandFrameBacking(raw), Code: commandFrameBacking(code), CodeBase: 0x100000}, &a)
				if e != nil || !step.Complete {
					t.Fatalf("native startup world: %v", e)
				}
				if entry == 0xcdca && step.Negative != (f.CCR&8 != 0) {
					t.Error("CDCA terminal negative flag differs")
				}
				want := f.Stages[i]
				if c.D != want.D || fileFrameHash(raw[:0x11280]) != want.BSSHash {
					t.Errorf("native stage%x D/BSS differs", entry)
				}
				for j, v := range a {
					if v.Address != want.A[j] {
						t.Errorf("native stage%x A%d differs", entry, j)
					}
				}
			}

			if c.D != f.D {
				t.Errorf("full native D differs: %08x expected %08x", c.D, f.D)
			}
			for i, v := range a {
				if v.Address != f.A[i] {
					t.Errorf("native A%d=%x expected %x", i, v.Address, f.A[i])
				}
			}
			if fileFrameHash(raw[:0x11280]) != f.BSSHash {
				t.Error("native complete BSS differs")
			}
			if fileFrameHash(raw[0x11280:]) != f.AdjacentHash {
				t.Error("native adjacent physical memory differs")
			}
			if fileFrameHash(code) != f.CodeHash {
				t.Error("native complete CODE differs")
			}
		})
	}
}

func TestNativeStartupWorldRejectsMissingBacking(t *testing.T) {
	for _, routine := range []int{0xcd22, 0xd9d8, 0xdbd4, 0x10b38} {
		if _, e := RunNativeStartupWorldFrame(routine, NativeStartupResetFrameCallbacks{}, nil); e == nil {
			t.Fatal(fmt.Sprintf("routine%x accepted missing source RAM", routine))
		}
	}
}
