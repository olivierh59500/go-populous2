package populous2

import "fmt"

type NativeDeferredFramePhase uint8

const (
	NativeDeferredFrameRead NativeDeferredFramePhase = iota
	NativeDeferredFrameTransport
	NativeDeferredFrameExecute
	NativeDeferredFrameClear
	NativeDeferredFrameDone
)

// NativeDeferredFrameState retains the enclosing $1744c MOVEM input, the live
// command continuation and the side/callback position across a pending UI or
// transport operation. The caller owns CallbackPhase's actual inner routine.
type NativeDeferredFrameState struct {
	Started       bool
	Caller        [8]uint32
	Command       NativeCommandRegisterContext
	Side          uint8
	Mode          uint8
	Phase         NativeDeferredFramePhase
	CallbackPhase uint32
}

type NativeDeferredFrameCallbacks struct {
	Memory    FollowerCleanupMemory
	Execute   func(int, *NativeCommandRegisterContext, *uint32) (bool, error)
	Transport func(int, uint8, *NativeCommandRegisterContext, *uint32) (bool, error)
}

// TickDeferredFrame is resumable original $1744c. Command-byte/XY clearing
// follows completed execution; neither a pending callback nor a later resume
// replays transport or an already completed side. The outer D registers are
// restored only when both original raw records have completed.
func (s *NativeDeferredFrameState) TickDeferredFrame(c *NativeFrameRegisterContext, cb NativeDeferredFrameCallbacks) (bool, error) {
	if s == nil || c == nil || !winMemoryValid(cb.Memory) {
		return false, fmt.Errorf("native deferred frame state/context missing")
	}
	if !s.Started {
		s.Started = true
		s.Caller = c.D
		s.Command = c.CommandContext()
		s.Phase = NativeDeferredFrameRead
	}
	for s.Phase != NativeDeferredFrameDone {
		address := 0xeb56 + int(s.Side)*10
		switch s.Phase {
		case NativeDeferredFrameRead:
			mode, e := cb.Memory.Read8(address + 8)
			if e != nil {
				return false, e
			}
			dispatch := [6]uint16{0x0c, 0x0e, 0x0e, 0x16, 0x3e, 0x80}
			if mode&1 != 0 || int(mode/2) >= len(dispatch) {
				return false, fmt.Errorf("native deferred frame transport byte%d outside dispatch", mode)
			}
			s.Mode = mode
			s.Command.D[0] = uint32(dispatch[mode/2])
			s.CallbackPhase = 0
			switch mode {
			case 0, 10:
				s.Phase = NativeDeferredFrameClear
			case 6, 8:
				s.Phase = NativeDeferredFrameTransport
			default:
				s.Phase = NativeDeferredFrameExecute
			}
		case NativeDeferredFrameTransport:
			if cb.Transport == nil {
				return false, fmt.Errorf("native deferred frame transport mode%d callback missing", s.Mode)
			}
			complete, e := cb.Transport(address, s.Mode, &s.Command, &s.CallbackPhase)
			if e != nil {
				return false, e
			}
			if !complete {
				return false, nil
			}
			s.CallbackPhase = 0
			s.Phase = NativeDeferredFrameExecute
		case NativeDeferredFrameExecute:
			if cb.Execute == nil {
				return false, fmt.Errorf("native deferred frame17500 callback missing")
			}
			complete, e := cb.Execute(address, &s.Command, &s.CallbackPhase)
			if e != nil {
				return false, e
			}
			if !complete {
				return false, nil
			}
			s.CallbackPhase = 0
			s.Phase = NativeDeferredFrameClear
		case NativeDeferredFrameClear:
			if e := cb.Memory.Write8(address+1, 0); e != nil {
				return false, e
			}
			if e := cb.Memory.Write16(address+2, 0); e != nil {
				return false, e
			}
			s.Side++
			if s.Side >= 2 {
				s.Phase = NativeDeferredFrameDone
			} else {
				s.Phase = NativeDeferredFrameRead
			}
		default:
			return false, fmt.Errorf("native deferred frame phase%d missing", s.Phase)
		}
	}
	c.D = s.Caller
	return true, nil
}
