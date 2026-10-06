package app

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"testing"

	"go-populous2/internal/engine"
)

func TestFractionalActorHeightMatchesAllOriginalSlopeCases(t *testing.T) {
	var packed []byte
	for shape := uint8(0); shape < 16; shape++ {
		for _, x := range []uint8{0, 31, 64, 128, 192, 255} {
			for _, y := range []uint8{0, 31, 64, 128, 192, 255} {
				_, height := fractionalSurfaceOffset(shape, x, y)
				var word [2]byte
				binary.BigEndian.PutUint16(word[:], uint16(int16(height)))
				packed = append(packed, word[:]...)
			}
		}
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(packed)); got != "6e2234b2651d721a5f7b1118b7d6b4166237006dcd8e9a8ffcc24d77683c5077" {
		t.Fatal("original 576-case actor surface digest differs", got)
	}
}

func TestActorProjectionUsesFractionalSurfaceRatherThanOneCorner(t *testing.T) {
	x, y := projectSurface(engine.Cell{Shape: 3, BaseAltitude: 2}, 12*256+192, 12*256+64, 8, 8)
	if x != 200 || y != 122 {
		t.Fatal("slope projection differs", x, y)
	}
	flatX, flatY := projectSurface(engine.Cell{Shape: 15, BaseAltitude: 7}, 12*256+128, 12*256+128, 8, 8)
	if flatX != 192 || flatY != 80 {
		t.Fatal("height-eight flat projection differs", flatX, flatY)
	}
}
