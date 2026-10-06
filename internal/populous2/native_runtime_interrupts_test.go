package populous2

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestNativeInterruptVectorsAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/native_runtime_interrupts_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Seed              int
			Status            uint16
			OldDivide, OldIRQ uint32
			InputD            [8]uint32
			Frames            []struct {
				Routine                    int
				D                          [8]uint32
				A                          [7]uint32
				Hardware                   []NativeFrameHardwareWrite
				LowHash, BSSHash, CodeHash string
			}
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 9 {
		t.Fatal("original vector corpus incomplete", err)
	}
	frames := 0
	for _, f := range catalog.Cases {
		if len(f.Frames) != 3 {
			t.Fatal("original install/swap sequence truncated")
		}
		low, bss := make([]byte, 256), make([]byte, 0x11280)
		for i := range low {
			low[i] = byte(i*13 + f.Seed*7)
		}
		for i := range bss {
			bss[i] = byte(i*3 + f.Seed*11)
		}
		binary.BigEndian.PutUint32(low[0x14:], f.OldDivide)
		binary.BigEndian.PutUint32(low[0x6c:], f.OldIRQ)
		c := NativeFrameRegisterContext{D: f.InputD, AddressBase: 0x200000}
		for _, want := range f.Frames {
			step, err := RunNativeInterruptVectors(want.Routine, NativeInterruptVectorCallbacks{RAM: commandFrameBacking(low), Memory: commandFrameBacking(bss), CodeBase: 0x100000, Frame: &c, ReadHardware16: func(address uint32) (uint16, error) {
				if address != 0xdff01c {
					t.Fatal("wrong hardware input", address)
				}
				return f.Status, nil
			}})
			if err != nil || !step.Complete || c.D != want.D || !reflect.DeepEqual(step.Hardware, want.Hardware) || fileFrameHash(low) != want.LowHash || fileFrameHash(bss) != want.BSSHash {
				t.Fatalf("native vector routine%x differs: D%x want%x step%+v err%v", want.Routine, c.D, want.D, step, err)
			}
			for i, a := range want.A {
				if a != 0x900000+uint32(i)*0x1000 {
					t.Fatal("source address preservation changed")
				}
			}
			frames++
		}
	}
	if frames != 27 {
		t.Fatal("native vector coverage changed", frames)
	}
}

func TestNativeResultZeroDivideRequiresActualRTEVector(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	c := NativeFrameRegisterContext{AddressBase: h.Memory.BSSBase, D: [8]uint32{0x81234567, 0x12340000, 2, 3, 4, 5, 6, 7}}
	before := c.D
	if err := nativeResultScoreDivide(h, &c); err == nil || c.D != before {
		t.Fatal("missing exception handler invented score success")
	}
	if _, err := RunNativeInterruptVectors(0x39e, NativeInterruptVectorCallbacks{RAM: h.Memory.RAM, Memory: h.Memory.BSS, CodeBase: h.Memory.CodeBase, Frame: &c, ReadHardware16: func(uint32) (uint16, error) { return 0, nil }}); err != nil {
		t.Fatal(err)
	}
	c.D = before
	if err := nativeResultScoreDivide(h, &c); err != nil || c.D != before {
		t.Fatal("installed RTE did not preserve dividend", err)
	}
	if _, err := RunNativeInterruptVectors(0x370, NativeInterruptVectorCallbacks{RAM: h.Memory.RAM, Memory: h.Memory.BSS, CodeBase: h.Memory.CodeBase, Frame: &c}); err != nil {
		t.Fatal(err)
	}
	c.D = before
	if err := nativeResultScoreDivide(h, &c); err == nil || c.D != before {
		t.Fatal("restored old vector was treated as installed RTE")
	}
}

func TestNativeResultZeroDivideAgainstOriginalExceptionAndRTE(t *testing.T) {
	data, err := os.ReadFile("testdata/native_runtime_interrupts_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		RTECases []struct {
			D, AfterD                                         [8]uint32
			BeforeSR, AfterSR, StackedSR                      uint16
			BeforeA7, AfterA7, HandlerPC, ResumePC, StackedPC uint32
			ExceptionCycles, RTECycles                        int
			LowHash, BSSHash                                  string
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.RTECases) != 18 {
		t.Fatal("original DIVU/RTE corpus incomplete", err)
	}
	for _, f := range catalog.RTECases {
		if f.AfterD != f.D || f.BeforeSR != f.AfterSR || f.StackedSR != f.BeforeSR || f.BeforeA7 != f.AfterA7 || f.HandlerPC != 0x10043e || f.ResumePC != 0x1039aa || f.StackedPC != f.ResumePC || f.ExceptionCycles != 34 || f.RTECycles != 20 {
			t.Fatal("original exception continuation changed")
		}
		h := nativeRuntimeHostTest(t)
		low, err := h.Host.Span(0, 256)
		if err != nil {
			t.Fatal(err)
		}
		clear(low)
		c := NativeFrameRegisterContext{AddressBase: h.Memory.BSSBase}
		if _, err := RunNativeInterruptVectors(0x39e, NativeInterruptVectorCallbacks{RAM: h.Memory.RAM, Memory: h.Memory.BSS, CodeBase: h.Memory.CodeBase, Frame: &c, ReadHardware16: func(uint32) (uint16, error) { return 0, nil }}); err != nil {
			t.Fatal(err)
		}
		c.D = f.D
		if err := nativeResultScoreDivide(h, &c); err != nil || c.D != f.AfterD {
			t.Fatal("native RTE score continuation differs", c.D, f.AfterD, err)
		}
		bss, err := h.Memory.SnapshotBSS()
		if err != nil {
			t.Fatal(err)
		}
		if fileFrameHash(low) != f.LowHash || fileFrameHash(bss) != f.BSSHash {
			t.Fatal("native exception/vector memory differs")
		}
	}
}
