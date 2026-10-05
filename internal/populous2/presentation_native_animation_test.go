package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// Original CPU calls include the palette loader, complete row RLE, all delta
// column operations, frame counters and the real front/back buffer swap.
func TestNativePresentationAnimationAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/presentation_animation_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Resource, Mode, Hash     string
			Step, Cursor, LoopOffset int
			Code                     uint32
			Palette                  [16]uint16
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 280 {
		t.Fatal("native presentation animation catalog is incomplete")
	}
	bundle := testBundle(t)
	var animation *NativeScreenAnimation
	for _, fixture := range catalog.Cases {
		if fixture.Step == 0 {
			key := "end.pak"
			if fixture.Resource == "JUDGE.PAK" {
				key = "judge.pak"
			}
			raw := append([]byte(nil), bundle.Raw[key]...)
			if fixture.Mode == "single-pass" {
				binary.BigEndian.PutUint16(raw[2:], 1)
			}
			animation, err = NewNativeScreenAnimation(raw)
		} else {
			err = animation.Advance()
		}
		if err != nil {
			t.Fatalf("%s/%s step %d: %v", fixture.Resource, fixture.Mode, fixture.Step, err)
		}
		if animation.Code != fixture.Code || animation.Cursor != fixture.Cursor || animation.LoopOffset != fixture.LoopOffset || animation.Palette != fixture.Palette || fmt.Sprintf("%x", sha256.Sum256(animation.Planes[:])) != fixture.Hash {
			t.Fatalf("%s/%s step %d differs: code %x/%x cursor %d/%d bookmark %d/%d image hash %x/%s", fixture.Resource, fixture.Mode, fixture.Step, animation.Code, fixture.Code, animation.Cursor, fixture.Cursor, animation.LoopOffset, fixture.LoopOffset, sha256.Sum256(animation.Planes[:]), fixture.Hash)
		}
	}
}
