package populous2

import "testing"

func TestNativePlagueAnimation(t *testing.T) {
	b := testBundle(t)
	if len(b.PlagueAnimation) < 10 {
		t.Fatal("native vulture sequence incomplete")
	}
	for _, frame := range b.PlagueAnimation {
		if frame.SoundCue != 1 || len(frame.Layers) == 0 {
			t.Fatal("vulture artwork or original caw cue lost")
		}
		for _, layer := range frame.Layers {
			if b.Sprites[0][layer.Sprite].Image == nil {
				t.Fatal("missing vulture sprite")
			}
		}
	}
	if _, err := DecodeAnimation(b.Executable, 1); err == nil {
		t.Fatal("unaligned animation accepted")
	}
}
