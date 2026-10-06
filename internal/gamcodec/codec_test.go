package gamcodec

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"go-populous2/internal/engine"
)

// Optional reference saves and catalogs remain private local input. The
// production codec depends only on semantic metadata and the engine package.
func TestPrivateOriginalGAMOrdinaryState(t *testing.T) {
	path := os.Getenv("POPULOUS2_GAM_TEST_FILE")
	catalogPath := os.Getenv("POPULOUS2_GAM_TEST_CATALOG")
	if path == "" || catalogPath == "" {
		t.Skip("set private GAM save and semantic catalog paths")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	var catalog Catalog
	if err := json.Unmarshal(metadata, &catalog); err != nil {
		t.Fatal(err)
	}
	document, err := Decode(data, catalog)
	if err != nil {
		t.Fatal(err)
	}
	w := document.World
	if w.Level.Number != 27 || w.Level.Landscape != 1 || w.Tick != 112 || w.Players[0].Mana != 2020 || w.Players[1].Mana != 116 || document.Metadata.Profile.Name != "DAMOCLES" || document.Metadata.Profile.Bolts != 13 {
		t.Fatalf("original reference session/profile differs: world%d land%d tick%d mana%d/%d profile%+v", w.Level.Number, w.Level.Landscape, w.Tick, w.Players[0].Mana, w.Players[1].Mana, document.Metadata.Profile)
	}
	if !bytes.Equal(document.Metadata.Original, data[:FileSize]) {
		t.Fatal("codec did not retain reserved user save fields")
	}
	roundtrip, err := Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(roundtrip, data[:FileSize]) {
		t.Fatal("unchanged original GAM did not roundtrip byte for byte")
	}
	if _, err := w.Snapshot().Restore(); err != nil {
		t.Fatal(err)
	}
	for range 10 {
		w.Step()
	}
	if w.Tick != 122 {
		t.Fatal("typed imported world did not continue normally")
	}
	continued, err := Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := Decode(continued, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.World.Tick != w.Tick || reloaded.World.Players[0].Mana != w.Players[0].Mana || reloaded.World.Players[1].Mana != w.Players[1].Mana {
		t.Fatal("continued GAM lost clock or mana")
	}
	for id, follower := range w.Followers {
		actual := reloaded.World.Followers[id]
		if follower.State != actual.State || follower.Population != actual.Population || follower.X != actual.X || follower.Y != actual.Y {
			t.Fatalf("continued GAM lost follower%d", id)
		}
	}
}

func TestGAMRecordReferenceConversions(t *testing.T) {
	for _, ref := range []engine.ActorRef{{Kind: engine.ActorFollower, Index: 1}, {Kind: engine.ActorEffect, Index: 249}, {Kind: engine.ActorScenery, Index: 0}, {Kind: engine.ActorWall, Index: 199}, {Kind: engine.ActorMagnet, Index: 1}} {
		file, err := fileReference(ref)
		if err != nil {
			t.Fatal(err)
		}
		got, err := reference(file)
		if err != nil || got != ref {
			t.Fatal("file reference did not convert to the same typed pool")
		}
	}
	if _, err := reference(53); err == nil {
		t.Fatal("unaligned follower reference accepted")
	}
	if _, err := Decode(make([]byte, FileSize-1), Catalog{}); err == nil {
		t.Fatal("short GAM save accepted")
	}
}
