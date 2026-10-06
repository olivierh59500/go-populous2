package main

import (
	"bytes"
	"image"
	"image/draw"
	"os"
	"testing"

	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func TestPrivateResultWindowMatchesOriginalNumericFieldsAndPixels(t *testing.T) {
	input := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if input == "" {
		t.Skip("set private original English UI directory")
	}
	source, err := populous2.LoadFS(os.DirFS(input))
	if err != nil {
		t.Fatal(err)
	}
	source, err = interfaceSource(source)
	if err != nil {
		t.Fatal(err)
	}
	output := t.TempDir()
	if err := exportResult(source, output); err != nil {
		t.Fatal(err)
	}
	descriptor, err := visualassets.LoadResult(os.DirFS(output))
	if err != nil {
		t.Fatal(err)
	}
	presentation, err := populous2.DecodeNativePresentation(source.Executable)
	if err != nil {
		t.Fatal(err)
	}
	font := &visualassets.Font{FirstCode: 32, Width: 8, Height: 8, Glyphs: make([][]uint8, len(presentation.Font.Glyphs))}
	for i, glyph := range presentation.Font.Glyphs {
		font.Glyphs[i] = append([]uint8(nil), glyph[:]...)
	}
	for _, state := range []visualassets.ResultState{
		{Winner: 0, Ticks: 500, Score: 13007, Local: visualassets.ResultStatistics{PeakPopulation: 4096, PeakMana: 200000, BattleWins: 15, LeaderLosses: 2}, Opponent: visualassets.ResultStatistics{PeakPopulation: 7000, PeakMana: 300000, BattleWins: 9, LeaderLosses: 3}},
		{Winner: 1, Ticks: 120000, Score: 65535, Local: visualassets.ResultStatistics{PeakPopulation: 1000000, PeakMana: 1234567, BattleWins: 9999, LeaderLosses: 1000}, Opponent: visualassets.ResultStatistics{PeakPopulation: 7777777, PeakMana: 9876543, BattleWins: 999, LeaderLosses: 9999}},
	} {
		convert := func(s visualassets.ResultStatistics) populous2.CampaignResultStatistics {
			return populous2.CampaignResultStatistics{PeakPopulation: s.PeakPopulation, PeakMana: s.PeakMana, BattleWins: s.BattleWins, LeaderLosses: s.LeaderLosses}
		}
		result, err := presentation.Result(1, uint16(2-state.Winner), state.Ticks, convert(state.Local), convert(state.Opponent), state.Score)
		if err != nil {
			t.Fatal(err)
		}
		expected, err := presentation.Compose(presentation.StartupPixels, result.Requester, descriptor.Layout.Palette)
		if err != nil {
			t.Fatal(err)
		}
		got := image.NewRGBA(image.Rect(0, 0, 320, 200))
		base, err := presentation.Compose(presentation.StartupPixels, &populous2.NativeRequester{Text: []byte{}}, descriptor.Layout.Palette)
		if err != nil {
			t.Fatal(err)
		}
		draw.Draw(got, got.Bounds(), base, image.Point{}, draw.Src)
		descriptor.Layout.Draw(got, font, descriptor.Values(state), nil)
		want := image.NewRGBA(got.Bounds())
		draw.Draw(want, want.Bounds(), expected, image.Point{}, draw.Src)
		if !bytes.Equal(got.Pix, want.Pix) {
			reportFirstPixelDifference(t, got, want)
		}
		for y := 0; y < 200; y++ {
			for x := 0; x < 320; x++ {
				actual := descriptor.Layout.ActionAt(x, y)
				command := presentation.Requesters.Click(result.Requester, x, y)
				if actual == "continue" && command != 2 || actual == "" && command != 0 {
					t.Fatalf("original result button region differs at%d,%d", x, y)
				}
			}
		}
	}
}
