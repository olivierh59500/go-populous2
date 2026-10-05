package populous2

import (
	"errors"
	"fmt"
)

// NativeTransportFrameCallbacks shares actual UI backing with the serial
// byte stream. ReceiveImage exposes the real ISR ring when available; TCP's
// receive pump already owns these bytes and indices under its own mutex.
type NativeTransportFrameCallbacks struct {
	NativeFileFrameCallbacks
	Port               NativeSerialPort
	ReceiveImage       func() ([381]byte, uint16, uint16)
	CallerStackPointer uint32
	WaitCPU            func(site, iterations uint32) (bool, error)
	// CallTransport carries the true CCR returned by children such as $10ad8.
	CallTransport func(NativeFileFrameCall, *uint32) (NativeSerialFrameChildResult, error)
}

func (c *NativeSerialConn) NativeTransportReceiveImage() ([381]byte, uint16, uint16) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ring.data, c.ring.read, c.ring.write
}

func transportFrameSyncReceive(cb NativeTransportFrameCallbacks) error {
	if cb.ReceiveImage == nil {
		return nil
	}
	data, read, write := cb.ReceiveImage()
	if e := cb.Memory.Write16(0x154, read); e != nil {
		return e
	}
	if e := cb.Memory.Write16(0x156, write); e != nil {
		return e
	}
	for i, v := range data {
		if e := cb.Memory.Write8(0x160+i, v); e != nil {
			return e
		}
	}
	return nil
}

func transportFrameAbortKeys(m FollowerCleanupMemory) (bool, error) {
	control, e := m.Read8(0xa7)
	if e != nil {
		return false, e
	}
	if control == 0 {
		return false, nil
	}
	left, e := m.Read8(0x6f)
	if e != nil {
		return false, e
	}
	right, e := m.Read8(0x71)
	return left != 0 || right != 0, e
}

func transportFrameDisconnect(cb NativeTransportFrameCallbacks) error {
	if e := DisconnectNativeSerial(NativeSerialCallbacks{Memory: cb.Memory, Port: cb.Port}); e != nil {
		return e
	}
	return transportFrameSyncReceive(cb)
}

// NativeTransportIOState is the counted $bbe/$c2a transfer wrapper.
// The zero-count receive-until-NUL entry is outside these two callers. It retains
// prefixes during host waits and restores D0-D2 even when the source returns
// negative CCR after an abort. Data is a borrowed raw source/destination span.
type NativeTransportIOState struct {
	Started, Complete bool
	Routine           int
	Saved             [8]uint32
	Data              []byte
	Position          int
	Zero, Negative    bool
	Failure           error
}

func (s *NativeTransportIOState) Advance(routine int, data []byte, cb NativeTransportFrameCallbacks) (NativeSerialFrameChildResult, error) {
	if s == nil || cb.Frame == nil || !winMemoryValid(cb.Memory) || !validNativeSerialPort(cb.Port) {
		return NativeSerialFrameChildResult{}, fmt.Errorf("native transport transfer backing missing")
	}
	if !s.Started {
		s.Started = true
		s.Routine = routine
		s.Saved = cb.Frame.D
		s.Data = data
		if routine != 0xbbe && routine != 0xc2a {
			return NativeSerialFrameChildResult{}, fmt.Errorf("native transfer routine unsupported")
		}
		count := int(uint16(s.Saved[0]))
		if count == 0 && routine == 0xc2a {
			count = 65536
		}
		if count == 0 || len(data) != count {
			return NativeSerialFrameChildResult{}, fmt.Errorf("native counted transfer source span disagrees with D0.W")
		}
	}
	if s.Routine != routine {
		return NativeSerialFrameChildResult{}, fmt.Errorf("native transport operation changed across wait")
	}
	if s.Complete {
		return NativeSerialFrameChildResult{Complete: true, Zero: s.Zero, Negative: s.Negative}, nil
	}
	finish := func(negative bool, cause error) (NativeSerialFrameChildResult, error) {
		s.Complete = true
		s.Negative = negative && routine == 0xbbe
		s.Zero = !negative && routine == 0xbbe
		s.Failure = cause
		for i := 0; i < 3; i++ {
			cb.Frame.D[i] = s.Saved[i]
		}
		return NativeSerialFrameChildResult{Complete: true, Zero: s.Zero, Negative: s.Negative}, nil
	}
	abort, e := transportFrameAbortKeys(cb.Memory)
	if e != nil {
		return NativeSerialFrameChildResult{}, e
	}
	if abort {
		if e := transportFrameDisconnect(cb); e != nil {
			return NativeSerialFrameChildResult{}, e
		}
		return finish(true, nil)
	}
	var n int
	if routine == 0xbbe {
		n, e = cb.Port.Read(s.Data[s.Position:])
	} else {
		n, e = cb.Port.Write(s.Data[s.Position:])
	}
	if n < 0 || n > len(s.Data)-s.Position {
		return NativeSerialFrameChildResult{}, fmt.Errorf("native serial prefix count invalid")
	}
	s.Position += n
	if n > 0 && routine == 0xc2a {
		if e := cb.Memory.Write16(0x158, 1); e != nil {
			return NativeSerialFrameChildResult{}, e
		}
	}
	if n > 0 && routine == 0xbbe && cb.ReceiveImage == nil {
		read, e := cb.Memory.Read16(0x154)
		if e != nil {
			return NativeSerialFrameChildResult{}, e
		}
		limit, e := cb.Memory.Read16(0x15e)
		if e != nil {
			return NativeSerialFrameChildResult{}, e
		}
		for i := 0; i < n; i++ {
			if read == limit {
				read = 0
			} else {
				read++
			}
		}
		if e := cb.Memory.Write16(0x154, read); e != nil {
			return NativeSerialFrameChildResult{}, e
		}
	}
	if sync := transportFrameSyncReceive(cb); sync != nil {
		return NativeSerialFrameChildResult{}, sync
	}
	if e != nil && !errors.Is(e, ErrNativeSerialWait) {
		if disconnect := transportFrameDisconnect(cb); disconnect != nil {
			return NativeSerialFrameChildResult{}, disconnect
		}
		return finish(true, e)
	}
	if s.Position < len(s.Data) {
		return NativeSerialFrameChildResult{}, nil
	}
	return finish(false, nil)
}

// Available is $ae2: only D0.W changes, with actual TST.W flags. A negative
// host/keyboard return disconnects through $1826e instead of repairing data.
type nativeTransportAvailableResult struct {
	NativeSerialFrameChildResult
	Failure error
}

func transportFrameAvailable(cb NativeTransportFrameCallbacks) (nativeTransportAvailableResult, error) {
	if cb.Frame == nil || cb.Port.Available == nil {
		return nativeTransportAvailableResult{}, fmt.Errorf("native serial availability missing")
	}
	abort, e := transportFrameAbortKeys(cb.Memory)
	if e != nil {
		return nativeTransportAvailableResult{}, e
	}
	n := 0
	if !abort {
		n, e = cb.Port.Available()
		if errors.Is(e, ErrNativeSerialWait) {
			return nativeTransportAvailableResult{}, nil
		}
		if e != nil {
			abort = true
		}
	}
	if abort {
		if e := transportFrameDisconnect(cb); e != nil {
			return nativeTransportAvailableResult{}, e
		}
		cb.Frame.Word(0, 0xffff)
		return nativeTransportAvailableResult{NativeSerialFrameChildResult: NativeSerialFrameChildResult{Complete: true, Negative: true}, Failure: e}, nil
	}
	if e := transportFrameSyncReceive(cb); e != nil {
		return nativeTransportAvailableResult{}, e
	}
	if cb.ReceiveImage != nil {
		read, err := cb.Memory.Read16(0x154)
		if err != nil {
			return nativeTransportAvailableResult{}, err
		}
		write, err := cb.Memory.Read16(0x156)
		if err != nil {
			return nativeTransportAvailableResult{}, err
		}
		value := int16(write - read)
		if value < 0 {
			value = -value
		}
		n = int(uint16(value))
	}
	cb.Frame.Word(0, uint16(n))
	return nativeTransportAvailableResult{NativeSerialFrameChildResult: NativeSerialFrameChildResult{Complete: true, Zero: uint16(n) == 0, Negative: int16(n) < 0}}, nil
}
