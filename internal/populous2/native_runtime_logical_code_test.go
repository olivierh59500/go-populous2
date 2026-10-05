package populous2

import "testing"

func TestNativeRuntimeLogicalCodePreservesScalarCallbackOwners(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	logical, err := h.LogicalCode()
	if err != nil {
		t.Fatal(err)
	}
	if err := logical.Write32(0x33616, 0xe17a); err != nil {
		t.Fatal(err)
	}
	if got, err := h.Memory.Code.Read32(0x33616); err != nil || got != 0x10e17a {
		t.Fatal("logical procedure did not reach canonical physical CODE", got, err)
	}
	h.Session.Presentation.Deadline1117C = 0x12345678
	if got, err := logical.Read32(0x1117c); err != nil || got != 0x12345678 {
		t.Fatal("logical scalar read bypassed its live presentation owner", got, err)
	}
	if err := logical.Write16(0xa30, 0x4567); err != nil {
		t.Fatal(err)
	}
	if h.Session.Presentation.Input.Mouse.PositionX != 0x4567 {
		t.Fatal("logical scalar write bypassed its live mouse owner")
	}
	if err := logical.Write16(0x3ea, 0xfffe); err != nil {
		t.Fatal(err)
	}
	if h.PresentationCode.InterruptWord() != 0xfffe || !h.Session.Presentation.InterruptChain {
		t.Fatal("logical interrupt WORD lost callback ownership")
	}
	if _, err := logical.Read16(1); err == nil {
		t.Fatal("logical view accepted unaligned WORD")
	}
}
