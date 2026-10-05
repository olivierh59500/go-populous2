package populous2

import (
	"encoding/binary"
	"fmt"
	"go-populous2/internal/amiga"
)

type GroundEffectRules struct {
	Offsets                              [45][2]int
	FontCount, SwampCount, GreeneryCount int
	Properties                           [256]uint16
}

func DecodeGroundEffectRules(exe *amiga.Executable) (GroundEffectRules, error) {
	var r GroundEffectRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33512 {
		return r, fmt.Errorf("ground effect tables missing")
	}
	code := exe.Hunks[0].Data
	r.FontCount = int(binary.BigEndian.Uint16(code[0x21002:]))
	r.SwampCount = int(binary.BigEndian.Uint16(code[0x20fa6:]))
	r.GreeneryCount = int(binary.BigEndian.Uint16(code[0x20fa4:]))
	for _, n := range []int{r.FontCount, r.SwampCount, r.GreeneryCount} {
		if n < 1 || n > 256 {
			return GroundEffectRules{}, fmt.Errorf("invalid ground effect count")
		}
	}
	for i := range r.Offsets {
		n := int(int16(binary.BigEndian.Uint16(code[0x20fa8+i*2:])))
		x := int(int8(byte(n)))
		y := (n - x) / 256
		if abs(x) > 4 || abs(y) > 4 {
			return GroundEffectRules{}, fmt.Errorf("invalid ground effect offset")
		}
		r.Offsets[i] = [2]int{x, y}
	}
	for i := range r.Properties {
		r.Properties[i] = binary.BigEndian.Uint16(code[0x33312+i*2:])
	}
	return r, nil
}
