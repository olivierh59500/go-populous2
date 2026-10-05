package populous2

import (
	"encoding/binary"
	"fmt"
	"go-populous2/internal/amiga"
)

// NativeFrameAudioState retains the mutable software queue in CODE. These
// flags/channels are source words, not an inferred list of emitted samples.
type NativeFrameAudioState struct {
	Entries  [1330]byte // $185a8..$18ada,133 ten-byte records.
	Channels [4]uint16  // $18426..$1842e.
}

func DecodeNativeFrameAudioState(exe *amiga.Executable) (NativeFrameAudioState, error) {
	var s NativeFrameAudioState
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x18ada {
		return s, fmt.Errorf("native audio CODE queue missing")
	}
	copy(s.Entries[:], exe.Hunks[0].Data[0x185a8:0x18ada])
	for i := range s.Channels {
		s.Channels[i] = binary.BigEndian.Uint16(exe.Hunks[0].Data[0x18426+i*2:])
	}
	return s, nil
}

type NativeFrameAudioCallbacks struct {
	// Command is the actual$190e4 boundary. It preserves D1-D7; the returned
	// long is native D0/status. A disabled device's original RTS retains D0,
	// while an initialized device needs its explicit real return value.
	Command func(control, data uint16, input uint32) (uint32, error)
}

// TickAudio is$182ce with its$183b6 software scheduler. Saved-register
// boundaries and the returned device status determine both channels and the
// data-register continuation consumed by the later$1744c stage.
func (s *NativeFrameAudioState) TickAudio(c *NativeFrameRegisterContext, cb NativeFrameAudioCallbacks) error {
	if s == nil || c == nil || cb.Command == nil {
		return fmt.Errorf("native frame audio state/device/context missing")
	}
	word := func(at int) (uint16, error) {
		if at < 0 || at+2 > len(s.Entries) || at&1 != 0 {
			return 0, fmt.Errorf("native audio record word%x unavailable", at)
		}
		return binary.BigEndian.Uint16(s.Entries[at:]), nil
	}
	byteAt := func(at int) (uint8, error) {
		if at < 0 || at >= len(s.Entries) {
			return 0, fmt.Errorf("native audio record byte%x unavailable", at)
		}
		return s.Entries[at], nil
	}
	device := func(control, data uint16) error {
		out, e := cb.Command(control, data, c.D[0])
		if e != nil {
			return e
		}
		c.D[0] = out
		return nil
	}
	var schedule func(int) error
	schedule = func(at int) error {
		priority, e := byteAt(at + 4)
		if e != nil {
			return e
		}
		if priority > 3 {
			return fmt.Errorf("native audio priority%d outside channels", priority)
		}
		c.D[0] = uint32(at)
		channel := s.Channels[priority]
		if uint16(c.D[0]) == channel {
			s.Channels[priority] = 0 - channel
			return nil
		}
		if channel != 0 {
			return nil
		}
		c.Word(0, 0-uint16(c.D[0]))
		s.Channels[priority] = uint16(c.D[0])
		c.D[0], c.D[1] = 1, 0
		c.Byte(1, priority)
		c.Word(0, uint16(c.D[0])<<uint(uint16(c.D[1])&63))
		flags, e := word(at + 2)
		if e != nil {
			return e
		}
		c.Word(0, uint16(c.D[0])|flags)
		data, e := word(at + 6)
		if e != nil {
			return e
		}
		if e := device(uint16(c.D[0]), data); e != nil {
			return e
		}
		c.D[1], c.D[0] = uint32(priority), 1
		c.Word(0, (uint16(c.D[0])<<uint(uint16(c.D[1])&63))|0x2000)
		control := uint16(c.D[0])
		value, e := byteAt(at + 5)
		if e != nil {
			return e
		}
		c.D[0] = uint32(value)
		return device(control, uint16(value))
	}
	for at := 10; at < 1330; at += 10 {
		flag, e := word(at)
		if e != nil {
			return e
		}
		if flag == 0 {
			continue
		}
		binary.BigEndian.PutUint16(s.Entries[at:], 0)
		if e := schedule(at); e != nil {
			return e
		}
		linked, e := word(at + 8)
		if e != nil {
			return e
		}
		c.Word(0, linked)
		if linked != 0 {
			if e := schedule(int(int16(linked))); e != nil {
				return e
			}
		}
	}
	for i := range s.Channels {
		channel := s.Channels[i]
		c.Word(0, channel)
		if channel != 0 {
			if int16(channel) < 0 {
				c.D[1] = uint32(i)
				c.D[0] = 1
				c.Word(0, (uint16(c.D[0])<<uint(uint16(c.D[1])))|0x10)
				if e := device(uint16(c.D[0]), 0); e != nil {
					return e
				}
				if int32(c.D[0]) <= 0 {
					s.Channels[i] = 0
				}
			} else {
				at := int(int16(channel))
				flags, e := word(at + 2)
				if e != nil {
					return e
				}
				c.Word(0, flags&0x20)
				s.Channels[i] = 0
				priority, e := byteAt(at + 4)
				if e != nil {
					return e
				}
				c.D[0], c.D[1] = 1, uint32(priority)
				c.Word(0, (uint16(c.D[0])<<uint(uint16(c.D[1])&63))|0x80)
				if e := device(uint16(c.D[0]), 0); e != nil {
					return e
				}
			}
		}
		s.Channels[i] = 0 - s.Channels[i]
	}
	return nil
}
