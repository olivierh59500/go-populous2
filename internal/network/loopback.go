package network

import (
	"context"
	"crypto/subtle"
	"fmt"
	"io"
	"net"
	"time"

	"go-populous2/internal/platformbridge"
)

// BluetoothTokenBytes authenticates the private on-device proxy, rather than
// the Bluetooth peer. Android uses a secure, paired RFCOMM socket for the peer.
const BluetoothTokenBytes = 16

// AcceptBluetoothProxy accepts Android's authenticated loopback stream. An
// unrelated local connection is rejected without consuming the listener, and
// cancellation interrupts both acceptance and a partially supplied token.
func AcceptBluetoothProxy(ctx context.Context, listener net.Listener, token []byte) (net.Conn, error) {
	if listener == nil || len(token) != BluetoothTokenBytes {
		return nil, fmt.Errorf("invalid Bluetooth proxy listener or token")
	}
	if _, err := platformbridge.NormalizeLoopbackAddress(listener.Addr().String()); err != nil {
		return nil, err
	}
	stopListener := context.AfterFunc(ctx, func() { _ = listener.Close() })
	defer stopListener()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("accept Bluetooth proxy: %w", err)
		}
		if _, err := platformbridge.NormalizeLoopbackAddress(conn.RemoteAddr().String()); err != nil {
			_ = conn.Close()
			continue
		}
		stopConnection := context.AfterFunc(ctx, func() { _ = conn.Close() })
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		var supplied [BluetoothTokenBytes]byte
		_, err = io.ReadFull(conn, supplied[:])
		valid := err == nil && subtle.ConstantTimeCompare(supplied[:], token) == 1
		clear(supplied[:])
		stopped := stopConnection()
		if !valid || !stopped || ctx.Err() != nil {
			_ = conn.Close()
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		_ = conn.SetDeadline(time.Time{})
		return conn, nil
	}
}

// DialBluetoothProxy writes only the local authentication prefix. All later
// traffic uses the same bounded, checked lockstep protocol as TCP multiplayer.
func DialBluetoothProxy(ctx context.Context, address string, token []byte) (net.Conn, error) {
	if len(token) != BluetoothTokenBytes {
		return nil, fmt.Errorf("invalid Bluetooth proxy token")
	}
	address, err := platformbridge.NormalizeLoopbackAddress(address)
	if err != nil {
		return nil, err
	}
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("connect Bluetooth proxy: %w", err)
	}
	cleanup := contextDeadline(ctx, conn)
	for remaining := token; len(remaining) > 0; {
		n, writeErr := conn.Write(remaining)
		if writeErr != nil || n <= 0 {
			cleanup()
			_ = conn.Close()
			if writeErr == nil {
				writeErr = io.ErrNoProgress
			}
			return nil, fmt.Errorf("authenticate Bluetooth proxy: %w", writeErr)
		}
		remaining = remaining[n:]
	}
	cleanup()
	return conn, nil
}
