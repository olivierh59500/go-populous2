package populous2

import (
	"encoding/json"
	"os"
	"testing"
)

type sceneryRenderFixture struct {
	Input struct {
		Name      string
		Animation uint16
		Age       uint8
		X, Y      int16
	}
	Draws []struct {
		Sprite                              int
		X, Y, Height, SourceHeight, Width   int16
		SourceSkip, PlaneStride, BlitHeight int
		Drawn                               bool
	}
}

// The original CPU executes $eae4 and both actual sprite blitters. These
// observations include clipping, original plane stride and source-row skips;
// hardware pixel colors and sound-queue writes are outside this geometry test.
func TestSceneryRenderingAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/scenery_render_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []sceneryRenderFixture
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 5340 {
		t.Fatal("native scenery renderer fixture catalog is incomplete")
	}
	rules, err := DecodeSceneryRenderRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules.Frames) != 12 {
		t.Fatal("native tree, boulder or fire animation frame is missing")
	}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			slices, err := rules.Plan(fixture.Input.Animation, int8(fixture.Input.Age), fixture.Input.X, fixture.Input.Y)
			if err != nil {
				t.Fatal(err)
			}
			index := 0
			for _, draw := range fixture.Draws {
				if !draw.Drawn {
					continue
				}
				if index >= len(slices) {
					t.Fatal("native blitter drew a layer absent from the render plan")
				}
				slice := slices[index]
				index++
				if slice.Sprite != draw.Sprite || slice.UnclippedX != draw.X || slice.UnclippedY != draw.Y || slice.VisibleHeight != draw.Height || slice.SourceHeight != int(draw.SourceHeight) || slice.SourceY != draw.SourceSkip || slice.PlaneStride != draw.PlaneStride || slice.Height != draw.BlitHeight {
					t.Fatalf("native blitter geometry differs: got %+v; original %+v", slice, draw)
				}
				// Native horizontal masks limit the destination to the viewport;
				// retain the corresponding source-column interval for RGBA drawing.
				left, right := max(0, int(draw.X)), min(320, int(draw.X+draw.Width))
				if slice.X != left || slice.Width != right-left || slice.SourceX != left-int(draw.X) || slice.Y != max(0, int(draw.Y)) || slice.Width <= 0 || slice.Height <= 0 || slice.X+slice.Width > 320 || slice.Y+slice.Height > 200 {
					t.Fatal("clipped scenery rectangle exceeds its native source or viewport")
				}
			}
			if index != len(slices) {
				t.Fatal("render plan contains a layer suppressed by the original blitter")
			}
		})
	}
}

func TestSceneryRenderRejectsUnknownAnimation(t *testing.T) {
	rules, err := DecodeSceneryRenderRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, animation := range []uint16{0, 0x25e, 0xf20, 0xffff} {
		if _, err := rules.Plan(animation, 0, 160, 100); err == nil {
			t.Fatal("unknown or terminal scenery animation was accepted")
		}
	}
	if _, err := DecodeSceneryRenderRules(nil); err == nil {
		t.Fatal("missing native tables were accepted")
	}
}
