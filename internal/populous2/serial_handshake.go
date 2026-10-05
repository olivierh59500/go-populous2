package populous2

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

type NativeSerialHandshakeCallbacks struct {
	Serial       NativeSerialCallbacks
	FrameCounter func() uint32 // Original interrupt counter at BSS+$12.
	// WaitCPU represents the two explicit100000-iteration source loops.
	// The host owns elapsed CPU timing; site identifies17fb2 or17fc6 so a
	// resumed wait is not confused with the following new wait.
	WaitCPU func(site, iterations uint32) (bool, error)
	Cancel  func() bool // Actual nonzero requester action at source poll sites.
	// Initialize performs10ad8 using the negotiated memory. False means that
	// its real world/UI continuation is pending, never an initialized World.
	Initialize func() (bool, error)
}

type nativeSerialHandshakePhase uint8

const (
	serialHandshakeStart nativeSerialHandshakePhase = iota
	serialHandshakeBeacon
	serialHandshakeWaitCounter
	serialHandshakeWrite
	serialHandshakeTokenAvailable
	serialHandshakeTokenRead
	serialHandshakeWaitCPU1
	serialHandshakeFlush
	serialHandshakeWaitCPU2
	serialHandshakeSignatureAvailable
	serialHandshakeSignatureRead
	serialHandshakeProfileAvailable
	serialHandshakeProfileRead
	serialHandshakeHeaderRead
	serialHandshakeGodRead
	serialHandshakeApply
	serialHandshakeInitialize
	serialHandshakeMessage
	serialHandshakeFinished
)

type NativeSerialHandshake struct {
	Rules                       *NativeSerialRules
	Baud                        uint16
	Attempt                     uint16
	SignatureBudget             int16
	PeerProfile                 uint8
	Connected, Ready, Cancelled bool
	Failure                     error
	phase, afterWrite           nativeSerialHandshakePhase
	deadline                    uint32
	data                        []byte
	position, target, remaining int
	message                     NativeSerialMessage
}
type NativeSerialHandshakeStep struct {
	Waiting, Complete, Connected, Ready, Cancelled bool
	Attempt                                        uint16
	Message                                        NativeSerialMessage
	Failure                                        error
}

func NewNativeSerialHandshake(r *NativeSerialRules, baud uint16) (*NativeSerialHandshake, error) {
	if r == nil || baud == 0 {
		return nil, fmt.Errorf("native serial handshake rules/baud missing")
	}
	return &NativeSerialHandshake{Rules: r, Baud: baud, SignatureBudget: -10}, nil
}
func (h *NativeSerialHandshake) status(wait bool) NativeSerialHandshakeStep {
	return NativeSerialHandshakeStep{Waiting: wait, Complete: h.phase == serialHandshakeFinished, Connected: h.Connected, Ready: h.Ready, Cancelled: h.Cancelled, Attempt: h.Attempt, Message: h.message, Failure: h.Failure}
}
func (h *NativeSerialHandshake) write(data []byte, after nativeSerialHandshakePhase) {
	h.data = append([]byte(nil), data...)
	h.position = 0
	h.afterWrite = after
	h.phase = serialHandshakeWrite
}
func serialReadBytes(m FollowerCleanupMemory, at, count int) ([]byte, error) {
	b := make([]byte, count)
	for i := range b {
		v, err := m.Read8(at + i)
		if err != nil {
			return nil, err
		}
		b[i] = v
	}
	return b, nil
}
func (h *NativeSerialHandshake) interrupted(err error, cb NativeSerialHandshakeCallbacks) error {
	h.Failure = err
	h.Connected, h.Ready = false, false
	h.phase = serialHandshakeFinished
	return DisconnectNativeSerial(cb.Serial)
}
func (h *NativeSerialHandshake) cancelled(cb NativeSerialHandshakeCallbacks) bool {
	if cb.Cancel != nil && cb.Cancel() {
		h.Cancelled = true
		h.phase = serialHandshakeFinished
		return true
	}
	return false
}
func (h *NativeSerialHandshake) available(count int, cb NativeSerialHandshakeCallbacks) (bool, error) {
	n, err := cb.Serial.Port.Available()
	if err != nil {
		return false, h.interrupted(err, cb)
	}
	return n >= count, nil
}
func (h *NativeSerialHandshake) readFixed(count int, cb NativeSerialHandshakeCallbacks) (bool, error) {
	n, err := cb.Serial.Port.Read(h.data[h.position:count])
	if n < 0 || n > count-h.position {
		return false, fmt.Errorf("native handshake read count invalid")
	}
	h.position += n
	if err != nil && !errors.Is(err, ErrNativeSerialWait) {
		return false, h.interrupted(err, cb)
	}
	return h.position == count, nil
}

// Advance translates17eec..181ac with explicit byte-stream and UI/CPU waits.
// It mutates received session/profile bytes immediately, as the original
// one-byte loops do. No inferred seed, rollback, profile normalization or
// successful initialization is added to a missing/pending host continuation.
func (h *NativeSerialHandshake) Advance(cb NativeSerialHandshakeCallbacks) (NativeSerialHandshakeStep, error) {
	var empty NativeSerialHandshakeStep
	if h == nil || h.Rules == nil || !winMemoryValid(cb.Serial.Memory) || !validNativeSerialPort(cb.Serial.Port) || cb.FrameCounter == nil || cb.WaitCPU == nil {
		return empty, fmt.Errorf("native handshake callbacks missing")
	}
	m := cb.Serial.Memory
	for transitions := 0; transitions < 64; transitions++ {
		switch h.phase {
		case serialHandshakeStart:
			if cb.Serial.Port.Configure == nil {
				return empty, fmt.Errorf("native serial baud configuration missing")
			}
			if err := cb.Serial.Port.Configure(h.Baud); err != nil {
				return empty, err
			}
			h.Connected, h.Ready = false, false
			h.SignatureBudget = -10
			h.Attempt = 0
			if err := cb.Serial.Port.Flush(); err != nil {
				return empty, err
			}
			h.phase = serialHandshakeBeacon
		case serialHandshakeBeacon:
			h.Attempt++
			h.message = NativeSerialConnecting
			if h.cancelled(cb) {
				return h.status(false), nil
			}
			h.deadline = cb.FrameCounter() + 6
			h.phase = serialHandshakeWaitCounter
		case serialHandshakeWaitCounter:
			// BGE compares the signed long words, including counter wrap.
			if int32(h.deadline) >= int32(cb.FrameCounter()) {
				return h.status(true), nil
			}
			h.write([]byte{h.Rules.Token}, serialHandshakeTokenAvailable)
		case serialHandshakeWrite:
			n, err := cb.Serial.Port.Write(h.data[h.position:])
			if n < 0 || n > len(h.data)-h.position {
				return empty, fmt.Errorf("native handshake write count invalid")
			}
			h.position += n
			if err != nil && !errors.Is(err, ErrNativeSerialWait) {
				if e := h.interrupted(err, cb); e != nil {
					return empty, e
				}
				return h.status(false), nil
			}
			if h.position < len(h.data) {
				return h.status(true), nil
			}
			h.phase = h.afterWrite
			h.position = 0
			h.data = nil
		case serialHandshakeTokenAvailable:
			ready, err := h.available(1, cb)
			if err != nil {
				return empty, err
			}
			if h.phase == serialHandshakeFinished {
				return h.status(false), nil
			}
			if !ready {
				h.phase = serialHandshakeBeacon
				continue
			}
			h.data = make([]byte, 1)
			h.phase = serialHandshakeTokenRead
		case serialHandshakeTokenRead:
			complete, err := h.readFixed(1, cb)
			if err != nil {
				return empty, err
			}
			if h.phase == serialHandshakeFinished {
				return h.status(false), nil
			}
			if !complete {
				return h.status(true), nil
			}
			if h.data[0] != h.Rules.Token {
				h.phase = serialHandshakeBeacon
				continue
			}
			h.phase = serialHandshakeWaitCPU1
		case serialHandshakeWaitCPU1:
			ready, err := cb.WaitCPU(0x17fb2, 100000)
			if err != nil {
				return empty, err
			}
			if !ready {
				return h.status(true), nil
			}
			h.phase = serialHandshakeFlush
		case serialHandshakeFlush:
			if err := cb.Serial.Port.Flush(); err != nil {
				return empty, err
			}
			h.phase = serialHandshakeWaitCPU2
		case serialHandshakeWaitCPU2:
			ready, err := cb.WaitCPU(0x17fc6, 100000)
			if err != nil {
				return empty, err
			}
			if !ready {
				return h.status(true), nil
			}
			h.write(h.Rules.Signature[:], serialHandshakeSignatureAvailable)
		case serialHandshakeSignatureAvailable:
			if h.cancelled(cb) {
				return h.status(false), nil
			}
			ready, err := h.available(4, cb)
			if err != nil {
				return empty, err
			}
			if h.phase == serialHandshakeFinished {
				return h.status(false), nil
			}
			if !ready {
				return h.status(true), nil
			}
			h.data = make([]byte, 4)
			h.position = 0
			h.phase = serialHandshakeSignatureRead
		case serialHandshakeSignatureRead:
			complete, err := h.readFixed(4, cb)
			if err != nil {
				return empty, err
			}
			if h.phase == serialHandshakeFinished {
				return h.status(false), nil
			}
			if !complete {
				return h.status(true), nil
			}
			if !bytes.Equal(h.data, h.Rules.Signature[:]) {
				h.SignatureBudget++
				if h.SignatureBudget == 0 {
					h.message = NativeSerialSignatureError
					h.Failure = fmt.Errorf("native serial signature retries exhausted")
					h.phase = serialHandshakeMessage
					continue
				}
				h.Attempt = 0
				if err := cb.Serial.Port.Flush(); err != nil {
					return empty, err
				}
				h.phase = serialHandshakeBeacon
				continue
			}
			profile, err := m.Read8(0xeb43)
			if err != nil {
				return empty, err
			}
			h.write([]byte{profile}, serialHandshakeProfileAvailable)
		case serialHandshakeProfileAvailable:
			if h.cancelled(cb) {
				return h.status(false), nil
			}
			ready, err := h.available(1, cb)
			if err != nil {
				return empty, err
			}
			if h.phase == serialHandshakeFinished {
				return h.status(false), nil
			}
			if !ready {
				return h.status(true), nil
			}
			h.data = make([]byte, 1)
			h.position = 0
			h.phase = serialHandshakeProfileRead
		case serialHandshakeProfileRead:
			complete, err := h.readFixed(1, cb)
			if err != nil {
				return empty, err
			}
			if h.phase == serialHandshakeFinished {
				return h.status(false), nil
			}
			if !complete {
				return h.status(true), nil
			}
			h.PeerProfile = h.data[0]
			profile, err := m.Read8(0xeb43)
			if err != nil {
				return empty, err
			}
			if profile == h.PeerProfile {
				h.message = NativeSerialSamePlayer
				h.Failure = fmt.Errorf("native serial peer selected the same player")
				h.phase = serialHandshakeMessage
				continue
			}
			if profile == 2 {
				h.target, h.remaining = 0xeb22, 14
				h.phase = serialHandshakeHeaderRead
				continue
			}
			data, err := serialReadBytes(m, 0xeb22, 14)
			if err != nil {
				return empty, err
			}
			h.write(data, serialHandshakeGodRead)
			// Side1 sends its profile before beginning the common receive.
			h.target, h.remaining = 0xe8f2, -236
		case serialHandshakeHeaderRead:
			if h.cancelled(cb) {
				return h.status(false), nil
			}
			ready, err := h.available(1, cb)
			if err != nil {
				return empty, err
			}
			if h.phase == serialHandshakeFinished {
				return h.status(false), nil
			}
			if !ready {
				return h.status(true), nil
			}
			one := []byte{0}
			n, err := cb.Serial.Port.Read(one)
			if n < 0 || n > 1 {
				return empty, fmt.Errorf("native header byte count invalid")
			}
			if n == 1 {
				if e := m.Write8(h.target, one[0]); e != nil {
					return empty, e
				}
				h.target++
				h.remaining--
			}
			if err != nil && !errors.Is(err, ErrNativeSerialWait) {
				if e := h.interrupted(err, cb); e != nil {
					return empty, e
				}
				return h.status(false), nil
			}
			if h.remaining != 0 {
				if n == 0 {
					return h.status(true), nil
				}
				continue
			}
			data, err := serialReadBytes(m, 0xea2c, 236)
			if err != nil {
				return empty, err
			}
			h.write(data, serialHandshakeGodRead)
			h.target, h.remaining = 0xe8f2, 236
		case serialHandshakeGodRead:
			if h.remaining < 0 {
				data, err := serialReadBytes(m, h.target, -h.remaining)
				if err != nil {
					return empty, err
				}
				h.write(data, serialHandshakeGodRead)
				h.target, h.remaining = 0xea2c, 236
				continue
			}
			if h.cancelled(cb) {
				return h.status(false), nil
			}
			ready, err := h.available(1, cb)
			if err != nil {
				return empty, err
			}
			if h.phase == serialHandshakeFinished {
				return h.status(false), nil
			}
			if !ready {
				return h.status(true), nil
			}
			one := []byte{0}
			n, err := cb.Serial.Port.Read(one)
			if n < 0 || n > 1 {
				return empty, fmt.Errorf("native god byte count invalid")
			}
			if n == 1 {
				if e := m.Write8(h.target, one[0]); e != nil {
					return empty, e
				}
				h.target++
				h.remaining--
			}
			if err != nil && !errors.Is(err, ErrNativeSerialWait) {
				if e := h.interrupted(err, cb); e != nil {
					return empty, e
				}
				return h.status(false), nil
			}
			if h.remaining != 0 {
				if n == 0 {
					return h.status(true), nil
				}
				continue
			}
			h.phase = serialHandshakeApply
		case serialHandshakeApply:
			profile, err := m.Read16(0xeb42)
			if err != nil {
				return empty, err
			}
			roles := [2]uint8{6, 8}
			if profile != 1 {
				roles = [2]uint8{8, 6}
			}
			for i, role := range roles {
				if err := m.Write8(0xeb5e+i*10, role); err != nil {
					return empty, err
				}
			}
			if err := m.Write16(0xeb44, 6); err != nil {
				return empty, err
			}
			seed, err := m.Read32(0xeb24)
			if err != nil {
				return empty, err
			}
			if err := m.Write32(0xeb28, seed); err != nil {
				return empty, err
			}
			h.Connected = true
			h.phase = serialHandshakeInitialize
		case serialHandshakeInitialize:
			if cb.Initialize == nil {
				return empty, fmt.Errorf("native multiplayer world initializer missing")
			}
			ready, err := cb.Initialize()
			if err != nil {
				return empty, err
			}
			if !ready {
				return h.status(true), nil
			}
			h.Ready = true
			h.phase = serialHandshakeFinished
		case serialHandshakeMessage:
			if cb.Serial.Message == nil {
				return empty, fmt.Errorf("native handshake error dialog continuation missing")
			}
			ack, err := cb.Serial.Message(h.message)
			if err != nil {
				return empty, err
			}
			if !ack {
				return h.status(true), nil
			}
			h.phase = serialHandshakeFinished
		case serialHandshakeFinished:
			return h.status(false), nil
		default:
			return empty, fmt.Errorf("native handshake phase invalid")
		}
	}
	return h.status(true), nil
}

// AccumulateCustomPowers is $11078 followed by $10d98's custom-record copy.
// Original custom defaults retain unlocked flags between invocations. The
// selected side contributes its36 signed raw flags from every completed
// five-world record, including the current record; both default sides are ORed.
func (r *NativeSerialRules) AccumulateCustomPowers(previous [250]byte, world, profile uint16, campaign []byte, landscape uint16, seedLow uint16) ([250]byte, error) {
	result := previous
	if r == nil || int(world/5+1)*250 > len(campaign) {
		return result, fmt.Errorf("native custom power history outside campaign")
	}
	start := 22
	if profile != 1 {
		start = 80
	}
	for record := 0; record <= int(world/5); record++ {
		for i := 0; i < 36; i++ {
			flag := campaign[record*250+start+i]
			result[22+i] |= flag
			result[80+i] |= flag
		}
	}
	binary.BigEndian.PutUint16(result[182:], landscape)
	binary.BigEndian.PutUint16(result[184:], seedLow)
	return result, nil
}
