package populous2

import (
	"bytes"
	"fmt"
	"net"
	"sync"
)

// nativeSerialRing retains the 381-byte receive buffer configured at $0a58.
// The original ISR can overwrite unread bytes, and $0ae2 reports the absolute
// cursor difference instead of a corrected modular count. Both behaviors
// remain visible to the protocol; a host stream is not an invented packet FIFO.
type nativeSerialRing struct {
	data        [381]byte
	read, write uint16
}

func (r *nativeSerialRing) receive(value byte) {
	r.data[r.write] = value
	if r.write == 380 {
		r.write = 0
	} else {
		r.write++
	}
}
func (r *nativeSerialRing) available() int {
	n := int(r.write) - int(r.read)
	if n < 0 {
		n = -n
	}
	return n
}
func (r *nativeSerialRing) take(dst []byte) int {
	n := 0
	for n < len(dst) && r.read != r.write {
		dst[n] = r.data[r.read]
		if r.read == 380 {
			r.read = 0
		} else {
			r.read++
		}
		n++
	}
	return n
}
func (r *nativeSerialRing) flush() { r.read, r.write = 0, 0 }

type nativeSerialWriteResult struct {
	count int
	err   error
}
type nativeSerialConnWrite struct {
	data   []byte
	result chan nativeSerialWriteResult
}

// NativeSerialConn adapts an actual net.Conn to the original ordered stream.
// Its receive pump and pending writes yield to the Game thread. TCP provides
// the bytes; it does not pretend to emulate the Amiga's physical baud clock.
type NativeSerialConn struct {
	conn       net.Conn
	mu         sync.Mutex
	ring       nativeSerialRing
	readError  error
	write      *nativeSerialConnWrite
	baud       uint16
	readerDone chan struct{}
	closeOnce  sync.Once
	closeError error
}

func NewNativeSerialConn(conn net.Conn) (*NativeSerialConn, error) {
	return newNativeSerialConn(conn, [381]byte{})
}

func newNativeSerialConn(conn net.Conn, history [381]byte) (*NativeSerialConn, error) {
	if conn == nil {
		return nil, fmt.Errorf("native serial connection missing")
	}
	c := &NativeSerialConn{conn: conn, baud: 300, readerDone: make(chan struct{})}
	c.ring.data = history
	go c.receive()
	return c, nil
}
func (c *NativeSerialConn) receive() {
	defer close(c.readerDone)
	var buffer [64]byte
	for {
		n, err := c.conn.Read(buffer[:])
		c.mu.Lock()
		for _, value := range buffer[:n] {
			c.ring.receive(value)
		}
		if err != nil {
			c.readError = err
		}
		c.mu.Unlock()
		if err != nil {
			return
		}
	}
}

func (c *NativeSerialConn) Port() NativeSerialPort {
	if c == nil {
		return NativeSerialPort{}
	}
	return NativeSerialPort{
		Available: func() (int, error) {
			c.mu.Lock()
			defer c.mu.Unlock()
			n := c.ring.available()
			if c.ring.read == c.ring.write && c.readError != nil {
				return 0, c.readError
			}
			return n, nil
		},
		Read: func(dst []byte) (int, error) {
			c.mu.Lock()
			defer c.mu.Unlock()
			n := c.ring.take(dst)
			if n == len(dst) {
				return n, nil
			}
			if c.readError != nil {
				return n, c.readError
			}
			return n, ErrNativeSerialWait
		},
		Write: c.send,
		Flush: func() error { c.mu.Lock(); defer c.mu.Unlock(); c.ring.flush(); return nil },
		Configure: func(baud uint16) error {
			if baud == 0 {
				return fmt.Errorf("native baud divider would divide by zero")
			}
			c.mu.Lock()
			c.baud = baud
			c.mu.Unlock()
			return nil
		},
	}
}
func (c *NativeSerialConn) send(src []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.write == nil {
		pending := &nativeSerialConnWrite{data: append([]byte(nil), src...), result: make(chan nativeSerialWriteResult, 1)}
		c.write = pending
		go func() { n, err := c.conn.Write(pending.data); pending.result <- nativeSerialWriteResult{n, err} }()
		return 0, ErrNativeSerialWait
	}
	if !bytes.Equal(src, c.write.data) {
		return 0, fmt.Errorf("native serial writes overlap a pending transfer")
	}
	select {
	case result := <-c.write.result:
		c.write = nil
		return result.count, result.err
	default:
		return 0, ErrNativeSerialWait
	}
}
func (c *NativeSerialConn) Baud() uint16 { c.mu.Lock(); defer c.mu.Unlock(); return c.baud }

// TerminalError observes the real receive-pump outcome, including an EOF
// following a buffered prefix. It does not drain, flush or change the source
// ring; native controllers must finish their own failure continuation first.
func (c *NativeSerialConn) TerminalError() error {
	if c == nil {
		return fmt.Errorf("native serial connection missing")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.readError
}
func (c *NativeSerialConn) NativeReceiveIndices() (uint16, uint16) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ring.read, c.ring.write
}
func (c *NativeSerialConn) Close() error {
	if c == nil {
		return nil
	}
	c.closeOnce.Do(func() { c.closeError = c.conn.Close() })
	return c.closeError
}
