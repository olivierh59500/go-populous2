package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type editorExportFixture struct {
	Input struct {
		Name     string
		D        [8]uint32
		Open     int32
		Short    int
		Close    int32
		Filename byte
	}
	D                              [8]uint32
	A                              [7]uint32
	CCR                            uint16
	BSSHash, CodeHash, WrittenHash string
	WrittenLength                  int
	Calls                          []struct {
		Kind, Name string
		D          [8]uint32
		A          [7]uint32
		Value      int32
		DataHash   string
		Length     int
	}
	CallKindsValuesHashes [][]int64
	ABICheckpoints        []struct {
		Index int
		D     [8]uint32
		A     [7]uint32
	}
	OpenName string
}

func TestNativeGameplayScreenExportAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/gameplay_editor_input_export_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var corpus struct {
		Cases      []editorExportFixture
		DataHashes []string
	}
	if e = json.Unmarshal(data, &corpus); e != nil {
		t.Fatal(e)
	}
	if len(corpus.Cases) != 108 {
		t.Fatal("nativeexportcorpuschanged")
	}
	for i := range corpus.Cases {
		c := &corpus.Cases[i]
		for _, numbers := range c.CallKindsValuesHashes {
			var call struct {
				Kind, Name string
				D          [8]uint32
				A          [7]uint32
				Value      int32
				DataHash   string
				Length     int
			}
			call.Kind = []string{"open", "write", "close"}[numbers[0]]
			call.Value = int32(numbers[1])
			call.DataHash = corpus.DataHashes[numbers[2]]
			call.Length = int(numbers[3])
			for _, abi := range c.ABICheckpoints {
				if abi.Index == len(c.Calls) {
					call.D, call.A = abi.D, abi.A
				}
			}
			if call.Kind == "open" {
				call.Name = c.OpenName
			}
			c.Calls = append(c.Calls, call)
		}
	}
	for _, f := range corpus.Cases {
		for _, pending := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-pending%v", f.Input.Name, pending), func(t *testing.T) {
				host, e := NewNativeHunkMemory(testBundle(t).Executable, []uint32{0x100000, 0x200000, 0x300000, 0x400000, 0x500000, 0x600000})
				if e != nil {
					t.Fatal(e)
				}
				bitmap := make([]byte, 32000)
				for i := range bitmap {
					bitmap[i] = byte(i*29 + 3)
				}
				if e = host.MapRegion(NativeHostRegion{Name: "realfrontscreen", Base: 0xa10000, Bytes: bitmap}); e != nil {
					t.Fatal(e)
				}
				raw, _ := host.Span(0x200000, 0x11280)
				code, _ := host.Span(0x100000, 0x3fa2c)
				for i := range raw {
					raw[i] = byte(i*13 + (i >> 5) + 11)
				}
				m, cm := commandFrameBacking(raw), commandFrameBacking(code)
				_ = m.Write32(0x1a, 0xa10000)
				_ = m.Write32(0x14c, 0xd00000)
				_ = cm.Write8(0x1a587, f.Input.Filename)
				frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
				state := NativeGameplayScreenExportState{}
				for i := range state.A {
					state.A[i] = NativeRequesterAddress{Address: 0x900000 + uint32(i)*0x1000, Absolute: true}
				}
				calls := 0
				written := []byte{}
				cb := NativeGameplayScreenExportCallbacks{NativeGameplayEditorInputCallbacks: NativeGameplayEditorInputCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Frame: &frame, Memory: m, Code: cm, RAM: host.Memory(), CodeBase: 0x100000}}, IO: func(call NativeGameplayScreenExportCall, phase *uint32) (NativeGameplayScreenExportResult, error) {
					if calls >= len(f.Calls) {
						return NativeGameplayScreenExportResult{}, fmt.Errorf("unexpectedexportoperation")
					}
					want := f.Calls[calls]
					if call.Operation != want.Kind || call.Name != want.Name || want.D != [8]uint32{} && frame.D != want.D || len(call.Data) != want.Length || fileFrameHash(call.Data) != want.DataHash {
						return NativeGameplayScreenExportResult{}, fmt.Errorf("actualexportcall%d differs", calls)
					}
					for i, v := range want.A {
						if want.D != [8]uint32{} && state.A[i].Address != v {
							return NativeGameplayScreenExportResult{}, fmt.Errorf("actualDOSaddressA%d differs", i)
						}
					}
					if pending && *phase == 0 {
						*phase = 1
						return NativeGameplayScreenExportResult{}, nil
					}
					if call.Operation == "write" && want.Value > 0 {
						written = append(written, call.Data[:want.Value]...)
					}
					calls++
					return NativeGameplayScreenExportResult{Complete: true, Value: want.Value}, nil
				}}
				var step NativeGameplayEditorInputStep
				for attempts := 0; attempts < 4000; attempts++ {
					step, e = state.Advance(cb)
					if e != nil {
						t.Fatal(e)
					}
					if step.Complete {
						break
					}
				}
				if !step.Complete || calls != len(f.Calls) {
					t.Fatal("screenexportcontinuationincomplete")
				}
				if frame.D != f.D {
					t.Fatal("outerexportDrestorechanged")
				}
				for i, v := range state.A {
					if v.Address != f.A[i] {
						t.Fatalf("exportA%d differs", i)
					}
				}
				if fileFrameHash(raw) != f.BSSHash || fileFrameHash(code) != f.CodeHash || fileFrameHash(written) != f.WrittenHash || len(written) != f.WrittenLength {
					t.Fatalf("screenexportrawbytes BSS%v CODE%v written%v", fileFrameHash(raw) == f.BSSHash, fileFrameHash(code) == f.CodeHash, fileFrameHash(written) == f.WrittenHash)
				}
			})
		}
	}
}
