package populous2

import (
	"fmt"
	"strconv"

	"go-populous2/internal/amiga"
)

type NativeEditorFrameRules struct {
	Frames NativeRenderFrameRules
	Keys   NativeInputRules
}
type NativeEditorFrameState struct {
	Started, Complete    bool
	PC                   int
	Registers            [8]uint32
	Modal                NativeFileTextModal
	ModalReturn          int
	ModalField, ModalEnd int
	Failed               error
}
type NativeEditorFrameCallbacks struct {
	NativeFileFrameCallbacks
	Image  *NativeImageRenderState
	Sprite func(NativePresentationSprite, []byte) error
}
type NativeEditorFrameStep struct {
	Complete, Waiting bool
	PC                int
	ChildRoutine      int
}

func DecodeNativeEditorFrameRules(exe *amiga.Executable) (NativeEditorFrameRules, error) {
	var r NativeEditorFrameRules
	var e error
	r.Frames, e = DecodeNativeRenderFrameRules(exe)
	if e != nil {
		return r, e
	}
	r.Keys, e = DecodeNativeInputRules(exe)
	return r, e
}

// Advance is the genuine$346a painting renderer and action dispatch. CODE
// fields/requester workspace and real modal waits survive across host frames.
func (s *NativeEditorFrameState) Advance(r *NativeEditorFrameRules, cb NativeEditorFrameCallbacks) (out NativeEditorFrameStep, failure error) {
	if s == nil || r == nil || cb.Frame == nil || cb.Image == nil || cb.Bitmap == nil || cb.Presentation == nil || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.Code) {
		return out, fmt.Errorf("native editor frame backing missing")
	}
	if s.Failed != nil {
		return out, s.Failed
	}
	if !s.Started {
		s.Started = true
		s.PC = 0x346a
		s.Registers = cb.Frame.D
	}
	cb.Frame.D = s.Registers
	defer func() {
		s.Registers = cb.Frame.D
		out.PC = s.PC
		if failure != nil {
			s.Failed = failure
		}
	}()
	if s.Complete {
		out.Complete = true
		return out, nil
	}
	c, m, code := cb.Frame, cb.Memory, cb.Code
	b := nativeRequesterFrameBacking{Code: code, Memory: m, CodeBase: cb.CodeBase, Frame: c, Bitmap: cb.Bitmap, Sound: cb.Sound, ReadAbsolute: cb.ReadAbsolute}
	readW := func(a int) (uint16, error) { return code.Read16(a) }
	record := func() (int, error) { index, e := readW(0x37bc); return 0xdde + int(int16(index)), e }
	number := func(field int) error {
		c.D[0] &= 0xffff
		text := []byte(strconv.Itoa(int(c.D[0])))
		for i, v := range append(text, 0) {
			if e := code.Write8(field+i, v); e != nil {
				return e
			}
		}
		return nil
	}
	parse := func(field int) error {
		c.D[1], c.D[2] = 0, 0
		for steps := 0; steps < 65536; steps++ {
			c.D[0] = 0
			v, e := code.Read8(field + steps)
			if e != nil {
				return e
			}
			c.Byte(0, v)
			if v == 0 {
				if uint16(c.D[2]) != 0 {
					c.D[1] = -c.D[1]
				}
				return nil
			}
			if v == '-' {
				c.D[2] = 0xffffffff
			}
			if int8(v) <= int8('9') {
				before := int8(v)
				c.Byte(0, v-'0')
				if before >= int8('0') {
					c.D[1] = uint32(uint16(c.D[1])) * 10
					c.D[1] += c.D[0]
				}
			}
		}
		return fmt.Errorf("native editor numeric field unbounded")
	}
	finish := func() (NativeEditorFrameStep, error) {
		s.Complete = true
		s.PC = 0x3588
		out.Complete = true
		return out, nil
	}
	for transitions := 0; transitions < 32; transitions++ {
		if s.Modal.Active {
			done, e := s.Modal.advance(b, cb.Presentation, r.Keys)
			if e != nil {
				return out, e
			}
			if !done {
				out.Waiting = true
				return out, nil
			}
			s.PC = s.ModalReturn
		}
		switch s.PC {
		case 0x346a:
			at, e := record()
			if e != nil {
				return out, e
			}
			v, e := m.Read16(at)
			if e != nil {
				return out, e
			}
			c.Word(0, v)
			if e := number(0x37ce); e != nil {
				return out, e
			}
			for i, offset := range []int{4, 5} {
				v, e := m.Read8(at + offset)
				if e != nil {
					return out, e
				}
				c.D[0] = uint32(v)
				if e := number(0x37e2 + i*20); e != nil {
					return out, e
				}
			}
			v, e = m.Read16(at + 2)
			if e != nil {
				return out, e
			}
			c.Word(0, v)
			if e := number(0x380a); e != nil {
				return out, e
			}
			c.D[3] = 1
			params := []NativeRequesterAddress{}
			for _, field := range []int{0x37ce, 0x37e2, 0x37f6, 0x380a} {
				params = append(params, NativeRequesterAddress{Code: true, Address: cb.CodeBase + uint32(field)})
			}
			if e := b.compile(cb.CodeBase+0x7a14, params); e != nil {
				return out, e
			}
			start, e := readW(0xab4e)
			if e != nil {
				return out, e
			}
			text := 0xab4e + int(int16(start))
			mode, e := m.Read16(0xeb44)
			if e != nil {
				return out, e
			}
			if mode != 8 {
				if e := code.Write8(text+0x77, 0); e != nil {
					return out, e
				}
				if e := code.Write16(0xab56, 32); e != nil {
					return out, e
				}
			} else {
				y, e := m.Read8(at + 5)
				if e != nil {
					return out, e
				}
				x, e := m.Read8(at + 4)
				if e != nil {
					return out, e
				}
				c.D[1] = uint32(y)
				c.D[0] = 64
				c.Word(0, uint16(c.D[0])-uint16(c.D[1]))
				c.D[3] = uint32(x)
				c.Word(0, uint16(c.D[0])+uint16(c.D[3])+1)
				c.Word(1, (uint16(c.D[1])+uint16(c.D[3]))>>1)
				c.Word(1, uint16(c.D[1])+3)
				target, e := m.Read32(0x1e)
				if e != nil {
					return out, e
				}
				bitmap, e := cb.Bitmap(target)
				if e != nil {
					return out, e
				}
				plan := NativeRenderFramePlan{}
				if e := r.Frames.descriptor(0x219da, NativeRenderFrameCallbacks{Frame: c, Bitmap: bitmap, Sprite: cb.Sprite}, &plan); e != nil {
					return out, e
				}
			}
			c.D[1] = 0
			tool, e := m.Read16(0xf10)
			if e != nil {
				return out, e
			}
			for offset := text; offset < text+65536; offset++ {
				v, e := code.Read8(offset)
				if e != nil {
					return out, e
				}
				c.Byte(0, v)
				if v == 0 {
					break
				}
				if v == 'c' {
					c.Word(1, uint16(c.D[1])+2)
					if uint16(c.D[1]) == tool {
						if e := code.Write8(offset, 'd'); e != nil {
							return out, e
						}
						break
					}
				}
			}
			x, e := readW(0xab50)
			if e != nil {
				return out, e
			}
			y, e := readW(0xab52)
			if e != nil {
				return out, e
			}
			c.Word(0, x)
			c.Word(1, y)
			target, e := m.Read32(0x1e)
			if e != nil {
				return out, e
			}
			if e := b.text(target, text); e != nil {
				return out, e
			}
			end, e := b.clickAt(false)
			if e != nil {
				return out, e
			}
			action := uint16(c.D[0])
			dispatch, e := readW(0x358a + int(int16(action)))
			if e != nil {
				return out, e
			}
			c.Word(1, dispatch)
			s.PC = 0x358a + int(int16(c.D[1]))
			if s.PC == 0x3588 {
				return finish()
			}
			if s.PC == 0x36b0 || s.PC == 0x36dc || s.PC == 0x3716 || s.PC == 0x3750 {
				field := map[int]int{0x36b0: 0x37ce, 0x36dc: 0x37e2, 0x3716: 0x37f6, 0x3750: 0x380a}[s.PC]
				s.ModalReturn = s.PC + 12
				s.ModalField, s.ModalEnd = field, int(int64(end)-int64(cb.CodeBase))
				s.PC = 0x4bba
				out.Waiting, out.ChildRoutine = true, 0x4bba
				return out, nil
			}
		case 0x4bba:
			if e := s.Modal.begin(b, s.ModalField, s.ModalEnd); e != nil {
				return out, e
			}
		case 0x35ac:
			selected, e := m.Read16(0xf10)
			if e != nil {
				return out, e
			}
			if uint16(c.D[0]) == selected {
				c.Word(0, 0)
			}
			if e := m.Write16(0xf10, uint16(c.D[0])); e != nil {
				return out, e
			}
			return finish()
		case 0x35c0, 0x35de, 0x3602, 0x362c:
			owner, e := m.Read16(0xeb42)
			if e != nil {
				return out, e
			}
			if s.PC >= 0x3602 {
				c.Word(0, 2)
				if owner != 1 {
					c.Word(0, 1)
				}
			} else {
				c.Word(0, owner)
			}
			c.D[0] = uint32(uint16(c.D[0])) * 314
			god := 0xe76a + int(int16(c.D[0]))
			mana, e := m.Read32(god)
			if e != nil {
				return out, e
			}
			if s.PC == 0x35c0 || s.PC == 0x3602 {
				mana += 8000
			} else {
				before := int32(mana)
				mana -= 8000
				if before <= 8000 {
					mana = 0
				}
			}
			if e := m.Write32(god, mana); e != nil {
				return out, e
			}
			return finish()
		case 0x365c, 0x366c:
			pointer, e := m.Read32(0xeb6a)
			if e != nil {
				return out, e
			}
			value := uint8(0x6a)
			if s.PC == 0x366c {
				value = 0x7a
			}
			if e := m.Write8(int(int64(pointer)-int64(c.AddressBase))+1, value); e != nil {
				return out, e
			}
			return finish()
		case 0x367c, 0x3698:
			index, e := readW(0x37bc)
			if e != nil {
				return out, e
			}
			c.Word(0, index)
			if s.PC == 0x367c {
				c.Word(0, uint16(c.D[0])+6)
				if int16(c.D[0]) < 300 {
					if e := code.Write16(0x37bc, uint16(c.D[0])); e != nil {
						return out, e
					}
				}
			} else {
				c.Word(0, uint16(c.D[0])-6)
				if int16(index) >= 6 {
					if e := code.Write16(0x37bc, uint16(c.D[0])); e != nil {
						return out, e
					}
				}
			}
			return finish()
		case 0x36bc, 0x36e8, 0x3722, 0x375c:
			field := map[int]int{0x36bc: 0x37ce, 0x36e8: 0x37e2, 0x3722: 0x37f6, 0x375c: 0x380a}[s.PC]
			if e := parse(field); e != nil {
				return out, e
			}
			at, e := record()
			if e != nil {
				return out, e
			}
			switch field {
			case 0x37ce:
				if e := m.Write16(at, uint16(c.D[1])); e != nil {
					return out, e
				}
			case 0x37e2, 0x37f6:
				if int16(c.D[1]) > 0 && int16(c.D[1]) < 64 {
					offset := 4
					if field == 0x37f6 {
						offset = 5
					}
					if e := m.Write8(at+offset, uint8(c.D[1])); e != nil {
						return out, e
					}
				}
			case 0x380a:
				if c.D[1]&1 == 0 && int16(c.D[1]) < 126 {
					if e := m.Write16(at+2, uint16(c.D[1])); e != nil {
						return out, e
					}
				}
			}
			return finish()
		default:
			return out, fmt.Errorf("native editor PC%#x unsupported", s.PC)
		}
	}
	return out, fmt.Errorf("native editor exceeded source transitions")
}
