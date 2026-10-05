package populous2

import "testing"

func TestNativeRuntimeStartupRestoresActualPanelBeforeAudioBoundary(t *testing.T) {
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
	panel, err := DecodeNativeStartupPanelFrameRules(h.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	state := NativeStartupHostFrameState{Startup: NativeStartupResetFrameState{Entry: 0x10ad8}}
	ownership := 0
	pending := 0
	cb, err := h.StartupCallbacks(&c, NativeStartupHostFrameCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Call: func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
		switch call.Routine {
		case 0x1da0:
			err := h.RestoreStartupPanel(&panel, call.Frame, call.A, func(_ bool, frame *NativeFrameRegisterContext, a *[7]NativeRequesterAddress) error {
				ownership++
				// This is a portable synchronous host ownership operation.
				// The source wrapper must preserve all D/A even if it clobbers.
				frame.D[0] ^= 0xff00
				a[6].Address ^= 0x100
				return nil
			})
			return NativeCommandFrameResult{Complete: err == nil}, err
		case 0x18474:
			pending++
			*phase++
			return NativeCommandFrameResult{}, nil
		}
		t.Fatalf("unexpected unresolved startup body%x", call.Routine)
		return NativeCommandFrameResult{}, nil
	}}}, NativeErrorFrameCallbacks{})
	if err != nil {
		t.Fatal(err)
	}
	step, err := state.Advance(&rules, cb)
	if err != nil || step.Complete || !step.Waiting || step.FlagsKnown || ownership == 0 || pending != 1 {
		t.Fatal("real startup panel failed or audio was falsely acknowledged", step, ownership, pending, err)
	}
	before, err := h.Memory.SnapshotBSS()
	if err != nil {
		t.Fatal(err)
	}
	step, err = state.Advance(&rules, cb)
	if err != nil || step.Complete || !step.Waiting || pending != 2 {
		t.Fatal("startup audio wait did not resume", step, pending, err)
	}
	after, err := h.Memory.SnapshotBSS()
	if err != nil || fileFrameHash(before) != fileFrameHash(after) {
		t.Fatal("audio wait restarted world/panel initialization", err)
	}
}
