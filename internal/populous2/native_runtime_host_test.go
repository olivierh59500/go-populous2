package populous2

import (
	"bytes"
	"io/fs"
	"os"
	"testing"

	embedded "go-populous2/assets"
)

func nativeRuntimeHostTest(t *testing.T) *NativeRuntimeHost {
	t.Helper()
	files, err := fs.Sub(embedded.Files, "amiga")
	if err != nil {
		t.Fatal(err)
	}
	low := make([]byte, 256)
	low[4] = 0
	low[5] = 0xd0
	host, err := NewNativeRuntimeHost(testBundle(t), files, NativeRuntimeHostConfig{HunkBases: []uint32{0x100000, 0x200000, 0x300000, 0x400000, 0x500000, 0x600000}, Regions: []NativeHostRegion{{Name: "explicit low vector RAM", Base: 0, Bytes: low}}, AllocationStart: 0x700000, AllocationLimit: 0x800000})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := host.Close(); err != nil {
			t.Error(err)
		}
	})
	return host
}

func TestNativeRuntimeHostHasOneOwnerAndNoImplicitStartup(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	code, err := h.Host.Span(0x100000, int(h.Bundle.Executable.Hunks[0].AllocatedBytes))
	if err != nil {
		t.Fatal(err)
	}
	if &code[0] != &h.World.NativeAI.Code[0] || &code[0] != &h.Code.Bytes[0] {
		t.Fatal("CODE has detached owners")
	}
	initial, err := h.Memory.SnapshotBSS()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(initial, make([]byte, 0x11280)) {
		t.Fatal("typed prototype overwrote zero-initialized HUNK1")
	}
	if h.Session.Presentation.ActiveCopper != 0 || len(h.Allocator.owned) != 0 {
		t.Fatal("constructor claimed source startup")
	}
	if _, err := h.Memory.RAM.Read8(0x220000); err == nil {
		t.Fatal("host padded missing native RAM")
	}
	if err := h.Memory.Code.Write16(0xa30, 0x1234); err != nil {
		t.Fatal(err)
	}
	if h.Session.Presentation.Input.Mouse.PositionX != 0x1234 {
		t.Fatal("physical cursor does not share input")
	}
	if err := h.Code.Logical().Write32(0x33616, 0xe17a); err != nil {
		t.Fatal(err)
	}
	if got, err := h.Memory.Code.Read32(0x33616); err != nil || got != 0x10e17a {
		t.Fatal("logical pointer is detached from runtime physical operand", got, err)
	}
	writes, err := h.InitializePresentation(NativeMouseSample{})
	if err != nil || len(writes) != 4 {
		t.Fatal("actual Copper builders failed", len(writes), err)
	}
	bitmap, err := h.Session.Presentation.BackBuffer()
	if err != nil {
		t.Fatal(err)
	}
	address, err := h.Memory.BSS.Read32(0x1e)
	if err != nil {
		t.Fatal(err)
	}
	physical, err := h.Bitmap(address)
	if err != nil || &bitmap[0] != &physical[0] {
		t.Fatal("physical loader and renderer screen allocations differ", err)
	}
}

func TestNativeRuntimeHostAllocationsUseRealLoaderAndLiveBSS(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	frame := NativeFrameRegisterContext{AddressBase: 0x200000, D: [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}}
	complete, err := h.AdvanceAllocations(0x1a43e, &frame, NativeErrorFrameCallbacks{})
	if err != nil || !complete || frame.D[0] != 1 {
		t.Fatal("native host startup allocation/load failed", complete, frame.D, err)
	}
	audio, err := h.Memory.BSS.Read32(0x3b4)
	if err != nil {
		t.Fatal(err)
	}
	background, err := h.Memory.BSS.Read32(0xdbe)
	if err != nil || audio != 0x700000 || background != 0x700000+NativeStartupAudioBytes {
		t.Fatal("live source pointers differ", audio, background, err)
	}
	fx, err := h.Host.Span(audio, NativeStartupAudioBytes)
	if err != nil {
		t.Fatal(err)
	}
	qaz, err := h.Bitmap(background)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(fx[:len(h.Bundle.Raw["fx.dat"])], h.Bundle.Raw["fx.dat"]) || !bytes.Equal(qaz, h.Bundle.Raw["qaz.pak"]) {
		t.Fatal("host load did not decode original disk bytes")
	}
	if pointer, err := h.Memory.Code.Read32(0x19e4a + 13*48); err != nil || pointer != audio {
		t.Fatal("FX descriptor did not share relocated CODE", pointer, err)
	}
	if len(h.Files.handles) != 0 {
		t.Fatal("host resource load leaked handles")
	}
	complete, err = h.AdvanceAllocations(0x1a4bc, &frame, NativeErrorFrameCallbacks{})
	if err != nil || !complete || len(h.Allocator.owned) != 0 {
		t.Fatal("actual source release failed", complete, err)
	}
	if _, err := h.Host.Span(audio, 1); err == nil {
		t.Fatal("released audio is still mapped")
	}
	if _, err := h.Bitmap(background); err == nil {
		t.Fatal("released background is still mapped")
	}
}

func TestNativeRuntimeHostRequiresOriginalLayoutAndExplicitAllocationRange(t *testing.T) {
	if _, err := NewNativeRuntimeHost(nil, nil, NativeRuntimeHostConfig{}); err == nil {
		t.Fatal("missing source layout accepted")
	}
	files, err := fs.Sub(embedded.Files, "amiga")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewNativeRuntimeHost(testBundle(t), files, NativeRuntimeHostConfig{HunkBases: []uint32{0x100000, 0x200000, 0x300000, 0x400000, 0x500000, 0x600000}}); err == nil {
		t.Fatal("unconfigured host allocation range accepted")
	}
}

type nativeRuntimeMissingFX struct{ fs.FS }

func (f nativeRuntimeMissingFX) Open(name string) (fs.File, error) {
	if name == "fx.dat" {
		return nil, os.ErrNotExist
	}
	return f.FS.Open(name)
}

func TestNativeRuntimeHostResourceFailureRetainsAllocation(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	files, err := fs.Sub(embedded.Files, "amiga")
	if err != nil {
		t.Fatal(err)
	}
	disk, err := NewNativeResourceFilesystem(nativeRuntimeMissingFX{FS: files})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Files.Close(); err != nil {
		t.Fatal(err)
	}
	h.Files = disk
	if _, err := h.InitializePresentation(NativeMouseSample{}); err != nil {
		t.Fatal(err)
	}
	frame := NativeFrameRegisterContext{AddressBase: 0x200000}
	complete, err := h.AdvanceAllocations(0x1a43e, &frame, NativeErrorFrameCallbacks{})
	if err != nil || complete {
		t.Fatal("missing FX was acknowledged as a completed startup", complete, err)
	}
	if h.allocationResource == nil || !h.allocationResource.ErrorActive || len(h.Allocator.owned) != 1 {
		t.Fatal("loader error did not retain its actual requester/allocation")
	}
	next := h.Allocator.Next
	complete, err = h.AdvanceAllocations(0x1a43e, &frame, NativeErrorFrameCallbacks{})
	if err != nil || complete || h.Allocator.Next != next || len(h.Allocator.owned) != 1 {
		t.Fatal("resumed requester repeated an allocation or completed falsely", complete, err)
	}
}
