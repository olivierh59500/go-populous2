package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"testing"
)

func TestActorProjectionMatchesNativeSlopeCases(t *testing.T) {
	var packed []byte
	for shape := 0; shape < 16; shape++ {
		for _, x := range []uint8{0, 31, 64, 128, 192, 255} {
			for _, y := range []uint8{0, 31, 64, 128, 192, 255} {
				_, vertical := (TerrainCell{Shape: uint8(shape)}).ActorOffset(x, y)
				var value [2]byte
				binary.BigEndian.PutUint16(value[:], uint16(int16(vertical)))
				packed = append(packed, value[:]...)
			}
		}
	}
	// Derived from 576 executions of the original $e392 dispatcher.
	if got := fmt.Sprintf("%x", sha256.Sum256(packed)); got != "6e2234b2651d721a5f7b1118b7d6b4166237006dcd8e9a8ffcc24d77683c5077" {
		t.Fatalf("native projection mismatch: %s", got)
	}
}
