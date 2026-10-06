package network

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"reflect"
	"testing"
	"time"

	"go-populous2/internal/engine"
)

func loopbackListener(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	return listener
}

func TestBluetoothProxyRejectsUnrelatedClientWithoutConsumingListener(t *testing.T) {
	listener := loopbackListener(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	token := bytes.Repeat([]byte{0x43}, BluetoothTokenBytes)
	accepted := make(chan net.Conn, 1)
	errors := make(chan error, 1)
	go func() {
		conn, err := AcceptBluetoothProxy(ctx, listener, token)
		if err != nil {
			errors <- err
			return
		}
		accepted <- conn
	}()
	stranger, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	_, _ = stranger.Write(bytes.Repeat([]byte{0x99}, BluetoothTokenBytes))
	_ = stranger.SetReadDeadline(time.Now().Add(time.Second))
	var one [1]byte
	if _, err := stranger.Read(one[:]); err != io.EOF {
		t.Fatalf("unauthenticated client was not closed: %v", err)
	}
	_ = stranger.Close()
	client, err := DialBluetoothProxy(ctx, listener.Addr().String(), token)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	select {
	case host := <-accepted:
		defer host.Close()
		go func() { _, _ = client.Write([]byte("lockstep")) }()
		payload := make([]byte, len("lockstep"))
		if _, err := io.ReadFull(host, payload); err != nil || string(payload) != "lockstep" {
			t.Fatalf("proxy damaged framed payload: %q, %v", payload, err)
		}
	case err := <-errors:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func TestBluetoothProxyCancellationInterruptsPartialAuthentication(t *testing.T) {
	listener := loopbackListener(t)
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() {
		_, err := AcceptBluetoothProxy(ctx, listener, make([]byte, BluetoothTokenBytes))
		finished <- err
	}()
	client, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	_, _ = client.Write([]byte{0})
	cancel()
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation returned %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("partial authentication blocked cancellation")
	}
}

func TestBluetoothProxyRequiresNumericLoopbackAndCompleteToken(t *testing.T) {
	for _, address := range []string{"localhost:1234", "192.168.1.1:1234", "127.0.0.1:0"} {
		if conn, err := DialBluetoothProxy(context.Background(), address, make([]byte, BluetoothTokenBytes)); err == nil {
			conn.Close()
			t.Fatalf("accepted non-private endpoint %s", address)
		}
	}
	if _, err := DialBluetoothProxy(context.Background(), "127.0.0.1:1234", []byte{1}); err == nil {
		t.Fatal("incomplete token accepted")
	}
}

// The middle pair emulates Java's transparent RFCOMM pumps. Each Android
// endpoint authenticates its own local proxy; peer payload remains unchanged.
func TestBluetoothProxyCarriesSnapshotRulesAndDeterministicRounds(t *testing.T) {
	hostListener, joinListener := loopbackListener(t), loopbackListener(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	hostToken := bytes.Repeat([]byte{1}, BluetoothTokenBytes)
	joinToken := bytes.Repeat([]byte{2}, BluetoothTokenBytes)
	type connectionResult struct {
		connection net.Conn
		err        error
	}
	hostReady := make(chan connectionResult, 1)
	go func() {
		conn, err := AcceptBluetoothProxy(ctx, hostListener, hostToken)
		hostReady <- connectionResult{conn, err}
	}()
	bridgeHost, err := DialBluetoothProxy(ctx, hostListener.Addr().String(), hostToken)
	if err != nil {
		t.Fatal(err)
	}
	defer bridgeHost.Close()
	bridgeJoinReady := make(chan connectionResult, 1)
	go func() {
		conn, err := AcceptBluetoothProxy(ctx, joinListener, joinToken)
		bridgeJoinReady <- connectionResult{conn, err}
	}()
	joinConn, err := DialBluetoothProxy(ctx, joinListener.Addr().String(), joinToken)
	if err != nil {
		t.Fatal(err)
	}
	defer joinConn.Close()
	hostResult, bridgeResult := <-hostReady, <-bridgeJoinReady
	if hostResult.err != nil || bridgeResult.err != nil {
		t.Fatalf("proxy setup: %v / %v", hostResult.err, bridgeResult.err)
	}
	defer hostResult.connection.Close()
	defer bridgeResult.connection.Close()
	go func() { _, _ = io.Copy(bridgeHost, bridgeResult.connection) }()
	go func() { _, _ = io.Copy(bridgeResult.connection, bridgeHost) }()

	hostWorld := networkWorld(t)
	initial := hostWorld.Snapshot()
	type hostHandshake struct {
		session *Session
		err     error
	}
	hostHandshakeReady := make(chan hostHandshake, 1)
	go func() {
		session, err := Host(ctx, hostResult.connection, hostWorld, "bluetooth-rules-test")
		hostHandshakeReady <- hostHandshake{session, err}
	}()
	joinSession, joinWorld, err := Join(ctx, joinConn, "bluetooth-rules-test")
	if err != nil {
		t.Fatal(err)
	}
	hostHandshakeResult := <-hostHandshakeReady
	if hostHandshakeResult.err != nil {
		t.Fatal(hostHandshakeResult.err)
	}
	hostSession := hostHandshakeResult.session
	defer hostSession.Close()
	defer joinSession.Close()
	if !reflect.DeepEqual(initial, joinWorld.Snapshot()) || hostSession.Side() != 0 || joinSession.Side() != 1 {
		t.Fatal("Bluetooth handshake changed the host world or assigned incorrect sides")
	}
	for round := 0; round < 8; round++ {
		completed := make(chan error, 1)
		go func() {
			_, err := hostSession.Advance(ctx, hostWorld, []Command{{Kind: "mode", Mode: engine.Mode(round % 4)}})
			completed <- err
		}()
		_, err := joinSession.Advance(ctx, joinWorld, []Command{{Kind: "rally", Target: engine.PowerTarget{X: 20 + round, Y: 25}}})
		if err != nil {
			t.Fatal(err)
		}
		if err := <-completed; err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(hostWorld.Snapshot(), joinWorld.Snapshot()) {
			t.Fatalf("Bluetooth peers diverged at round %d", round)
		}
	}
	before := hostWorld.Snapshot()
	_ = bridgeHost.Close()
	if _, err := hostSession.Advance(ctx, hostWorld, nil); err == nil || !reflect.DeepEqual(before, hostWorld.Snapshot()) {
		t.Fatal("Bluetooth disconnection mutated the local simulation")
	}
}
