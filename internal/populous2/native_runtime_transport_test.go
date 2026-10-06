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
