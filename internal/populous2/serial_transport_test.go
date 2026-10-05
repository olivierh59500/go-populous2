package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"syscall"
	"testing"
	"time"
)

func TestNativeSerialPairedConnectionsHandshakeAndCommands(t *testing.T) {
	rules, err := DecodeNativeSerialRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	left, right := net.Pipe()
	conns := [2]*NativeSerialConn{}
	conns[0], err = NewNativeSerialConn(left)
	if err != nil {
		t.Fatal(err)
	}
	conns[1], err = NewNativeSerialConn(right)
	if err != nil {
		t.Fatal(err)
	}
	defer conns[0].Close()
	defer conns[1].Close()
	var memory [2]FollowerCleanupMemory
	var b [2][]byte
	var handshake [2]*NativeSerialHandshake
	var callbacks [2]NativeSerialHandshakeCallbacks
	clock := uint32(0)
	var initialized [2]int
	for side := 0; side < 2; side++ {
		memory[side], b[side] = serialFixtureMemory(t, serialNativeInput{Profile: uint16(side + 1), Transports: [3]uint8{2, 4, 0}})
		_ = memory[side].Write32(0xeb24, uint32(0x0001a3b5+side*100))
		_ = memory[side].Write32(0xeb28, uint32(0x88776655+side))
		_ = memory[side].Write16(0xeb22, 1)
		_ = memory[side].Write16(0xeb2c, 0x155)
		_ = memory[side].Write16(0xeb2e, 0x2aa)
		own := 0xe8f2 + side*314
		for i := 0; i < 236; i++ {
			b[side][own+i] = byte(i*17 + side*51)
		}
		handshake[side], err = NewNativeSerialHandshake(rules, 4800)
		if err != nil {
			t.Fatal(err)
		}
		waitSite, waitCalls := uint32(0), 0
		callbacks[side] = NativeSerialHandshakeCallbacks{Serial: NativeSerialCallbacks{Memory: memory[side], Port: conns[side].Port(), Message: func(kind NativeSerialMessage) (bool, error) {
			t.Fatalf("unexpected paired handshake message%d", kind)
			return false, nil
		}}, FrameCounter: func() uint32 { return clock }, WaitCPU: func(site, count uint32) (bool, error) {
			if count != 100000 {
				t.Fatal("source CPU delay amount lost")
			}
			if site != waitSite {
				waitSite, waitCalls = site, 0
			}
			waitCalls++
			return waitCalls >= 10, nil
		}, Initialize: func() (bool, error) { initialized[side]++; return true, nil }}
	}
	deadline := time.Now().Add(5 * time.Second)
	for !handshake[0].Ready || !handshake[1].Ready {
		if time.Now().After(deadline) {
			t.Fatalf("paired native handshake stalled: %d/%d", handshake[0].phase, handshake[1].phase)
		}
		clock++
		for side := 0; side < 2; side++ {
			step, err := handshake[side].Advance(callbacks[side])
			if err != nil {
				t.Fatal(err)
			}
			if step.Complete && !step.Ready {
				t.Fatalf("paired native handshake failed: %+v", step)
			}
		}
		time.Sleep(100 * time.Microsecond)
	}
	if initialized != [2]int{1, 1} {
		t.Fatal("connection fabricated or repeated a world initialization continuation")
	}
	if !reflect.DeepEqual(b[0][0xeb22:0xeb30], b[1][0xeb22:0xeb30]) || !reflect.DeepEqual(b[0][0xe8f2:0xe9de], b[1][0xe8f2:0xe9de]) || !reflect.DeepEqual(b[0][0xea2c:0xeb18], b[1][0xea2c:0xeb18]) {
		t.Fatal("real stream handshake did not transmit actual negotiated header/profiles")
	}
	if [2]byte{b[0][0xeb5e], b[0][0xeb68]} != [2]byte{6, 8} || [2]byte{b[1][0xeb5e], b[1][0xeb68]} != [2]byte{8, 6} {
		t.Fatal("real peer roles differ from source")
	}
	for tick := 0; tick < 100; tick++ {
		var schedulers [2]*NativeSerialDeferred
		var executed [2][]int
		var peerCallbacks [2]NativeSerialDeferredCallbacks
		for side := 0; side < 2; side++ {
			at := 0xeb56 + side*10
			_ = memory[side].Write8(at+1, uint8(6+side*2))
			_ = memory[side].Write8(at+2, uint8(tick))
			_ = memory[side].Write8(at+3, uint8(tick+side))
			schedulers[side] = NewNativeSerialDeferred(NativeCommandRegisterContext{})
			peerCallbacks[side] = NativeSerialDeferredCallbacks{Serial: NativeSerialCallbacks{Memory: memory[side], Port: conns[side].Port(), Message: func(kind NativeSerialMessage) (bool, error) {
				t.Fatalf("native RNG diverged at tick%d", tick)
				return false, nil
			}}, Execute: func(caller int, context *NativeCommandRegisterContext) error {
				executed[side] = append(executed[side], caller)
				command, _ := memory[side].Read8(caller + 1)
				if command != 6 && command != 8 {
					return fmt.Errorf("wrong command received%d", command)
				}
				rng, _ := memory[side].Read32(0xeb28)
				return memory[side].Write32(0xeb28, rng*1103515245+uint32(command))
			}}
		}
		deadline := time.Now().Add(time.Second)
		for !schedulers[0].Complete || !schedulers[1].Complete {
			if time.Now().After(deadline) {
				t.Fatalf("paired packet stages stalled at tick%d", tick)
			}
			for side := 0; side < 2; side++ {
				if _, err := schedulers[side].Advance(peerCallbacks[side]); err != nil {
					t.Fatal(err)
				}
			}
			time.Sleep(50 * time.Microsecond)
		}
		if !reflect.DeepEqual(executed[0], []int{0xeb56, 0xeb60}) || !reflect.DeepEqual(executed[0], executed[1]) || binary.BigEndian.Uint32(b[0][0xeb28:]) != binary.BigEndian.Uint32(b[1][0xeb28:]) {
			t.Fatal("paired serial scheduler did not execute both owners in original order")
		}
	}
}

func TestNativeSerialReceiveRingAgainstOriginalISR(t *testing.T) {
	data, err := os.ReadFile("testdata/serial_ring_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			Read, Write, BeforeRead, BeforeWrite, AfterRead, AfterWrite uint16
			FeedHex, TakenHex, RingHash                                 string
			Available                                                   int
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil || len(corpus.Cases) != 48 {
		t.Fatalf("native receive ISR corpus incomplete: %v", err)
	}
	for _, c := range corpus.Cases {
		ring := nativeSerialRing{read: c.Read, write: c.Write}
		for i := range ring.data {
			ring.data[i] = byte(i*7 + 13)
		}
		feed, err := hex.DecodeString(c.FeedHex)
		if err != nil {
			t.Fatal(err)
		}
		for _, value := range feed {
			ring.receive(value)
		}
		if ring.available() != c.Available || ring.read != c.BeforeRead || ring.write != c.BeforeWrite {
			t.Fatal("native ISR wrap/overwrite/availability differs")
		}
		var dst [8]byte
		n := ring.take(dst[:])
		want, err := hex.DecodeString(c.TakenHex)
		if err != nil {
			t.Fatal(err)
		}
		if hex.EncodeToString(dst[:n]) != hex.EncodeToString(want) || ring.read != c.AfterRead || ring.write != c.AfterWrite || fmt.Sprintf("%x", sha256.Sum256(ring.data[:])) != c.RingHash {
			t.Fatal("native BBE prefix/read cursor/backing bytes differ")
		}
	}
}

func TestNativeSerialReceiveRingRetainsNativeWrapAndOverwrite(t *testing.T) {
	var ring nativeSerialRing
	for i := 0; i < 380; i++ {
		ring.receive(byte(i))
	}
	var first [379]byte
	if ring.take(first[:]) != 379 || ring.read != 379 || ring.write != 380 {
		t.Fatal("native ring cursor progression differs")
	}
	ring.receive(0xab)
	ring.receive(0xcd)
	if ring.available() != 378 {
		t.Fatal("native absolute cursor-difference quirk was normalized")
	}
	var tail [3]byte
	if ring.take(tail[:]) != 3 || tail != [3]byte{123, 0xab, 0xcd} || ring.read != ring.write {
		t.Fatal("native read cursor did not cross its inclusive380 wrap")
	}
	ring.flush()
	for i := 0; i < 381; i++ {
		ring.receive(byte(i))
	}
	if ring.available() != 0 || ring.take(tail[:]) != 0 {
		t.Fatal("native full-ring overwrite was converted to a corrected FIFO")
	}
}

func TestNativeSerialTCPConnectionCarriesOriginalBytes(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		if errors.Is(err, syscall.EPERM) {
			t.Skip("sandbox denied localhost TCP bind; paired net.Conn proofs run independently; rerun this test with approved loopback access")
		}
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	acceptError := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			acceptError <- err
			return
		}
		accepted <- conn
	}()
	client, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	var server net.Conn
	select {
	case server = <-accepted:
	case err := <-acceptError:
		t.Fatal(err)
	case <-time.After(time.Second):
		t.Fatal("TCP accept stalled")
	}
	left, err := NewNativeSerialConn(client)
	if err != nil {
		t.Fatal(err)
	}
	right, err := NewNativeSerialConn(server)
	if err != nil {
		t.Fatal(err)
	}
	defer left.Close()
	defer right.Close()
	payload := []byte{0x3f, 'A', 'B', 'C', 'D', 1, 6, 9, 12, 0x81, 0x23, 0x45, 0x67}
	written, read := 0, 0
	got := make([]byte, len(payload))
	deadline := time.Now().Add(time.Second)
	for written < len(payload) || read < len(payload) {
		if time.Now().After(deadline) {
			t.Fatal("real TCP native byte transfer stalled")
		}
		if written < len(payload) {
			n, err := left.Port().Write(payload[written:])
			if err != nil && err != ErrNativeSerialWait {
				t.Fatal(err)
			}
			written += n
		}
		if read < len(payload) {
			n, err := right.Port().Read(got[read:])
			if err != nil && err != ErrNativeSerialWait {
				t.Fatal(err)
			}
			read += n
		}
		time.Sleep(100 * time.Microsecond)
	}
	if !reflect.DeepEqual(got, payload) {
		t.Fatal("TCP adapter introduced framing or changed original protocol bytes")
	}
}
