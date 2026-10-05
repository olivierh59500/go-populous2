package populous2

import "fmt"

// NativeReinterpretedSpriteRequest preserves ECDE's native shortened-height
// plane starts. It intentionally differs from the full-stride cropped entry.
type NativeReinterpretedSpriteRequest struct{ Sprite NativePresentationSprite }

func (b *NativeSpriteBitmapBank) PaintReinterpreted(request NativeReinterpretedSpriteRequest, bitmap []byte) error {
	p := request.Sprite
	if b == nil || p.Sprite < 0 || p.Sprite >= len(b.Sprites) {
		return fmt.Errorf("native reinterpreted sprite outside bank")
	}
	s := b.Sprites[p.Sprite]
	height := int(p.Height)
	if height <= 0 || height > s.Height || s.Width%8 != 0 {
		return fmt.Errorf("native shortened plane height outside cached source")
	}
	n := s.Width / 8 * 5 * height
	if n > len(s.Planes) {
		return fmt.Errorf("native shortened plane span outside source")
	}
	s.Height = height
	s.Planes = s.Planes[:n]
	return s.Paint(p, bitmap)
}
