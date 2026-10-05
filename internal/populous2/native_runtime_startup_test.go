package populous2

import "testing"

func TestNativeRuntimeStartupBindsRealBodiesAndRetainsPanelBoundary(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	if _, err := h.InitializePresentation(NativeMouseSample{}); err != nil {
		t.Fatal(err)
	}
	c := NativeFrameRegisterContext{AddressBase: 0x200000}
	if done, err := h.AdvanceAllocations(0x1a43e, &c, NativeErrorFrameCallbacks{}); err != nil || !done {
		t.Fatal(done, err)
	}
	for _, patch := range []struct {
		at    int
		value uint16
	}{{0xeb44, 4}, {0xeb42, 1}, {0xeb46, 0}, {0xeb22, 0}} {
		if err := h.Memory.BSS.Write16(patch.at, patch.value); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.Memory.BSS.Write32(0x3ac, 0xffffffff); err != nil {
		t.Fatal(err)
	}
	background, err := h.Memory.BSS.Read32(0xdbe)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Memory.BSS.Write32(0x22, background); err != nil {
		t.Fatal(err)
	}
	rules, err := DecodeNativeStartupHostFrameRules(h.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	state := NativeStartupHostFrameState{Startup: NativeStartupResetFrameState{Entry: 0x10ad8}}
	calls := 0
	supplied := NativeStartupHostFrameCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Call: func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
		if call.Routine != 0x1da0 {
			t.Fatalf("unexpected unresolved startup body%x", call.Routine)
		}
		calls++
		*phase++
		return NativeCommandFrameResult{}, nil
	}}}
	cb, err := h.StartupCallbacks(&c, supplied, NativeErrorFrameCallbacks{})
	if err != nil {
		t.Fatal(err)
	}
	step, err := state.Advance(&rules, cb)
	if err != nil || step.Complete || !step.Waiting || step.FlagsKnown || calls != 1 {
		t.Fatal("runtime startup did not retain its real panel operation", step, calls, err)
	}
	before, err := h.Memory.SnapshotBSS()
	if err != nil {
		t.Fatal(err)
	}
	step, err = state.Advance(&rules, cb)
	if err != nil || step.Complete || !step.Waiting || calls != 2 {
		t.Fatal("runtime startup resumed incorrectly", step, calls, err)
	}
	after, err := h.Memory.SnapshotBSS()
	if err != nil {
		t.Fatal(err)
	}
	if fileFrameHash(before) != fileFrameHash(after) {
		t.Fatal("pending panel restarted the source constructor")
	}
	if h.allocationResource != nil || len(h.Files.handles) != 0 {
		t.Fatal("startup resources retained completed operations")
	}
}
