package populous2

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestNativeFramePixelsMatchActualCopperBackground(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	if _, err := h.InitializePresentation(NativeMouseSample{}); err != nil {
		t.Fatal(err)
	}
	p := h.Session.Presentation
	for i := 0x408; i < len(p.Chip); i++ {
		p.Chip[i] = byte(i*7 + 13)
	}
	image, err := p.Image()
	if err != nil {
		t.Fatal(err)
	}
	got := make([]byte, 320*200*4)
	if err := p.WriteRGBA(got, false); err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			r, g, b, a := image.At(x, y).RGBA()
			off := (y*320 + x) * 4
			if got[off] != byte(r>>8) || got[off+1] != byte(g>>8) || got[off+2] != byte(b>>8) || got[off+3] != byte(a>>8) {
				t.Fatal("active Copper background differs", x, y)
			}
		}
	}
}

func TestNativeFramePixelsComposeActualAttachedCursorWithoutMutation(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	if _, err := h.InitializePresentation(NativeMouseSample{}); err != nil {
		t.Fatal(err)
	}
	p := h.Session.Presentation
	frame := NativeFrameRegisterContext{AddressBase: 0x200000}
	if _, err := p.VBlank(NativeMouseSample{CounterX: 40, CounterY: 30}, h.Memory.BSS, &frame); err != nil {
		t.Fatal(err)
	}
	// Source cursor coordinates are20,15. Install one visible attached-pair
	// data bit in actual pointer RAM; Copper owns both numeric pointers.
	address := p.PointerBase + 0x2834 + uint32(int32(int16(p.Input.Mouse.Image)))
	first := int(address-p.PointerBase) - 72
	second := int(address - p.PointerBase)
	for row := 0; row < 16; row++ {
		clear(p.PointerData[first+4+row*4 : first+8+row*4])
		clear(p.PointerData[second+4+row*4 : second+8+row*4])
	}
	binary.BigEndian.PutUint16(p.PointerData[first+4:], 0x8000)
	before := append([]byte(nil), p.PointerData...)
	background, composed := make([]byte, 320*200*4), make([]byte, 320*200*4)
	if err := p.WriteRGBA(background, false); err != nil {
		t.Fatal(err)
	}
	if err := p.WriteRGBA(composed, true); err != nil {
		t.Fatal(err)
	}
	changed := 0
	for i := 0; i < len(composed); i += 4 {
		if !bytes.Equal(background[i:i+4], composed[i:i+4]) {
			changed++
			if i/4 != 15*320+20 {
				t.Fatal("attached cursor offset differs", i/4)
			}
		}
	}
	if changed != 1 || !bytes.Equal(before, p.PointerData) {
		t.Fatal("cursor composition altered source or pixel extent", changed)
	}
}
