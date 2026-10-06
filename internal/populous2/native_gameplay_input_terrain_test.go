package populous2

import (
	"encoding/json"
	"os"
	"testing"
)

func TestNativeGameplayTerrainChildrenAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/gameplay_input_terrain_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Input struct {
				Routine int
				Name    string
				D       [8]uint32
				A       [7]uint32
				Patches []nativeHeroPatch
			}
			D                 [8]uint32
			A                 [7]uint32
			BSSHash, CodeHash string
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil || len(corpus.Cases) != 102 {
		t.Fatal("terrain native corpus incomplete", err)
	}
	rules, err := DecodeNativeRenderFrameRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range corpus.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			host := nativeHostTestMemory(t)
			ram := host.Memory()
			memory := nativeOffsetMemory(ram, 0x200000)
			code := nativeOffsetMemory(ram, 0x100000)
			for _, p := range f.Input.Patches {
				renderFramePatch(memory, p)
			}
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			var a [7]NativeRequesterAddress
			for i, v := range f.Input.A {
				a[i] = NativeRequesterAddress{Address: v, Absolute: true}
			}
			step, err := NativeGameplayTerrainChild(&rules, NativeStartupResetFrameCallbacks{Memory: memory, Code: code, RAM: ram, CodeBase: 0x100000, Frame: &frame}, NativeStartupResetFrameCall{Routine: f.Input.Routine, Frame: &frame, A: &a})
			if err != nil || !step.Complete || frame.D != f.D {
				t.Fatal("terrain source registers differ", frame.D, f.D, err)
			}
			for i, v := range a {
				if v.Address != f.A[i] {
					t.Fatal("terrain source address output differs", i, v.Address, f.A[i])
				}
			}
			if fileFrameHash(fileFrameMemoryBytes(t, memory)) != f.BSSHash {
				t.Fatal("terrain source BSS differs")
			}
			bytes, err := host.Span(0x100000, 0x3fa2c)
			if err != nil || fileFrameHash(bytes) != f.CodeHash {
				t.Fatal("terrain source CODE differs", err)
			}
		})
	}
}
