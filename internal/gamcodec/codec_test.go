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
	document.Metadata.CameraX, document.Metadata.CameraY, document.Metadata.ProfileSide, document.Metadata.GameMode = 9, 7, 1, 4
	edited, err := Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	session, err := Decode(edited, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if session.Metadata.CameraX != 9 || session.Metadata.CameraY != 7 || session.Metadata.ProfileSide != 1 || session.Metadata.GameMode != 4 {
		t.Fatal("metadata-only edits were lost through the unchanged export path")
	}
	document.Metadata.CameraX, document.Metadata.CameraY, document.Metadata.ProfileSide, document.Metadata.GameMode = 8, 4, 0, 2
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

func TestFreshGAMDocumentRoundTripPreservesNamedSessionState(t *testing.T) {
	land := engine.Landscape{}
	for stage := 1; stage < engine.TownStages; stage++ {
		land.WorkTicks[stage] = 8
		land.EmigrationDivisor[stage] = 3
		land.PopulationLimit[stage] = 1000
		land.PopulationAdd[stage] = 1
	}
	level := engine.Level{Seed: 4311}
	for owner := range level.Players {
		level.Players[owner] = engine.PlayerOptions{Mana: 1000, ReactionDelay: 7}
	}
	w, err := engine.NewWorld(level, land)
	if err != nil {
		t.Fatal(err)
	}
	catalog := Catalog{Levels: []engine.Level{level}}
	for i := range catalog.Landscapes {
		catalog.Landscapes[i] = land
	}
	for i := 0; i < 32; i++ {
		catalog.Geometry[i] = uint8(i % 16)
	}
	profile := engine.NewDeity("BLUE")
	document, err := NewDocument(w, catalog, profile, 0, 2, 8, 4)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(encoded, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) != FileSize || decoded.Metadata.CameraX != 8 || decoded.Metadata.CameraY != 4 || decoded.World.Players[0].Mana != 1000 || decoded.Metadata.Profile.Name != "BLUE" {
		t.Fatal("fresh GAM lost named session fields")
	}
	for range 20 {
		w.Step()
		decoded.World.Step()
	}
	if w.Tick != decoded.World.Tick || w.Players[0].Mana != decoded.World.Players[0].Mana {
		t.Fatal("fresh GAM replay did not retain the basic continuation")
	}
}

func TestFreshGAMWithFollowersContinuesWithoutLosingMotionOrAI(t *testing.T) {
	land := engine.Landscape{}
	for stage := 1; stage < engine.TownStages; stage++ {
		land.WorkTicks[stage] = 8
		land.EmigrationDivisor[stage] = 3
		land.PopulationLimit[stage] = 1000
		land.PopulationAdd[stage] = 1
		land.ManaAdd[stage] = 2
	}
	level := engine.Level{Seed: 4311}
	for owner := range level.Players {
		level.Players[owner] = engine.PlayerOptions{Groups: 2, Population: 500, Mana: 10000, MovementSpeed: 20, ReactionDelay: 7}
	}
	w, err := engine.NewWorld(level, land)
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		w.Step()
	}
	catalog := Catalog{Levels: []engine.Level{level}}
	for i := range catalog.Landscapes {
		catalog.Landscapes[i] = land
	}
	for i := 0; i < 256; i++ {
		catalog.Geometry[i] = uint8(i % 16)
	}
	document, err := NewDocument(w, catalog, engine.NewDeity("BLUE"), 0, 2, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	data, err := Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Decode(data, catalog)
	if err != nil {
		t.Fatal(err)
	}
	for pass := 0; pass < 20; pass++ {
		w.Step()
		restored.World.Step()
		for id, f := range w.Followers {
			got := restored.World.Followers[id]
			if f.State != got.State || f.Population != got.Population || f.X != got.X || f.Y != got.Y {
				t.Fatalf("fresh GAM follower%d diverged at pass%d", id, pass+1)
			}
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
