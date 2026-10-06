package populous2

import (
	"encoding/binary"
	"fmt"
)

// WriteRGBA decodes the active Copper/bitplanes into caller-owned pixels.
// Optional hardware-sprite composition follows the original pair0/1 cursor
// pointers and attached four-plane palette. It does not mutate source RAM.
func (s *NativeFramePresentationState) WriteRGBA(dst []byte, cursor bool) error {
	if s == nil || len(dst) != 320*200*4 {
		return fmt.Errorf("native displayed pixel backing missing")
	}
	at, err := s.chipAt(s.ActiveCopper, 2)
	if err != nil {
		return err
	}
	var planes [4]uint32
	var sprites [8]uint32
	var colors [32]uint16
	var displayX, displayY uint16
	terminated := false
	for n := 0; n < 256; n++ {
		if at+4 > len(s.Chip) {
			return fmt.Errorf("native displayed Copper terminator missing")
		}
		reg, value := binary.BigEndian.Uint16(s.Chip[at:]), binary.BigEndian.Uint16(s.Chip[at+2:])
		at += 4
		if reg == 0xffff && value == 0xfffe {
			terminated = true
			break
		}
		if reg == 0x8e {
			displayX = value & 255
			displayY = value >> 8
		}
		if reg >= 0x180 && reg <= 0x1be && reg&1 == 0 {
			colors[(reg-0x180)/2] = value
		}
		if reg >= 0xe0 && reg <= 0xee && reg&1 == 0 {
			p := &planes[(reg-0xe0)/4]
			if reg&2 == 0 {
				*p = *p&65535 | uint32(value)<<16
			} else {
				*p = *p&0xffff0000 | uint32(value)
			}
		}
		if reg >= 0x120 && reg <= 0x13e && reg&1 == 0 {
			p := &sprites[(reg-0x120)/4]
			if reg&2 == 0 {
				*p = *p&65535 | uint32(value)<<16
			} else {
				*p = *p&0xffff0000 | uint32(value)
			}
		}
	}
	if !terminated {
		return fmt.Errorf("native displayed Copper exceeds actual list")
	}
	var planar [4][]byte
	for i, p := range planes {
		off, err := s.chipAt(p, 8000)
		if err != nil {
			return err
		}
		planar[i] = s.Chip[off : off+8000]
	}
	write := func(pixel int, index uint8) {
		v := colors[index]
		off := pixel * 4
		dst[off], dst[off+1], dst[off+2], dst[off+3] = byte(v>>8&15)*17, byte(v>>4&15)*17, byte(v&15)*17, 255
	}
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			off := y*40 + x/8
			bit := uint(7 - x%8)
			index := uint8(0)
			for p := 0; p < 4; p++ {
				index |= (planar[p][off] >> bit & 1) << uint(p)
			}
			write(y*320+x, index)
		}
	}
	if !cursor {
		return nil
	}
	// The original pointer uses attached sprite0/1. Both source headers are
	// updated by790/91C; unrelated zero sprites are not invented as images.
	for pair := 0; pair < 2; pair += 2 {
		var banks [2][]byte
		for i := 0; i < 2; i++ {
			off := int(int64(sprites[pair+i]) - int64(s.PointerBase))
			if off < 0 || off > len(s.PointerData)-4 {
				return fmt.Errorf("native displayed cursor source outside pointer RAM")
			}
			banks[i] = s.PointerData[off:]
		}
		pos, control := binary.BigEndian.Uint16(banks[1]), binary.BigEndian.Uint16(banks[1][2:])
		if control&0x80 == 0 {
			return fmt.Errorf("native displayed cursor expects attached sprite pair")
		}
		start := int(pos>>8) + int(control&4)<<6
		end := int(control>>8) + int(control&2)<<7
		x := int(pos&255)*2 + int(control&1) - int(displayX&0xfe)
		y := start - int(displayY)
		height := end - start
		if height < 0 || height > 256 {
			return fmt.Errorf("native displayed cursor row extent invalid")
		}
		for row := 0; row < height; row++ {
			off := 4 + row*4
			if off+4 > len(banks[0]) || off+4 > len(banks[1]) {
				return fmt.Errorf("native displayed cursor rows outside actual RAM")
			}
			words := [4]uint16{binary.BigEndian.Uint16(banks[0][off:]), binary.BigEndian.Uint16(banks[0][off+2:]), binary.BigEndian.Uint16(banks[1][off:]), binary.BigEndian.Uint16(banks[1][off+2:])}
			for col := 0; col < 16; col++ {
				px, py := x+col, y+row
				if px < 0 || px >= 320 || py < 0 || py >= 200 {
					continue
				}
				index := uint8(0)
				for plane, w := range words {
					index |= uint8(w>>uint(15-col)&1) << uint(plane)
				}
				if index != 0 {
					write(py*320+px, 16+index)
				}
			}
		}
	}
	return nil
}
