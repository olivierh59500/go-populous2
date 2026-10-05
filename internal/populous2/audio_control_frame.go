package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

// NativeAudioControlFrameRules keeps the actual descriptor CODE used by the
// menu wrappers. Pause/resume are not an outer register-preserving operation:
// $1842e returns D1.W=16, and $18474 leaves its real D0/D1 results visible.
type NativeAudioControlFrameRules struct{ code []byte }

func DecodeNativeAudioControlFrameRules(exe *amiga.Executable) (NativeAudioControlFrameRules, error) {
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x18b16 {
		return NativeAudioControlFrameRules{}, fmt.Errorf("native audio control CODE missing")
	}
	return NativeAudioControlFrameRules{code: exe.Hunks[0].Data}, nil
}

type NativeAudioControlFrameCallbacks struct {
	Memory   FollowerCleanupMemory
	Frame    *NativeFrameRegisterContext
	CodeBase uint32
	// These are actual device/PCM operations, not acknowledgments. The
	// original children preserve D1-D7 and return their complete D0 status.
	Command      func(control, data uint16, inputD0 uint32) (uint32, error)
	MusicCommand func(control, data uint16, inputD0 uint32) (uint32, error)
}
type NativeAudioControlFrameCall struct {
	Routine       int
	Control, Data uint16
	D             [8]uint32
	Status        uint32
	Disabled      bool
}
type NativeAudioControlFrameStep struct {
	Complete bool
	Calls    []NativeAudioControlFrameCall
	// Resume's silent-menu branch leaves A0 pointing at its fixed descriptor.
	// Other operations preserve the caller's address registers.
	A0Assigned bool
	A0         NativeRequesterAddress
}

func NativeAudioControlDeviceCallbacks(d *NativeAudioDevice, memory FollowerCleanupMemory, frame *NativeFrameRegisterContext) NativeAudioControlFrameCallbacks {
	cb := NativeAudioControlFrameCallbacks{Memory: memory, Frame: frame}
	if d != nil {
		cb.CodeBase = d.CodeBase
		cb.Command = d.Command
		cb.MusicCommand = d.MusicCommand
	}
	return cb
}
func NativeAudioControlPCMCallbacks(p *NativeAudioPCM, memory FollowerCleanupMemory, frame *NativeFrameRegisterContext) NativeAudioControlFrameCallbacks {
	cb := NativeAudioControlFrameCallbacks{Memory: memory, Frame: frame}
	if p != nil {
		cb.CodeBase = p.device.CodeBase
		cb.Command = p.Command
		cb.MusicCommand = p.MusicCommand
	}
	return cb
}
func (r *NativeAudioControlFrameRules) descriptor(offset uint16) (int, error) {
	at := 0x185a8 + int(int16(offset))
	if r == nil || at < 0 || at&1 != 0 || at > len(r.code)-10 {
		return 0, fmt.Errorf("native audio control descriptor%x outside aligned CODE", offset)
	}
	return at, nil
}
func audioControlCommand(cb NativeAudioControlFrameCallbacks, out *NativeAudioControlFrameStep, routine int, control, data uint16) error {
	c := cb.Frame
	call := NativeAudioControlFrameCall{Routine: routine, Control: control, Data: data, D: c.D}
	operation := cb.MusicCommand
	if routine == 0x190e4 {
		gate, e := cb.Memory.Read32(0x3b4)
		if e != nil {
			return e
		}
		call.Disabled = int32(gate) <= 0
		operation = cb.Command
	}
	status := c.D[0]
	if !call.Disabled {
		if operation == nil {
			return fmt.Errorf("native audio control child%x missing", routine)
		}
		var e error
		status, e = operation(control, data, c.D[0])
		if e != nil {
			return e
		}
	}
	c.D[0], call.Status = status, status
	out.Calls = append(out.Calls, call)
	return nil
}
func nativeAudioShiftWord(value uint16, count uint8) uint16 {
	return uint16(uint32(value) << uint(count&63))
}

// Run executes the actual synchronous $1842e/$18474/$184f6 body. The signed
// BSS$3b4 gate applies only to $190e4. Primary $1932c remains required even
// when secondary audio is disabled; no blanket disabled return replaces it.
func (r *NativeAudioControlFrameRules) Run(routine int, cb NativeAudioControlFrameCallbacks) (out NativeAudioControlFrameStep, failure error) {
	if r == nil || cb.Frame == nil || cb.Memory.Read16 == nil || cb.Memory.Read32 == nil {
		return out, fmt.Errorf("native audio control frame/backing missing")
	}
	c := cb.Frame
	command := func(control, data uint16) error { return audioControlCommand(cb, &out, 0x190e4, control, data) }
	music := func(control, data uint16) error { return audioControlCommand(cb, &out, 0x1932c, control, data) }
	switch routine {
	case 0x1842e:
		c.Word(1, 1)
		for {
			c.Word(0, uint16(c.D[1]))
			c.Word(0, uint16(c.D[0])|0x2000)
			if e := command(uint16(c.D[0]), 0); e != nil {
				return out, e
			}
			c.Word(1, uint16(c.D[1])<<1)
			if int16(c.D[1]) > 8 {
				break
			}
		}
		flag, e := cb.Memory.Read16(0x3bc)
		if e != nil {
			return out, e
		}
		if flag != 0 {
			if e = music(0x2006, 0); e != nil {
				return out, e
			}
		}
	case 0x18474:
		flag, e := cb.Memory.Read16(0x3bc)
		if e != nil {
			return out, e
		}
		if flag != 0 {
			if e = music(0xa6, 0); e != nil {
				return out, e
			}
			if e = music(0x2006, 32); e != nil {
				return out, e
			}
		} else {
			at, e := r.descriptor(0x1ae)
			if e != nil {
				return out, e
			}
			out.A0Assigned = true
			out.A0 = NativeRequesterAddress{Code: true, Address: cb.CodeBase + uint32(at)}
			c.D[0], c.D[1] = 1, 0
			c.Byte(1, r.code[at+4])
			c.Word(0, nativeAudioShiftWord(uint16(c.D[0]), uint8(c.D[1])))
			c.Word(0, uint16(c.D[0])|binary.BigEndian.Uint16(r.code[at+2:]))
			if e = command(uint16(c.D[0]), binary.BigEndian.Uint16(r.code[at+6:])); e != nil {
				return out, e
			}
			c.D[1] = 0
			c.Byte(1, r.code[at+4])
			c.D[0] = 1
			c.Word(0, nativeAudioShiftWord(uint16(c.D[0]), uint8(c.D[1])))
			c.Word(0, uint16(c.D[0])|0x2000)
			control := uint16(c.D[0])
			c.D[0] = 0
			c.Byte(0, r.code[at+5])
			if e = command(control, uint16(c.D[0])); e != nil {
				return out, e
			}
		}
	case 0x184f6:
		saved := c.D
		at, e := r.descriptor(uint16(c.D[0]))
		if e != nil {
			return out, e
		}
		c.D[0], c.D[1] = 1, 0
		c.Word(1, 2)
		c.Word(0, nativeAudioShiftWord(uint16(c.D[0]), uint8(c.D[1])))
		c.Word(0, uint16(c.D[0])|binary.BigEndian.Uint16(r.code[at+2:]))
		c.Word(0, uint16(c.D[0])&^0x20)
		if e = command(uint16(c.D[0]), binary.BigEndian.Uint16(r.code[at+6:])); e != nil {
			return out, e
		}
		volume := func(at int) error {
			c.D[1] = 0
			c.Byte(1, r.code[at+4])
			c.D[0] = 1
			c.Word(0, nativeAudioShiftWord(uint16(c.D[0]), uint8(c.D[1])))
			c.Word(0, uint16(c.D[0])|0x2000)
			control := uint16(c.D[0])
			c.D[0] = 0
			c.Byte(0, r.code[at+5])
			return command(control, uint16(c.D[0]))
		}
		if e = volume(at); e != nil {
			return out, e
		}
		c.Word(0, binary.BigEndian.Uint16(r.code[at+8:]))
		if uint16(c.D[0]) != 0 {
			at, e = r.descriptor(uint16(c.D[0]))
			if e != nil {
				return out, e
			}
			c.D[0], c.D[1] = 1, 0
			c.Byte(1, r.code[at+4])
			c.Word(0, nativeAudioShiftWord(uint16(c.D[0]), uint8(c.D[1])))
			c.Word(0, uint16(c.D[0])|binary.BigEndian.Uint16(r.code[at+2:]))
			c.Word(0, uint16(c.D[0])&^0x20)
			if e = command(uint16(c.D[0]), binary.BigEndian.Uint16(r.code[at+6:])); e != nil {
				return out, e
			}
			if e = volume(at); e != nil {
				return out, e
			}
		}
		c.D = saved
	default:
		return out, fmt.Errorf("native audio control routine%x unsupported", routine)
	}
	out.Complete = true
	return out, nil
}
