package populous2

import "testing"

func TestNativeRuntimeInterruptVectorsShareLowRAMAndRestoreOriginalHandlers(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	for _, v := range []struct {
		at    int
		value uint32
	}{{0x14, 0x00c01234}, {0x6c, 0x00d05678}} {
		if err := h.Memory.RAM.Write32(v.at, v.value); err != nil {
			t.Fatal(err)
		}
	}
	frame := NativeFrameRegisterContext{AddressBase: h.Memory.BSSBase, D: [8]uint32{0x12345678, 2, 3, 4, 5, 6, 7, 8}}
	step, err := h.InterruptVectors(0x39e, &frame, func(at uint32) (uint16, error) {
		if at != 0xdff01c {
			t.Fatal("interrupt status read at the wrong hardware address", at)
		}
		return 0x4567, nil
	}, func(NativeFrameHardwareWrite) error { return nil })
	if err != nil || !step.Complete || len(step.Hardware) != 2 || frame.D[0] != 0x12348060 {
		t.Fatal(step, frame.D, err)
	}
	for _, v := range []struct {
		at    int
		value uint32
	}{{0x14, h.Memory.CodeBase + 0x43e}, {0x6c, h.Memory.CodeBase + 0x3ec}} {
		if got, err := h.Memory.RAM.Read32(v.at); err != nil || got != v.value {
			t.Fatal("active vector detached", got, err)
		}
	}
	if got, err := h.Memory.BSS.Read32(2); err != nil || got != 0x00c01234 {
		t.Fatal("saved divide handler detached", got, err)
	}
	if got, err := h.Memory.BSS.Read32(6); err != nil || got != 0x00d05678 {
		t.Fatal("saved interrupt handler detached", got, err)
	}
	frame.D[0], frame.D[1] = 17, 19
	if err := nativeResultScoreDivide(h, &frame); err != nil {
		t.Fatal(err)
	}
	frame.D[0], frame.D[1] = 17, 0x20000
	if err := nativeResultScoreDivide(h, &frame); err != nil || frame.D[0] != 17 {
		t.Fatal("installed RTE did not preserve the dividend", frame.D, err)
	}
	if _, err := h.InterruptVectors(0x370, &frame, nil, nil); err != nil {
		t.Fatal(err)
	}
	for _, v := range []struct {
		at    int
		value uint32
	}{{0x14, 0x00c01234}, {0x6c, 0x00d05678}} {
		if got, err := h.Memory.RAM.Read32(v.at); err != nil || got != v.value {
			t.Fatal("original vector not restored", got, err)
		}
	}
	if err := nativeResultScoreDivide(h, &frame); err == nil {
		t.Fatal("result assumed an exception handler after restoring the original vector")
	}
}
