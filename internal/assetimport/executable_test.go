package assetimport

import (
	"bytes"
	"encoding/binary"
	"go-populous2/internal/amiga"
	"testing"
)

func TestHunkEndRepairInsertsOnlyStructuralSeparator(t *testing.T) {
	original := make([]byte, 16)
	copy(original, []byte{1, 2, 3, 4, 5, 6, 7, 8})
	binary.BigEndian.PutUint32(original[8:], amiga.HunkBSS)
	copy(original[12:], []byte{9, 10, 11, 12})
	saved := bytes.Clone(original)
	repaired, err := insertHunkEnd(original, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(repaired) != len(original)+4 || !bytes.Equal(repaired[:8], original[:8]) || binary.BigEndian.Uint32(repaired[8:]) != 1010 || !bytes.Equal(repaired[12:], original[8:]) || !bytes.Equal(original, saved) {
		t.Fatal("structural repair changed original payload")
	}
	for _, at := range []int{-1, 0, 7, 16} {
		if _, err := insertHunkEnd(original, at); err == nil {
			t.Fatal("invalid repair boundary accepted", at)
		}
	}
	if _, err := prepareExecutable(original); err == nil {
		t.Fatal("unrecognized executable was repaired")
	}
}
