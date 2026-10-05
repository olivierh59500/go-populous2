package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

// NativeImageRenderState retains the two mutable CODE spans touched by
// $ee32: last layer Y at $eee0, and the raw sound descriptor/counter bank.
// Keep one shared state across image draws and the original audio consumer.
type NativeImageRenderState struct {
	LastY     uint16
	AudioBank [0x532]byte // CODE $185a8, including untouched descriptor bytes.
}

type NativeEditorCursorRules struct{ code []byte }

func DecodeNativeEditorCursorRules(exe *amiga.Executable) (NativeEditorCursorRules, error) {
	var r NativeEditorCursorRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x26956 {
		return r, fmt.Errorf("native cursor/image tables missing")
	}
	r.code = exe.Hunks[0].Data
	return r, nil
}

func (r *NativeEditorCursorRules) NewImageState() NativeImageRenderState {
	var s NativeImageRenderState
	if r != nil && len(r.code) >= 0x185a8+len(s.AudioBank) {
		s.LastY = binary.BigEndian.Uint16(r.code[0xeee0:])
		copy(s.AudioBank[:], r.code[0x185a8:0x185a8+len(s.AudioBank)])
	}
	return s
}

func (r *NativeEditorCursorRules) word(a int) (uint16, error) {
	if r == nil || a < 0 || a&1 != 0 || a+2 > len(r.code) {
		return 0, fmt.Errorf("native cursor word outside CODE")
	}
	return binary.BigEndian.Uint16(r.code[a:]), nil
}

// Preview is the exact editor prefix of $1be8, ending before normal pointer
// admission/dispatch at $1c14. Even a zero tool writes D2.W; a zero edit flag
// preserves all eight incoming data registers.
func (r *NativeEditorCursorRules) Preview(m FollowerCleanupMemory, s *NativeImageRenderState, d *[8]uint32) ([]NativePresentationSprite, error) {
	if r == nil || s == nil || d == nil || m.Read16 == nil {
		return nil, fmt.Errorf("native editor cursor backing missing")
	}
	edit, err := m.Read16(0xf0e)
	if err != nil || edit == 0 {
		return nil, err
	}
	tool, err := m.Read16(0xf10)
	if err != nil {
		return nil, err
	}
	d[2] = hudWord(d[2], tool)
	if tool == 0 {
		return nil, nil
	}
	frame, err := r.word(0x20ac0 + int(int16(tool)))
	if err != nil {
		return nil, err
	}
	d[2] = hudWord(d[2], frame)
	x, err := m.Read16(0x138)
	if err != nil {
		return nil, err
	}
	y, err := m.Read16(0x13a)
	if err != nil {
		return nil, err
	}
	d[0], d[1] = hudWord(d[0], x), hudWord(d[1], y)
	return r.DrawImage(s, d)
}

// DrawImage translates $ee32/$eeda. D0/D1 are restored to their exact incoming
// longs. D2/D3/D6/D7 retain original hardware-blitter outputs, including
// clipping exits; D4/D5 are preserved by all829 actual descriptor primitives.
// The dormant EF5C software branch has no original descriptor and is not
// fabricated as a constant-register fallback for a modified executable.
func (r *NativeEditorCursorRules) DrawImage(s *NativeImageRenderState, d *[8]uint32) ([]NativePresentationSprite, error) {
	if r == nil || s == nil || d == nil {
		return nil, fmt.Errorf("native image render state/context missing")
	}
	x, y := d[0], d[1]
	defer func() { d[0], d[1] = x, y }()
	d[2] &= 0xffff
	frame := d[2]
	if frame >= 0x2c38 {
		return nil, nil
	}
	d[2] &= 3
	if d[2] != 0 {
		return nil, nil
	}
	cue, err := r.word(0x23d1a + int(frame) + 2)
	if err != nil {
		return nil, err
	}
	d[2] = uint32(cue)
	if cue != 0 && cue < 0x532 {
		if cue&1 != 0 {
			return nil, fmt.Errorf("native image sound counter is unaligned")
		}
		value := binary.BigEndian.Uint16(s.AudioBank[int(cue):])
		binary.BigEndian.PutUint16(s.AudioBank[int(cue):], value+1)
	}
	image, err := r.word(0x23d1a + int(frame))
	if err != nil {
		return nil, err
	}
	if int16(image) < 0 {
		return nil, nil
	}
	var plan []NativePresentationSprite
	seen := map[uint16]bool{}
	for layers := 0; layers < 256; layers++ {
		if seen[image] {
			return nil, fmt.Errorf("native image composite has an unbounded cycle")
		}
		seen[image] = true
		at := 0x26956 + int(uint16(image*2))
		if at < 0 || at+6 > len(r.code) {
			return nil, fmt.Errorf("native image layer outside CODE")
		}
		descriptor := int(binary.BigEndian.Uint16(r.code[at+2:]))
		da := 0x21626 + descriptor
		if descriptor%12 != 0 || descriptor/12 >= 830 || da+12 > len(r.code) {
			return nil, fmt.Errorf("native image descriptor outside original bank")
		}
		half, height := int16(binary.BigEndian.Uint16(r.code[da+4:])), binary.BigEndian.Uint16(r.code[da+6:])
		procedure := binary.BigEndian.Uint32(r.code[da+8:])
		px := uint16(int16(int8(r.code[at]))) - uint16(half) + uint16(x)
		py := uint16(int16(int8(r.code[at+1]))) - height + uint16(y)
		s.LastY = py
		d[2] = hudWord(d[2], height)
		if procedure == 0xf0ee || procedure == 0xf3a0 {
			plan = append(plan, NativePresentationSprite{Sprite: descriptor / 12, X: int16(px), Y: int16(py), HalfWidth: half, Height: int16(height), Routine: procedure})
			r.blitterRegisters(procedure, px, py, d)
		} else if procedure == 0xef5c {
			return nil, fmt.Errorf("modified software sprite needs real bitmap/register execution")
		}
		image = binary.BigEndian.Uint16(r.code[at+4:])
		if image == 0 {
			return plan, nil
		}
	}
	return nil, fmt.Errorf("native image exceeds bounded original layer chain")
}

func (r *NativeEditorCursorRules) blitterRegisters(procedure uint32, x, y uint16, d *[8]uint32) {
	wide := procedure == 0xf3a0
	stride := uint16(d[2]) * 2
	if wide {
		stride *= 2
	}
	d[7] = hudWord(d[7], stride)
	height := uint16(d[2])
	if int16(y) < 0 {
		removed := uint16(-y)
		before := height
		height -= removed
		d[2] = hudWord(d[2], height)
		// SUB.W; BLE uses N==V, so an overflowing NEG.W($8000)
		// must retain the original signed operand comparison.
		if int16(before) <= int16(removed) {
			return
		}
	} else {
		d[6] = 200
		if int16(y) >= 200 {
			return
		}
		d[3] = hudWord(d[3], y*40)
		bottom := uint16(y + height - 200)
		if int16(bottom) >= 0 {
			height -= bottom
			d[2] = hudWord(d[2], height)
		}
	}
	width := uint16(2)
	if int16(x) < 0 {
		if !wide {
			if int16(x) <= -16 {
				return
			}
		} else {
			if int16(x) <= -32 {
				return
			}
			if int16(x) >= -16 {
				width = 3
			}
		}
	} else {
		column := (x & 0xfff0) >> 3
		d[3] = hudWord(d[3], column)
		if !wide {
			d[6] = 40
			if int16(column) >= 40 {
				return
			}
			d[6] = 38
			if column == 38 {
				width = 1
			}
		} else {
			d[6] = 34
			if int16(column) <= 34 {
				width = 3
			} else {
				d[6] = 36
				if int16(column) <= 36 {
					width = 2
				} else {
					d[6] = 38
					if int16(column) > 38 {
						return
					}
					width = 1
				}
			}
		}
	}
	d[2] = hudWord(d[2], uint16(height<<6)+width)
}
