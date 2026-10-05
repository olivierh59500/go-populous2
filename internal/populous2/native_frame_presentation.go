package populous2

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"

	"go-populous2/internal/amiga"
)

const NativeFrameChipBytes = 65032

type NativeFrameHardwareWrite struct {
	PC, Address, Value uint32
	Width              uint8
}

// NativeFramePresentationState retains real HUNK4 Copper/screen RAM and
// HUNK3 pointer-image RAM. Addresses are native numeric labels, never host
// pointers. LowTail extends the input backing to the native runtime seam.
type NativeFramePresentationState struct {
	Input                                            NativeInputState
	LowTail                                          [0xdc2 - 0x14c]byte
	Chip                                             []byte
	PointerData                                      []byte
	ChipBase, PointerBase                            uint32
	CopperSelector, SpritePatchPointer, ActiveCopper uint32 // CODE $77a/$77e and COP1LC.
	Deadline1117C                                    uint32
	InterruptChain                                   bool // CODE word $3ea, not a fabricated interrupt acknowledgment.
	clockPending                                     bool
}

type NativeFrameClockCallbacks struct {
	Memory FollowerCleanupMemory
	// Palette is genuine $102e4 with source CODE palettes $3361a/$33844.
	// It may remain pending at a real VBlank wait; previous clock work is
	// not repeated on resume. Surviving registers remain in the full frame.
	Palette func(*NativeFrameRegisterContext) (bool, error)
}

type NativeFrameVBlankResult struct {
	Pointer                NativePointerPlan
	Hardware               []NativeFrameHardwareWrite
	ChainOriginalInterrupt bool
}

func NewNativeFramePresentationState(exe *amiga.Executable, chipBase, pointerBase uint32) (*NativeFramePresentationState, error) {
	if exe == nil || len(exe.Hunks) < 4 || len(exe.Hunks[0].Data) < 0x11180 {
		return nil, fmt.Errorf("native frame presentation resources missing")
	}
	input, err := NewNativeInputState(exe)
	if err != nil {
		return nil, err
	}
	code := exe.Hunks[0].Data
	return &NativeFramePresentationState{Input: input, Chip: make([]byte, NativeFrameChipBytes), PointerData: append([]byte(nil), exe.Hunks[3].Data...), ChipBase: chipBase, PointerBase: pointerBase, CopperSelector: binary.BigEndian.Uint32(code[0x77a:]), Deadline1117C: binary.BigEndian.Uint32(code[0x1117c:]), InterruptChain: binary.BigEndian.Uint16(code[0x3ea:]) != 0}, nil
}

// Memory owns each low byte exactly once, including aliases across $14c.
// The raw World runtime begins at $dc2 and remains supplied by the caller.
func (s *NativeFramePresentationState) Memory(base FollowerCleanupMemory) FollowerCleanupMemory {
	tail := base
	tail.Read8 = func(at int) (uint8, error) {
		if s != nil && at >= 0x14c && at < 0xdc2 {
			return s.LowTail[at-0x14c], nil
		}
		if base.Read8 == nil {
			return 0, fmt.Errorf("native frame byte outside backing")
		}
		return base.Read8(at)
	}
	tail.Write8 = func(at int, v uint8) error {
		if s != nil && at >= 0x14c && at < 0xdc2 {
			s.LowTail[at-0x14c] = v
			return nil
		}
		if base.Write8 == nil {
			return fmt.Errorf("native frame byte outside backing")
		}
		return base.Write8(at, v)
	}
	return s.Input.Memory(tail)
}

func (s *NativeFramePresentationState) chipAt(address uint32, size int) (int, error) {
	if s == nil {
		return 0, fmt.Errorf("native frame chip backing missing")
	}
	at := int(int64(address) - int64(s.ChipBase))
	if at < 0 || size < 0 || at > len(s.Chip)-size {
		return 0, fmt.Errorf("native frame chip address %x outside retained RAM", address)
	}
	return at, nil
}

// Initialize executes the two $49a builders from $440. Each list contains
// its original register/value words, four native plane pointers and hardware
// sprite pointers; each bitmap is four contiguous 8,000-byte planes.
func (s *NativeFramePresentationState) Initialize(exe *amiga.Executable, sample NativeMouseSample) ([]NativeFrameHardwareWrite, error) {
	if s == nil || exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x3365a || len(s.Chip) != NativeFrameChipBytes {
		return nil, fmt.Errorf("native frame Copper initialization missing")
	}
	if err := s.Input.ResetMouseCounters(sample); err != nil {
		return nil, err
	}
	code := exe.Hunks[0].Data
	writes := []NativeFrameHardwareWrite{}
	for buffer := 0; buffer < 2; buffer++ {
		start := 8 + buffer*512
		at := start
		putLong := func(v uint32) { binary.BigEndian.PutUint32(s.Chip[at:], v); at += 4 }
		putWord := func(v uint16) { binary.BigEndian.PutUint16(s.Chip[at:], v); at += 2 }
		putLong(0)
		putLong(0)
		for _, v := range []uint32{0x01004200, 0x01020000, 0x01040024} {
			putLong(v)
		}
		if s.Input.word(0x26) != 0 {
			putLong(0x008e4781)
			putLong(0x00900fc1)
		} else {
			putLong(0x008e2c81)
			putLong(0x0090f4c1)
		}
		putLong(0x00920038)
		putLong(0x009400d0)
		putWord(0x0108)
		putWord(0)
		putWord(0x010a)
		putWord(0)
		for i := 0; i < 32; i++ {
			putWord(uint16(0x180 + i*2))
			putWord(binary.BigEndian.Uint16(code[0x3361a+i*2:]))
		}
		planeBase := s.ChipBase + 0x408 + uint32(buffer*32000)
		for plane := 0; plane < 4; plane++ {
			p := planeBase + uint32(plane*8000)
			putWord(uint16(0xe0 + plane*4))
			putWord(uint16(p >> 16))
			putWord(uint16(0xe2 + plane*4))
			putWord(uint16(p))
		}
		binary.BigEndian.PutUint32(s.Chip[start+4:], s.ChipBase+uint32(at))
		for sprite := 0; sprite < 8; sprite++ {
			p := s.ChipBase + uint32(start)
			if sprite < 2 {
				p = s.PointerBase + 0x27ec + uint32(sprite*72)
			}
			putWord(uint16(0x120 + sprite*4))
			putWord(uint16(p >> 16))
			putWord(uint16(0x122 + sprite*4))
			putWord(uint16(p))
		}
		putLong(0x0901ff00)
		putLong(0x009c8010)
		putLong(0xfffffffe)
		s.SpritePatchPointer = s.ChipBase + uint32(start+4)
		s.ActiveCopper = s.ChipBase + uint32(start+8)
		writes = append(writes, NativeFrameHardwareWrite{0x5b2, 0xdff096, 0x83f0, 2}, NativeFrameHardwareWrite{0x5c4, 0xdff080, s.ActiveCopper, 4})
		binary.BigEndian.PutUint32(s.Chip[buffer*4:], s.ActiveCopper)
		s.Input.setLong(0x1a+buffer*4, planeBase)
	}
	return writes, nil
}

// Swap is actual $72e: it swaps BSS screen pointers, toggles the Copper-table
// selector, activates that list and clears the VBlank-ready word. MOVEM.L
// preserves every caller data register; pixels remain in their actual RAM.
func (s *NativeFramePresentationState) Swap(c *NativeFrameRegisterContext) ([]NativeFrameHardwareWrite, error) {
	if s == nil || c == nil {
		return nil, fmt.Errorf("native frame swap state/context missing")
	}
	front, back := s.Input.long(0x1a), s.Input.long(0x1e)
	s.Input.setLong(0x1a, back)
	s.Input.setLong(0x1e, front)
	s.CopperSelector ^= 4
	at, err := s.chipAt(s.ChipBase+s.CopperSelector, 4)
	if err != nil {
		return nil, err
	}
	s.ActiveCopper = binary.BigEndian.Uint32(s.Chip[at:])
	s.SpritePatchPointer = s.ActiveCopper - 4
	s.Input.setWord(0xa, 0)
	return []NativeFrameHardwareWrite{{0x760, 0xdff080, s.ActiveCopper, 4}}, nil
}

func (s *NativeFramePresentationState) applyPointer(p NativePointerPlan) error {
	header, err := s.chipAt(s.SpritePatchPointer, 4)
	if err != nil {
		return err
	}
	address := binary.BigEndian.Uint32(s.Chip[header:])
	for i, offset := range []uint32{14, 10, 6, 2} {
		at, err := s.chipAt(address+offset, 2)
		if err != nil {
			return err
		}
		binary.BigEndian.PutUint16(s.Chip[at:], p.Pointers[i])
	}
	at := int(int64(p.ImageAddress) - int64(s.PointerBase))
	if at < 0 || at > len(s.PointerData)-76 {
		return fmt.Errorf("native cursor sprite address %x outside retained HUNK3", p.ImageAddress)
	}
	for _, component := range []int{1, 0, 2, 3} {
		s.PointerData[at+component] = p.Control[component]
		s.PointerData[at+72+component] = p.Control[component]
	}
	return nil
}

// VBlank composes $790/$91c with the interrupt's real counters and writes
// the pointer words/control bytes into retained Copper and sprite RAM. The
// saved caller registers survive; chaining the previous OS IRQ is explicit.
func (s *NativeFramePresentationState) VBlank(sample NativeMouseSample, memory FollowerCleanupMemory, c *NativeFrameRegisterContext) (NativeFrameVBlankResult, error) {
	var result NativeFrameVBlankResult
	if s == nil || c == nil || memory.Read32 == nil {
		return result, fmt.Errorf("native VBlank backing missing")
	}
	clock, err := memory.Read32(0xf40)
	if err != nil {
		return result, err
	}
	result.Pointer, err = s.Input.PollMouse(sample, clock, s.PointerBase, nil)
	if err != nil {
		return result, err
	}
	if err = s.applyPointer(result.Pointer); err != nil {
		return result, err
	}
	// The interrupt advances these counters only after $790/$91c finishes
	// the actual Copper and sprite writes, preserving exception prefixes.
	s.Input.setWord(0xa, 1)
	s.Input.setWord(0xe, s.Input.word(0xe)+1)
	s.Input.setLong(0x16, s.Input.long(0x16)+1)
	s.Input.setLong(0x12, s.Input.long(0x12)+1)
	result.Hardware = []NativeFrameHardwareWrite{{0x88e, 0xdff034, 0x0c00, 2}}
	result.ChainOriginalInterrupt = s.InterruptChain
	if !s.InterruptChain {
		result.Hardware = append(result.Hardware, NativeFrameHardwareWrite{0x436, 0xdff09c, 0x20, 2})
	}
	return result, nil
}

// AdvanceClock is $110cc/$11180. It may wait for genuine VBlanks when the
// mouse is at the origin, or remain inside the palette callback. F40 advances
// before main rendering and physics, only outside the pause gate.
func (s *NativeFramePresentationState) AdvanceClock(c *NativeFrameRegisterContext, cb NativeFrameClockCallbacks) (bool, error) {
	if s == nil || c == nil || !winMemoryValid(cb.Memory) {
		return false, fmt.Errorf("native clock frame backing missing")
	}
	m := nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	if !s.clockPending {
		c.Word(0, s.Input.word(0x138)+s.Input.word(0x13a))
		if uint16(c.D[0]) == 0 && int32(s.Input.long(0x12)) <= 8 {
			return false, nil
		}
		s.Input.setLong(0x12, 0)
		if m.word(0x3b0) == 0 {
			c.D[0] = s.Deadline1117C
			if c.D[0] == 0 {
				c.D[0] = m.long(0xf40) + 3
				s.Deadline1117C = c.D[0]
			}
			if int32(c.D[0]) <= int32(m.long(0xf40)) {
				s.Deadline1117C = 0
				s.clockPending = true
			}
		}
	}
	if s.clockPending {
		if cb.Palette == nil {
			return false, fmt.Errorf("native clock palette $102e4 continuation missing")
		}
		complete, err := cb.Palette(c)
		if err != nil || !complete {
			return false, err
		}
		s.clockPending = false
	}
	if m.word(0xf3c) == 0 {
		c.D[0] = m.long(0xf40)
		if int32(c.D[0]) > int32(s.Input.long(0x148)) {
			s.Input.setWord(0x140, 0)
			s.Input.setWord(0x142, 0)
		}
		c.D[0]++
		m.putLong(0xf40, c.D[0])
	}
	m.putWord(0xe8ee, m.word(0xeb2c))
	m.putWord(0xea28, m.word(0xeb2e))
	for reg, at := range []int{0x5f44, 0x5f46} {
		c.RestoreWord(reg, m.word(at))
		if int16(c.D[reg]) < 0 {
			c.D[reg] = 0
		} else if int16(c.D[reg]) >= 56 {
			c.D[reg] = 56
		}
		m.putWord(at, uint16(c.D[reg]))
	}
	return m.err == nil, m.err
}

// BackBuffer exposes the actual native drawing target, not a separate image
// cache. The register-bearing render producer writes these four planes before
// the physics pass; Swap subsequently makes them the displayed Copper source.
func (s *NativeFramePresentationState) BackBuffer() ([]byte, error) {
	if s == nil {
		return nil, fmt.Errorf("native frame buffer state missing")
	}
	at, err := s.chipAt(s.Input.long(0x1e), 32000)
	if err != nil {
		return nil, err
	}
	return s.Chip[at : at+32000], nil
}

// Image follows the selected Copper's four actual bitplane pointers and first
// sixteen RGB4 palette words. It is a portable presentation of real RAM; the
// host still owns raster timing, audio hardware and compositing sprite pixels.
func (s *NativeFramePresentationState) Image() (*image.Paletted, error) {
	if s == nil {
		return nil, fmt.Errorf("native displayed Copper state missing")
	}
	at, err := s.chipAt(s.ActiveCopper, 2)
	if err != nil {
		return nil, err
	}
	var pointers [4]uint32
	var palette [16]color.RGBA
	for words := 0; words < 256; words++ {
		if at+4 > len(s.Chip) {
			return nil, fmt.Errorf("native Copper list lacks its terminator")
		}
		reg, v := binary.BigEndian.Uint16(s.Chip[at:]), binary.BigEndian.Uint16(s.Chip[at+2:])
		at += 4
		if reg == 0xffff && v == 0xfffe {
			break
		}
		if reg >= 0x180 && reg < 0x1a0 {
			index := (reg - 0x180) / 2
			palette[index] = color.RGBA{uint8(v>>8&15) * 17, uint8(v>>4&15) * 17, uint8(v&15) * 17, 255}
		}
		if reg >= 0xe0 && reg <= 0xee && reg&1 == 0 {
			plane := (reg - 0xe0) / 4
			if reg&2 == 0 {
				pointers[plane] = pointers[plane]&0xffff | uint32(v)<<16
			} else {
				pointers[plane] = pointers[plane]&0xffff0000 | uint32(v)
			}
		}
	}
	planes := make([]byte, 32000)
	for plane, p := range pointers {
		at, err := s.chipAt(p, 8000)
		if err != nil {
			return nil, err
		}
		copy(planes[plane*8000:], s.Chip[at:at+8000])
	}
	pixels, err := nativeScreenIndices(planes)
	if err != nil {
		return nil, err
	}
	return nativeIndexedImage(pixels, palette)
}
