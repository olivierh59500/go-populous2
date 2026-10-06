package populous2

import (
	"context"
	"fmt"
	"net"
	"sync"
)

// NativeNetworkEndpoint owns a configured asynchronous TCP connection. It is
// only a host transport for the original serial bytes; it adds no game packet
// framing or synchronization repair. Poll never blocks the native frame.
type NativeNetworkEndpoint struct {
	mu       sync.Mutex
	listener net.Listener
	conn     net.Conn
	err      error
	done     bool
	cancel   context.CancelFunc
}

func ListenNativeNetwork(address string) (*NativeNetworkEndpoint, error) {
	if address == "" {
		return nil, fmt.Errorf("native listen address missing")
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	e := &NativeNetworkEndpoint{listener: listener}
	go func() {
		conn, err := listener.Accept()
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.done {
			if conn != nil {
				conn.Close()
			}
			return
		}
		e.conn, e.err, e.done = conn, err, true
	}()
	return e, nil
}

func DialNativeNetwork(ctx context.Context, address string) (*NativeNetworkEndpoint, error) {
	if ctx == nil || address == "" {
		return nil, fmt.Errorf("native dial context/address missing")
	}
	ctx, cancel := context.WithCancel(ctx)
	e := &NativeNetworkEndpoint{cancel: cancel}
	go func() {
		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.done {
			if conn != nil {
				conn.Close()
			}
			return
		}
		e.conn, e.err, e.done = conn, err, true
	}()
	return e, nil
}

func (e *NativeNetworkEndpoint) Address() string {
	if e == nil {
		return ""
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.listener != nil {
		return e.listener.Addr().String()
	}
	return ""
}

func (e *NativeNetworkEndpoint) Poll() (net.Conn, bool, error) {
	if e == nil {
		return nil, false, fmt.Errorf("native network endpoint missing")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.conn, e.done, e.err
}

func (e *NativeNetworkEndpoint) Close() error {
	if e == nil {
		return nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancel != nil {
		e.cancel()
	}
	e.done = true
	var err error
	if e.conn != nil {
		err = e.conn.Close()
		e.conn = nil
	}
	if e.listener != nil {
		closeErr := e.listener.Close()
		if err == nil {
			err = closeErr
		}
		e.listener = nil
	}
	return err
}
