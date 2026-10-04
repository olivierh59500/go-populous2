package populous2

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"sync"
	"testing"
	"testing/fstest"

	"go-populous2/internal/amiga"
)

var bundleOnce sync.Once
var sharedBundle *Bundle
var sharedError error

func testBundle(t *testing.T) *Bundle {
	t.Helper()
	bundleOnce.Do(func() { sharedBundle, sharedError = Load() })
	if sharedError != nil {
		t.Fatal(sharedError)
	}
	return sharedBundle
}

func TestOriginalResourcesAndSprites(t *testing.T) {
	b := testBundle(t)
	if len(b.Resources) != 26 || len(b.Levels) != 1000 || len(b.Spells) != 29 {
		t.Fatalf("wrong catalog sizes: %d / %d / %d", len(b.Resources), len(b.Levels), len(b.Spells))
	}
	// These fingerprints are recorded from the supplied disk B. XOR validation
	// exercises every original compressed stream, not an artificial encoder.
	for name, want := range map[string]string{
		"block0.pak":   "68d6312a48e8aac0d95f246a05818d0f914ecdfdeffdad7052c227e14ee8812e",
		"s16-0.pak":    "d96d8137f653eda79f96f9f867009b529fc1c8574f1eacc32111c043e0417cfe",
		"conquest.pak": "e85d474b65a5e185a375a69c7059837a275fcabce53a8fe5d8825accf6393652",
		"end.pak":      "a4bb6992a3169c8b88d3431d06e4d2b55a329fe6d0111a7596f8a024990df696",
	} {
		if got := amiga.Digest(b.Raw[name]); got != want {
			t.Fatalf("%s hash %s, want %s", name, got, want)
		}
	}
	for i := range b.Landscapes {
		if len(b.Tiles[i]) != 255 || len(b.Sprites[i]) != 830 {
			t.Fatalf("land %d incomplete", i)
		}
	}
	if b.Sprites[0][681].Image.Bounds().Dx() != 32 || b.Sprites[0][1].Image.Bounds().Dx() != 16 {
		t.Fatal("16/32-pixel sprite layouts lost")
	}
	if bytes.Equal(b.Sprites[0][681].Image.Pix, b.Sprites[1][681].Image.Pix) {
		t.Fatal("landscape sprite differences were not applied")
	}
	if b.Landscapes[0].PopulationLimit[18] != 4000 || b.Landscapes[0].ManaAdd[18] != 100 {
		t.Fatal("19-stage town tables were decoded as Populous 1")
	}
}

func TestCampaignCodesSeedsAndPowers(t *testing.T) {
	b := testBundle(t)
	if CodeForLevel(0) != "DOEGAC" {
		t.Fatal("original first world name missing")
	}
	if n, ok := DecodeLevelCode(" doegac "); !ok || n != 0 {
		t.Fatal("world code lookup failed")
	}
	if _, ok := DecodeLevelCode("not a world"); ok {
		t.Fatal("unknown code accepted")
	}
	for i := 1; i < 5; i++ {
		if b.Levels[i].Seed != uint16(uint32(b.Levels[0].Seed)+uint32(i)*727) {
			t.Fatal("subworld seed increment incorrect")
		}
	}
	options := b.Levels[0].Players[0].Powers
	for _, id := range []SpellID{RaiseLower, PapalMagnet, Perseus, Armageddon, FireColumn} {
		if !options[id] {
			t.Fatalf("DOEGAC power %d missing", id)
		}
	}
	if options[Lightning] || options[Volcano] {
		t.Fatal("late spells accidentally available on first world")
	}
	for _, spell := range b.Spells {
		if spell.ID == RaiseLower && spell.Cost != 5 || spell.ID == FireColumn && spell.Cost != 5625 || spell.ID == Baptism && spell.Cost != 6250 {
			t.Fatal("original base power costs lost")
		}
	}
	if _, err := DecodeCampaign(b.Raw["conquest.pak"][:49999]); err == nil {
		t.Fatal("truncated campaign accepted")
	}
}

func TestPlanarMaskAnd32PixelGroups(t *testing.T) {
	var palette [16]color.RGBA
	palette[1] = color.RGBA{R: 255, A: 255}
	palette[2] = color.RGBA{G: 255, A: 255}
	// Each half has its own mask and four words; set different colors at its
	// first pixel. This catches treating a 32-pixel row as five 32-bit planes.
	data := make([]byte, 20)
	binary.BigEndian.PutUint16(data[2:], 0x8000)
	binary.BigEndian.PutUint16(data[14:], 0x8000)
	img, err := DecodeInterleaved(data, 32, 1, palette)
	if err != nil {
		t.Fatal(err)
	}
	if img.RGBAAt(0, 0) != palette[1] || img.RGBAAt(16, 0) != palette[2] {
		t.Fatal("32-pixel planar grouping incorrect")
	}
	data[0] = 0x80
	img, err = DecodeInterleaved(data, 32, 1, palette)
	if err != nil {
		t.Fatal(err)
	}
	if img.RGBAAt(0, 0).A != 0 {
		t.Fatal("mask polarity reversed")
	}
}

func TestResourceLookupAndCompressionCorruption(t *testing.T) {
	files := fstest.MapFS{"folder/BLOCK0.PAK": {Data: []byte("raw")}}
	r := Resource{Name: "block0.pak", ReadLimit: 10}
	data, err := LoadResource(files, r)
	if err != nil || string(data) != "raw" {
		t.Fatalf("case-insensitive lookup: %v", err)
	}
	files["other/block0.pak"] = &fstest.MapFile{Data: []byte("duplicate")}
	if _, err := LoadResource(files, r); err == nil {
		t.Fatal("ambiguous filename accepted")
	}
	for _, data := range [][]byte{nil, make([]byte, 12), {0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 1}} {
		if _, err := DecodePacked(data); err == nil {
			t.Fatal("malformed packed data accepted")
		}
	}
}

func TestSpriteDifferenceValidatesRangesAndIsolatesBase(t *testing.T) {
	base := []byte{1, 2, 3}
	diff := make([]byte, 23)
	binary.BigEndian.PutUint32(diff, 20)
	binary.BigEndian.PutUint32(diff[4:], 1)
	binary.BigEndian.PutUint16(diff[8:], 1)
	binary.BigEndian.PutUint32(diff[10:], 0xffffffff)
	copy(diff[20:], []byte{8, 9})
	got, err := ApplySpriteDifference(base, diff)
	if err != nil || !bytes.Equal(got, []byte{1, 8, 9}) || !bytes.Equal(base, []byte{1, 2, 3}) {
		t.Fatalf("difference application: %v %v", got, err)
	}
	binary.BigEndian.PutUint32(diff[4:], 3)
	if _, err := ApplySpriteDifference(base, diff); err == nil {
		t.Fatal("difference beyond destination accepted")
	}
}

func FuzzPacked(f *testing.F) {
	f.Add(make([]byte, 12))
	f.Add([]byte("short"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		_, _ = DecodePacked(data)
	})
}
