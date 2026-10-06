package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	embedded "go-populous2/assets"
)

type startupHostFixture struct {
	Input struct {
		Name                                             string
		Mode, Profile, World, Land, Custom, PauseRoutine int
		Seed                                             uint32
		D                                                [8]uint32
	}
	Boundary                                  int
	D                                         [8]uint32
	A                                         [7]uint32
	CCR                                       uint16
	BSSHash, CodeHash, ChipHash, CampaignHash string
	Calls                                     []int
	Pause                                     *struct {
		Routine                     int
		D                           [8]uint32
		A                           [7]uint32
		CCR                         uint16
		BSSHash, CodeHash, ChipHash string
	}
	Steps int
}

func startupHostFixtureRAM(t *testing.T, base []byte, f startupHostFixture) []byte {
	t.Helper()
	ram := append([]byte(nil), base...)
	raw, code, chip := ram[0x200000:0x211280], ram[0x100000:0x13fa2c], ram[0x500000:0x500000+65032]
	for i := range raw {
		raw[i] = byte(i*13 + (i >> 4) + 23)
	}
	for i := range chip {
		chip[i] = byte(i*7 + (i >> 7) + 41)
	}
	m, cm := commandFrameBacking(raw), commandFrameBacking(code)
	for _, v := range [][2]int{{0xeb44, f.Input.Mode}, {0xeb42, f.Input.Profile}, {0xeb46, f.Input.World}, {0xeb22, f.Input.Land}, {0xeb2a, int(uint16(f.Input.Seed))}} {
		if err := m.Write16(v[0], uint16(v[1])); err != nil {
			t.Fatal(err)
		}
	}
	_ = m.Write32(0xeb28, f.Input.Seed)
	_ = m.Write32(0x3ac, 0xffffffff)
	_ = cm.Write16(0xa2a, 0x120)
	for _, at := range []int{0x22, 0x1e, 0xdbe} {
		_ = m.Write32(at, 0x500400)
	}
	if f.Input.Custom != 0 {
		for side := 0; side < 2; side++ {
			at := 0x20630 + side*58
			groups := f.Input.Custom*3 + side
			if f.Input.Custom == 3 {
				groups = 401
			}
			for _, v := range [][2]int{{0, groups}, {2, 100 + side*71}, {4, 32 + side}, {6, 2 + side}, {8, 700 + side}, {10, 3 + side}, {12, side + 1}, {20, 0xffff}} {
				_ = cm.Write16(at+v[0], uint16(v[1]))
			}
			if f.Input.Custom == 2 {
				_ = cm.Write16(at+20, uint16(0x0102+side*0x3d3b))
			}
		}
	}
	return ram
}

func TestNativeStartupHostRealBodiesAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/startup_host_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []startupHostFixture }
	if err = json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 156 {
		t.Fatalf("native startup-host corpus changed: %d", len(catalog.Cases))
	}
	bundle := testBundle(t)
	rules, err := DecodeNativeStartupHostFrameRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	base := resourceFrameInitialRAM(t)
	copy(base[0x300000:], bundle.Raw["conquest.pak"])
	files, err := embedded.DataFS()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			ram := startupHostFixtureRAM(t, base, f)
			raw, code, chip := ram[0x200000:0x211280], ram[0x100000:0x13fa2c], ram[0x500000:0x500000+65032]
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			presentation, err := NewNativeFramePresentationState(bundle.Executable, 0x500000, 0x400000)
			if err != nil {
				t.Fatal(err)
			}
			presentation.Chip = chip
			presentation.Input.Mouse.Image = 0x120
			bitmap := func(address uint32) ([]byte, error) {
				at := int(int64(address) - 0x500000)
				if at < 0 || at > len(chip)-32000 {
					return nil, fmt.Errorf("actual startup bitmap%x unavailable", address)
				}
				return chip[at : at+32000], nil
			}
			disk, err := NewNativeResourceFilesystem(files)
			if err != nil {
				t.Fatal(err)
			}
			resource := NativeResourceHostFrameCallbacks{NativeErrorFrameCallbacks: NativeErrorFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Presentation: presentation, Bitmap: bitmap}}, IO: disk.IO}
			state := NativeStartupHostFrameState{Startup: NativeStartupResetFrameState{Entry: 0x10ad8}}
			for i := range state.Startup.A {
				state.Startup.A[i] = NativeRequesterAddress{Address: 0x700000 + uint32(i)*0x1000, Absolute: true}
			}
			cb := NativeStartupHostFrameCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Frame: &frame, Code: commandFrameBacking(code), Memory: commandFrameBacking(raw), RAM: commandFrameBacking(ram), CodeBase: 0x100000}, Resource: &resource, Bitmap: bitmap}
			calls := 0
			pauseCalls := 0
			paused := f.Input.PauseRoutine != 0
			if paused {
				cb.Resource = nil
			}
			cb.Call = func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
				if paused {
					pauseCalls++
					want := f.Pause
					if want == nil || call.Routine != want.Routine || call.Frame.D != want.D {
						return NativeCommandFrameResult{}, fmt.Errorf("native nested resource-entry frame differs")
					}
					for i, a := range *call.A {
						if a.Address != want.A[i] {
							return NativeCommandFrameResult{}, fmt.Errorf("native resource-entry A%d differs", i)
						}
					}
					if fileFrameHash(raw) != want.BSSHash || fileFrameHash(code) != want.CodeHash || fileFrameHash(chip) != want.ChipHash {
						return NativeCommandFrameResult{}, fmt.Errorf("native resource-entry whole memory differs")
					}
					*phase++
					return NativeCommandFrameResult{}, nil
				}
				calls++
				if call.Routine != f.Boundary || call.Frame.D != f.D {
					return NativeCommandFrameResult{}, fmt.Errorf("genuine startup boundary%x D differs:%08x/%08x", call.Routine, call.Frame.D, f.D)
				}
				for i, a := range *call.A {
					if a.Address != f.A[i] {
						return NativeCommandFrameResult{}, fmt.Errorf("genuine startup boundary A%d differs:%x/%x", i, a.Address, f.A[i])
					}
				}
				*phase++ // Retain the actual external operation; no invented return.
				return NativeCommandFrameResult{}, nil
			}
			if paused {
				step, err := state.Advance(&rules, cb)
				if err != nil {
					t.Fatal(err)
				}
				if step.Complete || !step.Waiting || state.Construction == nil || state.Power == nil && state.Campaign == nil {
					t.Fatal("genuine nested constructor/loader state was lost")
				}
				paused = false
				cb.Resource = &resource
			}
			for resume := 0; resume < 2; resume++ {
				step, err := state.Advance(&rules, cb)
				if err != nil {
					t.Fatal(err)
				}
				if step.Complete || !step.Waiting || step.FlagsKnown {
					t.Fatal("unresolved original child was declared complete")
				}
				if fileFrameHash(raw) != f.BSSHash || fileFrameHash(code) != f.CodeHash || fileFrameHash(chip) != f.ChipHash || fileFrameHash(ram[0x300000:0x300000+50000]) != f.CampaignHash {
					t.Fatal("composed original startup full memory/pixels differ")
				}
			}
			if f.Input.PauseRoutine != 0 && pauseCalls != 1 {
				t.Fatal("native intermediate resource prefix replayed")
			}
			if calls != 2 {
				t.Fatal("pending source boundary restarted another prefix")
			}
		})
	}
}

func TestNativeStartupHostMissingChildIsExplicit(t *testing.T) {
	data, err := os.ReadFile("testdata/startup_host_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []startupHostFixture }
	if err = json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	f := catalog.Cases[0]
	ram := startupHostFixtureRAM(t, resourceFrameInitialRAM(t), f)
	c := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
	s := NativeStartupHostFrameState{Startup: NativeStartupResetFrameState{Entry: 0x10ad8}}
	_, err = s.Advance(&NativeStartupHostFrameRules{}, NativeStartupHostFrameCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Frame: &c, Code: commandFrameBacking(ram[0x100000:0x13fa2c]), Memory: commandFrameBacking(ram[0x200000:0x211280]), RAM: commandFrameBacking(ram), CodeBase: 0x100000}})
	if err == nil || s.Startup.Finished {
		t.Fatal("missing actual resource body silently completed startup")
	}
}
