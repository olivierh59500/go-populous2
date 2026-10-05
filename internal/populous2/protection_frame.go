package populous2

import (
	"encoding/binary"
	"fmt"
	"math/bits"

	"go-populous2/internal/amiga"
)

// NativeProtectionFrameRules retains the original FACES resource and the
// actual masked drawing register ABI. $314a is the Zeus protection requester;
// the town renderer invokes it, but it is not a town-information dialogue.
type NativeProtectionFrameRules struct {
	Render NativeRenderFrameRules
	Faces  [24]NativePreparedSprite
}

func DecodeNativeProtectionFrameRules(exe *amiga.Executable, faces []byte) (NativeProtectionFrameRules, error) {
	var r NativeProtectionFrameRules
	var err error
	r.Render, err = DecodeNativeRenderFrameRules(exe)
	if err != nil {
		return r, err
	}
	code := exe.Hunks[0].Data
	for i := range r.Faces {
		at := 0x212ba + i*12
		offset := int(binary.BigEndian.Uint32(code[at:])) - 0xa6cc
		width := int(binary.BigEndian.Uint16(code[at+4:])) * 2
		height := int(binary.BigEndian.Uint16(code[at+6:]))
		length := width / 8 * 5 * height
		if width != 32 || height <= 0 || offset < 0 || length > len(faces)-offset {
			return r, fmt.Errorf("native protection FACES descriptor%d outside resource", i)
		}
		prepared, e := PrepareNativeMaskedPlanes(faces[offset:offset+length], width, height)
		if e != nil {
			return r, e
		}
		r.Faces[i] = NativePreparedSprite{Width: width, Height: height, Planes: prepared}
	}
	return r, nil
}

type NativeProtectionFrameCallbacks struct {
	Code, Memory FollowerCleanupMemory
	CodeBase     uint32
	Frame        *NativeFrameRegisterContext
	Presentation *NativeFramePresentationState
	Bitmap       func(uint32) ([]byte, error)
	Sound        func(uint16, *NativeFrameRegisterContext) error
	// Beam reads the actual source VPOSR word at physical $dff006 once.
	// The source adds the retained interrupt counter; no host RNG replaces it.
	Beam func() (uint16, error)
	// Ownership implements the synchronous $e28/$e4c host blitter handoff.
	// It is distinct from a video wait; its MOVEM restores every data register.
	Ownership func(owned bool, frame *NativeFrameRegisterContext) error
	// Call supplies real resource load/reload.
	// Their native children can suspend without restarting the requester.
	Call func(NativeFileFrameCall, *uint32) (NativeCommandFrameResult, error)
}

type NativeProtectionFrameStep struct {
	Complete, Waiting bool
	PC                int
	Calls             []int
}

// NativeProtectionFrameState is the resumable $314a..$3348 frame. Address
// arguments are kept separately from numeric BSS/CODE labels. The complete
// caller data registers are restored only after a correct answer, the final
// fade and the genuine graphics reload have completed.
type NativeProtectionFrameState struct {
	Started, Finished bool
	PC                int
	Registers, Saved  [8]uint32
	A                 [7]NativeRequesterAddress
	savedA            [7]NativeRequesterAddress
	ChildActive       bool
	ChildPhase        uint32
	ChildRoutine      int
	childSaved        [8]uint32
	selected          uint32
	palette           *NativeFramePaletteState
	failed            error
}

func (s *NativeProtectionFrameState) Advance(r *NativeProtectionFrameRules, cb NativeProtectionFrameCallbacks) (out NativeProtectionFrameStep, failure error) {
	if s == nil || r == nil || cb.Frame == nil || cb.Presentation == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) {
		return out, fmt.Errorf("native protection frame/backing missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		s.Started, s.PC = true, 0x314a
		s.Registers, s.Saved = cb.Frame.D, cb.Frame.D
		s.savedA = s.A
	}
	cb.Frame.D = s.Registers
	defer func() {
		s.Registers, out.PC = cb.Frame.D, s.PC
		if failure != nil {
			s.failed = failure
		}
	}()
	if s.Finished {
		out.Complete = true
		return out, nil
	}
	c, code, m := cb.Frame, cb.Code, cb.Memory
	b := nativeRequesterFrameBacking{Code: code, Memory: m, CodeBase: cb.CodeBase, Frame: c, Bitmap: cb.Bitmap, Sound: cb.Sound}
	caddr := func(at int) NativeRequesterAddress {
		return NativeRequesterAddress{Address: cb.CodeBase + uint32(at), Code: true}
	}
	child := func(routine, next int, preserve bool) (bool, error) {
		if cb.Call == nil {
			return false, fmt.Errorf("native protection child%x missing", routine)
		}
		if !s.ChildActive {
			s.ChildActive, s.ChildRoutine = true, routine
			s.childSaved = c.D
			out.Calls = append(out.Calls, routine)
		}
		if s.ChildRoutine != routine {
			return false, fmt.Errorf("native protection child changed while suspended")
		}
		result, e := cb.Call(NativeFileFrameCall{Routine: routine, A: s.A, Frame: c}, &s.ChildPhase)
		if e != nil {
			return false, e
		}
		if !result.Complete {
			out.Waiting = true
			return false, nil
		}
		if preserve {
			c.D = s.childSaved
		}
		s.ChildActive, s.ChildPhase, s.PC = false, 0, next
		return true, nil
	}
	palette := func(first, second, next int) (bool, error) {
		if s.palette == nil {
			bank := func(at int) (NativeFramePaletteBank, error) {
				p := NativeFramePaletteBank{Address: cb.CodeBase + uint32(at)}
				for i := range p.Words {
					var e error
					p.Words[i], e = code.Read16(at + i*2)
					if e != nil {
						return p, e
					}
				}
				return p, nil
			}
			source, e := bank(first)
			if e != nil {
				return false, e
			}
			target, e := bank(second)
			if e != nil {
				return false, e
			}
			s.A[2], s.A[3] = caddr(first), caddr(second)
			s.palette = NewNativeFramePaletteState(source, target, cb.CodeBase)
		}
		done, e := s.palette.Advance(cb.Presentation, c, m)
		if e != nil || !done {
			out.Waiting = !done
			return false, e
		}
		s.palette, s.PC = nil, next
		return true, nil
	}
	for transitions := 0; transitions < 128; transitions++ {
		switch s.PC {
		case 0x314a:
			if e := protectionFrameOwnership(cb, true); e != nil {
				return out, e
			}
			s.PC = 0x3154
		case 0x3154:
			cursor, e := code.Read16(0xa2a)
			if e != nil {
				return out, e
			}
			if e := m.Write16(0xddc, cursor); e != nil {
				return out, e
			}
			if e := code.Write16(0xa2a, 0); e != nil {
				return out, e
			}
			cb.Presentation.Input.Mouse.Image = 0
			c.Word(0, 8)
			s.PC = 0x316a
		case 0x316a:
			ok, e := child(0x19cd0, 0x3170, false)
			if e != nil || !ok {
				return out, e
			}
		case 0x3170:
			if cb.Beam == nil {
				return out, fmt.Errorf("native protection VPOSR reader missing")
			}
			beam, e := cb.Beam()
			if e != nil {
				return out, e
			}
			counter, e := m.Read16(0xe)
			if e != nil {
				return out, e
			}
			c.D[0] = uint32(beam)
			c.Word(0, uint16(c.D[0])+counter)
			if e := frameDivide(c, 0, 64); e != nil {
				return out, e
			}
			c.Swap(0)
			c.D[0] &^= 1
			selection, e := code.Read16(0x335e + int(int16(c.D[0])))
			if e != nil {
				return out, e
			}
			c.Word(0, selection)
			c.Word(2, uint16(c.D[0]))
			c.Word(0, uint16(c.D[0])<<7)
			for i, reg := range []int{4, 5, 6} {
				c.Word(0, bits.RotateLeft16(uint16(c.D[0]), 3))
				c.Word(reg, uint16(c.D[0]))
				c.Word(reg, uint16(c.D[reg])&7)
				if e := code.Write8(0x326e-i, uint8(c.D[reg])); e != nil {
					return out, e
				}
			}
			s.selected = c.D[2]
			s.A[1], s.A[2], s.A[3] = caddr(0x6e02), NativeRequesterAddress{}, caddr(0x326f)
			c.D[3] = 1
			if e := b.compile(s.A[1].Address, nil); e != nil {
				return out, e
			}
			s.PC = 0x31ce
		case 0x31ce:
			ready, e := m.Read16(0xa)
			if e != nil {
				return out, e
			}
			if ready == 0 {
				out.Waiting = true
				return out, nil
			}
			if e := m.Write16(0xa, 0); e != nil {
				return out, e
			}
			s.PC = 0x31da
		case 0x31da:
			ready, e := m.Read16(0xa)
			if e != nil {
				return out, e
			}
			if ready == 0 {
				out.Waiting = true
				return out, nil
			}
			target, e := m.Read32(0x1e)
			if e != nil {
				return out, e
			}
			start, e := code.Read16(0xab4e)
			if e != nil {
				return out, e
			}
			column, e := code.Read16(0xab50)
			if e != nil {
				return out, e
			}
			row, e := code.Read16(0xab52)
			if e != nil {
				return out, e
			}
			c.Word(0, column)
			c.Word(1, row)
			if e := b.text(target, 0xab4e+int(int16(start))); e != nil {
				return out, e
			}
			c.D[2] = s.selected
			s.PC = 0x3214
		case 0x3214:
			if e := r.portrait(cb); e != nil {
				return out, e
			}
			gate, e := m.Read16(0x3b0)
			if e != nil {
				return out, e
			}
			s.PC = 0x3234
			if gate == 0 {
				s.PC = 0x3222
			}
		case 0x3222:
			ok, e := palette(0x3361a, 0x3c528, 0x3234)
			if e != nil || !ok {
				return out, e
			}
		case 0x3234:
			if e := fileFrameSwap(b, cb.Presentation); e != nil {
				return out, e
			}
			if _, e := b.click(); e != nil {
				return out, e
			}
			start, e := code.Read16(0xab4e)
			if e != nil {
				return out, e
			}
			at := 0xab4e + int(int16(start)) + 0x2f3
			action := uint16(c.D[0])
			branch, e := code.Read16(0x3258 + int(int16(action)))
			if e != nil {
				return out, e
			}
			c.Word(0, branch)
			target := 0x3258 + int(int16(c.D[0]))
			if target == 0x31ce {
				s.PC = target
				continue
			}
			switch {
			case target >= 0x3270 && target <= 0x327c && target%4 == 0:
				at += (0x327c - target) / 4
				v, e := code.Read8(at)
				if e != nil {
					return out, e
				}
				v++
				if int8(v) > '9' {
					v = '0'
				}
				if e := code.Write8(at, v); e != nil {
					return out, e
				}
			case target >= 0x3290 && target <= 0x329c && target%4 == 0:
				at += (0x329c - target) / 4
				v, e := code.Read8(at)
				if e != nil {
					return out, e
				}
				v--
				if int8(v) < '0' {
					v = '9'
				}
				if e := code.Write8(at, v); e != nil {
					return out, e
				}
			case target == 0x32b0:
				c.D[2] = s.selected
				c.Word(2, uint16(c.D[2])&0x1ff)
				c.D[2] = uint32(uint16(c.D[2])) * 7
				c.Word(0, uint16(c.D[2]))
				c.Word(0, uint16(c.D[0])>>1)
				c.Word(2, uint16(c.D[2])+uint16(c.D[0]))
				c.D[0], c.D[1] = 0, 0
				for i := 0; i < 4; i++ {
					v, e := code.Read8(at + i)
					if e != nil {
						return out, e
					}
					c.Byte(0, v)
					c.Byte(0, uint8(c.D[0])-'0')
					c.Word(1, uint16(c.D[1])+uint16(c.D[0]))
					if i != 3 {
						c.D[1] = uint32(uint16(c.D[1])) * 10
					}
				}
				for i := 0; i < 4; i++ {
					if e := code.Write8(at+i, '0'); e != nil {
						return out, e
					}
				}
				if uint16(c.D[2]) == uint16(c.D[1]) {
					s.PC = 0x3310
					continue
				}
			default:
				return out, fmt.Errorf("native protection action%x reaches unimplemented source%x", action, target)
			}
			s.PC = 0x31ce
		case 0x3310:
			cursor, e := m.Read16(0xddc)
			if e != nil {
				return out, e
			}
			if e := code.Write16(0xa2a, cursor); e != nil {
				return out, e
			}
			cb.Presentation.Input.Mouse.Image = cursor
			s.PC = 0x331e
		case 0x331e:
			ok, e := palette(0x33844, 0x3361a, 0x3330)
			if e != nil || !ok {
				return out, e
			}
		case 0x3330:
			if e := m.Write16(0x3b8, 1); e != nil {
				return out, e
			}
			s.PC = 0x3338
		case 0x3338:
			ok, e := child(0x1a32a, 0x333e, false)
			if e != nil || !ok {
				return out, e
			}
		case 0x333e:
			c.D = s.Saved
			s.A = s.savedA
			s.PC = 0x3342
		case 0x3342:
			if e := protectionFrameOwnership(cb, false); e != nil {
				return out, e
			}
			s.PC = 0x3348
		case 0x3348:
			s.Finished, out.Complete, s.PC = true, true, 0
			return out, nil
		default:
			return out, fmt.Errorf("native protection continuation unknown%x", s.PC)
		}
	}
	return out, fmt.Errorf("native protection frame transition limit exceeded")
}

func protectionFrameOwnership(cb NativeProtectionFrameCallbacks, owned bool) error {
	if cb.Ownership == nil {
		return fmt.Errorf("native protection blitter ownership callback missing")
	}
	saved := cb.Frame.D
	e := cb.Ownership(owned, cb.Frame)
	cb.Frame.D = saved
	return e
}

// portrait translates $baa8: the opaque48x64 backing is copied by native
// word order, then mouth/eyes/headpiece use the real $f3a0 register outputs.
func (r *NativeProtectionFrameRules) portrait(cb NativeProtectionFrameCallbacks) error {
	c, code := cb.Frame, cb.Code
	if cb.Bitmap == nil {
		return fmt.Errorf("native protection bitmap resolver missing")
	}
	target, e := cb.Memory.Read32(0x1e)
	if e != nil {
		return e
	}
	bitmap, e := cb.Bitmap(target)
	if e != nil {
		return e
	}
	if len(bitmap) != 32000 {
		return fmt.Errorf("native protection bitmap size differs")
	}
	base, e := code.Read16(0x334a)
	if e != nil {
		return e
	}
	c.D[1] = 63
	source := 0x3ddf8
	for row := 0; row < 64; row++ {
		for plane := 0; plane < 4; plane++ {
			for word := 0; word < 3; word++ {
				v, e := code.Read16(source)
				if e != nil {
					return e
				}
				source += 2
				c.RestoreWord(2+word, v)
				at := int(int16(base)) + row*40 + plane*8000 + word*2
				if at < 0 || at > len(bitmap)-2 {
					return fmt.Errorf("native protection backing outside bitmap")
				}
				binary.BigEndian.PutUint16(bitmap[at:], v)
			}
		}
		c.Word(1, uint16(c.D[1])-1)
	}
	c.D[3] = 2
	for part := 0; part < 3; part++ {
		c.D[0] = 0
		variant, e := code.Read8(0x326e - part)
		if e != nil {
			return e
		}
		c.Byte(0, variant)
		bank, e := code.Read16(0x334c + part*6)
		if e != nil {
			return e
		}
		c.Word(0, uint16(c.D[0])+bank)
		index := int(uint16(c.D[0]))
		c.D[0] = uint32(uint16(c.D[0])) * 12
		descriptor := 0x212ba + int(int16(c.D[0]))
		x, e := code.Read16(0x334e + part*6)
		if e != nil {
			return e
		}
		y, e := code.Read16(0x3350 + part*6)
		if e != nil {
			return e
		}
		height, e := code.Read16(descriptor + 6)
		if e != nil {
			return e
		}
		c.Word(0, x)
		c.Word(1, y)
		c.Word(2, height)
		c.Word(4, height)
		c.Word(4, -uint16(c.D[4])+16)
		if int16(c.D[4]) > 0 {
			c.Word(1, uint16(c.D[1])+uint16(c.D[4]))
		}
		if index < 0 || index >= len(r.Faces) {
			return fmt.Errorf("native protection face descriptor%d needs adjacent resource backing", index)
		}
		sprite := NativePresentationSprite{Sprite: index, X: int16(c.D[0]), Y: int16(c.D[1]), HalfWidth: 16, Height: int16(height), Routine: 0xf3a0}
		saved3 := c.D[3]
		if e := protectionFrameOwnership(cb, false); e != nil {
			return e
		}
		if e := r.Render.primitiveRegisters(0xf3a0, c); e != nil {
			return e
		}
		if e := r.Faces[index].Paint(sprite, bitmap); e != nil {
			return e
		}
		if e := protectionFrameOwnership(cb, true); e != nil {
			return e
		}
		c.D[3] = saved3
		c.Word(3, uint16(c.D[3])-1)
	}
	return nil
}
