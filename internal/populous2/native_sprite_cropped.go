package populous2

import "fmt"

// NativeCroppedSpriteRequest represents $f0e8/$f398's distinct source stride
// and visible height. Sprite contains destination geometry and visible rows.
type NativeCroppedSpriteRequest struct {
	Sprite       NativePresentationSprite
	SourceHeight int16
	SourceRow    int16
}

// PaintCropped retains the original cached source-plane stride. Native scenery
// draws the leading visible rows; no source-plane buffer is repacked per draw.
func (b *NativeSpriteBitmapBank) PaintCropped(request NativeCroppedSpriteRequest, bitmap []byte) error {
	if b == nil || request.Sprite.Sprite < 0 || request.Sprite.Sprite >= len(b.Sprites) {
		return fmt.Errorf("native cropped sprite outside prepared bank")
	}
	if request.SourceRow != 0 {
		return fmt.Errorf("native cropped source displacement needs its original DMA proof")
	}
	s := b.Sprites[request.Sprite.Sprite]
	if request.SourceHeight <= 0 || int(request.SourceHeight) != s.Height || request.Sprite.Height <= 0 || request.Sprite.Height > request.SourceHeight {
		return fmt.Errorf("native cropped sprite source/visible height differs")
	}
	p := request.Sprite
	switch p.Routine {
	case 0xf0e8:
		p.Routine = 0xf0ee
	case 0xf398:
		p.Routine = 0xf3a0
	default:
		return fmt.Errorf("native cropped sprite procedure unsupported")
	}
	return s.paintRows(p, bitmap, int(request.Sprite.Height))
}
