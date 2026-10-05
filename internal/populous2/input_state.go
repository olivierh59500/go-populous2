package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

// NativeInputState retains the actual low BSS keyboard, mouse and VBlank
// bytes. The array deliberately retains overlaps such as word $0e/byte $0f
// and long $138/its two cursor words; there are no parallel cached values.
type NativeInputState struct {
	Low   [0x14c]byte
	Mouse NativeMouseInputState // Mutable CODE $a2a..$a34, separate from BSS.
}

type NativeMouseInputState struct {
	Image, CounterX, CounterY, PositionX, PositionY, MaximumY uint16
}

type NativeInputRules struct {
	Keys [192]uint8 // Original unshifted/shifted character banks at $100f4.
}

// Hardware samples are explicit original register values. Button booleans
// express the active-low CIA/pot bits after the real event adapter converts
// them; counter bytes are the wrapping JOY0DAT device counters.
type NativeMouseSample struct {
	CounterX, CounterY uint8
	Left, Right        bool
}

type NativePointerPlan struct {
	ImageAddress uint32
	Pointers     [4]uint16 // Copper words +$0e,+$0a,+$06,+$02, in write order.
	Control      [4]byte   // Both original sprites share these four bytes.
}

func DecodeNativeInputRules(exe *amiga.Executable) (NativeInputRules, error) {
	var r NativeInputRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x101b4 {
		return r, fmt.Errorf("native input character banks missing")
	}
	copy(r.Keys[:], exe.Hunks[0].Data[0x100f4:0x101b4])
	return r, nil
}

func NewNativeInputState(exe *amiga.Executable) (NativeInputState, error) {
	var s NativeInputState
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0xa36 {
		return s, fmt.Errorf("native mouse initial state missing")
	}
	values := []*uint16{&s.Mouse.Image, &s.Mouse.CounterX, &s.Mouse.CounterY, &s.Mouse.PositionX, &s.Mouse.PositionY, &s.Mouse.MaximumY}
	for i, value := range values {
		*value = binary.BigEndian.Uint16(exe.Hunks[0].Data[0xa2a+i*2:])
	}
	return s, nil
}

func (s *NativeInputState) byte(a int) byte         { return s.Low[a] }
func (s *NativeInputState) word(a int) uint16       { return binary.BigEndian.Uint16(s.Low[a:]) }
func (s *NativeInputState) long(a int) uint32       { return binary.BigEndian.Uint32(s.Low[a:]) }
func (s *NativeInputState) setWord(a int, v uint16) { binary.BigEndian.PutUint16(s.Low[a:], v) }
func (s *NativeInputState) setLong(a int, v uint32) { binary.BigEndian.PutUint32(s.Low[a:], v) }

// Memory overlays only the retained low span and delegates higher addresses
// to the supplied raw runtime. Reads crossing the boundary preserve bytes
// from each backing rather than pretending both belong to one typed field.
func (s *NativeInputState) Memory(base FollowerCleanupMemory) FollowerCleanupMemory {
	var m FollowerCleanupMemory
	m.Read8 = func(a int) (uint8, error) {
		if s != nil && a >= 0 && a < len(s.Low) {
			return s.Low[a], nil
		}
		if base.Read8 == nil {
			return 0, fmt.Errorf("native input byte outside retained backing")
		}
		return base.Read8(a)
	}
	m.Write8 = func(a int, v uint8) error {
		if s != nil && a >= 0 && a < len(s.Low) {
			s.Low[a] = v
			return nil
		}
		if base.Write8 == nil {
			return fmt.Errorf("native input byte outside retained backing")
		}
		return base.Write8(a, v)
	}
	m.Read16 = func(a int) (uint16, error) {
		if a&1 != 0 {
			return 0, fmt.Errorf("native input word is unaligned")
		}
		h, err := m.Read8(a)
		if err != nil {
			return 0, err
		}
		l, err := m.Read8(a + 1)
		return uint16(h)<<8 | uint16(l), err
	}
	m.Read32 = func(a int) (uint32, error) {
		h, err := m.Read16(a)
		if err != nil {
			return 0, err
		}
		l, err := m.Read16(a + 2)
		return uint32(h)<<16 | uint32(l), err
	}
	m.Write16 = func(a int, v uint16) error {
		if a&1 != 0 {
			return fmt.Errorf("native input word is unaligned")
		}
		if err := m.Write8(a, uint8(v>>8)); err != nil {
			return err
		}
		return m.Write8(a+1, uint8(v))
	}
	m.Write32 = func(a int, v uint32) error {
		if err := m.Write16(a, uint16(v>>16)); err != nil {
			return err
		}
		return m.Write16(a+2, uint16(v))
	}
	return m
}

// NativeKeyWire converts an actual Amiga raw key code to the byte read by
// $620. Press bytes are odd; the matching release byte is one less. Modern
// key mapping belongs to the caller and must use the actual native key code.
func NativeKeyWire(raw uint8, down bool) (uint8, error) {
	if raw > 127 {
		return 0, fmt.Errorf("native raw key outside 128-key bank")
	}
	wire := ^(raw << 1)
	if !down {
		wire &^= 1
	}
	return wire, nil
}

// KeyboardInterrupt is $620's buffer mutation before chaining to the OS.
// Its MOVEM restores D0/A0; every incoming data register survives unchanged.
func (s *NativeInputState) KeyboardInterrupt(wire uint8) error {
	if s == nil {
		return fmt.Errorf("native keyboard backing missing")
	}
	if wire&1 != 0 {
		at := 0x32 + int(wire)
		if s.Low[at] == 0 {
			s.Low[0x2a] = wire
		}
		s.Low[at] += wire
		s.setWord(0x132, s.word(0x132)+1)
	} else {
		s.Low[0x32+int(wire)+1] = 0
		if int16(s.word(0x132)) > 0 {
			s.setWord(0x132, s.word(0x132)-1)
		}
	}
	return nil
}

// Character executes $10096, including unconsumed even bytes, shift-bank
// selection and the retained last-key/seen-key bytes. Only D0 is assigned.
func (r NativeInputRules) Character(s *NativeInputState, registers *[8]uint32) (uint8, error) {
	if s == nil {
		return 0, fmt.Errorf("native keyboard backing missing")
	}
	s.Low[0x28], s.Low[0x2c] = 0, 0
	wire := s.Low[0x2a]
	value := uint8(0)
	if wire != 0 && wire&1 != 0 {
		s.Low[0x2a], s.Low[0x2c], s.Low[0x2e] = 0, wire, 1
		index := uint8(-wire) >> 1
		if index < 96 {
			if s.Low[0x6f] != 0 || s.Low[0x71] != 0 {
				index += 96
			}
			value = r.Keys[index]
			s.Low[0x28] = value
		}
	}
	if registers != nil {
		registers[0] = uint32(value)
	}
	return value, nil
}

// ResetMouseCounters is $8f0. It preserves all data registers and does not
// move the logical cursor when establishing a device counter baseline.
func (s *NativeInputState) ResetMouseCounters(sample NativeMouseSample) error {
	if s == nil {
		return fmt.Errorf("native mouse backing missing")
	}
	s.Mouse.CounterX, s.Mouse.CounterY = uint16(sample.CounterX), uint16(sample.CounterY)
	return nil
}

func nativeMouseDelta(new uint8, old uint16) uint16 {
	delta := uint16(new) - old
	if int16(delta) > 127 {
		delta -= 256
	}
	if int16(delta) < -127 {
		delta += 256
	}
	return delta
}

// PollMouse follows $790/$91c. pointerBase is the actual HUNK3 address origin
// pointer-image data, not a guessed zero. Drawing/copper writes are returned
// as a pure plan; input latches/counts and cursor coordinates are real bytes.
func (s *NativeInputState) PollMouse(sample NativeMouseSample, simulationClock uint32, pointerBase uint32, registers *[8]uint32) (NativePointerPlan, error) {
	var p NativePointerPlan
	if s == nil {
		return p, fmt.Errorf("native mouse backing missing")
	}
	x := nativeMouseDelta(sample.CounterX, s.Mouse.CounterX) + s.Mouse.PositionX
	s.Mouse.CounterX = uint16(sample.CounterX)
	if int16(x) < 0 {
		x = 0
	}
	if int16(x) > 639 {
		x = 639
	}
	s.Mouse.PositionX = x
	s.setWord(0x138, x>>1)
	oldCounterY := s.Mouse.CounterY
	y := nativeMouseDelta(sample.CounterY, oldCounterY) + s.Mouse.PositionY
	s.Mouse.CounterY = uint16(sample.CounterY)
	if int16(y) < 0 {
		y = 0
	}
	if int16(y) > int16(s.Mouse.MaximumY) {
		y = s.Mouse.MaximumY
	}
	s.Mouse.PositionY = y
	s.setWord(0x13a, y>>1)
	for _, b := range []struct {
		down                  bool
		count, latch, pending int
	}{{sample.Left, 0x13c, 0x144, 0x140}, {sample.Right, 0x13e, 0x146, 0x142}} {
		if b.down {
			s.setWord(b.count, s.word(b.count)+1)
			if s.word(b.latch) == 0 {
				s.setWord(b.latch, 1)
				if s.word(b.pending) == 0 {
					s.setWord(b.pending, 1)
					s.setLong(0x134, s.long(0x138))
					s.setLong(0x148, simulationClock)
				}
			}
		} else {
			s.setWord(b.latch, 0)
			s.setWord(b.count, 0)
		}
	}
	address := pointerBase + 0x2834 + uint32(int32(int16(s.Mouse.Image)))
	p.Pointers = [4]uint16{uint16(address), uint16(address >> 16), uint16(address - 72), uint16((address - 72) >> 16)}
	p.ImageAddress = address - 72
	cursorX := uint16(s.word(0x138) + 128)
	control := uint8(0x80)
	if cursorX&1 != 0 {
		control |= 1
	}
	p.Control[1] = uint8(cursorX >> 1)
	cursorY := uint16(s.word(0x13a) + 44)
	if s.word(0x26) != 0 {
		cursorY += 27
	}
	if cursorY&0x100 != 0 {
		control |= 4
	}
	p.Control[0] = uint8(cursorY)
	cursorY += 16
	if cursorY&0x100 != 0 {
		control |= 2
	}
	p.Control[2], p.Control[3] = uint8(cursorY), control
	if registers != nil {
		registers[0] = (p.ImageAddress & 0xffff0000) | uint32(cursorY)
		registers[1] = (registers[1] & 0xffff0000) | uint32(oldCounterY&0xff00) | uint32(control)
	}
	return p, nil
}

// VBlank wraps mouse polling with the original interrupt's save/restore, so
// all incoming data registers survive. Its real counters still advance.
func (s *NativeInputState) VBlank(sample NativeMouseSample, simulationClock uint32, pointerBase uint32) (NativePointerPlan, error) {
	p, err := s.PollMouse(sample, simulationClock, pointerBase, nil)
	if err != nil {
		return p, err
	}
	s.setWord(0xa, 1)
	s.setWord(0xe, s.word(0xe)+1)
	s.setLong(0x16, s.long(0x16)+1)
	s.setLong(0x12, s.long(0x12)+1)
	return p, nil
}
