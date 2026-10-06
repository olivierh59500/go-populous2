package populous2

import (
	"bytes"
	"testing"
)

func TestNativeRuntimeSculptKeepsSourcePlannerScratchAndMinimap(t *testing.T) {
	var hosts [2]*NativeRuntimeHost
	for side := range hosts {
		h := nativeRuntimeHostTest(t)
		hosts[side] = h
		if _, err := h.InitializePresentation(NativeMouseSample{}); err != nil {
			t.Fatal(err)
		}
		background := make([]byte, 32000)
		if err := h.Host.MapRegion(NativeHostRegion{Name: "actual source background", Base: 0xa10000, Bytes: background}); err != nil {
			t.Fatal(err)
		}
		if err := h.Memory.BSS.Write32(0x22, 0xa10000); err != nil {
			t.Fatal(err)
		}
		for cell := 0; cell < 4096; cell++ {
			if err := h.Memory.BSS.Write8(0xf44+cell*4, 3); err != nil {
				t.Fatal(err)
			}
			if err := h.Memory.BSS.Write8(0xf45+cell*4, 15); err != nil {
				t.Fatal(err)
			}
		}
		if err := h.Session.BeginRaw(h.World, NativeFrameRegisterContext{AddressBase: 0x200000}); err != nil {
			t.Fatal(err)
		}
		h.Session.bitmapResolver = h.Bitmap
		t.Cleanup(func() { h.Session.finish(nil) })
	}
	input := [8]uint32{32, 32, 1, 0, 0x12345678, 0x87654321, 0x11223344, 0x55667788}
	command := NativeCommandRegisterContext{D: input}
	if _, err := hosts[0].World.commandSculpt(NativeCommandCall{Routine: 0xd80c, Context: &command}); err != nil {
		t.Fatal(err)
	}
	frame := NativeFrameRegisterContext{D: input, AddressBase: 0x200000}
	var a [7]NativeRequesterAddress
	h := hosts[1]
	step, err := RunNativeGameplayEditorInput(0xd80c, NativeGameplayEditorInputCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Code: h.Memory.Code, Memory: h.Memory.BSS, RAM: h.Memory.RAM, CodeBase: h.Memory.CodeBase, Frame: &frame}, Bitmap: h.Bitmap}, &a)
	if err != nil || !step.Complete {
		t.Fatal(step, err)
	}
	if command.D != frame.D {
		t.Fatal("runtime sculpt registers differ from actual source leaf", command.D, frame.D)
	}
	raw0, err := hosts[0].Memory.SnapshotBSS()
	if err != nil {
		t.Fatal(err)
	}
	raw1, err := hosts[1].Memory.SnapshotBSS()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw0, raw1) {
		t.Fatal("runtime sculpt BSS differs from source leaf")
	}
	if !bytes.Equal(hosts[0].Code.RawData()[0xd6bc:0xd80c], hosts[1].Code.RawData()[0xd6bc:0xd80c]) {
		t.Fatal("runtime sculpt discarded source planner scratch")
	}
	bitmap0, err := hosts[0].Bitmap(0xa10000)
	if err != nil {
		t.Fatal(err)
	}
	bitmap1, err := hosts[1].Bitmap(0xa10000)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bitmap0, bitmap1) || step.ChangedPixels == 0 {
		t.Fatal("runtime sculpt omitted native minimap updates", step.ChangedPixels)
	}
}
