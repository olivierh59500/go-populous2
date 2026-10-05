package populous2

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"go-populous2/internal/fixedstep"
)

func TestEndingPALPlaybackMatchesOriginalFrameSequence(t *testing.T) {
	bundle := testBundle(t)
	presentation, err := DecodeNativePresentation(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	catalog := readNativePresentationCatalog(t)
	for _, initial := range []uint16{0, 0xffff} {
		playback, err := NewNativeEndingPlayback(bundle.Raw["end.pak"], presentation, initial)
		if err != nil {
			t.Fatal(err)
		}
		clock := fixedstep.New(SimulationRate, 60)
		blanks, step := 0, 0
		for _, reference := range catalog.Ending {
			if reference.InitialPhase != initial {
				continue
			}
			if reference.Step != 0 {
				wantedBlank := 1 + 4*reference.Step
				for blanks < wantedBlank {
					for range clock.Advance() {
						blanks++
						advanced, err := playback.VBlank()
						if err != nil {
							t.Fatal(err)
						}
						if advanced {
							step++
						}
						if step != max(0, (blanks-1)/4) {
							t.Fatalf("ending changed outside the original four-blank boundary: blank%d step%d", blanks, step)
						}
					}
				}
			}
			ending := playback.Ending
			if step != reference.Step || ending.ScrollPhase != reference.Phase || ending.TextOffset != reference.Offset || fmt.Sprintf("%x", sha256.Sum256(ending.Animation.Planes[:])) != reference.Hash {
				t.Fatalf("PAL playback differs from original phase%x step%d", initial, reference.Step)
			}
		}
	}
}

func TestEndingClearsInitialKeyboardAndExitsAfterNextSwap(t *testing.T) {
	bundle := testBundle(t)
	presentation, err := DecodeNativePresentation(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	playback, err := NewNativeEndingPlayback(bundle.Raw["end.pak"], presentation, 0)
	if err != nil {
		t.Fatal(err)
	}
	playback.PressKey()
	if changed, err := playback.VBlank(); err != nil || changed || playback.Finished || playback.Keyboard {
		t.Fatal("initial keyboard event survived the native clear")
	}
	playback.PressKey()
	for blank := 2; blank <= 5; blank++ {
		changed, err := playback.VBlank()
		if err != nil || changed != (blank == 5) || playback.Finished != (blank == 5) {
			t.Fatalf("keyboard exit did not follow the native swap at blank%d", blank)
		}
	}
	before := playback.Ending.Animation.Planes
	if changed, err := playback.VBlank(); err != nil || changed || before != playback.Ending.Animation.Planes {
		t.Fatal("completed ending continued advancing")
	}
}
