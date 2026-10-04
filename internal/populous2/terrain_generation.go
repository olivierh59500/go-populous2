package populous2

import (
	"encoding/binary"
	"fmt"
	"go-populous2/internal/amiga"
)

func DecodeHillParameters(exe *amiga.Executable) ([4][4]int, error) {
	var hills [4][4]int
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x2072a+36 {
		return hills, fmt.Errorf("native hill parameters missing")
	}
	code := exe.Hunks[0].Data
	for i := range hills {
		for j := range hills[i] {
			hills[i][j] = int(int16(binary.BigEndian.Uint16(code[0x2072a+(i*4+j)*2:])))
		}
	}
	if binary.BigEndian.Uint16(code[0x2072a+32:]) != 0xff9d {
		return hills, fmt.Errorf("native hill table terminator missing")
	}
	for _, hill := range hills {
		if hill[0] < 1 || hill[2] < 1 || hill[1] < 0 || hill[3] < 0 || hill[0]+hill[1] > 64 || hill[2]+hill[3] > 64 {
			return hills, fmt.Errorf("invalid native hill bounds")
		}
	}
	return hills, nil
}
