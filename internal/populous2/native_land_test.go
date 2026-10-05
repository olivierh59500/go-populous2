package populous2

import (
	"bytes"
	"fmt"
	"testing"
)

func TestWorldLoadsOriginalLANDWithoutSharingMutableCODE(t *testing.T) {
	b := testBundle(t)
	original := append([]byte(nil), b.NativeAI.Code...)
	worlds := [4]*World{}
	for land := range worlds {
		levelIndex := -1
		for i, l := range b.Levels {
			if int(l.Terrain) == land {
				levelIndex = i
				break
			}
		}
		if levelIndex < 0 {
			t.Fatal("campaign landscape missing", land)
		}
		w, err := NewWorld(b, levelIndex, false)
		if err != nil {
			t.Fatal(err)
		}
		worlds[land] = w
		want := b.Raw[fmt.Sprintf("land%d.dat", land)]
		if !bytes.Equal(w.NativeAI.Code[0x3365a:0x33886], want) {
			t.Fatal("World retained wrong native LAND", land)
		}
		if !bytes.Equal(w.NativeAI.Code[0x33744:0x33844], want[0xea:0x1ea]) {
			t.Fatal("terrain minimap uses wrong native LAND colors", land)
		}
	}
	worlds[0].NativeAI.Code[0x33744] ^= 0xff
	for land := 1; land < 4; land++ {
		if !bytes.Equal(worlds[land].NativeAI.Code[0x3365a:0x33886], b.Raw[fmt.Sprintf("land%d.dat", land)]) {
			t.Fatal("world CODE leaked across resource banks", land)
		}
	}
	if !bytes.Equal(b.NativeAI.Code, original) {
		t.Fatal("world mutated immutable Bundle CODE")
	}
}
