package populous2

import (
	"encoding/binary"
	"errors"
	"fmt"

	"go-populous2/internal/amiga"
)

var ErrNativeSerialWait = errors.New("native serial operation is waiting")

// NativeSerialPort is an ordered byte stream. Read and Write can return a
// transferred prefix with ErrNativeSerialWait; controllers preserve that
// prefix and yield without replaying simulation or earlier commands.
type NativeSerialPort struct {
	Available func() (int, error)
	Read      func([]byte) (int, error)
	Write     func([]byte) (int, error)
	Flush     func() error
	Configure func(uint16) error
}

type NativeSerialMessage uint8

const (
	NativeSerialConnecting NativeSerialMessage = iota
	NativeSerialSignatureError
	NativeSerialSamePlayer
	NativeSerialWaitingPlayer
	NativeSerialLandscapeMismatch
)

type NativeSerialRules struct {
	Presentation  *NativePresentation
	BaudRates     [5]uint16
	baudText      [5][]byte
	messages      [5][]byte
	mismatch      []byte
	Signature     [4]byte
	Token         byte
	DefaultCustom [250]byte
}

func DecodeNativeSerialRules(exe *amiga.Executable) (*NativeSerialRules, error) {
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x2072a {
		return nil, fmt.Errorf("native serial tables missing")
	}
	p, err := DecodeNativePresentation(exe)
	if err != nil {
		return nil, err
	}
	r := &NativeSerialRules{Presentation: p}
	code := exe.Hunks[0].Data
	r.Token = code[0x182c0]
	copy(r.Signature[:], code[0x182c2:0x182c6])
	if r.Token != 0x3f || string(r.Signature[:]) != "ABCD" {
		return nil, fmt.Errorf("native serial handshake signature unsupported")
	}
	for i := range r.BaudRates {
		at := 0x4b06 + i*8
		r.BaudRates[i] = binary.BigEndian.Uint16(code[at+6:])
		for end := at; end < at+6; end++ {
			if code[end] == 0 {
				r.baudText[i] = append([]byte(nil), code[at:end]...)
				break
			}
		}
		if r.baudText[i] == nil {
			return nil, fmt.Errorf("native baud label missing")
		}
	}
	for i, at := range []int{0xa9b3, 0xa97c, 0xa999, 0xa9d0, 0xa9ee} {
		end := at
		for end < len(code) && code[end] != 0 {
			end++
		}
		if end == len(code) || end-at > 80 {
			return nil, fmt.Errorf("native serial message missing")
		}
		r.messages[i] = append([]byte(nil), code[at:end]...)
	}
	// $17f24 writes the attempt number into this existing message buffer.
	r.messages[NativeSerialConnecting] = append([]byte(nil), code[0xa9b3:0xa9ca]...)
	r.mismatch, err = NativeRequesterTemplate(exe, 0x86f0)
	if err != nil {
		return nil, err
	}
	copy(r.DefaultCustom[:], code[0x20630:0x2072a])
	return r, nil
}

func (r *NativeSerialRules) Message(kind NativeSerialMessage, attempt uint16) (*NativeRequester, error) {
	if r == nil || r.Presentation == nil || int(kind) >= len(r.messages) {
		return nil, fmt.Errorf("native serial presentation missing")
	}
	text := r.messages[kind]
	if kind == NativeSerialConnecting {
		text = append(append([]byte(nil), text...), r.Presentation.decimal(uint32(int32(int16(attempt))))...)
	}
	if kind == NativeSerialLandscapeMismatch {
		return r.Presentation.Requesters.Compile(r.mismatch, [][]byte{text})
	}
	return r.Presentation.Compile(NativeMenuMessage, [][]byte{text})
}

type NativeSerialCallbacks struct {
	Memory FollowerCleanupMemory
	Port   NativeSerialPort
	// Message returns false until the real error dialog is acknowledged.
	Message func(NativeSerialMessage) (bool, error)
	// SwitchProfile executes the original $111ae body when requested.
	SwitchProfile func(uint16) error
}

func validNativeSerialPort(p NativeSerialPort) bool {
	return p.Available != nil && p.Read != nil && p.Write != nil && p.Flush != nil
}

// DisconnectNativeSerial translates $1826e. The third ten-byte probe aliases
// the command pointer/redraw header at $eb6a; it is not silently limited to
// the two logical player records. The ring flush is a real host operation.
func DisconnectNativeSerial(cb NativeSerialCallbacks) error {
	if !winMemoryValid(cb.Memory) || cb.Port.Flush == nil {
		return fmt.Errorf("native serial disconnect callbacks missing")
	}
	if err := cb.Port.Flush(); err != nil {
		return err
	}
	for _, at := range []int{0xeb56, 0xeb60, 0xeb6a} {
		mode, err := cb.Memory.Read8(at + 8)
		if err != nil {
			return err
		}
		if mode == 0 {
			continue
		}
		next := uint8(4)
		if mode == 2 || mode == 6 {
			next = 2
		}
		if err := cb.Memory.Write8(at+8, next); err != nil {
			return err
		}
	}
	return cb.Memory.Write16(0xeb44, 4)
}

type nativeSerialPacketPhase uint8

const (
	serialPacketPrepare nativeSerialPacketPhase = iota
	serialPacketTransfer
	serialPacketCompare
	serialPacketMessage
	serialPacketDone
)

type NativeSerialPacket struct {
	Caller       int
	Mode         uint8
	phase        nativeSerialPacketPhase
	position     int
	payload      [8]byte
	Failure      error
	Disconnected bool
}
type NativeSerialPacketStep struct {
	Waiting, Complete, Disconnected bool
	Transferred                     int
	Failure                         error
}

func NewNativeSerialPacket(caller int, mode uint8) (*NativeSerialPacket, error) {
	if mode != 6 && mode != 8 {
		return nil, fmt.Errorf("native serial prelude requires transport6/8")
	}
	return &NativeSerialPacket{Caller: caller, Mode: mode}, nil
}

// Advance is $17494/$174bc through the call to $17500. It updates the actual
// caller D0: sender8, receiver's raw RNG long. A mismatch disconnects before
// allowing command execution; it never adopts the received RNG as a repair.
func (p *NativeSerialPacket) Advance(c *NativeCommandRegisterContext, cb NativeSerialCallbacks) (NativeSerialPacketStep, error) {
	var step NativeSerialPacketStep
	if p == nil || c == nil || !winMemoryValid(cb.Memory) || !validNativeSerialPort(cb.Port) {
		return step, fmt.Errorf("native serial packet callbacks missing")
	}
	for {
		switch p.phase {
		case serialPacketPrepare:
			c.D[0] = 8
			if p.Mode == 6 {
				rng, err := cb.Memory.Read32(0xeb28)
				if err != nil {
					return step, err
				}
				if err := cb.Memory.Write32(p.Caller+4, rng); err != nil {
					return step, err
				}
			}
			for i := range p.payload {
				v, err := cb.Memory.Read8(p.Caller + i)
				if err != nil {
					return step, err
				}
				p.payload[i] = v
			}
			p.phase = serialPacketTransfer
		case serialPacketTransfer:
			var n int
			var err error
			if p.Mode == 6 {
				n, err = cb.Port.Write(p.payload[p.position:])
			} else {
				n, err = cb.Port.Read(p.payload[p.position:])
			}
			if n < 0 || n > len(p.payload)-p.position {
				return step, fmt.Errorf("native serial host returned invalid byte count")
			}
			if p.Mode == 8 {
				for i := 0; i < n; i++ {
					if e := cb.Memory.Write8(p.Caller+p.position+i, p.payload[p.position+i]); e != nil {
						return step, e
					}
				}
			}
			p.position += n
			if err != nil && !errors.Is(err, ErrNativeSerialWait) {
				p.Failure = err
				if e := DisconnectNativeSerial(cb); e != nil {
					return step, e
				}
				p.Disconnected = true
				p.phase = serialPacketCompare
			} else if p.position == len(p.payload) {
				p.phase = serialPacketCompare
			} else {
				return NativeSerialPacketStep{Waiting: true, Transferred: p.position}, nil
			}
		case serialPacketCompare:
			if p.Mode == 8 {
				got, err := cb.Memory.Read32(p.Caller + 4)
				if err != nil {
					return step, err
				}
				c.D[0] = got
				want, err := cb.Memory.Read32(0xeb28)
				if err != nil {
					return step, err
				}
				if got != want {
					p.phase = serialPacketMessage
					continue
				}
			}
			p.phase = serialPacketDone
		case serialPacketMessage:
			if cb.Message == nil {
				return step, fmt.Errorf("native RNG mismatch dialog continuation missing")
			}
			ack, err := cb.Message(NativeSerialLandscapeMismatch)
			if err != nil {
				return step, err
			}
			if !ack {
				return NativeSerialPacketStep{Waiting: true, Transferred: p.position, Failure: p.Failure}, nil
			}
			if err := DisconnectNativeSerial(cb); err != nil {
				return step, err
			}
			p.Disconnected = true
			p.phase = serialPacketDone
		case serialPacketDone:
			return NativeSerialPacketStep{Complete: true, Transferred: p.position, Disconnected: p.Disconnected, Failure: p.Failure}, nil
		default:
			return step, fmt.Errorf("native serial packet phase invalid")
		}
	}
}

type NativeSerialDeferredCallbacks struct {
	Serial  NativeSerialCallbacks
	Execute func(int, *NativeCommandRegisterContext) error
}

// NativeSerialDeferred retains the exact $1744c slot/context continuation
// across a host I/O wait. Begin/Advance belongs after the once-only physics,
// effects and scenario stages; callers must not rerun them while Waiting.
type NativeSerialDeferred struct {
	Context  NativeCommandRegisterContext
	Side     int
	packet   *NativeSerialPacket
	prepared bool
	Complete bool
	Executed int
}
type NativeSerialDeferredStep struct {
	Waiting, Complete bool
	Executed          int
	Failure           error
}

func NewNativeSerialDeferred(input NativeCommandRegisterContext) *NativeSerialDeferred {
	return &NativeSerialDeferred{Context: input}
}

func (s *NativeSerialDeferred) Advance(cb NativeSerialDeferredCallbacks) (NativeSerialDeferredStep, error) {
	var step NativeSerialDeferredStep
	if s == nil || !winMemoryValid(cb.Serial.Memory) {
		return step, fmt.Errorf("native serial deferred memory missing")
	}
	for s.Side < 2 {
		address := 0xeb56 + s.Side*10
		mode, err := cb.Serial.Memory.Read8(address + 8)
		if err != nil {
			return step, err
		}
		if !s.prepared {
			dispatch := [6]uint16{0x0c, 0x0e, 0x0e, 0x16, 0x3e, 0x80}
			if mode&1 != 0 || int(mode/2) >= len(dispatch) {
				return step, fmt.Errorf("native serial deferred transport%d outside dispatch", mode)
			}
			s.Context.D[0] = uint32(dispatch[mode/2])
			if mode == 6 || mode == 8 {
				s.packet, err = NewNativeSerialPacket(address, mode)
				if err != nil {
					return step, err
				}
			}
			s.prepared = true
		}
		if s.packet != nil {
			packetStep, err := s.packet.Advance(&s.Context, cb.Serial)
			if err != nil {
				return step, err
			}
			if packetStep.Waiting {
				return NativeSerialDeferredStep{Waiting: true, Executed: s.Executed, Failure: packetStep.Failure}, nil
			}
			step.Failure = packetStep.Failure
		}
		if mode == 2 || mode == 4 || mode == 6 || mode == 8 {
			if cb.Execute == nil {
				return step, fmt.Errorf("native serial deferred command executor missing")
			}
			if err := cb.Execute(address, &s.Context); err != nil {
				return step, err
			}
			s.Executed++
		}
		if err := cb.Serial.Memory.Write8(address+1, 0); err != nil {
			return step, err
		}
		if err := cb.Serial.Memory.Write16(address+2, 0); err != nil {
			return step, err
		}
		s.Side++
		s.prepared = false
		s.packet = nil
	}
	s.Complete = true
	step.Complete, step.Executed = true, s.Executed
	return step, nil
}

// NativeSerialResume represents the menu's $181c0 packet, including the
// speed overwrite and rule-word OR. Its error branch leaves ten native stack
// bytes unpopped; this is reported explicitly instead of claiming a successful
// return or silently repairing source control flow.
type NativeSerialResume struct {
	phase        uint8
	position     int
	payload      [10]byte
	Complete     bool
	Failure      error
	Disconnected bool
	nativeReturn uint32
}
type NativeSerialResumeStep struct {
	Waiting, Complete bool
	Disconnected      bool
	Failure           error
	NativeStackReturn uint32
}

func (s *NativeSerialResume) Advance(cb NativeSerialCallbacks) (NativeSerialResumeStep, error) {
	var step NativeSerialResumeStep
	if s == nil || !winMemoryValid(cb.Memory) {
		return step, fmt.Errorf("native serial resume callbacks missing")
	}
	for {
		switch s.phase {
		case 0:
			pointer, err := cb.Memory.Read32(0xeb6a)
			if err != nil {
				return step, err
			}
			mode, err := cb.Memory.Read8(int(pointer) + 8)
			if err != nil {
				return step, err
			}
			if mode != 6 && mode != 8 {
				s.phase = 3
				continue
			}
			if !validNativeSerialPort(cb.Port) {
				return step, fmt.Errorf("native serial resume port missing")
			}
			for i, at := range []int{0xeb42, 0xe90c, 0xea46, 0xeb2c, 0xeb2e} {
				word, err := cb.Memory.Read16(at)
				if err != nil {
					return step, err
				}
				binary.BigEndian.PutUint16(s.payload[i*2:], word)
			}
			s.phase = 1
		case 1:
			n, err := cb.Port.Write(s.payload[s.position:])
			if n < 0 || n > 10-s.position {
				return step, fmt.Errorf("native menu serial write count invalid")
			}
			s.position += n
			if err != nil && !errors.Is(err, ErrNativeSerialWait) {
				if e := DisconnectNativeSerial(cb); e != nil {
					return step, e
				}
				s.Failure, s.Disconnected = err, true
				// $0c2a's abort return is not inspected by $181c0. It still
				// reaches the availability check with its local stack packet.
				s.phase, s.position = 2, 0
				continue
			}
			if s.position < 10 {
				return NativeSerialResumeStep{Waiting: true}, nil
			}
			s.phase = 2
			s.position = 0
		case 2:
			available, err := cb.Port.Available()
			if err != nil {
				if e := DisconnectNativeSerial(cb); e != nil {
					return step, e
				}
				s.Failure, s.Disconnected = err, true
				s.nativeReturn = binary.BigEndian.Uint32(s.payload[:])
				s.phase = 6
				continue
			}
			if available < 10 {
				return NativeSerialResumeStep{Waiting: true}, nil
			}
			s.phase = 4
		case 4:
			n, err := cb.Port.Read(s.payload[s.position:])
			if n < 0 || n > 10-s.position {
				return step, fmt.Errorf("native menu serial read count invalid")
			}
			s.position += n
			if err != nil && !errors.Is(err, ErrNativeSerialWait) {
				if e := DisconnectNativeSerial(cb); e != nil {
					return step, e
				}
				s.Failure, s.Disconnected = err, true
				// The caller does not inspect $0bbe's failed-read flags.
				// Remaining stack bytes retain the local outgoing values.
				s.phase = 5
				continue
			}
			if s.position < 10 {
				return NativeSerialResumeStep{Waiting: true}, nil
			}
			s.phase = 5
		case 5:
			profile := binary.BigEndian.Uint16(s.payload[:])
			local, err := cb.Memory.Read16(0xeb42)
			if err != nil {
				return step, err
			}
			if profile == local {
				if cb.SwitchProfile == nil {
					return step, fmt.Errorf("native menu profile-switch continuation missing")
				}
				if err := cb.SwitchProfile(profile); err != nil {
					return step, err
				}
			}
			for i, at := range []int{0xe90c, 0xea46} {
				if err := cb.Memory.Write16(at, binary.BigEndian.Uint16(s.payload[2+i*2:])); err != nil {
					return step, err
				}
			}
			for i, at := range []int{0xeb2c, 0xeb2e} {
				old, err := cb.Memory.Read16(at)
				if err != nil {
					return step, err
				}
				if err := cb.Memory.Write16(at, old|binary.BigEndian.Uint16(s.payload[6+i*2:])); err != nil {
					return step, err
				}
			}
			s.phase = 3
		case 3:
			s.Complete = true
			return NativeSerialResumeStep{Complete: true, Failure: s.Failure, Disconnected: s.Disconnected}, nil
		case 6:
			return NativeSerialResumeStep{Failure: s.Failure, Disconnected: s.Disconnected, NativeStackReturn: s.nativeReturn}, nil
		default:
			return step, fmt.Errorf("native menu serial phase invalid")
		}
	}
}
