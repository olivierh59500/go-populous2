package populous2

import (
	"context"
	"errors"
	"io"
	"syscall"
	"testing"
	"time"
)

func TestNativeNetworkEndpointPollsRealConnectionWithoutFrameBlocking(t *testing.T) {
	server, err := ListenNativeNetwork("127.0.0.1:0")
	if err != nil {
		if errors.Is(err, syscall.EPERM) {
			t.Skip("sandbox denied loopback bind; rerun with approved local socket access")
		}
		t.Fatal(err)
	}
	defer server.Close()
	if _, ready, err := server.Poll(); ready || err != nil {
		t.Fatal("listener completed before actual client", ready, err)
	}
	client, err := DialNativeNetwork(context.Background(), server.Address())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	deadline := time.Now().Add(time.Second)
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
			t.Fatal("actual endpoint connection stalled")
		}
		time.Sleep(100 * time.Microsecond)
	}
	s, _, _ := server.Poll()
	c, _, _ := client.Poll()
	done := make(chan error, 1)
	go func() { _, err := c.Write([]byte{0, 1, 2, 255}); done <- err }()
	var packet [4]byte
	if _, err := io.ReadFull(s, packet[:]); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if packet != [4]byte{0, 1, 2, 255} {
		t.Fatal("endpoint added framing to native bytes")
	}
}
