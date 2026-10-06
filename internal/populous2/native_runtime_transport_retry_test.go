package populous2

import (
	"net"
	"testing"
	"time"
)

func TestNativeRuntimeRetryRequiresSourceFallbackAndCompletedChildren(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	left, right := net.Pipe()
	conn, err := NewNativeSerialConn(left)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	s, err := h.NewTransport(conn, NativeTransportFrameCallbacks{}, func(bool, *NativeFrameRegisterContext) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if err := right.Close(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for conn.TerminalError() == nil {
		if time.Now().After(deadline) {
			t.Fatal("actual EOF missing")
		}
		time.Sleep(100 * time.Microsecond)
	}
	if err := h.Memory.BSS.Write16(0xeb44, 4); err != nil {
		t.Fatal(err)
	}
	check := func(want bool) {
		t.Helper()
		got, err := s.CanRetryHostStream()
		if err != nil || got != want {
			t.Fatal("retry source boundary differs", got, want, err)
		}
	}
	check(true)
	h.Session.Phase = NativeFrameSessionPhysics
	check(false)
	h.Session.Phase = NativeFrameSessionMenu
	if err := h.Memory.BSS.Write8(0xeb5e, 8); err != nil {
		t.Fatal(err)
	}
	check(false)
	_ = h.Memory.BSS.Write8(0xeb5e, 2)
	s.packets[0] = &NativeSerialPacket{}
	check(false)
	s.packets[0] = nil
	s.mismatch = &NativeErrorFrameState{}
	check(false)
	s.mismatch = nil
	s.Handshake.Started = true
	check(false)
	s.Handshake.Finished = true
	s.Resume = NativeTransportResumeState{Started: true, PC: 0x18266}
	check(false)
	s.Resume = NativeTransportResumeState{}
	s.Transfer = NativeTransportIOState{Started: true}
	check(false)
	s.Transfer.Complete = true
	check(true)
}
