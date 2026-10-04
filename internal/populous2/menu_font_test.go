package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"go-populous2/internal/amiga"
)

func TestMenuFontAgainstOriginal68000TextRenderer(t *testing.T) {
	data, err := os.ReadFile("testdata/menu_font_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Fixtures []struct {
			Name, Text    string
			Column, Row   int
			IndicesSHA256 string `json:"indices_sha256"`
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Fixtures) != 8 {
		t.Fatalf("invalid native font fixture catalog: %v", err)
	}
	font, err := DecodeNativeMenuFont(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			pixels := make([]uint8, NativeMenuWidth*NativeMenuHeight)
			if err := font.DrawIndices(pixels, fixture.Text, fixture.Column, fixture.Row); err != nil {
				t.Fatal(err)
			}
			if hash := fmt.Sprintf("%x", sha256.Sum256(pixels)); hash != fixture.IndicesSHA256 {
				t.Fatalf("native text pixels differ: %s", hash)
			}
		})
	}
}

func TestMenuFontRejectsUnsupportedLayoutAndUnsafeCoordinates(t *testing.T) {
	if _, err := DecodeNativeMenuFont(nil); err == nil {
		t.Fatal("missing executable accepted")
	}
	exe := *testBundle(t).Executable
	exe.Hunks = append([]amiga.Hunk(nil), exe.Hunks...)
	exe.Hunks[0].Data = append([]byte(nil), exe.Hunks[0].Data...)
	exe.Hunks[0].Data[0x50ae] ^= 1
	if _, err := DecodeNativeMenuFont(&exe); err == nil {
		t.Fatal("unsupported font descriptor accepted")
	}
	font, err := DecodeNativeMenuFont(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, point := range [][2]int{{-1, 0}, {40, 0}, {0, -1}, {0, 193}} {
		if err := font.DrawIndices(make([]uint8, NativeMenuWidth*NativeMenuHeight), "A", point[0], point[1]); err == nil {
			t.Fatal("out-of-display text position accepted")
		}
	}
	if err := font.DrawIndices(make([]uint8, 10), "A", 0, 0); err == nil {
		t.Fatal("short text buffer accepted")
	}
}
