package populous2

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeRuntimeScreenExportPreservesOriginalRowsAndCaller(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	runtimeFilesPrelude(t, h)
	root := t.TempDir()
	export, err := NewNativeRuntimeScreenExport(root, false)
	if err != nil {
		t.Fatal(err)
	}
	defer export.Close()
	at, err := h.Memory.BSS.Read32(0x1a)
	if err != nil {
		t.Fatal(err)
	}
	bitmap, err := h.Bitmap(at)
	if err != nil {
		t.Fatal(err)
	}
	for i := range bitmap {
		bitmap[i] = byte(i*17 + 11)
	}
	c := NativeFrameRegisterContext{AddressBase: h.Memory.BSSBase, D: [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}}
	var a [7]NativeRequesterAddress
	for i := range a {
		a[i] = NativeRequesterAddress{Address: 0x900000 + uint32(i)*0x1000, Absolute: true}
	}
	savedD, savedA := c.D, a
	for visit := 0; visit < 2; visit++ {
		phase := uint32(0)
		step, err := export.AdvanceChild(h, NativeStartupResetFrameCall{Routine: 0x1a55a, Frame: &c, A: &a}, &phase)
		if err != nil || !step.Complete || c.D != savedD || a != savedA || export.State != nil {
			t.Fatal("native export lost caller continuation", step, c.D, err)
		}
		name := []byte("POPA.SCR")
		name[3] += byte(visit)
		payload, err := os.ReadFile(filepath.Join(root, string(name)))
		if err != nil {
			t.Fatal(err)
		}
		if len(payload) != 32104 || string(payload[:4]) != "FORM" || string(payload[8:12]) != "ILBM" {
			t.Fatal("native export file header differs", len(payload))
		}
		expected := make([]byte, 0, 32000)
		for y := 0; y < 200; y++ {
			for plane := 0; plane < 4; plane++ {
				start := plane*8000 + y*40
				expected = append(expected, bitmap[start:start+40]...)
			}
		}
		if !bytes.Equal(payload[104:], expected) {
			t.Fatal("native row/plane ordering changed")
		}
	}
	if _, err := NewNativeRuntimeScreenExport(filepath.Join(root, "missing"), false); err == nil {
		t.Fatal("export silently created an unconfigured directory")
	}
	if _, err := export.Files.Resolve("../outside.SCR"); err == nil {
		t.Fatal("export resolved outside its selected root")
	}
}
