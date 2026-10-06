package populous2

import (
	"encoding/json"
	"os"
	"testing"
)

type nativeProgressionAnimationInput struct {
	Name   string
	Index  uint16
	Frames int
	D      [8]uint32
}
type nativeProgressionAnimationFixture struct {
	Input  nativeProgressionAnimationInput
	Frames []struct {
		Frame                                    int
		D                                        [8]uint32
		A                                        [7]uint32
		BSSHash, CodeHash, ChipHash, PointerHash string
	}
}

func TestNativeRuntimeProgressionAnimationAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/native_runtime_progression_animation_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var corpus struct {
		Cases []nativeProgressionAnimationFixture
	}
	if e = json.Unmarshal(data, &corpus); e != nil {
		t.Fatal(e)
	}
	if len(corpus.Cases) != 6 {
		t.Fatal("native interpreter corpus changed")
	}
	for _, f := range corpus.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			h := nativeRuntimeHostTest(t)
			runtimeFilesPrelude(t, h)
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: h.Memory.BSSBase}
			frame.Word(0, f.Input.Index)
			cb, e := h.ResourceCallbacks(&frame, NativeErrorFrameCallbacks{})
			if e != nil {
				t.Fatal(e)
			}
			source := NativeResourceHostFrameState{}
			step, e := source.Advance(&h.ResourceRules, cb)
			if e != nil || !step.Complete {
				t.Fatal(step, e)
			}
			frame.D = f.Input.D
			frame.D[0] = nativeAnimationInit
			a := [7]NativeRequesterAddress{}
			for i := range a {
				a[i] = NativeRequesterAddress{Address: 0x900000 + uint32(i)*0x1000, Absolute: true}
			}
			operand := 0xb274
			if f.Input.Index == 25 {
				operand = 0xb14e
			}
			p, e := h.Memory.Code.Read32(operand)
			if e != nil {
				t.Fatal(e)
			}
			a[4] = NativeRequesterAddress{Address: p, Absolute: true}
			body := NativeStartupResetFrameCallbacks{Code: h.Memory.Code, Memory: h.Memory.BSS, RAM: h.Memory.RAM, CodeBase: h.Memory.CodeBase, Frame: &frame}
			for _, want := range f.Frames {
				if e = RunNativeProgressionAnimation(body, &a); e != nil {
					t.Fatal(e)
				}
				if frame.D != want.D {
					t.Fatalf("frame%d D got%08x want%08x", want.Frame, frame.D, want.D)
				}
				for i := range a {
					if a[i].Address != want.A[i] {
						t.Fatalf("frame%d A%d got%x want%x", want.Frame, i, a[i].Address, want.A[i])
					}
				}
				raw, e := h.Memory.SnapshotBSS()
				if e != nil {
					t.Fatal(e)
				}
				code := make([]byte, 0x3fa2c)
				for i := range code {
					code[i], e = h.Memory.Code.Read8(i)
					if e != nil {
						t.Fatal(e)
					}
				}
				if fileFrameHash(raw) != want.BSSHash || fileFrameHash(code) != want.CodeHash || fileFrameHash(h.Session.Presentation.Chip) != want.ChipHash || fileFrameHash(h.Session.Presentation.PointerData[:15260]) != want.PointerHash {
					t.Fatalf("frame%d raw backing differs BSS%v CODE%v chip%v pointer%v", want.Frame, fileFrameHash(raw) == want.BSSHash, fileFrameHash(code) == want.CodeHash, fileFrameHash(h.Session.Presentation.Chip) == want.ChipHash, fileFrameHash(h.Session.Presentation.PointerData[:15260]) == want.PointerHash)
				}
				b := nativeRequesterFrameBacking{Code: h.Memory.Code, Memory: h.Memory.BSS, CodeBase: h.Memory.CodeBase, Frame: &frame}
				if e = fileFrameSwap(b, h.Session.Presentation); e != nil {
					t.Fatal(e)
				}
			}
		})
	}
}
