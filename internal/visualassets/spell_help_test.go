package visualassets

import "testing"

func TestHelpSequenceKeepsOpeningFramesAndExplicitLoop(t *testing.T) {
	sequence := HelpSequence{Frames: make([]HelpFrame, 4), LoopStart: 1}
	for _, sample := range []struct{ age, index int }{{-1, 0}, {0, 0}, {1, 1}, {2, 2}, {3, 3}, {4, 1}, {5, 2}, {6, 3}, {7, 1}, {1000, 1}} {
		if got := sequence.FrameIndex(sample.age); got != sample.index {
			t.Fatal("help animation opening or loop changed", sample, got)
		}
	}
	if (&HelpSequence{}).FrameIndex(0) != -1 {
		t.Fatal("empty help sequence returned an image")
	}
}
