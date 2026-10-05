package populous2

import (
	"io"
	"testing"
)

func TestNativeRuntimeAudioBorrowsActualStartupAllocations(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	frame := NativeFrameRegisterContext{AddressBase: 0x200000, D: [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}}
	if _, _, err := h.InitializeAudio(&frame, 0); err == nil {
		t.Fatal("unloaded native audio acknowledged")
	}
	if done, err := h.AdvanceAllocations(0x1a43e, &frame, NativeErrorFrameCallbacks{}); err != nil || !done {
		t.Fatal(done, err)
	}
	saved := frame.D
	device, writes, err := h.InitializeAudio(&frame, 0)
	if err != nil || len(writes) != 3 {
		t.Fatal("source audio initialization failed", writes, err)
	}
	if &device.Code[0] != &h.Code.RawData()[0] {
		t.Fatal("native audio has a detached CODE owner")
	}
	resource, err := h.Memory.BSS.Read32(0x3b4)
	if err != nil {
		t.Fatal(err)
	}
	fx, err := h.Host.Span(resource, NativeStartupAudioBytes)
	if err != nil || &fx[0] != &device.FX[0] {
		t.Fatal("native audio has a detached FX owner", err)
	}
	if frame.D[0] != resource+4 {
		t.Fatal("source initialization D0 differs")
	}
	for i := 1; i < 8; i++ {
		if frame.D[i] != saved[i] {
			t.Fatal("source initialization changed saved caller register", i)
		}
	}
	pcm, err := NewNativeAudioPCMWithDMA(device, NativePALAudioTiming(44100, 0), NativePALPaulaDMAConfig(0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pcm.MusicCommand(0x2006, 0, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := pcm.MusicCommand(0x2006, 32, 0); err != nil {
		t.Fatal(err)
	}
	// This bounded test uses one goroutine, which is also the host's required
	// shared-CODE ownership contract. Concurrent playback needs host locking.
	data := make([]byte, 4410*4)
	if _, err := io.ReadFull(pcm, data); err != nil {
		t.Fatal(err)
	}
	if device.long(0x18ec6) != resource+4 {
		t.Fatal("source playback replaced the allocated resource pointer")
	}
}
