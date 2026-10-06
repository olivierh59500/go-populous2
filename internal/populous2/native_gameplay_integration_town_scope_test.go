package populous2

import (
	"errors"
	"testing"
)

func TestNativeFrameSessionRestoresPreviousTownEvaluator(t *testing.T) {
	for _, failure := range []error{nil, errors.New("retained native child failed")} {
		h, _, frame := nativeGameplayIntegrationStartup(t, 0, 4311)
		calls := 0
		h.World.nativeTownEvaluation = func(NativeRecordReference) (int, error) { calls++; return 42, nil }
		if err := h.Session.BeginRaw(h.World, frame); err != nil {
			t.Fatal(err)
		}
		if h.Session.previousTownEvaluation == nil {
			t.Fatal("previous owner was discarded")
		}
		h.Session.finish(failure)
		stage, err := h.World.evaluateNativeTown(52)
		if err != nil || stage != 42 || calls != 1 || h.World.nativeCallDepth != 0 || h.Session.previousTownEvaluation != nil {
			t.Fatal("finish did not restore caller's evaluator", stage, calls, err)
		}
	}
}

func TestNativeFXColorWritePrecedesActiveNoDrawDispatch(t *testing.T) {
	b := testBundle(t)
	rules, err := DecodeNativeCommandRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	raw := make([]byte, 0x11280)
	m := commandFrameBacking(raw)
	_ = m.Write8(0xc80c, 1) // Active owner with the native state0 no-draw tail.
	frame := NativeFrameRegisterContext{D: [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}}
	wrote, called := false, false
	err = rules.TickFrameFX(&frame, NativeFrameFXCallbacks{Memory: m, WriteCode16: func(at int, value uint16) error {
		if at != 0x15b7a || value != 5 || frame.D != [8]uint32{1, 2, 3, 4, 5, 6, 7, 8} {
			t.Fatal("1483C color write moved past register assignment")
		}
		wrote = true
		return nil
	}, Tick: func(ref NativeRecordReference, c *NativeFrameRegisterContext) (NativeFrameFXStep, error) {
		if !wrote || ref != 0x5140 {
			t.Fatal("active actor dispatched before source color write")
		}
		called = true
		return NativeFrameFXStep{}, nil
	}})
	if err != nil || !wrote || !called {
		t.Fatal("source active no-draw color write omitted", err)
	}
}

// nativeGameplayEnterProtectionAnswer supplies the real source UI events
// needed when a newly formed town reaches314A. It edits the displayed digits
// and submits them; only3342/fade/resource reload may complete the requester.
func nativeGameplayEnterProtectionAnswer(t *testing.T, h *NativeRuntimeHost, frame *NativeRuntimeFrame) {
	t.Helper()
	children := frame.RenderChildren
	if children == nil || !children.actorActive {
		return
	}
	advanceBlank := func() {
		p := &h.Session.Presentation.Input
		if _, err := h.Session.Presentation.VBlank(NativeMouseSample{CounterX: uint8(p.Mouse.CounterX), CounterY: uint8(p.Mouse.CounterY)}, h.Memory.BSS, &h.Session.Frame); err != nil {
			t.Fatal(err)
		}
		if _, err := frame.Advance(); err != nil {
			t.Fatal(err)
		}
	}
	for n := 0; children.Protection.PC != 0x31ce; n++ {
		if n > 40 {
			t.Fatalf("protection did not reach actual digit input:%x", children.Protection.PC)
		}
		advanceBlank()
	}
	selection := uint16(children.Protection.selected) & 0x1ff
	value := selection * 7
	answer := value + (value >> 1)
	digits := [4]byte{byte(answer/1000%10) + '0', byte(answer/100%10) + '0', byte(answer/10%10) + '0', byte(answer%10) + '0'}
	code := h.Memory.Code
	start, err := code.Read16(0xab4e)
	if err != nil {
		t.Fatal(err)
	}
	digitAt := 0xab4e + int(int16(start)) + 0x2f3
	click := func(action int) {
		nativeRuntimeClickAction(t, h, &h.Session.Frame, action)
		if _, err := frame.Advance(); err != nil {
			t.Fatal(err)
		}
		advanceBlank()
	}
	actionFor := func(target int) int {
		for action := 2; action <= 18; action += 2 {
			offset, err := code.Read16(0x3258 + action)
			if err != nil {
				t.Fatal(err)
			}
			if 0x3258+int(int16(offset)) == target {
				return action
			}
		}
		t.Fatalf("native protection button target%x absent", target)
		return 0
	}
	for pos, want := range digits {
		for n := 0; ; n++ {
			got, err := code.Read8(digitAt + pos)
			if err != nil {
				t.Fatal(err)
			}
			if got == want {
				break
			}
			if n >= 10 {
				t.Fatalf("native digit%d did not advance", pos)
			}
			click(actionFor(0x327c - pos*4))
		}
	}
	click(actionFor(0x32b0))
	for n := 0; !children.Protection.Finished; n++ {
		if n > 40 {
			t.Fatalf("actual protection fade/reload did not finish:%x", children.Protection.PC)
		}
		advanceBlank()
	}
	if flag, err := h.Memory.BSS.Read16(0x3b8); err != nil || flag != 1 {
		t.Fatal("source challenge did not acknowledge the submitted answer", flag, err)
	}
}
