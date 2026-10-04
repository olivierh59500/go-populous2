package populous2

import (
	"encoding/binary"
	"fmt"
	"go-populous2/internal/amiga"
)

type AnimationFrame struct {
	Layers   []SpriteLayer
	SoundCue int
}

// DecodeAnimation reads the original four-byte image/cue records. Negative
// image words terminate or loop the sequence; cue words address the ten-byte
// sound descriptor table, as in CODE:$ee32.
func DecodeAnimation(exe *amiga.Executable, offset int) ([]AnimationFrame, error) {
	if exe == nil || len(exe.Hunks) == 0 || offset < 0 || offset%4 != 0 {
		return nil, fmt.Errorf("invalid native animation offset")
	}
	code := exe.Hunks[0].Data
	frames := []AnimationFrame{}
	for step := 0; step < 256; step++ {
		at := 0x23d1a + offset + step*4
		if at+4 > len(code) {
			return nil, fmt.Errorf("native animation exceeds CODE")
		}
		image := int16(binary.BigEndian.Uint16(code[at:]))
		if image < 0 {
			if len(frames) == 0 {
				return nil, fmt.Errorf("empty native animation")
			}
			return frames, nil
		}
		layers, err := decodeImageLayers(code, uint16(image))
		if err != nil {
			return nil, err
		}
		cue := int(binary.BigEndian.Uint16(code[at+2:]))
		if cue%10 != 0 || cue/10 >= 133 {
			return nil, fmt.Errorf("invalid native animation cue")
		}
		frames = append(frames, AnimationFrame{Layers: layers, SoundCue: cue / 10})
	}
	return nil, fmt.Errorf("unterminated native animation")
}
