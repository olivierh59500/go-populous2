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

func TestNativeFireColumnCompositeLayersExceedOldLimit(t *testing.T) {
	b := testBundle(t)
	for _, reference := range []struct{ offset, count int }{{0x4b8, 36}, {0x4bc, 47}, {0x4c0, 38}} {
		frame := b.FireColumns.Frames[reference.offset]
		if len(frame.Layers) != reference.count {
			t.Fatalf("native flame frame $%x: %d layers", reference.offset, len(frame.Layers))
		}
	}
	code := append([]byte(nil), b.Executable.Hunks[0].Data...)
	// A real cycle remains rejected, independent of the bounded layer count.
	at := 0x26956 + 729*2
	code[at+4] = byte(729 >> 8)
	code[at+5] = byte(729 & 255)
	if _, err := decodeImageLayers(code, 729); err == nil {
		t.Fatal("cyclic composite accepted")
	}
}
