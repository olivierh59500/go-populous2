package populous2

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

type dosFrameInput struct {
	Menu    bool
	Name    string
	Routine int
	D       [8]uint32
	Cursor  uint16
	Files   []struct {
		Name string
		Data []byte
		Dir  bool
	}
	Path               string
	Overwrite, Clobber bool
	Limit              int
}
type dosFrameFixture struct {
	ChipHash, PointerHash string
	Scratch               []byte
	Input                 dosFrameInput
	D                     [8]uint32
	BSSHash               string
	FIB                   []byte
	CodeWords             []uint16
	Library               []struct {
		Vector            int
		D, Output         [8]uint32
		A                 [7]uint32
		BSSHash, CodeHash string
		D0                uint32
	}
	Children []struct {
		Routine int
		D       [8]uint32
		Hash    string
		Cursor  uint16
	}
	FinalFiles []struct {
		Name, Hash string
		Length     int
	}
}

func TestNativeDOSFrameAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/dos_frame_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var corpus struct{ Cases []dosFrameFixture }
	if e := json.Unmarshal(data, &corpus); e != nil {
		t.Fatal(e)
	}
	if len(corpus.Cases) != 126 {
		t.Fatalf("native DOS corpus changed %d", len(corpus.Cases))
	}
	for _, f := range corpus.Cases {
		for _, async := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-async%v", f.Input.Name, async), func(t *testing.T) {
				dir := filepath.Join(t.TempDir(), f.Input.Name)
				if e := os.Mkdir(dir, 0700); e != nil {
					t.Fatal(e)
				}
				for _, entry := range f.Input.Files {
					path := filepath.Join(dir, entry.Name)
					if entry.Dir {
						if e := os.Mkdir(path, 0700); e != nil {
							t.Fatal(e)
						}
					} else if e := os.WriteFile(path, entry.Data, 0600); e != nil {
						t.Fatal(e)
					}
				}
				fs := NewNativeDOSFilesystem(dir)
				fs.Async = async
				defer fs.Close()
				port := fs.Port()
				raw := make([]byte, 0x11280)
				for i := NativeGAMStart; i < NativeGAMEnd; i++ {
					raw[i] = byte(i*37 + 11)
				}
				m := commandFrameBacking(raw)
				var presentation *NativeFramePresentationState
				if f.Input.Menu {
					var e error
					presentation, e = NewNativeFramePresentationState(testBundle(t).Executable, 0x500000, 0x400000)
					if e != nil {
						t.Fatal(e)
					}
					presentation.InterruptChain = false
					if _, e = presentation.Initialize(testBundle(t).Executable, NativeMouseSample{}); e != nil {
						t.Fatal(e)
					}
					m = presentation.Memory(m)
				}
				_ = m.Write32(0xf32, 0x1240)
				_ = m.Write32(0xf36, 0x82)
				_ = m.Write32(0x14c, 0x900000)
				code := fileFrameRelocatedCode(t)
				cm := commandFrameBacking(code)
				if presentation != nil {
					original := cm
					cm.Read8 = func(at int) (uint8, error) {
						if at == 0xa2a {
							return uint8(presentation.Input.Mouse.Image >> 8), nil
						}
						if at == 0xa2b {
							return uint8(presentation.Input.Mouse.Image), nil
						}
						return original.Read8(at)
					}
					cm.Write8 = func(at int, v uint8) error {
						if at == 0xa2a {
							presentation.Input.Mouse.Image = presentation.Input.Mouse.Image&255 | uint16(v)<<8
						}
						if at == 0xa2b {
							presentation.Input.Mouse.Image = presentation.Input.Mouse.Image&0xff00 | uint16(v)
						}
						return original.Write8(at, v)
					}
					cm.Read16 = func(at int) (uint16, error) {
						if at == 0xa2a {
							return presentation.Input.Mouse.Image, nil
						}
						return original.Read16(at)
					}
					cm.Write16 = func(at int, v uint16) error {
						if at == 0xa2a {
							presentation.Input.Mouse.Image = v
						}
						return original.Write16(at, v)
					}
				}

				_ = cm.Write16(0xa2a, f.Input.Cursor)
				pathAt := 0x4440
				if f.Input.Routine != 0x19936 {
					pathAt = 0x43c0
				}
				copy(code[pathAt:], f.Input.Path)
				code[pathAt+len(f.Input.Path)] = 0
				frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
				a := [7]NativeRequesterAddress{}
				for i := range a {
					a[i] = NativeRequesterAddress{Address: 0x100000 + uint32(0x200+i*4), Code: true}
				}
				a[0] = NativeRequesterAddress{Address: 0x100000 + uint32(pathAt), Code: true}
				a[1] = NativeRequesterAddress{Address: 0x200000 + 0x3be}
				a[4] = NativeRequesterAddress{Address: 0x200000 + uint32(f.Input.Limit)}
				index, children := 0, 0
				overwrite := NativeDOSOverwriteState{}
				menuClicked := false
				cb := NativeDOSFrameCallbacks{Code: cm, Memory: m, CodeBase: 0x100000, Frame: &frame, Port: NativeDOSPort{Call: func(call NativeDOSLibraryCall, phase *uint32) (NativeDOSLibraryResult, error) {
					if index >= len(f.Library) {
						return NativeDOSLibraryResult{}, fmt.Errorf("unexpected library vector%d", call.Vector)
					}
					want := f.Library[index]
					if *phase == 0 {
						if call.Vector != want.Vector || frame.D != want.D {
							return NativeDOSLibraryResult{}, fmt.Errorf("library%d entry D differs got%08x want%08x", call.Vector, frame.D, want.D)
						}
						if fileFrameHash(fileFrameMemoryBytes(t, m)) != want.BSSHash || fileFrameHash(code[0x199f8:0x19afc]) != want.CodeHash {
							return NativeDOSLibraryResult{}, fmt.Errorf("library%d raw prefix differs BSS%v FIB%v", call.Vector, fileFrameHash(fileFrameMemoryBytes(t, m)) == want.BSSHash, fileFrameHash(code[0x199f8:0x19afc]) == want.CodeHash)
						}
					}
					result, e := port.Call(call, phase)
					if e != nil || !result.Complete {
						return result, e
					}
					if f.Input.Clobber {
						for i := 1; i < 8; i++ {
							frame.D[i] ^= 0x7a5c0000 + uint32(index)*0x101 + uint32(i)*0x10101
						}
					}
					output := frame.D
					output[0] = result.D0
					if output != want.Output || result.D0 != want.D0 {
						return NativeDOSLibraryResult{}, fmt.Errorf("actual filesystem vector%d output differs got%08x want%08x", call.Vector, output, want.Output)
					}
					index++
					return result, nil
				}}, Call: func(call NativeFileFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
					if f.Input.Menu && call.Routine == 0x341e {
						resolver := func(address uint32) ([]byte, error) {
							at := int(address - presentation.ChipBase)
							if at < 0 || at > len(presentation.Chip)-32000 {
								return nil, fmt.Errorf("chip address unavailable")
							}
							return presentation.Chip[at : at+32000], nil
						}
						step, e := overwrite.Advance(NativeFileFrameCallbacks{Code: cm, Memory: m, CodeBase: 0x100000, Frame: &frame, Presentation: presentation, Bitmap: resolver, Sound: func(uint16, *NativeFrameRegisterContext) error { return nil }})
						if e != nil {
							return NativeCommandFrameResult{}, e
						}
						if !step.Complete && !menuClicked {
							action := 2
							if !f.Input.Overwrite {
								action = 4
							}
							start := 0xab4e + int(int16(binary.BigEndian.Uint16(code[0xab4e:])))
							width := int(binary.BigEndian.Uint16(code[0xab54:]))
							count, x, y := 0, 0, 0
							for i := 0; code[start+i] != 0; i++ {
								v := code[start+i]
								if int8(v) > 0x5a && int8(code[0x4e92+int(v)-0x5b]) > 0 {
									count += 2
									if count == action {
										x = (int(binary.BigEndian.Uint16(code[0xab50:])) + i%(width+1)) * 8
										y = int(binary.BigEndian.Uint16(code[0xab52:])) + i/(width+1)*8
										break
									}
								}
							}
							irq := func(x, y uint8, left bool) error {
								_, e := presentation.VBlank(NativeMouseSample{CounterX: x, CounterY: y, Left: left}, m, &frame)
								return e
							}
							for int(presentation.Input.Mouse.PositionX) != x*2 || int(presentation.Input.Mouse.PositionY) != y*2 {
								dx, dy := x*2-int(presentation.Input.Mouse.PositionX), y*2-int(presentation.Input.Mouse.PositionY)
								dx = max(-100, min(100, dx))
								dy = max(-100, min(100, dy))
								if e := irq(uint8(int(presentation.Input.Mouse.CounterX)+dx), uint8(int(presentation.Input.Mouse.CounterY)+dy), false); e != nil {
									return NativeCommandFrameResult{}, e
								}
							}
							if e := irq(uint8(presentation.Input.Mouse.CounterX), uint8(presentation.Input.Mouse.CounterY), true); e != nil {
								return NativeCommandFrameResult{}, e
							}
							menuClicked = true
						}
						return NativeCommandFrameResult{Complete: step.Complete}, nil
					}
					if children >= len(f.Children) {
						return NativeCommandFrameResult{}, fmt.Errorf("unexpected DOS child%x", call.Routine)
					}
					want := f.Children[children]
					if *phase == 0 {
						if call.Routine != want.Routine || frame.D != want.D || fileFrameHash(fileFrameMemoryBytes(t, m)) != want.Hash || binary.BigEndian.Uint16(code[0xa2a:]) != want.Cursor {
							return NativeCommandFrameResult{}, fmt.Errorf("DOS child%x prefix differs got%08x want%08x", call.Routine, frame.D, want.D)
						}
						if async {
							*phase = 1
							return NativeCommandFrameResult{Zero: true}, nil
						}
					}
					if call.Routine == 0x341e {
						for i := range frame.D {
							frame.D[i] ^= 0x312000 + uint32(i)*0x11111
						}
						frame.D[0] = 0
						if f.Input.Overwrite {
							frame.D[0] = 1
						}
					} else {
						for i := range frame.D {
							frame.D[i] ^= 0x439000 + uint32(i)*0x12121
						}
					}
					children++
					return NativeCommandFrameResult{Complete: true}, nil
				}}
				state := NativeDOSFrameState{}
				complete := false
				for resumes := 0; resumes < 100000 && !complete; resumes++ {
					step, e := state.Advance(f.Input.Routine, a, cb)
					if e != nil {
						t.Fatal(e)
					}
					complete = step.Complete
					if !complete {
						runtime.Gosched()
					}
				}
				if !complete || index != len(f.Library) || children != len(f.Children) {
					t.Fatalf("source DOS continuation incomplete %v/%d/%d", complete, index, children)
				}
				if f.Input.Menu {
					if fileFrameHash(presentation.Chip) != f.ChipHash || fileFrameHash(presentation.PointerData[:15260]) != f.PointerHash || !bytes.Equal(code[0xab4e:0xab4e+2048], f.Scratch) {
						t.Fatalf("actual composed overwrite differs chip%v pointer%v scratch%v", fileFrameHash(presentation.Chip) == f.ChipHash, fileFrameHash(presentation.PointerData[:15260]) == f.PointerHash, bytes.Equal(code[0xab4e:0xab4e+2048], f.Scratch))
					}
				}
				if frame.D != f.D {
					t.Fatalf("complete native D differs got%08x want%08x", frame.D, f.D)
				}
				if fileFrameHash(fileFrameMemoryBytes(t, m)) != f.BSSHash || !bytes.Equal(code[0x199f8:0x19afc], f.FIB) || binary.BigEndian.Uint16(code[0xa2a:]) != f.CodeWords[0] || binary.BigEndian.Uint16(code[0x443e:]) != f.CodeWords[1] {
					t.Fatal("complete native BSS/FIB/CODE differs")
				}
				for _, want := range f.FinalFiles {
					data, e := os.ReadFile(filepath.Join(dir, want.Name))
					if e != nil {
						t.Fatal(e)
					}
					if len(data) != want.Length || fileFrameHash(data) != want.Hash {
						t.Fatal("actual filesystem bytes differ from native transfer")
					}
				}
			})
		}
	}
}

func TestNativeDOSFilesystemIdentityPendingAndErrors(t *testing.T) {
	dir := t.TempDir()
	if e := os.WriteFile(filepath.Join(dir, "lower.GAM"), []byte{1, 2, 3}, 0600); e != nil {
		t.Fatal(e)
	}
	fs := NewNativeDOSFilesystem(dir)
	fs.Async = true
	defer fs.Close()
	frame := NativeFrameRegisterContext{}
	a := [7]NativeRequesterAddress{}
	call := NativeDOSLibraryCall{Vector: -30, Frame: &frame, A: &a, Path: []byte("LOWER.GAM")}
	frame.D[2] = 1005
	port := fs.Port()
	phase := uint32(0)
	result, e := port.Call(call, &phase)
	if e != nil || result.Complete || phase == 0 {
		t.Fatal("real async Open did not retain operation", e)
	}
	frozen := phase
	for attempts := 0; !result.Complete && attempts < 100000; attempts++ {
		result, e = port.Call(call, &phase)
		if e != nil {
			t.Fatal(e)
		}
		if !result.Complete && phase != frozen {
			t.Fatal("pending Open replaced its operation")
		}
		runtime.Gosched()
	}
	if !result.Complete || result.D0 == 0 {
		t.Fatal("native uppercase identity did not resolve actual file")
	}
	handle := result.D0
	call = NativeDOSLibraryCall{Vector: -36, Handle: handle, Frame: &frame, A: &a}
	phase = 0
	for attempts := 0; attempts < 100000; attempts++ {
		result, e = port.Call(call, &phase)
		if e != nil {
			t.Fatal(e)
		}
		if result.Complete {
			break
		}
		runtime.Gosched()
	}
	if !result.Complete || result.D0 != 0xffffffff {
		t.Fatal("actual Close did not return native DOSTRUE")
	}
	call = NativeDOSLibraryCall{Vector: -30, Frame: &frame, A: &a, Path: []byte("MISSING.GAM")}
	phase = 0
	for attempts := 0; attempts < 100000; attempts++ {
		result, e = port.Call(call, &phase)
		if e != nil {
			t.Fatal(e)
		}
		if result.Complete {
			break
		}
		runtime.Gosched()
	}
	if !result.Complete || result.D0 != 0 || fs.LastError() == nil {
		t.Fatal("actual missing-file failure was lost")
	}
	if _, e := os.Stat(filepath.Join(dir, "MISSING.GAM")); !os.IsNotExist(e) {
		t.Fatal("failed old-file Open created a file")
	}
}
