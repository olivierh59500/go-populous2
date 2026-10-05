package populous2

import (
	"bytes"
	"image"
	"image/draw"
	"testing"

	"go-populous2/internal/fixedstep"
)

func TestNativeResultOverlayPreservesOriginalCPUComposition(t *testing.T) {
	bundle := testBundle(t)
	presentation, err := DecodeNativePresentation(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	var planes [32000]byte
	for index := range planes {
		planes[index] = byte(index*7 + 13)
	}
	background, err := nativeScreenIndices(planes[:])
	if err != nil {
		t.Fatal(err)
	}
	palette := bundle.Landscapes[0].Palettes[0]
	base, err := nativeIndexedImage(background, palette)
	if err != nil {
		t.Fatal(err)
	}
	for _, reference := range readNativePresentationCatalog(t).Results {
		r, err := presentation.Result(reference.Input.Selected, reference.Input.Eliminated, reference.Input.Ticks, reference.Input.Local, reference.Input.Opponent, reference.Input.Score)
		if err != nil {
			t.Fatal(err)
		}
		overlay, err := presentation.Overlay(r.Requester, palette)
		if err != nil {
			t.Fatal(err)
		}
		expected, err := presentation.Compose(background, r.Requester, palette)
		if err != nil {
			t.Fatal(err)
		}
		actualRGBA := image.NewRGBA(base.Bounds())
		draw.Draw(actualRGBA, actualRGBA.Bounds(), base, image.Point{}, draw.Src)
		draw.Draw(actualRGBA, actualRGBA.Bounds(), overlay, image.Point{}, draw.Over)
		expectedRGBA := image.NewRGBA(base.Bounds())
		draw.Draw(expectedRGBA, expectedRGBA.Bounds(), expected, image.Point{}, draw.Src)
		if !bytes.Equal(actualRGBA.Pix, expectedRGBA.Pix) {
			t.Fatalf("native requester overlay differs from CPU-verified composition for%s", reference.Input.Name)
		}
		if overlay.RGBAAt(0, 0).A != 0 {
			t.Fatal("requester overlay overwrote untouched screen background")
		}
	}
}

func TestNativeResultWaitUsesPALTimeAtSixtyUpdates(t *testing.T) {
	playback := NativeResultPlayback{Wait: 101}
	clock := fixedstep.New(SimulationRate, 60)
	blanks := 0
	for update := 1; update <= 122; update++ {
		for range clock.Advance() {
			blanks++
			if shown := playback.VBlank(); shown != (blanks >= 101) {
				t.Fatal("result requester bypassed the original101-VBlank wait")
			}
		}
		if update < 122 && playback.Wait == 0 {
			t.Fatal("result appeared before the native wait elapsed")
		}
	}
	if playback.Wait != 0 || blanks != 101 {
		t.Fatal("result wait drifted relative to PAL time")
	}
}

func TestOriginalStartupHasFiveVisibleActions(t *testing.T) {
	presentation, err := DecodeNativePresentation(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	r := presentation.StartupRequester
	for row := 0; row < 7; row++ {
		got := presentation.Requesters.Click(r, (r.Column+2)*8, r.Row+row*8+1)
		want := 0
		if row < 5 {
			want = (row + 1) * 2
		}
		if got != want {
			t.Fatalf("original startup row%d action%d want%d", row, got, want)
		}
	}
}
