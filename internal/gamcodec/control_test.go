package gamcodec

import (
	"encoding/binary"
	"testing"

	"go-populous2/internal/engine"
)

func TestGAMPreservesDistinctHumanComputerAndAssistedControl(t *testing.T) {
	for _, tc := range []struct {
		name               string
		computer, assisted bool
		word               uint16
	}{{"human", false, false, 2}, {"computer", true, false, 4}, {"assisted", false, true, 18}} {
		t.Run(tc.name, func(t *testing.T) {
			catalog := continuationCatalog()
			w := continuationWorld(t, catalog)
			w.Players[1].Computer, w.Players[1].Assisted = tc.computer, tc.assisted
			doc, err := NewDocument(w, catalog, engine.NewDeity("BAD"), 1, 4, 1, 2)
			if err != nil {
				t.Fatal(err)
			}
			data, err := Encode(doc)
			if err != nil {
				t.Fatal(err)
			}
			if binary.BigEndian.Uint16(data[0xe76a+2*314+0x1a-fileStart:]) != tc.word {
				t.Fatal("control did not use its original file value")
			}
			got, err := Decode(data, catalog)
			if err != nil {
				t.Fatal(err)
			}
			if got.Metadata.ProfileSide != 1 || got.World.Players[1].Computer != tc.computer || got.World.Players[1].Assisted != tc.assisted {
				t.Fatal("I AM side or distinct assistance state was lost")
			}
		})
	}
}
