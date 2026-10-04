package populous2

import (
	"bytes"
	"testing"
)

func TestOriginalDeityPortraitParts(t *testing.T) {
	b := testBundle(t)
	first, err := b.DeityArt.Portrait([3]uint8{})
	if err != nil {
		t.Fatal(err)
	}
	for part := 0; part < 3; part++ {
		parts := [3]uint8{}
		parts[part] = 1
		variant, err := b.DeityArt.Portrait(parts)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(first.Pix, variant.Pix) {
			t.Fatalf("native face part %d did not change", part)
		}
	}
	if _, err := b.DeityArt.Portrait([3]uint8{8}); err == nil {
		t.Fatal("invalid face variant accepted")
	}
}

func TestDeityProfileAndExperienceSurviveWorldSave(t *testing.T) {
	w, err := NewWorld(testBundle(t), 0, false)
	if err != nil {
		t.Fatal(err)
	}
	w.Deity = NewDeity("OLYMPUS")
	w.Deity.CycleFace(0, 1)
	w.Deity.AllocateBolt(Fire)
	w.Experience[0] = w.Deity.Experience
	restored, err := ReadSave(testBundle(t), bytes.NewReader(encodeSnapshot(t, w)))
	if err != nil || restored.Deity != w.Deity || restored.Experience != w.Experience {
		t.Fatalf("saved profile changed: %v", err)
	}
}
