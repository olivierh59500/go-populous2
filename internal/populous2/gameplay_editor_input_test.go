package populous2

import (
	"encoding/json"
	"os"
	"testing"
)

type editorInputFixture struct {
	Input struct {
		Name                   string
		Entry, Pattern, Cursor int
		D                      [8]uint32
		A                      [7]uint32
		Old, Counter           uint32
		X, Y, Mask             uint16
		Wall                   bool
	}
	D                             [8]uint32
	A                             [7]uint32
	CCR                           uint16
	BSSHash, CodeHash, BitmapHash string
}

func TestNativeGameplayEditorInputAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/gameplay_editor_input_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var corpus struct{ Cases []editorInputFixture }
	if e = json.Unmarshal(data, &corpus); e != nil {
		t.Fatal(e)
	}
	if len(corpus.Cases) != 354 {
		t.Fatal("nativeeditorcorpuschanged")
	}
	for _, f := range corpus.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			host, e := NewNativeHunkMemory(testBundle(t).Executable, []uint32{0x100000, 0x200000, 0x300000, 0x400000, 0x500000, 0x600000})
			if e != nil {
				t.Fatal(e)
			}
			bitmap := make([]byte, 32000)
			if e = host.MapRegion(NativeHostRegion{Name: "actualbackgroundbitmap", Base: 0xa10000, Bytes: bitmap}); e != nil {
				t.Fatal(e)
			}
			raw, _ := host.Span(0x200000, 0x11280)
			code, _ := host.Span(0x100000, 0x3fa2c)
			for i := range raw {
				raw[i] = byte(i*13 + (i >> 5) + 11)
			}
			for i := range bitmap {
				bitmap[i] = byte(i*29 + f.Input.Pattern*41 + 3)
			}
			m, cm := commandFrameBacking(raw), commandFrameBacking(code)
			for cell := 0; cell < 4096; cell++ {
				height, tile := byte(3), byte(0)
				switch f.Input.Pattern {
				case 1:
					if cell%137 == 0 {
						height = 4
					}
				case 2:
					height = byte(cell % 8)
				case 3:
					height = 0
					tile = byte(cell%15 + 1)
				case 4:
					height = 7
					tile = 15
				case 5:
					if cell < 2048 {
						height = 2
					} else {
						height = 5
					}
				case 6:
					height = byte((cell/64 + cell%64) % 4)
					tile = byte(cell % 256)
				}
				_ = m.Write8(0xf44+cell*4, height)
				_ = m.Write8(0xf45+cell*4, tile)
			}
			_ = m.Write32(0x22, 0xa10000)
			_ = m.Write16(0xeb6e, uint16(f.Input.Cursor))
			_ = m.Write32(0xf36, f.Input.Old)
			_ = m.Write16(0xf30, uint16(f.Input.Counter))
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			if f.Input.Entry == 0xd80c {
				for cell := 0; cell < 4096; cell++ {
					_ = m.Write16(0xf46+cell*4, 0)
				}
				if f.Input.Wall {
					_ = m.Write16(0xf46+(int(f.Input.X)+int(f.Input.Y)*64)*4, 0x5140)
					_ = m.Write8(0xc800, 0x1a)
					_ = m.Write16(0xc802, 0)
				}
				frame.Word(0, f.Input.X)
				frame.Word(1, f.Input.Y)
				frame.Word(3, f.Input.Mask)
			}
			a := [7]NativeRequesterAddress{}
			for i, v := range f.Input.A {
				a[i] = NativeRequesterAddress{Address: v, Absolute: true}
			}
			cb := NativeGameplayEditorInputCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Frame: &frame, Memory: m, Code: cm, RAM: host.Memory(), CodeBase: 0x100000}, Bitmap: func(at uint32) ([]byte, error) { return host.Span(at, 32000) }}
			var step NativeGameplayEditorInputStep
			if f.Input.Entry == 0xd80c {
				step, e = RunNativeGameplayEditorPlannedRaise(cb, &a)
			} else {
				step, e = RunNativeGameplayEditorInput(f.Input.Entry, cb, &a)
			}
			if e != nil {
				t.Fatal(e)
			}
			if !step.Complete {
				t.Fatal("nativeeditorbodydidnotcomplete")
			}
			if frame.D != f.D {
				t.Fatalf("allDgot%08x want%08x", frame.D, f.D)
			}
			for i, v := range a {
				if v.Address != f.A[i] {
					t.Fatalf("A%d got%x want%x", i, v.Address, f.A[i])
				}
			}
			if fileFrameHash(raw) != f.BSSHash || fileFrameHash(code) != f.CodeHash || fileFrameHash(bitmap) != f.BitmapHash {
				t.Fatalf("nativeeditor BSS%v CODE%v bitmap%v", fileFrameHash(raw) == f.BSSHash, fileFrameHash(code) == f.CodeHash, fileFrameHash(bitmap) == f.BitmapHash)
			}
		})
	}
}

func TestNativeGameplayEditorMirrorRetainsOddQueueAliasTrap(t *testing.T) {
	host, e := NewNativeHunkMemory(testBundle(t).Executable, []uint32{0x100000, 0x200000, 0x300000, 0x400000, 0x500000, 0x600000})
	if e != nil {
		t.Fatal(e)
	}
	bitmap := make([]byte, 32000)
	if e = host.MapRegion(NativeHostRegion{Name: "actualbackgroundbitmap", Base: 0xa10000, Bytes: bitmap}); e != nil {
		t.Fatal(e)
	}
	raw, _ := host.Span(0x200000, 0x11280)
	code, _ := host.Span(0x100000, 0x3fa2c)
	for i := range raw {
		raw[i] = byte(i*13 + (i >> 5) + 11)
	}
	for cell := 0; cell < 4096; cell++ {
		raw[0xf44+cell*4] = byte(cell % 8)
		raw[0xf45+cell*4] = 0
	}
	m := commandFrameBacking(raw)
	_ = m.Write32(0x22, 0xa10000)
	_ = m.Write16(0xeb6e, 0xfffe)
	frame := NativeFrameRegisterContext{AddressBase: 0x200000}
	a := [7]NativeRequesterAddress{}
	for i := range a {
		a[i] = NativeRequesterAddress{Address: 0x900000 + uint32(i)*0x1000, Absolute: true}
	}
	step, e := RunNativeGameplayEditorInput(0xd97e, NativeGameplayEditorInputCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Frame: &frame, Memory: m, Code: commandFrameBacking(code), RAM: host.Memory(), CodeBase: 0x100000}, Bitmap: func(at uint32) ([]byte, error) { return host.Span(at, 32000) }}, &a)
	if e == nil || step.Complete {
		t.Fatal("nativeoddwritebecameguessedcompletion")
	}
	cursor, _ := m.Read16(0xeb6e)
	if cursor&1 == 0 {
		t.Fatal("rawFIFOselfaliaswasnormalized")
	}
}
