package gamcodec

import (
	"bytes"
	"encoding/binary"
	"testing"

	"go-populous2/internal/engine"
)

func TestGAMSelectionAndReturnTimerSurviveMetadataOnlyEdits(t *testing.T) {
	catalog := continuationCatalog()
	w := continuationWorld(t, catalog)
	addCodecFollower(t, w, 1, 20, 20, 0, 100, engine.Walking)
	addCodecFollower(t, w, 2, 21, 20, 0, 200, engine.Walking)
	document, err := NewDocument(w, catalog, engine.NewDeity("BLUE"), 0, 2, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	document.Metadata.SelectedFollower, document.Metadata.BackupFollower, document.Metadata.SelectionReturnFrames = 1, 2, 73
	encoded, err := Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	if binary.BigEndian.Uint32(encoded[0xf36-fileStart:]) != 52 || binary.BigEndian.Uint32(encoded[0xf32-fileStart:]) != 104 || binary.BigEndian.Uint16(encoded[0xf30-fileStart:]) != 73 {
		t.Fatal("named selections did not match the original file fields")
	}
	decoded, err := Decode(encoded, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Metadata.SelectedFollower != 1 || decoded.Metadata.BackupFollower != 2 || decoded.Metadata.SelectionReturnFrames != 73 {
		t.Fatal("metadata-only selection edits were lost")
	}
	again, err := Encode(decoded)
	if err != nil || !bytes.Equal(encoded, again) {
		t.Fatal("unchanged selection save was not byte-exact", err)
	}
	decoded.Metadata.SelectedFollower = 2
	changed, err := Encode(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if binary.BigEndian.Uint32(changed[0xf36-fileStart:]) != 104 {
		t.Fatal("unchanged-world optimization discarded a new UI selection")
	}
	bad := append([]byte(nil), encoded...)
	binary.BigEndian.PutUint32(bad[0xf36-fileStart:], 53)
	if _, err := Decode(bad, catalog); err == nil {
		t.Fatal("unaligned selection file record accepted")
	}
	bad = append([]byte(nil), encoded...)
	binary.BigEndian.PutUint16(bad[0xf30-fileStart:], 101)
	if _, err := Decode(bad, catalog); err == nil {
		t.Fatal("selection countdown beyond its source range accepted")
	}
	decoded.Metadata.SelectedFollower = engine.FollowerCapacity
	if _, err := Encode(decoded); err == nil {
		t.Fatal("selection outside the follower pool exported")
	}
}
