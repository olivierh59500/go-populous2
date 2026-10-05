package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNativeStartupAllocationAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/startup_allocation_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Input struct {
				Name              string
				Routine           uint32
				Audio, Background uint32
				D                 [8]uint32
			}
			D                 [8]uint32
			A                 [7]uint32
			BSSHash, CodeHash string
			Calls             []struct {
				Kind                                string
				Free                                bool
				Size, Flags, Address, A0, A6, Value uint32
				D                                   [8]uint32
			}
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 36 {
		t.Fatal("native allocation corpus incomplete", err)
	}
	for _, f := range catalog.Cases {
		for _, pending := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-pending%v", f.Input.Name, pending), func(t *testing.T) {
				raw := make([]byte, 0x11280)
				memory := commandFrameBacking(raw)
				if f.Input.Routine == 0x1a4bc {
					_ = memory.Write32(0x3b4, f.Input.Audio)
					_ = memory.Write32(0xdbe, f.Input.Background)
				}
				code := fileFrameRelocatedCode(t)
				cm := commandFrameBacking(code)
				frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
				state := NativeStartupAllocationState{}
				for i := range state.A {
					state.A[i] = NativeRequesterAddress{Address: 0x800000 + uint32(i)*0x1000, Absolute: true}
				}
				calls := 0
				allocations := 0
				cb := NativeStartupAllocationCallbacks{Code: cm, Memory: memory, CodeBase: 0x100000, Frame: &frame, ReadExecBase: func() (uint32, error) { return 0xd00000, nil }, Allocate: func(call NativeStartupAllocationCall, phase *uint32) (NativeStartupAllocationResult, error) {
					if calls >= len(f.Calls) {
						return NativeStartupAllocationResult{}, fmt.Errorf("extra host allocation")
					}
					want := f.Calls[calls]
					if want.Kind != "allocation" || call.Free != want.Free || call.Size != want.Size || (!call.Free && call.Flags != want.Flags) || call.Free && call.Address != want.Address || call.Frame.D != want.D {
						return NativeStartupAllocationResult{}, fmt.Errorf("native allocation inputs differ at%d", calls)
					}
					if pending && *phase == 0 {
						*phase = 1
						return NativeStartupAllocationResult{}, nil
					}
					*phase = 0
					call.Frame.D[1] ^= 0x01020304
					value := f.Input.Audio
					if allocations > 0 {
						value = f.Input.Background
					}
					if call.Free {
						value = 0xa5a50000 + uint32(allocations)
					}
					allocations++
					if value != want.Value {
						return NativeStartupAllocationResult{}, fmt.Errorf("declared allocation result differs")
					}
					calls++
					return NativeStartupAllocationResult{Complete: true, Value: value}, nil
				}, Call: func(call NativeFileFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
					if calls >= len(f.Calls) {
						return NativeCommandFrameResult{}, fmt.Errorf("extra resource call")
					}
					want := f.Calls[calls]
					if want.Kind != "resource" || call.Routine != 0x19cd0 || call.Frame.D != want.D || call.A[0].Address != want.A0 || call.A[6].Address != want.A6 {
						return NativeCommandFrameResult{}, fmt.Errorf("native allocation resource inputs differ")
					}
					if pending && *phase == 0 {
						*phase = 1
						return NativeCommandFrameResult{}, nil
					}
					*phase = 0
					call.Frame.D[0] = uint32(uint16(call.Frame.D[0])&1) + 1
					calls++
					return NativeCommandFrameResult{Complete: true}, nil
				}}
				complete, err := state.Advance(int(f.Input.Routine), cb)
				for tries := 0; err == nil && !complete; tries++ {
					if tries > 8 {
						t.Fatal("allocation child did not resume")
					}
					complete, err = state.Advance(int(f.Input.Routine), cb)
				}
				if err != nil || !complete || frame.D != f.D || fileFrameHash(raw) != f.BSSHash || fileFrameHash(code) != f.CodeHash || calls != len(f.Calls) {
					t.Fatal("native allocation frame differs", complete, err, frame.D, f.D, calls, len(f.Calls))
				}
				for i, a := range state.A {
					if a.Address != f.A[i] {
						t.Fatal("native allocation address register differs", i, a.Address, f.A[i])
					}
				}
			})
		}
	}
}
