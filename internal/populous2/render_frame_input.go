package populous2

import "fmt"

// NativeRenderFrameChildren are real source call boundaries. Each callback
// owns its complete output registers and bitmap mutations; a required missing
// body is an error, never a guessed zero/preserved continuation.
type NativeRenderFrameChildren struct {
	SelectedHit func(*NativeFrameRegisterContext) error      // $2914/$11180.
	Painting    func(*NativeFrameRegisterContext) error      // $346a and real modal editing.
	DrawActor   func(int, *NativeFrameRegisterContext) error // $e45c, A3 is a BSS-relative raw address.
}

// ClampNativeFrameCamera is the complete$11180 MOVEM.W/clamp sequence.
func ClampNativeFrameCamera(m FollowerCleanupMemory, c *NativeFrameRegisterContext) error {
	if c == nil || !winMemoryValid(m) {
		return fmt.Errorf("native camera clamp backing/frame missing")
	}
	x, err := m.Read16(0x5f44)
	if err != nil {
		return err
	}
	y, err := m.Read16(0x5f46)
	if err != nil {
		return err
	}
	c.RestoreWord(0, x)
	c.RestoreWord(1, y)
	for _, reg := range []int{1, 0} {
		if int16(c.D[reg]) < 0 {
			c.D[reg] = 0
		}
		if int16(c.D[reg]) >= 56 {
			c.D[reg] = 56
		}
	}
	if err := m.Write16(0x5f44, uint16(c.D[0])); err != nil {
		return err
	}
	return m.Write16(0x5f46, uint16(c.D[1]))
}

// SelectedHit is the genuine$2914 prefix followed by the exact$11180 clamp.
func (r *NativeRenderFrameRules) SelectedHit(cb NativeRenderFrameCallbacks) error {
	if r == nil || cb.Frame == nil || !winMemoryValid(cb.Memory) {
		return fmt.Errorf("native selected camera-hit backing/frame missing")
	}
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	c.D[0] = m.long(0xf36)
	if c.D[0] == 0 {
		return m.err
	}
	at := int(int64(c.D[0]) - int64(c.AddressBase))
	c.D[0] = 0
	c.Byte(0, m.byte(at+6))
	c.Word(0, uint16(c.D[0])-4)
	m.putWord(0x5f44, uint16(c.D[0]))
	c.Byte(0, m.byte(at+8))
	c.Word(0, uint16(c.D[0])-4)
	m.putWord(0x5f46, uint16(c.D[0]))
	if m.err != nil {
		return m.err
	}
	return ClampNativeFrameCamera(cb.Memory, c)
}

// BackgroundCopy exposes the real $c8c2 direction: BLTAPT=$22, BLTDPT=$1e.
// The original body performs no data-register assignment.
func (r *NativeRenderFrameRules) BackgroundCopy(m FollowerCleanupMemory, c *NativeFrameRegisterContext) (NativePresentationCopy, error) {
	var p NativePresentationCopy
	if r == nil || c == nil || m.Read32 == nil {
		return p, fmt.Errorf("native background copy pointers/frame missing")
	}
	var err error
	p.Source, err = m.Read32(0x22)
	if err != nil {
		return p, err
	}
	p.Destination, err = m.Read32(0x1e)
	p.Bytes, p.Control, p.Size = 32000, 0x09f0, 0xc814
	return p, err
}

// Highlights is $29f8. Every XOR child restores all eight longs, while its
// parent retains the last MOVEM.W coordinates as sign-extended longs.
func (r *NativeRenderFrameRules) Highlights(cb NativeRenderFrameCallbacks) error {
	if r == nil || cb.Frame == nil || !winMemoryValid(cb.Memory) || len(cb.Bitmap) != 32000 {
		return fmt.Errorf("native highlights frame/bitmap missing")
	}
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	paint := func(a, pattern int) error {
		x, err := r.word(a)
		if err != nil {
			return err
		}
		y, err := r.word(a + 2)
		if err != nil {
			return err
		}
		c.RestoreWord(0, x)
		c.RestoreWord(1, y)
		width := 4
		if int16(x) > 36 {
			width = int(int16(39-x)) + 1
		}
		at := int(int16(y*40 + x))
		for row := 0; row < 12; row++ {
			if width < 1 || at < 0 || at+width > len(cb.Bitmap) || pattern+row*4+width > len(r.code) {
				return fmt.Errorf("native XOR highlight outside bitmap/CODE")
			}
			for column := 0; column < width; column++ {
				cb.Bitmap[at+column] ^= r.code[pattern+row*4+column]
			}
			at += 40
			if at >= 8000 {
				break
			}
		}
		return nil
	}
	if m.word(0xeb18) == 12 {
		if err := paint(0x3329c, 0x3dd68); err != nil {
			return err
		}
	}
	c.Word(0, m.word(0xeb42))
	c.D[0] = uint32(uint16(c.D[0])) * 314
	god := 0xe76a + int(int16(c.D[0]))
	mode := m.word(god + 12)
	for _, v := range []struct {
		mode   uint16
		offset int
	}{{16, 24}, {14, 20}, {20, 16}, {18, 12}} {
		if mode == v.mode {
			if err := paint(0x33294+v.offset, 0x3dd68); err != nil {
				return err
			}
		}
	}
	c.Word(0, m.word(0xf3a))
	c.Word(0, uint16(c.D[0])*2)
	if err := paint(0x332b0+int(int16(c.D[0])), 0x3dd98); err != nil {
		return err
	}
	return m.err
}

// text implements $509a's real fullD/software-byte behavior. The caller
// supplies its actual retained string and starting byte-column/pixel row.
func (r *NativeRenderFrameRules) text(cb NativeRenderFrameCallbacks, text []byte) error {
	c := cb.Frame
	y := uint16(c.D[1])
	c.D[1] = uint32(y) * 40
	start := int(int16(c.D[1])) + int(int16(c.D[0]))
	line, column := start, uint16(c.D[0])
	for _, ch := range append(append([]byte(nil), text...), 0) {
		c.D[0] = uint32(ch)
		if ch == 0 {
			return nil
		}
		c.Word(0, uint16(c.D[0])-32)
		if int16(c.D[0]) < 0 && uint16(c.D[0]) == 0xffea {
			column = uint16(start % 40)
			c.Word(1, uint16(c.D[1])+40)
			if int16(c.D[1]) >= 8000 {
				return fmt.Errorf("native text newline reached original ILLEGAL")
			}
			line += 320
			start = line
			continue
		}
		c.Word(0, uint16(c.D[0])<<5)
		if int16(column) >= 40 {
			return nil
		}
		font := 0x33c68 + int(int16(c.D[0]))
		if font < 0 || font+32 > len(r.code) {
			return fmt.Errorf("native text glyph outside retained CODE")
		}
		for row := 0; row < 8; row++ {
			for plane := 0; plane < 4; plane++ {
				at := start + row*40 + plane*8000
				if at < 0 || at >= len(cb.Bitmap) {
					return fmt.Errorf("native glyph outside retained bitmap")
				}
				cb.Bitmap[at] = r.code[font+row*4+plane]
			}
		}
		start++
		column++
	}
	return nil
}

// Countdown is $501a. An inactive signed timer preserves every long; text
// drawing restoresD6/D7 while its actual D0/D1 outputs remain visible.
func (r *NativeRenderFrameRules) Countdown(cb NativeRenderFrameCallbacks) error {
	if r == nil || cb.Frame == nil || !winMemoryValid(cb.Memory) || len(cb.Bitmap) != 32000 {
		return fmt.Errorf("native countdown frame/bitmap missing")
	}
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	timer := m.word(0xf14)
	if int16(timer) <= 0 {
		return m.err
	}
	m.putWord(0xf14, timer-1)
	for i := 0; i < 2; i++ {
		x, err := r.word(0x20752 + i*4)
		if err != nil {
			return err
		}
		y, err := r.word(0x20754 + i*4)
		if err != nil {
			return err
		}
		c.Word(0, x)
		c.Word(1, y)
		at := 0xf16 + i*12
		if m.byte(at) == 0 {
			continue
		}
		var text []byte
		for j := 0; j < 65536; j++ {
			v := m.byte(at + j)
			if m.err != nil {
				return m.err
			}
			if v == 0 {
				break
			}
			text = append(text, v)
			if j == 65535 {
				return fmt.Errorf("native countdown string unbounded")
			}
		}
		if err := r.text(cb, text); err != nil {
			return err
		}
	}
	return m.err
}

// Selected translates $1e18 through its original child boundaries. The
// actor draw at$e45c remains mandatory until its full raw renderer is bound.
func (r *NativeRenderFrameRules) Selected(cb NativeRenderFrameCallbacks, children NativeRenderFrameChildren) (NativeRenderFramePlan, error) {
	p := NativeRenderFramePlan{Pixels: []NativeHUDPixel{}, Sprites: []NativePresentationSprite{}}
	if r == nil || cb.Frame == nil || cb.Image == nil || !winMemoryValid(cb.Memory) || len(cb.Bitmap) != 32000 {
		return p, fmt.Errorf("native selected frame/bitmap missing")
	}
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	if m.word(0xf0e) != 0 {
		if children.Painting == nil {
			return p, fmt.Errorf("native selected painting child346a missing")
		}
		return p, children.Painting(c)
	}
	at, saved2, draw, err := r.selectedPrefix(cb, children, &p)
	if err != nil || !draw {
		return p, err
	}
	if children.DrawActor == nil {
		return p, fmt.Errorf("native selected actor childe45c missing")
	}
	if err := children.DrawActor(at, c); err != nil {
		return p, err
	}
	return p, r.selectedSuffix(cb, at, saved2, &p, nil)
}

// selectedPrefix executes the normal $1e18 entry exactly once, stopping at
// its actual $e45c actor call with the parent command word already hidden.
func (r *NativeRenderFrameRules) selectedPrefix(cb NativeRenderFrameCallbacks, children NativeRenderFrameChildren, p *NativeRenderFramePlan) (int, uint32, bool, error) {
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	if m.word(0x146) != 0 {
		c.RestoreWord(0, m.word(0x134))
		c.RestoreWord(1, m.word(0x136))
		x, err := r.word(0x21292)
		if err != nil {
			return 0, 0, false, err
		}
		c.Word(0, uint16(c.D[0])-x)
		if int16(m.word(0x134)) >= int16(x) {
			y, err := r.word(0x21294)
			if err != nil {
				return 0, 0, false, err
			}
			c.Word(1, uint16(c.D[1])-y)
			if int16(m.word(0x136)) >= int16(y) {
				width, err := r.word(0x21296)
				if err != nil {
					return 0, 0, false, err
				}
				before := int16(c.D[0])
				c.Word(0, uint16(c.D[0])-width)
				if before < int16(width) {
					height, err := r.word(0x21298)
					if err != nil {
						return 0, 0, false, err
					}
					before := int16(c.D[1])
					c.Word(1, uint16(c.D[1])-height)
					if before < int16(height) {
						m.putWord(0x142, 0)
						if children.SelectedHit == nil {
							return 0, 0, false, fmt.Errorf("native selected hit child2914 missing")
						}
						if err := children.SelectedHit(c); err != nil {
							return 0, 0, false, err
						}
					}
				}
			}
		}
	}
	timer := m.word(0xf30)
	if timer != 0 {
		m.putWord(0xf30, timer-1)
		if timer == 1 {
			m.putLong(0xf36, m.long(0xf32))
		}
	}
	var at int
	for tries := 0; tries < 3; tries++ {
		c.D[0] = m.long(0xf36)
		if c.D[0] == 0 {
			return 0, 0, false, m.err
		}
		at = int(int64(c.D[0]) - int64(c.AddressBase))
		if m.byte(at+12) != 0 {
			break
		}
		m.putLong(0xf36, 0)
		if m.word(0xf30) == 0 {
			return 0, 0, false, m.err
		}
		m.putWord(0xf30, 0)
		m.putLong(0xf36, m.long(0xf32))
		if tries == 2 {
			return 0, 0, false, fmt.Errorf("native selected fallback pointer unbounded")
		}
	}
	p.Drawn = true
	x, err := r.word(0x2128a)
	if err != nil {
		return 0, 0, false, err
	}
	y, err := r.word(0x2128c)
	if err != nil {
		return 0, 0, false, err
	}
	c.RestoreWord(0, x)
	c.RestoreWord(1, y)
	c.Word(2, m.word(0xeb18))
	saved2 := c.D[2]
	m.putWord(0xeb18, 0xffff)
	return at, saved2, true, m.err
}

// selectedSuffix resumes immediately after the actor returns, restoring the
// original command word before drawing its weapon and population indicators.
func (r *NativeRenderFrameRules) selectedSuffix(cb NativeRenderFrameCallbacks, at int, saved2 uint32, p *NativeRenderFramePlan, ownership func(bool, *NativeFrameRegisterContext) error) error {
	c, m := cb.Frame, nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	c.D[2] = saved2
	m.putWord(0xeb18, uint16(c.D[2]))
	x, err := r.word(0x2128e)
	if err != nil {
		return err
	}
	y, err := r.word(0x21290)
	if err != nil {
		return err
	}
	c.RestoreWord(0, x)
	c.RestoreWord(1, y)
	c.D[2] = uint32(m.byte(at + 25))
	c.Word(2, uint16(c.D[2])*2)
	weapon, err := r.word(0x20b60 + int(int16(c.D[2])))
	if err != nil {
		return err
	}
	c.Word(2, weapon+0x140)
	sprites, err := r.Images.DrawImage(cb.Image, &c.D)
	if err != nil {
		return err
	}
	if err := renderFrameSprites(cb, p, sprites); err != nil {
		return err
	}
	c.D[3] = m.long(at + 26)
	for pointer := 0x2129a; pointer < 0x212ba; pointer += 4 {
		x, err := r.word(pointer)
		if err != nil {
			return err
		}
		y, err := r.word(pointer + 2)
		if err != nil {
			return err
		}
		c.Word(0, x)
		c.Word(1, y)
		c.D[2] = c.D[3]
		if c.D[2] == 0 {
			return m.err
		}
		c.D[3] >>= 4
		c.Word(2, uint16(c.D[2])&15)
		c.Word(1, uint16(c.D[1])-uint16(c.D[2]))
		saved3 := c.D[3]
		if err := r.descriptorOwned(0x21626+0x84c, cb, p, ownership); err != nil {
			return err
		}
		c.D[3] = saved3
	}
	return m.err
}
