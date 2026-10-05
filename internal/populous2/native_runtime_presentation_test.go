package populous2

import "testing"

func TestNativeRuntimePresentationFieldsRemainLiveThroughPhysicalCallbacks(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	if _, err := h.InitializePresentation(NativeMouseSample{}); err != nil {
		t.Fatal(err)
	}
	p := h.Session.Presentation
	for _, field := range []struct {
		offset int
		value  uint32
	}{{0x77a, p.CopperSelector}, {0x77e, p.SpritePatchPointer}, {0x1117c, p.Deadline1117C}} {
		if got, err := h.Memory.RAM.Read32(0x100000 + field.offset); err != nil || got != field.value {
			t.Fatal("initialized live presentation field detached", field, got, err)
		}
	}
	frame := NativeFrameRegisterContext{AddressBase: 0x200000}
	if _, err := p.Swap(&frame); err != nil {
		t.Fatal(err)
	}
	if got, err := h.Memory.Code.Read32(0x77e); err != nil || got != p.SpritePatchPointer {
		t.Fatal("swap changed a detached CODE field", got, err)
	}
	if err := h.Memory.Code.Write32(0x1117c, 0x12345678); err != nil {
		t.Fatal(err)
	}
	if p.Deadline1117C != 0x12345678 {
		t.Fatal("source clock write did not reach presentation")
	}
	p.Deadline1117C = 0xabcdef01
	if got, err := h.Memory.RAM.Read32(0x11117c); err != nil || got != 0xabcdef01 {
		t.Fatal("presentation clock mutation did not reach physical view", got, err)
	}
	if err := h.Memory.Code.Write16(0x3ea, 0x8000); err != nil {
		t.Fatal(err)
	}
	if !p.InterruptChain || h.PresentationCode.InterruptWord() != 0x8000 {
		t.Fatal("interrupt WORD normalized")
	}
	if err := h.Memory.RAM.Write16(0x100a30, 0x4567); err != nil {
		t.Fatal(err)
	}
	if p.Input.Mouse.PositionX != 0x4567 {
		t.Fatal("presentation overlay hid underlying mouse owner")
	}
	if err := h.Memory.RAM.Write32(0x200f40, 0x11223344); err != nil {
		t.Fatal(err)
	}
	if got, err := h.Memory.BSS.Read32(0xf40); err != nil || got != 0x11223344 {
		t.Fatal("presentation overlay hid underlying World owner", got, err)
	}
}
