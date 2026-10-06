// Package network synchronizes two semantic Go simulations with deterministic
// ordered input rounds. Transport errors stop a session before world mutation.
package network

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"go-populous2/internal/engine"
)

const ProtocolVersion = 1
const maximumFrameBytes = 16 << 20
const maximumCommands = 64

type Command struct {
	Kind     string             `json:"kind"`
	Power    engine.PowerID     `json:"power,omitempty"`
	Target   engine.PowerTarget `json:"target,omitempty"`
	Mode     engine.Mode        `json:"mode,omitempty"`
	Follower int                `json:"follower,omitempty"`
}

type message struct {
	Version  int              `json:"version"`
	Kind     string           `json:"kind"`
	Side     int              `json:"side"`
	Round    uint64           `json:"round,omitempty"`
	Digest   string           `json:"digest"`
	Rules    string           `json:"rules"`
	Snapshot *engine.Snapshot `json:"snapshot,omitempty"`
	Commands []Command        `json:"commands,omitempty"`
}

type Session struct {
	mu         sync.Mutex
	conn       net.Conn
	side       int
	round      uint64
	rules      string
	failure    error
	closeOnce  sync.Once
	closeError error
}

func (s *Session) Side() int { return s.side }
func (s *Session) Close() error {
	s.closeOnce.Do(func() { s.closeError = s.conn.Close() })
	return s.closeError
}

func contextDeadline(ctx context.Context, conn net.Conn) func() {
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	} else {
		conn.SetDeadline(time.Time{})
	}
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { conn.SetDeadline(time.Now()); close(done) })
	return func() {
		if !stop() {
			<-done
		}
		conn.SetDeadline(time.Time{})
	}
}

func writeMessage(conn net.Conn, value message) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(data) > maximumFrameBytes {
		return fmt.Errorf("network frame exceeds size limit")
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(data)))
	for _, part := range [][]byte{header[:], data} {
		for len(part) > 0 {
			n, err := conn.Write(part)
			if err != nil {
				return err
			}
			if n <= 0 {
				return io.ErrNoProgress
			}
			part = part[n:]
		}
	}
	return nil
}
func readMessage(conn net.Conn) (message, error) {
	var value message
	var header [4]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return value, err
	}
	size := binary.BigEndian.Uint32(header[:])
	if size == 0 || size > maximumFrameBytes {
		return value, fmt.Errorf("invalid network frame length")
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(conn, data); err != nil {
		return value, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return value, fmt.Errorf("network frame contains trailing data")
	}
	if value.Version != ProtocolVersion {
		return value, fmt.Errorf("unsupported network protocol version")
	}
	return value, nil
}
