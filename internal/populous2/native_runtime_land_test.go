package populous2

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestNativeRuntimeLANDReloadRetainsPhysicalCodeOwner(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	owner := &h.World.NativeAI.Code[0]
	original := append([]byte(nil), h.Bundle.NativeAI.Code...)
	for _, name := range []string{"land0.dat", "land1.dat", "land2.dat", "land3.dat", "land0.dat"} {
		if err := h.World.retainNativeLAND(h.Bundle.Raw[name]); err != nil {
			t.Fatal(err)
		}
		if &h.World.NativeAI.Code[0] != owner {
			t.Fatal("LAND reload detached the physical CODE owner")
		}
		if !bytes.Equal(h.Code.RawData()[0x3365a:0x33886], h.Bundle.Raw[name]) {
			t.Fatal("physical loader view retained a stale LAND", name)
		}
	}
	if !bytes.Equal(h.Bundle.NativeAI.Code, original) {
		t.Fatal("host LAND reload modified the immutable Bundle")
	}
}

func TestNativeRuntimeSessionMinimapUsesRelocatedProcedure(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	if _, err := h.InitializePresentation(NativeMouseSample{}); err != nil {
		t.Fatal(err)
	}
	if err := h.World.retainNativeLAND(h.Bundle.Raw["land0.dat"]); err != nil {
		t.Fatal(err)
	}
	logical := h.Code.Logical()
	if err := logical.Write32(0x33616, 0xe17a); err != nil {
		t.Fatal(err)
	}
	if err := h.Memory.Code.Write16(0x33612, 96); err != nil {
		t.Fatal(err)
	}
	if err := h.Memory.Code.Write16(0x33614, 70); err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			if err := h.Memory.BSS.Write8(0xf45+(x+y*64)*4, byte(x*17+y*43+5)); err != nil {
				t.Fatal(err)
			}
		}
	}
	d := [8]uint32{0x12345678, 0x89abcdef, 0x13572468, 0x24681357, 0xaabbccdd, 0x11226778, 0x12345678, 0x33445566}
	if err := h.Session.BeginRaw(h.World, NativeFrameRegisterContext{D: d, AddressBase: 0x200000}); err != nil {
		t.Fatal(err)
	}
	defer h.Session.finish(nil)
	address, err := h.Memory.BSS.Read32(0x1e)
	if err != nil {
		t.Fatal(err)
	}
	bitmap, err := h.Bitmap(address)
	if err != nil {
		t.Fatal(err)
	}
	for i := range bitmap {
		bitmap[i] = byte(i*7 + 13)
	}
	expected := append([]byte(nil), bitmap...)
	legacyCode := append([]byte(nil), h.World.NativeAI.Code...)
	binary.BigEndian.PutUint32(legacyCode[0x33616:], 0xe17a)
	frame := NativeFrameRegisterContext{D: d, AddressBase: 0x200000}
	if err := DrawNativeMinimapFrame(legacyCode, h.Memory.BSS, &frame, expected); err != nil {
		t.Fatal(err)
	}
	command := NativeCommandRegisterContext{D: d}
	phase := uint32(0)
	step, err := h.Session.AdvanceBuiltInCommandChild(NativeCommandFrameCall{NativeCommandCall: NativeCommandCall{Routine: 0xd8cc, Caller: 0xeb56, Context: &command}, TargetA0: address}, &phase)
	if err != nil || !step.Complete || command.D != frame.D || !bytes.Equal(bitmap, expected) {
		t.Fatal("runtime minimap command differs from proven original-body implementation", step, err)
	}
	if procedure, err := h.Memory.Code.Read32(0x33616); err != nil || procedure != 0x10e17a {
		t.Fatal("minimap normalized the physical procedure operand", procedure, err)
	}
}
