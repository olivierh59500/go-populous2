package populous2

import (
	"errors"
	"io"
	"net"
	"syscall"
	"testing"
	"time"
)

func TestNativeNetworkSessionRetriesOnlyExplicitlyAfterActualEOF(t *testing.T) {
	server, err := NewNativeNetworkSession("127.0.0.1:0", "")
	if errors.Is(err, syscall.EPERM) {
		t.Skip("sandbox denied loopback bind; rerun with approved socket access")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	address := server.Endpoint.Address()
	client, err := NewNativeNetworkSession("", address)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	waitPair := func() (*NativeSerialConn, *NativeSerialConn) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for {
			a, ar, ae := server.Poll()
			b, br, be := client.Poll()
			if ae != nil || be != nil {
				t.Fatal(ae, be)
			}
			if ar && br {
				return a, b
			}
			if time.Now().After(deadline) {
				t.Fatal("actual TCP session did not connect")
			}
			time.Sleep(100 * time.Microsecond)
		}
	}
	first, peer := waitPair()
	prefix := []byte{1, 8, 22, 23}
	if n, err := peer.conn.Write(prefix); err != nil || n != len(prefix) {
		t.Fatal(n, err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		n, err := first.Port().Available()
		if err != nil {
			t.Fatal(err)
		}
		if n == len(prefix) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("source receive prefix missing")
		}
		time.Sleep(100 * time.Microsecond)
	}
	if retried, err := server.Retry(); err != nil || retried {
		t.Fatal("live stream replaced", retried, err)
	}
	if err := peer.Close(); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(time.Second)
	for first.TerminalError() == nil {
		if time.Now().After(deadline) {
			t.Fatal("receive pump did not observe actual EOF")
		}
		time.Sleep(100 * time.Microsecond)
	}
	if !errors.Is(first.TerminalError(), io.EOF) {
		t.Fatal(first.TerminalError())
	}
	same, ready, err := server.Poll()
	if err != nil || !ready || same != first {
		t.Fatal("ordinary polling silently retried the dead stream", ready, err)
	}
	if retried, err := server.Retry(); err != nil || !retried {
		t.Fatal("explicit retry did not reopen listener", retried, err)
	}
	if server.Endpoint.Address() != address {
		t.Fatal("port-zero listener address changed during explicit retry")
	}
	next, err := NewNativeNetworkSession("", address)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	deadline = time.Now().Add(3 * time.Second)
	for {
		a, ar, ae := server.Poll()
		_, br, be := next.Poll()
		if ae != nil || be != nil {
			t.Fatal(ae, be)
		}
		if ar && br {
			if a == first {
				t.Fatal("retry reused the closed serial owner")
			}
			data, read, write := a.NativeTransportReceiveImage()
			if read != 0 || write != 0 {
				t.Fatal("new ring inherited active receive indices")
			}
			for i, v := range prefix {
				if data[i] != v {
					t.Fatal("source buffer history cleared on retry")
				}
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("explicit retry did not accept next peer")
		}
		time.Sleep(100 * time.Microsecond)
	}
}

func TestNativeNetworkSessionRetriesInitialDialFailure(t *testing.T) {
	listener, err := ListenNativeNetwork("127.0.0.1:0")
	if errors.Is(err, syscall.EPERM) {
		t.Skip("sandbox denied loopback bind")
	}
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Address()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	client, err := NewNativeNetworkSession("", address)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	deadline := time.Now().Add(3 * time.Second)
	for {
		_, ready, err := client.Endpoint.Poll()
		if ready && err != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("failed dial was not observed")
		}
		time.Sleep(100 * time.Microsecond)
	}
	server, err := NewNativeNetworkSession(address, "")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	retried, err := client.Retry()
	if err != nil || !retried {
		t.Fatal("explicit failed-dial retry missing", retried, err)
	}
	deadline = time.Now().Add(3 * time.Second)
	for {
		_, a, ae := server.Poll()
		_, b, be := client.Poll()
		if ae != nil || be != nil {
			t.Fatal(ae, be)
		}
		if a && b {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("dial retry did not reach the new peer")
		}
		time.Sleep(100 * time.Microsecond)
	}
}

type nativeRetryWriteFailureConn struct{ net.Conn }

func (c nativeRetryWriteFailureConn) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestNativeSerialTerminalErrorRetainsWriteFailure(t *testing.T) {
	left, right := net.Pipe()
	defer right.Close()
	conn, err := NewNativeSerialConn(nativeRetryWriteFailureConn{left})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	deadline := time.Now().Add(time.Second)
	for {
		_, err := conn.Port().Write([]byte{1, 2, 3})
		if errors.Is(err, io.ErrClosedPipe) {
			break
		}
		if err != nil && !errors.Is(err, ErrNativeSerialWait) {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatal("write failure did not return")
		}
		time.Sleep(100 * time.Microsecond)
	}
	if !errors.Is(conn.TerminalError(), io.ErrClosedPipe) {
		t.Fatal("terminal observation ignored the write failure", conn.TerminalError())
	}
}
