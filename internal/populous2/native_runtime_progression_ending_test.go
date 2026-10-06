package populous2

import (
	"encoding/json"
	"os"
	"testing"
)

type nativeProgressionEndingFixture struct {
	Input struct {
		Name  string
		Phase uint16
		Steps int
		D     [8]uint32
	}
	Frames []struct {
		PC                                       int
		D                                        [8]uint32
		A                                        [7]uint32
		BSSHash, CodeHash, ChipHash, PointerHash string
	}
}

func TestNativeRuntimeProgressionEndingAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/native_runtime_progression_ending_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var corpus struct {
		Cases []nativeProgressionEndingFixture
	}
	if e = json.Unmarshal(data, &corpus); e != nil {
		t.Fatal(e)
	}
	if len(corpus.Cases) != 6 {
		t.Fatal("native ending corpus changed")
	}
	for _, f := range corpus.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			h := nativeRuntimeHostTest(t)
			runtimeFilesPrelude(t, h)
			_ = h.Memory.Code.Write16(0xb240, f.Input.Phase)
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: h.Memory.BSSBase}
			state := NativeRuntimeEndingState{}
			for i := range state.A {
				state.A[i] = NativeRequesterAddress{Address: 0x900000 + uint32(i)*0x1000, Absolute: true}
			}
			for index, want := range f.Frames {
				step, e := state.Advance(h, &frame, NativeRuntimeProgressionCallbacks{})
				if e != nil {
					t.Fatal(e)
				}
				if frame.D != want.D {
					t.Fatalf("stage%d D got%08x want%08x", index, frame.D, want.D)
				}
				for i := range state.A {
					if state.A[i].Address != want.A[i] {
						t.Fatalf("stage%d A%d got%x want%x", index, i, state.A[i].Address, want.A[i])
					}
				}
				if step.Complete != (want.PC == 0) {
					t.Fatalf("stage%d completion differs sourcePC%x", index, want.PC)
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
					t.Fatalf("stage%d backing differs BSS%v CODE%v chip%v pointer%v", index, fileFrameHash(raw) == want.BSSHash, fileFrameHash(code) == want.CodeHash, fileFrameHash(h.Session.Presentation.Chip) == want.ChipHash, fileFrameHash(h.Session.Presentation.PointerData[:15260]) == want.PointerHash)
				}
				if index < f.Input.Steps {
					left := index == f.Input.Steps-1
					if _, e = h.Session.Presentation.VBlank(NativeMouseSample{Left: left}, h.Memory.BSS, &frame); e != nil {
						t.Fatal(e)
					}
				}
			}
		})
	}
}
