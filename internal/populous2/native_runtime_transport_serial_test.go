package populous2

import (
	"net"
	"testing"
	"time"
)

func TestNativeRuntimeSerialPortsPreserveBufferAndTemporaryWordCount(t *testing.T) {
	left, right := net.Pipe()
	h := nativeRuntimeHostTest(t)
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
	frame := NativeFrameRegisterContext{D: [8]uint32{0x12340001, 2, 3, 4, 5, 6, 7, 8}, AddressBase: 0x200000}
	if err := h.Memory.BSS.Write16(0x15a, 4800); err != nil {
		t.Fatal(err)
	}
	if err := h.Memory.BSS.Write16(0x15e, 380); err != nil {
		t.Fatal(err)
	}
	writeDone := make(chan error, 1)
	go func() { _, err := right.Write([]byte{0x41}); writeDone <- err }()
	deadline := time.Now().Add(time.Second)
	for {
		n, err := conn.Port().Available()
		if err != nil {
			t.Fatal(err)
		}
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("real receiver did not get source byte")
		}
		time.Sleep(100 * time.Microsecond)
	}
	phase := uint32(0)
	step, err := transport.SerialChild(NativeSerialFrameCall{NativeFileFrameCall: NativeFileFrameCall{Routine: 0xab4, Frame: &frame}}, &phase)
	if err != nil || !step.Complete || frame.D[0] != 0x12340001 || conn.Baud() != 4800 {
		t.Fatal("source baud operation lost MOVEM context", step, err)
	}
	step, err = transport.SerialChild(NativeSerialFrameCall{NativeFileFrameCall: NativeFileFrameCall{Routine: 0xae2, Frame: &frame}}, &phase)
	if err != nil || !step.Complete || frame.D[0] != 0x12340001 || step.Zero {
		t.Fatal("baud configuration flushed actual ring or lost word context", step, frame.D, err)
	}
	if err := <-writeDone; err != nil {
		t.Fatal(err)
	}
	var a [7]NativeRequesterAddress
	a[0] = NativeRequesterAddress{Address: 0x104b40, Code: true}
	phase = 0
	step, err = transport.SerialChild(NativeSerialFrameCall{NativeFileFrameCall: NativeFileFrameCall{Routine: 0xbbe, Frame: &frame, A: a}}, &phase)
	if err != nil || !step.Complete {
		t.Fatal("actual one-byte receive failed", step, err)
	}
	if value, err := h.Memory.Code.Read8(0x4b40); err != nil || value != 0x41 {
		t.Fatal("serial receive did not write actual CODE buffer", value, err)
	}
	readDone := make(chan struct {
		n    int
		data [2]byte
		err  error
	}, 1)
	go func() {
		var data [2]byte
		n, err := right.Read(data[:])
		readDone <- struct {
			n    int
			data [2]byte
			err  error
		}{n, data, err}
	}()
	phase = 0
	for {
		step, err = transport.SerialChild(NativeSerialFrameCall{NativeFileFrameCall: NativeFileFrameCall{Routine: 0xc2a, Frame: &frame}, StackBytes: []byte{13, 13}}, &phase)
		if err != nil {
			t.Fatal(err)
		}
		if step.Complete {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("serial temporary word send stalled")
		}
		time.Sleep(100 * time.Microsecond)
	}
	result := <-readDone
	if result.err != nil || result.n != 1 || result.data[0] != 13 || frame.D[0] != 0x12340001 {
		t.Fatal("temporary word changed the source one-byte transfer", result, frame.D)
	}
}
