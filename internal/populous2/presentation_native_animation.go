package populous2

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
)

const (
	nativeAnimationInit  = uint32(0x494e4954)
	nativeAnimationFrame = uint32(0x46524d32)
	nativeAnimationStop  = uint32(0x53544f50)
)

// NativeScreenAnimation translates $106f8. JUDGE.PAK and END.PAK contain
// the original palette, a row-RLE first frame and vertical-column deltas.
// Code, Cursor and LoopOffset retain the native D0/A4/A5 control state.
type NativeScreenAnimation struct {
	Data               []byte
	Planes             [32000]byte
	Palette            [16]uint16
	Code               uint32
	Cursor, LoopOffset int
	otherPlanes        [32000]byte
}

func NewNativeScreenAnimation(data []byte) (*NativeScreenAnimation, error) {
	if len(data) < 36 {
		return nil, fmt.Errorf("native screen animation header missing")
	}
	a := &NativeScreenAnimation{Data: append([]byte(nil), data...), Code: nativeAnimationInit}
	if err := a.Advance(); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *NativeScreenAnimation) read8() (byte, error) {
	if a.Cursor < 0 || a.Cursor >= len(a.Data) {
		return 0, fmt.Errorf("native animation cursor outside resource")
	}
	v := a.Data[a.Cursor]
	a.Cursor++
	return v, nil
}

// Advance performs one native interpreter call and models the caller's buffer
// swap. Ordinary deltas update the other screen, which is two frames old;
// only FRM2 copies the current screen before its first delta. Callers schedule
// updates according to the screen's VBlank waits, not the simulation clock.
func (a *NativeScreenAnimation) Advance() error {
	if a == nil {
		return fmt.Errorf("native screen animation missing")
	}
	if a.Code == nativeAnimationStop {
		return nil
	}
	if a.Code == nativeAnimationInit {
		a.LoopOffset = int(binary.BigEndian.Uint32(a.Data))
		a.Cursor = 4
		for i := range a.Palette {
			a.Palette[i] = binary.BigEndian.Uint16(a.Data[a.Cursor:])
			if a.Palette[i] > 0xfff {
				return fmt.Errorf("invalid native animation palette")
			}
			a.Cursor += 2
		}
		for y := range 200 {
			for plane := range 4 {
				for x := 0; x < 40; {
					control, err := a.read8()
					if err != nil {
						return err
					}
					count := int(control) + 1
					if int8(control) < 0 {
						count = -int(int8(control)) + 1
					}
					if x+count > 40 {
						return fmt.Errorf("native animation RLE exceeds row")
					}
					value := byte(0)
					if int8(control) < 0 {
						value, err = a.read8()
						if err != nil {
							return err
						}
					}
					for range count {
						if int8(control) >= 0 {
							value, err = a.read8()
							if err != nil {
								return err
							}
						}
						a.Planes[plane*8000+y*40+x] = value
						x++
					}
				}
			}
		}
		a.Code = nativeAnimationFrame
		return nil
	}
	if a.Code == nativeAnimationFrame {
		a.otherPlanes = a.Planes
	} else {
		a.Planes, a.otherPlanes = a.otherPlanes, a.Planes
	}
	if err := a.delta(); err != nil {
		return err
	}
	if a.Code == nativeAnimationFrame {
		header := uint32(a.LoopOffset)
		if uint16(header) != 0 {
			a.LoopOffset = 0
			a.Code = header >> 16
		} else {
			a.LoopOffset = a.Cursor
			a.Code = header&0xffff0000 | header>>16
		}
		return nil
	}
	a.Code = a.Code&0xffff0000 | uint32(uint16(a.Code)-1)
	if uint16(a.Code) == 1 {
		if uint16(a.Code>>16) == 0 {
			a.Code = nativeAnimationStop
			// $1075a copies the displayed screen over the just-decoded
			// back screen, discarding this terminal delta.
			a.Planes = a.otherPlanes
		} else {
			a.Code = a.Code&0xffff0000 | a.Code>>16
			a.Cursor = a.LoopOffset
		}
	}
	return nil
}

func (a *NativeScreenAnimation) delta() error {
	for plane := range 4 {
		first, err := a.read8()
		if err != nil {
			return err
		}
		if first == 0xff {
			continue
		}
		a.Cursor--
		for column := range 40 {
			operations, err := a.read8()
			if err != nil {
				return err
			}
			row := 0
			for range int(operations) {
				control, err := a.read8()
				if err != nil {
					return err
				}
				if int8(control) > 0 {
					row += int(control)
					continue
				}
				count := int(control & 0x7f)
				value := byte(0)
				if control == 0 {
					length, err := a.read8()
					if err != nil {
						return err
					}
					count = int(length)
					value, err = a.read8()
					if err != nil {
						return err
					}
				}
				if count == 0 || row+count > 200 {
					return fmt.Errorf("native animation delta exceeds plane")
				}
				for range count {
					if control != 0 {
						value, err = a.read8()
						if err != nil {
							return err
						}
					}
					a.Planes[plane*8000+row*40+column] = value
					row++
				}
			}
		}
	}
	return nil
}

func nativeScreenIndices(planes []byte) ([]byte, error) {
	if len(planes) != 32000 {
		return nil, fmt.Errorf("native four-plane screen size differs")
	}
	pixels := make([]byte, 320*200)
	for y := range 200 {
		for x := range 320 {
			for plane := range 4 {
				pixels[y*320+x] |= (planes[plane*8000+y*40+x/8] >> uint(7-x%8) & 1) << uint(plane)
			}
		}
	}
	return pixels, nil
}

func nativeIndexedImage(pixels []byte, palette [16]color.RGBA) (*image.Paletted, error) {
	if len(pixels) != 320*200 {
		return nil, fmt.Errorf("native indexed screen size differs")
	}
	colors := make(color.Palette, 16)
	for index, color := range palette {
		colors[index] = color
	}
	img := image.NewPaletted(image.Rect(0, 0, 320, 200), colors)
	copy(img.Pix, pixels)
	return img, nil
}

func (a *NativeScreenAnimation) Image() (*image.Paletted, error) {
	if a == nil {
		return nil, fmt.Errorf("native screen animation missing")
	}
	pixels, err := nativeScreenIndices(a.Planes[:])
	if err != nil {
		return nil, err
	}
	var palette [16]color.RGBA
	for index, word := range a.Palette {
		palette[index] = AmigaColor(word)
	}
	return nativeIndexedImage(pixels, palette)
}
