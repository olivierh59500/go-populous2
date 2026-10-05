package populous2

import (
	"bytes"
	"io/fs"
	"testing"

	embedded "go-populous2/assets"
)

func TestNativeHostStartupAllocationsLoadActualFXAndBackground(t *testing.T) {
	bundle := testBundle(t)
	memory := nativeHostTestMemory(t)
	ram := memory.Memory()
	if err := memory.MapRegion(NativeHostRegion{Name: "configured low vector RAM", Base: 0, Bytes: make([]byte, 256)}); err != nil {
		t.Fatal(err)
	}
	_ = ram.Write32(4, 0xd00000)
	allocator, err := NewNativeHostAllocator(memory, 0x700000, 0x800000)
	if err != nil {
		t.Fatal(err)
	}
	files, err := fs.Sub(embedded.Files, "amiga")
	if err != nil {
		t.Fatal(err)
	}
	disk, err := NewNativeResourceFilesystem(files)
	if err != nil {
		t.Fatal(err)
	}
	defer disk.Close()
	rules, err := DecodeNativeResourceFrameRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	frame := NativeFrameRegisterContext{D: [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}, AddressBase: 0x200000}
	input, err := NewNativeInputState(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	relative := func(base uint32) FollowerCleanupMemory {
		m := ram
		m.Read8 = func(at int) (uint8, error) { return ram.Read8(int(base) + at) }
		m.Write8 = func(at int, value uint8) error { return ram.Write8(int(base)+at, value) }
		m.Read16 = func(at int) (uint16, error) { return ram.Read16(int(base) + at) }
		m.Write16 = func(at int, value uint16) error { return ram.Write16(int(base)+at, value) }
		m.Read32 = func(at int) (uint32, error) { return ram.Read32(int(base) + at) }
		m.Write32 = func(at int, value uint32) error { return ram.Write32(int(base)+at, value) }
		return m
	}
	resource := NativeResourceFrameState{}
	state := NativeStartupAllocationState{}
	cb := NativeStartupAllocationCallbacks{Code: relative(0x100000), Memory: relative(0x200000), CodeBase: 0x100000, Frame: &frame, ReadExecBase: func() (uint32, error) { return ram.Read32(4) }, Allocate: allocator.Allocate, Call: func(call NativeFileFrameCall, _ *uint32) (NativeCommandFrameResult, error) {
		if resource.Finished {
			resource = NativeResourceFrameState{}
		}
		step, err := resource.Advance(&rules, NativeResourceFrameCallbacks{RAM: ram, CodeBase: 0x100000, Frame: call.Frame, Input: &input, IO: disk.IO})
		return NativeCommandFrameResult{Complete: step.Complete}, err
	}}
	complete, err := state.Advance(0x1a43e, cb)
	if err != nil || !complete || frame.D[0] != 1 {
		t.Fatal("native allocated startup load failed", complete, err, frame.D)
	}
	audio, _ := ram.Read32(0x2003b4)
	background, _ := ram.Read32(0x200dbe)
	if audio != 0x700000 || background != audio+NativeStartupAudioBytes || len(allocator.owned) != 2 {
		t.Fatal("configured host allocations differ", audio, background)
	}
	fx, err := memory.Span(audio, NativeStartupAudioBytes)
	if err != nil {
		t.Fatal(err)
	}
	qaz, err := memory.Span(background, NativeStartupBackgroundBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(fx[:len(bundle.Raw["fx.dat"])], bundle.Raw["fx.dat"]) || !bytes.Equal(qaz, bundle.Raw["qaz.pak"]) {
		t.Fatal("actual FX/QAZ resources did not populate their real allocations")
	}
	if pointer, _ := ram.Read32(0x100000 + 0x19e4a + 13*48); pointer != audio {
		t.Fatal("native FX descriptor does not use allocated RAM")
	}
	if pointer, _ := ram.Read32(0x100000 + 0x19e4a + 14*48); pointer != background {
		t.Fatal("native QAZ descriptor does not use allocated RAM")
	}
	if len(disk.handles) != 0 {
		t.Fatal("native allocated load leaked host handles")
	}
	state = NativeStartupAllocationState{}
	complete, err = state.Advance(0x1a4bc, cb)
	if err != nil || !complete || len(allocator.owned) != 0 {
		t.Fatal("native startup release failed", complete, err)
	}
	if _, err := memory.Span(audio, 1); err == nil {
		t.Fatal("released audio allocation remained mapped")
	}
	if _, err := memory.Span(background, 1); err == nil {
		t.Fatal("released backdrop allocation remained mapped")
	}
}

func TestNativeHostAllocatorExhaustionIsNativeZero(t *testing.T) {
	m := &NativeHostMemory{}
	a, err := NewNativeHostAllocator(m, 0x1000, 0x1010)
	if err != nil {
		t.Fatal(err)
	}
	result, err := a.Allocate(NativeStartupAllocationCall{Size: 16, Flags: 2}, nil)
	if err != nil || result.Value != 0x1000 {
		t.Fatal(result, err)
	}
	result, err = a.Allocate(NativeStartupAllocationCall{Size: 2, Flags: 2}, nil)
	if err != nil || result.Value != 0 || !result.Complete {
		t.Fatal("allocation exhaustion changed native result", result, err)
	}
	if _, err := a.Allocate(NativeStartupAllocationCall{Free: true, Address: 0x1000, Size: 15}, nil); err == nil {
		t.Fatal("wrong free size accepted")
	}
}
