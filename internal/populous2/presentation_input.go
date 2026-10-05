package populous2

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strconv"

	"go-populous2/internal/amiga"
)

// NativePresentationInputRules translates the normal-view stages preceding
// $1f5e, and the selected-pane camera/editor callbacks. Low UI words and raw
// actor pointers are supplied by Memory; painting's mutable CODE buffers have
// their own retained state, rather than aliasing unrelated BSS at the same
// numeric offset.
type NativePresentationInputRules struct {
	code       []byte
	Requesters NativeRequesterRules
	Paint      []byte
}

// NativePresentationCopy is the $c8c2 blitter request. Its 800 rows of 20
// words copy all four contiguous 8,000-byte planes from$22 to$1e.
type NativePresentationCopy struct {
	Source, Destination uint32
	Bytes               int
	Control, Size       uint16
}

type NativePresentationHighlight struct {
	Column, Row int16
	Pattern     int // Original CODE address of the twelve four-byte rows.
}

// NativePaintingState retains the original $37bc index, four twenty-byte
// numeric fields, and $ab4e requester workspace. Compile patches only bytes
// written by the original body, preserving scratch beyond its terminator.
type NativePaintingState struct {
	Index   uint16
	Fields  [4][20]byte
	Scratch [2048]byte
}

type NativePaintingPlan struct {
	Requester *NativeRequester
	Marker    []NativePresentationSprite
	Action    uint16
	Audio     []uint16 // Original $184f6 arguments, not PCM sound indices.
}

type NativePaintingCallbacks struct {
	Memory      FollowerCleanupMemory
	AddressBase uint32
	// EditNumber is the original modal $4bba boundary. It receives and returns
	// the retained byte field and surviving registers. It is mandatory only
	// when a real click enters one of the four numeric fields. Keyboard/timer
	// polling inside that modal is separate from the normal frame stage.
	EditNumber func(field int, value []byte, context *NativeHUDRegisters) ([]byte, error)
}

func DecodeNativePresentationInputRules(exe *amiga.Executable) (NativePresentationInputRules, error) {
	var r NativePresentationInputRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x3ddc8 {
		return r, fmt.Errorf("native pre-HUD presentation data missing")
	}
	r.code = exe.Hunks[0].Data
	var err error
	if r.Requesters, err = DecodeNativeRequesterRules(exe); err != nil {
		return r, err
	}
	r.Paint, err = NativeRequesterTemplate(exe, int(NativeMenuPaint))
	return r, err
}

func (r *NativePresentationInputRules) NewPaintingState() NativePaintingState {
	var s NativePaintingState
	if r == nil || len(r.code) < 0xab4e+len(s.Scratch) {
		return s
	}
	s.Index = binary.BigEndian.Uint16(r.code[0x37bc:])
	for i := range s.Fields {
		copy(s.Fields[i][:], r.code[0x37ce+i*20:0x37ce+(i+1)*20])
	}
	copy(s.Scratch[:], r.code[0xab4e:0xab4e+len(s.Scratch)])
	return s
}

// CopyBackground follows $c8c2. Both OS lock wrappers preserve every register;
// the intervening hardware writes do not assign D4, D5 or D7.
func (*NativePresentationInputRules) CopyBackground(m FollowerCleanupMemory, c *NativeHUDRegisters) (NativePresentationCopy, error) {
	var p NativePresentationCopy
	if c == nil || m.Read32 == nil {
		return p, fmt.Errorf("native pre-HUD bitmap pointers missing")
	}
	var err error
	p.Source, err = m.Read32(0x22)
	if err != nil {
		return p, err
	}
	p.Destination, err = m.Read32(0x1e)
	if err != nil {
		return p, err
	}
	p.Bytes, p.Control, p.Size = 32000, 0x09f0, 0xc814
	return p, nil
}

// Highlights is $29f8. Each $2a9e XOR child saves/restores D0-D7/A0-A4.
// The deity address uses the signed low word of MULU, like ADDA.W in source.
func (r *NativePresentationInputRules) Highlights(m FollowerCleanupMemory, c *NativeHUDRegisters) ([]NativePresentationHighlight, error) {
	if r == nil || c == nil || m.Read16 == nil {
		return nil, fmt.Errorf("native pre-HUD highlight input missing")
	}
	var p []NativePresentationHighlight
	add := func(offset, pattern int) error {
		if offset < 0 || offset+4 > len(r.code) {
			return fmt.Errorf("native highlight coordinates outside CODE")
		}
		p = append(p, NativePresentationHighlight{int16(binary.BigEndian.Uint16(r.code[offset:])), int16(binary.BigEndian.Uint16(r.code[offset+2:])), pattern})
		return nil
	}
	command, err := m.Read16(0xeb18)
	if err != nil {
		return nil, err
	}
	if command == 12 {
		if err := add(0x33294+8, 0x3dd68); err != nil {
			return nil, err
		}
	}
	side, err := m.Read16(0xeb42)
	if err != nil {
		return nil, err
	}
	mode, err := m.Read16(0xe76a + int(int16(side*314)) + 12)
	if err != nil {
		return nil, err
	}
	for _, v := range []struct {
		mode   uint16
		offset int
	}{{16, 24}, {14, 20}, {20, 16}, {18, 12}} {
		if mode == v.mode {
			if err := add(0x33294+v.offset, 0x3dd68); err != nil {
				return nil, err
			}
		}
	}
	category, err := m.Read16(0xf3a)
	if err != nil {
		return nil, err
	}
	if err := add(0x33294+0x1c+int(int16(category*2)), 0x3dd98); err != nil {
		return nil, err
	}
	return p, nil
}

// XORHighlights translates $2a9e's byte writes into the first bitmap plane.
// Rows stop at its 8,000-byte end; right-edge width includes column39. Invalid
// caller geometry returns an error instead of silently writing outside RAM.
func (r *NativePresentationInputRules) XORHighlights(bitmap []byte, plan []NativePresentationHighlight) error {
	if r == nil || len(bitmap) < 32000 {
		return fmt.Errorf("native highlight bitmap missing")
	}
	for _, p := range plan {
		width := 4
		if p.Column > 36 {
			width = int(int16(uint16(39)-uint16(p.Column))) + 1
		}
		if width < 1 || p.Pattern < 0 || p.Pattern+48 > len(r.code) {
			return fmt.Errorf("native highlight source outside retained CODE")
		}
		at := int(int16(uint16(p.Row)*40 + uint16(p.Column)))
		for row := range 12 {
			if at < 0 || at+width > len(bitmap) {
				return fmt.Errorf("native highlight write outside supplied bitmap")
			}
			for column := range width {
				bitmap[at+column] ^= r.code[p.Pattern+row*4+column]
			}
			at += 40
			if at >= 8000 {
				break
			}
		}
	}
	return nil
}

// SelectedHit translates $2914/$11180. Native coordinates are the high
// bytes of the selected fixed-point X/Y. Dead owners are not filtered here.
func (*NativePresentationInputRules) SelectedHit(cb NativePaintingCallbacks, c *NativeHUDRegisters) error {
	if c == nil || !winMemoryValid(cb.Memory) {
		return fmt.Errorf("native selected camera input missing")
	}
	m := cb.Memory
	p, err := m.Read32(0xf36)
	if err != nil || p == 0 {
		return err
	}
	at := int(int64(p) - int64(cb.AddressBase))
	for i, offset := range []int{6, 8} {
		v, err := m.Read8(at + offset)
		if err != nil {
			return err
		}
		camera := int(v) - 4
		if camera < 0 {
			camera = 0
		} else if camera >= 56 {
			camera = 56
		}
		if err := m.Write16(0x5f44+i*2, uint16(camera)); err != nil {
			return err
		}
	}
	return nil
}

func paintingField(field *[20]byte) []byte {
	if n := bytes.IndexByte(field[:], 0); n >= 0 {
		return field[:n]
	}
	return field[:]
}

func (r *NativePresentationInputRules) compilePainting(s *NativePaintingState, c *NativeHUDRegisters) (*NativeRequester, error) {
	params := make([][]byte, 4)
	for i := range params {
		params[i] = paintingField(&s.Fields[i])
	}
	compiledParameters := append([][]byte(nil), params...)
	for i, value := range compiledParameters {
		// $37be always contains four live pointers. A pointed-to empty
		// string enters the native field loop and pads with k; the generic
		// requester API also models absent pointers, which this caller
		// never supplies. A single k yields identical padded text while
		// params below retains the actual zero-length register count.
		if len(value) == 0 {
			compiledParameters[i] = []byte{'k'}
		}
	}
	p, err := r.Requesters.Compile(r.Paint, compiledParameters)
	if err != nil {
		return nil, err
	}
	// The native field compiler assigns MOVEQ-1,D4, counts the string,
	// and then subtracts the field length. Only its low-word subtraction
	// follows that initial full-long assignment.
	position, parameter := 4, 0
	for position < len(r.Paint) {
		v := r.Paint[position]
		position++
		if v == 0 {
			break
		}
		if v&0x80 != 0 {
			v -= 0x25
		}
		if v == '{' {
			if parameter < len(params) && len(params[parameter]) != 0 {
				position += len(params[parameter]) - 1
			}
			parameter++
		} else if v == 'v' && parameter < len(params) {
			end := position
			for end < len(r.Paint) && r.Paint[end] != 'w' && r.Paint[end] != 0x9c {
				end++
			}
			c.D4 = 0xffff0000 | uint32(uint16(len(params[parameter])))
			c.D4 = hudWord(c.D4, uint16(len(params[parameter])-(end-position)))
			position, parameter = end, parameter+1
		}
	}
	if len(p.Text)+11 > len(s.Scratch) {
		return nil, fmt.Errorf("native painting requester exceeds workspace")
	}
	for i, v := range []int{10, p.Column, p.Row, p.Width, p.Height} {
		binary.BigEndian.PutUint16(s.Scratch[i*2:], uint16(v))
	}
	copy(s.Scratch[10:], p.Text)
	s.Scratch[10+len(p.Text)] = 0
	return p, nil
}

// PaintText reproduces the software glyph writes of $509a into four native
// planes. The marker remains a separate hardware-blitter plan; it does not
// silently alter a software bitmap in the CPU oracle either.
func (r *NativePresentationInputRules) PaintText(bitmap []byte, p NativePaintingPlan) error {
	if r == nil || p.Requester == nil || len(bitmap) != 32000 {
		return fmt.Errorf("native painting text bitmap missing")
	}
	x, y := p.Requester.Column, p.Requester.Row
	for _, ch := range p.Requester.Text {
		if ch == '\n' {
			x, y = p.Requester.Column, y+8
			continue
		}
		if x >= 40 {
			break
		}
		if ch < 32 || ch > 127 || x < 0 || y < 0 || y+8 > 200 {
			return fmt.Errorf("native painting glyph outside verified display bank")
		}
		for row := range 8 {
			for plane := range 4 {
				bitmap[plane*8000+(y+row)*40+x] = r.code[0x33c68+int(ch-32)*32+row*4+plane]
			}
		}
		x++
	}
	return nil
}

func (r *NativePresentationInputRules) clickPainting(s *NativePaintingState, m FollowerCleanupMemory) (uint16, error) {
	pressed, err := m.Read16(0x140)
	if err != nil || pressed == 0 {
		return 0, err
	}
	x, err := m.Read16(0x134)
	if err != nil {
		return 0, err
	}
	y, err := m.Read16(0x136)
	if err != nil {
		return 0, err
	}
	word := func(i int) uint16 { return binary.BigEndian.Uint16(s.Scratch[i*2:]) }
	column, row, width, height := word(1), word(2), word(3), word(4)
	dx, dy := uint16((x>>3)-column), uint16(y-row)
	if int16(dx) < 0 || int16(uint16(dx-width)) >= 0 || int16(dy) < 0 || int16(uint16(dy-height)) > 0 {
		return 0, nil
	}
	if err := m.Write16(0x140, 0); err != nil {
		return 0, err
	}
	start := int(int16(word(0)))
	rowStart := start + int(int16(uint16((dy/8)*(width+1))))
	index := rowStart + int(int16(dx))
	if rowStart < 0 || index < rowStart || index >= len(s.Scratch) {
		return 0, fmt.Errorf("native painting click outside retained workspace")
	}
	active := false
	for at := rowStart; at <= index; at++ {
		marker := r.Requesters.marker(s.Scratch[at])
		if marker > 0 {
			active = true
		} else if marker < 0 {
			active = false
		}
	}
	if !active {
		return 0, nil
	}
	action, selected := uint16(0), -1
	for at := start; at <= index; at++ {
		if r.Requesters.marker(s.Scratch[at]) > 0 {
			action += 2
			selected = at
		}
	}
	if selected >= 0 && (s.Scratch[selected] == 'c' || s.Scratch[selected] == 'd') {
		s.Scratch[selected] ^= 7 // Native c/d radio toggle.
	}
	return action, nil
}

// Painting follows $346a..$378a. Text drawing $509a and sound $184f6 save
// the relevant registers; the pure plan leaves their actual display/audio
// consumers to the caller. Modal numeric editing is an explicit callback.
func (r *NativePresentationInputRules) Painting(s *NativePaintingState, cb NativePaintingCallbacks, c *NativeHUDRegisters) (NativePaintingPlan, error) {
	var plan NativePaintingPlan
	if r == nil || s == nil || c == nil || !winMemoryValid(cb.Memory) {
		return plan, fmt.Errorf("native painting state/input missing")
	}
	m := cb.Memory
	at := 0xdde + int(int16(s.Index))
	if at&1 != 0 {
		return plan, fmt.Errorf("native painting record has an unaligned word address")
	}
	values := [4]uint16{}
	var err error
	if values[0], err = m.Read16(at); err != nil {
		return plan, err
	}
	for i, offset := range []int{4, 5} {
		b, e := m.Read8(at + offset)
		if e != nil {
			return plan, e
		}
		values[i+1] = uint16(b)
	}
	if values[3], err = m.Read16(at + 2); err != nil {
		return plan, err
	}
	for i, v := range values {
		value := []byte(strconv.Itoa(int(v)))
		copy(s.Fields[i][:], value)
		s.Fields[i][len(value)] = 0
	}
	plan.Requester, err = r.compilePainting(s, c)
	if err != nil {
		return plan, err
	}
	mode, err := m.Read16(0xeb44)
	if err != nil {
		return plan, err
	}
	if mode != 8 {
		s.Scratch[10+0x77] = 0
		binary.BigEndian.PutUint16(s.Scratch[8:], 32)
		plan.Requester.Height = 32
	} else {
		x, y := values[1], values[2]
		dx, dy := uint16(64-y+x+1), uint16((y+x)>>1)+3
		descriptor := 0x219da
		half := int16(binary.BigEndian.Uint16(r.code[descriptor+4:]))
		height := int16(binary.BigEndian.Uint16(r.code[descriptor+6:]))
		plan.Marker = []NativePresentationSprite{{Sprite: (descriptor - 0x21626) / 12, X: int16(dx), Y: int16(dy), HalfWidth: half, Height: height, Routine: binary.BigEndian.Uint32(r.code[descriptor+8:])}}
		c.D7 = hudWord(c.D7, uint16(height*half/4))
	}
	selected, err := m.Read16(0xf10)
	if err != nil {
		return plan, err
	}
	number := uint16(0)
	for i := 10; i < len(s.Scratch) && s.Scratch[i] != 0; i++ {
		if s.Scratch[i] == 'c' {
			number += 2
			if number == selected {
				s.Scratch[i] = 'd'
				break
			}
		}
	}
	// Retain only the displayed prefix, while raw scratch beyond a hidden
	// terminator remains available to the original click scan.
	end := bytes.IndexByte(s.Scratch[10:], 0)
	plan.Requester.Text = append([]byte(nil), s.Scratch[10:10+end]...)
	plan.Action, err = r.clickPainting(s, m)
	if err != nil || plan.Action == 0 {
		return plan, err
	}
	plan.Audio = append(plan.Audio, 0x1d6)
	action := plan.Action
	if action < 10 {
		if action == selected {
			action = 0
		}
		return plan, m.Write16(0xf10, action)
	}
	if action >= 10 && action <= 16 {
		side, err := m.Read16(0xeb42)
		if err != nil {
			return plan, err
		}
		if action >= 14 {
			if side == 1 {
				side = 2
			} else {
				side = 1
			}
		}
		god := 0xe76a + int(int16(side*314))
		mana, err := m.Read32(god)
		if err != nil {
			return plan, err
		}
		if action == 10 || action == 14 {
			mana += 8000
		} else {
			before := mana
			mana -= 8000
			// ADDI.L -8000; BGT tests N==V as well as Z. A negative
			// old value that wraps positive still enters the CLR.L branch.
			if int32(before) <= 8000 {
				mana = 0
			}
		}
		return plan, m.Write32(god, mana)
	}
	switch action {
	case 18, 20:
		p, err := m.Read32(0xeb6a)
		if err != nil {
			return plan, err
		}
		value := uint8(0x7a)
		if action == 20 {
			value = 0x6a
		}
		return plan, m.Write8(int(int64(p)-int64(cb.AddressBase))+1, value)
	case 22, 24, 26, 28:
		field := int((action - 22) / 2)
		if cb.EditNumber == nil {
			return plan, fmt.Errorf("native painting numeric-input continuation missing")
		}
		value, err := cb.EditNumber(field, append([]byte(nil), paintingField(&s.Fields[field])...), c)
		if err != nil {
			return plan, err
		}
		if len(value) > 18 || bytes.IndexByte(value, 0) >= 0 {
			return plan, fmt.Errorf("native painting numeric field exceeds retained input buffer")
		}
		copy(s.Fields[field][:], value)
		s.Fields[field][len(value)] = 0
		parsed := nativePaintingNumber(value)
		switch field {
		case 0:
			return plan, m.Write16(at, uint16(parsed))
		case 1, 2:
			if int16(parsed) > 0 && int16(parsed) < 64 {
				return plan, m.Write8(at+3+field, uint8(parsed))
			}
		case 3:
			if parsed&1 == 0 && int16(parsed) < 126 {
				return plan, m.Write16(at+2, uint16(parsed))
			}
		}
	case 30:
		index := uint16(s.Index + 6)
		if int16(index) < 300 {
			s.Index = index
		}
	case 32:
		index := uint16(s.Index - 6)
		if int16(index) >= 0 {
			s.Index = index
		}
	default:
		return plan, fmt.Errorf("native painting action outside original dispatch")
	}
	return plan, nil
}

// $378e accepts digits anywhere, remembers any minus sign, and performs
// MULU on the low word before each next digit. It is not strconv.Atoi.
func nativePaintingNumber(text []byte) uint32 {
	var n uint32
	negative := false
	for _, b := range text {
		if b == '-' {
			negative = true
		}
		if int8(b) > int8('9') {
			continue
		}
		digit := uint8(b - '0')
		if int8(digit) >= 0 {
			n = uint32(uint16(n))*10 + uint32(digit)
		}
	}
	if negative {
		n = -n
	}
	return n
}
