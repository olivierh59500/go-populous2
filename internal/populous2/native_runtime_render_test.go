package populous2

import "testing"

func TestNativeRuntimeRenderBindingsUseActualWindowAndCode(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	if _, err := h.InitializePresentation(NativeMouseSample{}); err != nil {
		t.Fatal(err)
	}
	bindings, err := h.MainRenderBindings(NativeSessionRenderBindings{})
	if err != nil || bindings.Rules == nil || bindings.Sprites == nil || bindings.Tiles == nil {
		t.Fatal("native runtime renderer resources missing", err)
	}
	if err := bindings.Code.Write32(0x2e3a, 0x12345678); err != nil {
		t.Fatal(err)
	}
	if got, err := h.Memory.RAM.Read32(0x102e3a); err != nil || got != 0x12345678 {
		t.Fatal("renderer code is detached", got, err)
	}
	address, err := h.Memory.BSS.Read32(0x1e)
	if err != nil {
		t.Fatal(err)
	}
	window, err := bindings.Window(address)
	if err != nil {
		t.Fatal(err)
	}
	bitmap, err := h.Bitmap(address)
	if err != nil || &window.Bytes[window.BitmapOffset] != &bitmap[0] {
		t.Fatal("renderer window does not share physical screen owner", err)
	}
	if bindings.PaintingAdvance != nil || bindings.Children.TownInfoAdvance != nil || bindings.SelectedOwnership != nil {
		t.Fatal("renderer acknowledged unsupplied native operations")
	}
	if err := h.Memory.Code.Write16(0x26958, 0x1234); err != nil {
		t.Fatal(err)
	}
	if value, err := bindings.Rules.Frames.Images.word(0x26958); err != nil || value != 0x1234 {
		t.Fatal("renderer animation table detached from active CODE", value, err)
	}
	if err := h.Code.Logical().Write32(0x21626+8, 0xf0ee); err != nil {
		t.Fatal(err)
	}
	if value, err := bindings.Rules.Frames.Images.procedure(0x21626 + 8); err != nil || value != 0xf0ee {
		t.Fatal("renderer procedure did not use linked view", value, err)
	}
}
