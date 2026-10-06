package populous2

import (
	"io"
	"net"
	"strings"
	"testing"
)

func TestNativeRuntimeResumeMenuChildRunsLocalBodyEveryVisit(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	left, right := net.Pipe()
	conn, err := NewNativeSerialConn(left)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	defer right.Close()
	transport, err := h.NewTransport(conn, NativeTransportFrameCallbacks{}, func(bool, *NativeFrameRegisterContext) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Memory.BSS.Write32(0xeb6a, h.Memory.BSSBase+0xeb56); err != nil {
		t.Fatal(err)
	}
	if err := h.Memory.BSS.Write8(0xeb5e, 2); err != nil {
		t.Fatal(err)
	}
	for visit := 0; visit < 2; visit++ {
		frame := NativeFrameRegisterContext{AddressBase: h.Memory.BSSBase, D: [8]uint32{0xffcc0001, uint32(visit), 2, 3, 4, 5, 6, 7}}
		original := frame.D
		phase := uint32(0)
		result, err := transport.ResumeMenuChild(NativeFileFrameCall{Routine: 0x181c0, Frame: &frame}, &phase)
		if err != nil || !result.Complete || !result.Zero || frame.D[0] != 0 {
			t.Fatal("local menu did not execute the real immediate source return", result, frame.D, err)
		}
		for register := 1; register < len(frame.D); register++ {
			if frame.D[register] != original[register] {
				t.Fatal("local resume damaged a caller register", register)
			}
		}
	}
}

func TestNativeRuntimeResumeMenuChildDoesNotAcknowledgeCorruptSourceReturn(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	if _, err := h.InitializePresentation(NativeMouseSample{}); err != nil {
		t.Fatal(err)
	}
	left, right := net.Pipe()
	conn, err := NewNativeSerialConn(left)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	defer right.Close()
	transport, err := h.NewTransport(conn, NativeTransportFrameCallbacks{}, func(bool, *NativeFrameRegisterContext) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	for _, patch := range []nativeHeroPatch{{0xeb6a, 4, h.Memory.BSSBase + 0xeb56}, {0xeb42, 2, 1}, {0xe90c, 2, 0x1234}, {0xeb44, 2, 6}, {0xeb5e, 1, 6}, {0xeb68, 1, 6}, {0xeb72, 1, 6}} {
		renderFramePatch(h.Memory.BSS, patch)
	}
	written := 0
	transport.Callbacks.Port = NativeSerialPort{
		Available: func() (int, error) { return 0, io.EOF },
		Read:      func([]byte) (int, error) { t.Fatal("source read after failed availability"); return 0, nil },
		Write:     func(data []byte) (int, error) { written += len(data); return len(data), nil },
		Flush:     func() error { return nil },
	}
	frame := NativeFrameRegisterContext{AddressBase: h.Memory.BSSBase}
	phase := uint32(0)
	result, err := transport.ResumeMenuChild(NativeFileFrameCall{Routine: 0x181c0, Frame: &frame}, &phase)
	if result.Complete || err == nil || !strings.Contains(err.Error(), "00011234") || written != 10 || frame.D[0] != 1 {
		t.Fatal("corrupt source RTS was replaced by a successful menu return", result, frame.D, written, err)
	}
}
