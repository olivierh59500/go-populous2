package populous2

import "fmt"

const (
	NativeStartupAudioBytes      = 0x1e0dc
	NativeStartupBackgroundBytes = 0x7d00
)

type NativeStartupAllocationCall struct {
	Size, Flags, Address uint32
	Free                 bool
	Frame                *NativeFrameRegisterContext
}
type NativeStartupAllocationResult struct {
	Complete bool
	Value    uint32
}
type NativeStartupAllocationCallbacks struct {
	Code, Memory FollowerCleanupMemory
	CodeBase     uint32
	Frame        *NativeFrameRegisterContext
	// ExecBase is the actual configured library pointer read from physical4.
	// It is distinct from BSS4 in the input backing.
	ReadExecBase func() (uint32, error)
	Allocate     func(NativeStartupAllocationCall, *uint32) (NativeStartupAllocationResult, error)
	Call         func(NativeFileFrameCall, *uint32) (NativeCommandFrameResult, error)
}

// NativeStartupAllocationState executes1A43E (allocation/load) or1A4BC
// (release). Host allocation and resource children can retain genuine waits.
type NativeStartupAllocationState struct {
	Started, Complete bool
	Routine, PC       int
	D                 [8]uint32
	A                 [7]NativeRequesterAddress
	Active            bool
	Phase             uint32
	allocCall         NativeStartupAllocationCall
	Failed            error
}

func (s *NativeStartupAllocationState) Advance(routine int, cb NativeStartupAllocationCallbacks) (complete bool, failure error) {
	if s == nil || cb.Frame == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) {
		return false, fmt.Errorf("native startup allocation backing missing")
	}
	if s.Failed != nil {
		return false, s.Failed
	}
	if s.Complete {
		return true, nil
	}
	if !s.Started {
		if routine != 0x1a43e && routine != 0x1a4bc {
			return false, fmt.Errorf("native allocation routine%x unavailable", routine)
		}
		s.Started = true
		s.Routine, s.PC = routine, routine
		s.D = cb.Frame.D
	} else if routine != s.Routine {
		return false, fmt.Errorf("native allocation invocation changed")
	}
	cb.Frame.D = s.D
	defer func() {
		s.D = cb.Frame.D
		if failure != nil {
			s.Failed = failure
		}
	}()
	c := cb.Frame
	exec := func() error {
		if cb.ReadExecBase == nil {
			return fmt.Errorf("native physical ExecBase input missing")
		}
		value, err := cb.ReadExecBase()
		if err == nil {
			s.A[6] = NativeRequesterAddress{Address: value, Absolute: true}
		}
		return err
	}
	allocate := func(size uint32, free bool, next int) (bool, error) {
		if cb.Allocate == nil {
			return false, fmt.Errorf("native host allocation operation missing")
		}
		if !s.Active {
			if err := exec(); err != nil {
				return false, err
			}
			if !free {
				c.D[1] = 2
			}
			c.D[0] = size
			s.allocCall = NativeStartupAllocationCall{Size: size, Flags: c.D[1], Free: free, Frame: c}
			if free {
				s.allocCall.Address = s.A[1].Address
			}
			s.Active = true
		}
		s.allocCall.Frame = c
		result, err := cb.Allocate(s.allocCall, &s.Phase)
		if err != nil {
			return false, err
		}
		if !result.Complete {
			return false, nil
		}
		c.D[0] = result.Value
		s.Active = false
		s.Phase = 0
		s.PC = next
		return true, nil
	}
	load := func(next int) (bool, error) {
		if cb.Call == nil {
			return false, fmt.Errorf("native allocation resource19CD0 child missing")
		}
		result, err := cb.Call(NativeFileFrameCall{Routine: 0x19cd0, Frame: c, A: s.A, Arguments: 1<<0 | 1<<6}, &s.Phase)
		if err != nil {
			return false, err
		}
		if !result.Complete {
			return false, nil
		}
		s.Phase = 0
		s.PC = next
		return true, nil
	}
	for transitions := 0; transitions < 20; transitions++ {
		switch s.PC {
		case 0x1a43e:
			done, err := allocate(NativeStartupAudioBytes, false, 0x1a450)
			if err != nil || !done {
				return false, err
			}
		case 0x1a450:
			if int32(c.D[0]) <= 0 {
				s.PC = 0x1a478
				continue
			}
			if err := cb.Memory.Write32(0x3b4, c.D[0]); err != nil {
				return false, err
			}
			c.Word(1, 13)
			c.D[1] = uint32(uint16(c.D[1])) * 48
			s.A[0] = NativeRequesterAddress{Address: cb.CodeBase + 0x19e4a, Code: true}
			if err := cb.Code.Write32(0x19e4a+int(int16(c.D[1])), c.D[0]); err != nil {
				return false, err
			}
			c.Word(0, 13)
			s.PC = 0x1a472
		case 0x1a472:
			done, err := load(0x1a478)
			if err != nil || !done {
				return false, err
			}
		case 0x1a478:
			done, err := allocate(NativeStartupBackgroundBytes, false, 0x1a48a)
			if err != nil || !done {
				return false, err
			}
		case 0x1a48a:
			if int32(c.D[0]) <= 0 {
				c.D[0] = 0
				s.Complete = true
				return true, nil
			}
			if err := cb.Memory.Write32(0xdbe, c.D[0]); err != nil {
				return false, err
			}
			c.Word(1, 14)
			c.D[1] = uint32(uint16(c.D[1])) * 48
			s.A[0] = NativeRequesterAddress{Address: cb.CodeBase + 0x19e4a, Code: true}
			if err := cb.Code.Write32(0x19e4a+int(int16(c.D[1])), c.D[0]); err != nil {
				return false, err
			}
			c.Word(0, 14)
			s.PC = 0x1a4ac
		case 0x1a4ac:
			done, err := load(0x1a4b2)
			if err != nil || !done {
				return false, err
			}
		case 0x1a4b2:
			c.D[0] = 1
			s.Complete = true
			return true, nil
		case 0x1a4bc:
			value, err := cb.Memory.Read32(0x3b4)
			if err != nil {
				return false, err
			}
			c.D[0] = value
			if int32(value) > 0 {
				s.A[1] = NativeRequesterAddress{Address: value, Absolute: true}
				s.PC = 0x1a4c4
			} else {
				s.PC = 0x1a4d6
			}
		case 0x1a4c4:
			done, err := allocate(NativeStartupAudioBytes, true, 0x1a4d6)
			if err != nil || !done {
				return false, err
			}
		case 0x1a4d6:
			value, err := cb.Memory.Read32(0xdbe)
			if err != nil {
				return false, err
			}
			c.D[0] = value
			if int32(value) > 0 {
				s.A[1] = NativeRequesterAddress{Address: value, Absolute: true}
				s.PC = 0x1a4de
			} else {
				s.Complete = true
				return true, nil
			}
		case 0x1a4de:
			done, err := allocate(NativeStartupBackgroundBytes, true, 0x1a4f0)
			if err != nil || !done {
				return false, err
			}
		case 0x1a4f0:
			s.Complete = true
			return true, nil
		default:
			return false, fmt.Errorf("native allocation PC%x unavailable", s.PC)
		}
	}
	return false, fmt.Errorf("native allocation transition limit reached")
}
