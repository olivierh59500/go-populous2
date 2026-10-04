package populous2

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// These layer coordinates and descriptor selections come from the original
// town renderer, with only the final resource-pixel drawing call skipped.
func TestNativeTownCenterLayersAgainstOriginalRenderer(t *testing.T) {
	data, err := os.ReadFile("testdata/town_center_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Input struct {
				Stage, Owner uint8
				Population   uint32
				Tick         uint16
			}
			Layers []SpriteLayer
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 233 {
		t.Fatal("native town center fixture catalog is incomplete")
	}
	art, err := DecodeNativeTownCenterArt(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for index, fixture := range catalog.Cases {
		frame, ok := art.Frame(int(fixture.Input.Stage), fixture.Input.Owner, fixture.Input.Population, fixture.Input.Tick)
		if !ok || !reflect.DeepEqual(frame.Layers, fixture.Layers) || frame.SoundCue != 0 {
			t.Fatalf("native center case%d stage%d owner%d population%d tick%d differs: got%+v want%+v", index, fixture.Input.Stage, fixture.Input.Owner, fixture.Input.Population, fixture.Input.Tick, frame.Layers, fixture.Layers)
		}
	}
}

func TestNativeTownCenterStagesUseExactCompositeSprites(t *testing.T) {
	b := testBundle(t)
	art, err := DecodeNativeTownCenterArt(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	baseSprites := [19]int{681, 681, 753, 753, 682, 683, 754, 684, 734, 685, 755, 686, 756, 687, 688, 689, 689, 690, 690}
	for stage, expected := range baseSprites {
		frame, ok := art.Frame(stage, 1, uint32(24*art.PopulationDivisors[stage]), 0)
		if !ok || len(frame.Layers) != 2 || frame.Layers[0].Sprite != expected || frame.Layers[1].Sprite != 89 {
			t.Fatal("native stage-to-center mapping differs")
		}
		for _, bank := range b.Sprites {
			for _, layer := range frame.Layers {
				if layer.Sprite < 0 || layer.Sprite >= len(bank) {
					t.Fatal("native town center sprite missing in landscape")
				}
			}
		}
	}
	// Callers can modify returned layers without corrupting the decoded bank.
	frame, _ := art.Frame(18, 2, 3984, 1)
	frame.Layers[0].Sprite = 0
	again, _ := art.Frame(18, 2, 3984, 1)
	if again.Layers[0].Sprite != 690 || again.Layers[1].Sprite != 92 {
		t.Fatal("town center frame mutated its decoded art")
	}
	for _, input := range []struct {
		stage int
		owner uint8
	}{{-1, 1}, {19, 1}, {0, 0}, {0, 3}} {
		if _, ok := art.Frame(input.stage, input.owner, 100, 0); ok {
			t.Fatal("invalid center stage or native owner was accepted")
		}
	}
}
