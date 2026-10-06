package populous2

import (
	"bytes"
	"net"
	"testing"
	"time"
)

func TestNativeRuntimeTransportPacketsUseRealConnectionsAndRawOwners(t *testing.T) {
	left, right := net.Pipe()
	var hosts [2]*NativeRuntimeHost
	var transport [2]*NativeRuntimeTransport
	var contexts [2]NativeCommandRegisterContext
	var phases [2]uint32
	ownership := [2]int{}
	for side, connection := range []net.Conn{left, right} {
		h := nativeRuntimeHostTest(t)
		hosts[side] = h
		conn, err := NewNativeSerialConn(connection)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { conn.Close() })
		index := side
		transport[side], err = h.NewTransport(conn, NativeTransportFrameCallbacks{}, func(_ bool, c *NativeFrameRegisterContext) error {
			ownership[index]++
			c.D[7] ^= 0xffffffff
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, patch := range []nativeHeroPatch{{0xeb28, 4, 0x12345678}, {0x15e, 2, 380}, {0xeb57, 1, 22}, {0xeb58, 2, 0x2021}} {
			renderFramePatch(h.Memory.BSS, patch)
		}
		contexts[side] = NativeCommandRegisterContext{D: [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}}
	}
	complete := [2]bool{}
	deadline := time.Now().Add(time.Second)
	for !complete[0] || !complete[1] {
		if time.Now().After(deadline) {
			t.Fatal("native runtime packet stream stalled")
		}
		for side := range hosts {
			if complete[side] {
				continue
			}
			mode := uint8(6 + side*2)
			var err error
			complete[side], err = transport[side].PacketCallback(0xeb56, mode, &contexts[side], &phases[side])
			if err != nil {
				t.Fatal(err)
			}
		}
		time.Sleep(100 * time.Microsecond)
	}
	var data [2][8]byte
	for side, h := range hosts {
		for i := range data[side] {
			v, err := h.Memory.BSS.Read8(0xeb56 + i)
			if err != nil {
				t.Fatal(err)
			}
			data[side][i] = v
		}
		if phases[side] != 2 || contexts[side].D[7] != 8 || ownership[side] < 2 {
			t.Fatal("packet host ownership changed native context", side, contexts[side], ownership[side])
		}
	}
	if !bytes.Equal(data[0][:], data[1][:]) || contexts[0].D[0] != 8 || contexts[1].D[0] != 0x12345678 {
		t.Fatal("native packet bytes/register outputs differ", data, contexts)
	}
}

func TestNativeRuntimeTransportMismatchRetainsOriginalErrorRequester(t *testing.T) {
	left, right := net.Pipe()
	defer right.Close()
	h := nativeRuntimeHostTest(t)
	if _, err := h.InitializePresentation(NativeMouseSample{}); err != nil {
		t.Fatal(err)
	}
	conn, err := NewNativeSerialConn(left)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	transport, err := h.NewTransport(conn, NativeTransportFrameCallbacks{}, func(bool, *NativeFrameRegisterContext) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	for _, patch := range []nativeHeroPatch{{0xeb28, 4, 0x12345678}, {0x15e, 2, 380}, {0xeb5e, 1, 8}, {0xeb68, 1, 6}} {
		renderFramePatch(h.Memory.BSS, patch)
	}
	written := make(chan error, 1)
	go func() { _, err := right.Write([]byte{1, 2, 3, 4, 0xde, 0xad, 0xbe, 0xef}); written <- err }()
	context := NativeCommandRegisterContext{}
	phase := uint32(0)
	deadline := time.Now().Add(time.Second)
	for transport.mismatch == nil {
		done, err := transport.PacketCallback(0xeb56, 8, &context, &phase)
		if err != nil {
			t.Fatal(err)
		}
		if done {
			t.Fatal("mismatched RNG completed without actual dialog")
		}
		if time.Now().After(deadline) {
			t.Fatal("mismatch requester did not begin")
		}
		time.Sleep(100 * time.Microsecond)
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	if transport.mismatch.Routine != 0x33b2 || transport.mismatch.A[1].Address == 0 {
		t.Fatal("source error requester context missing")
	}
	if random, err := h.Memory.BSS.Read32(0xeb28); err != nil || random != 0x12345678 {
		t.Fatal("mismatch repaired native RNG", random, err)
	}
	if mode, err := h.Memory.BSS.Read8(0xeb5e); err != nil || mode != 8 {
		t.Fatal("mismatch disconnected before source acknowledgement", mode, err)
	}
}
