package populous2

import (
	"io"
	"sync"
	"testing"
)

func TestNativeRuntimePCMUsesHostOwnerForFrameAndAudio(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	var stream *NativeRuntimePCMAccess
	err := h.Access.Execute(func() error {
		frame := NativeFrameRegisterContext{AddressBase: 0x200000}
		if done, err := h.AdvanceAllocations(0x1a43e, &frame, NativeErrorFrameCallbacks{}); err != nil || !done {
			t.Fatalf("source allocations failed:%v/%v", done, err)
		}
		device, _, err := h.InitializeAudio(&frame, 0)
		if err != nil {
			return err
		}
		stream, err = h.CreatePCM(device, NativePALAudioTiming(44100, 0), NativePALPaulaDMAConfig(0))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	operations, err := stream.WithinExecuteCallbacks()
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	var failure [2]error
	workers.Add(2)
	go func() {
		defer workers.Done()
		data := make([]byte, 400*4)
		for i := 0; i < 8; i++ {
			if _, err := io.ReadFull(stream, data); err != nil {
				failure[0] = err
				return
			}
		}
	}()
	go func() {
		defer workers.Done()
		for i := 0; i < 8; i++ {
			failure[1] = h.Access.Execute(func() error {
				if err := h.Memory.BSS.Write32(0xf40, uint32(i)); err != nil {
					return err
				}
				_, err := operations.MusicCommand(0x2006, 32, 0)
				return err
			})
			if failure[1] != nil {
				return
			}
		}
	}()
	workers.Wait()
	for _, err := range failure {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := h.Access.Execute(func() error {
		if value, err := h.Memory.BSS.Read32(0xf40); err != nil || value != 7 {
			t.Fatal("owned frame mutations lost", value, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
