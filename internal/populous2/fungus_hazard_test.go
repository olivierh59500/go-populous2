package populous2

import (
	"encoding/json"
	"os"
	"testing"

	"go-populous2/internal/amiga"
)

type nativeFungusHazardCase struct {
	Name            string
	Tile            uint8
	Hero            int
	InitialSubstate uint8
	Owner           uint8
	Kind, State     uint8
	Animation       int
	Population      int
	ActiveOwner     uint8
	XFraction       uint8
	YFraction       uint8
	SoundArguments  []uint16
	Occupancy       uint16
}

// The independent native helper runs the entire $12c3c follower prepass and
// $124a2 cleanup. Only the hardware audio entry is stubbed after observing D0.
func TestFungusHazardAgainstNativeFollowerPrepass(t *testing.T) {
	data, err := os.ReadFile("testdata/fungus_hazard_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var reference struct{ Cases []nativeFungusHazardCase }
	if err := json.Unmarshal(data, &reference); err != nil {
		t.Fatal(err)
	}
	if len(reference.Cases) != 17 {
		t.Fatalf("native hazard catalog has %d cases, expected seventeen", len(reference.Cases))
	}
	bundle := testBundle(t)
	rules, err := DecodeFungusHazardRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range reference.Cases {
		t.Run(fixture.Name, func(t *testing.T) {
			if fixture.Owner == 3 {
				// $12c42 bypasses terrain checks for the neutral owner. This
				// is the caller's prepass filter, not part of Enter's signature.
				if fixture.Kind != 2 || fixture.State != fixture.InitialSubstate || fixture.Population != 100 || len(fixture.SoundArguments) != 0 {
					t.Fatal("native neutral-owner bypass changed")
				}
				return
			}
			decision := rules.Enter(bundle.GroundRules.Properties[fixture.Tile], fixture.Hero >= 0, fixture.Hero)
			if fixture.Kind == 2 {
				if decision.Applies || fixture.Population != 100 || len(fixture.SoundArguments) != 0 {
					t.Fatalf("native survivor became a death: %+v", decision)
				}
				if fixture.Hero == 1 && !decision.Immune {
					t.Fatal("native Adonis immunity missing")
				}
				return
			}
			if !decision.Applies || decision.Immune || decision.Kind != fixture.Kind || decision.State != fixture.State || decision.Animation != fixture.Animation {
				t.Fatalf("death decision %+v differs from native %+v", decision, fixture)
			}
			if decision.CenterFraction != fixture.XFraction || decision.CenterFraction != fixture.YFraction || fixture.Population != 0 {
				t.Fatal("native death position/population changed")
			}
			if fixture.ActiveOwner != fixture.Owner || fixture.Occupancy != 52 || decision.CleanupMode != 1 {
				t.Fatal("native death actor was removed before animation")
			}
			if len(fixture.SoundArguments) != 1 || decision.RawSoundArgument != fixture.SoundArguments[0] || decision.SoundCue != int(fixture.SoundArguments[0]/10) {
				t.Fatal("native sound argument/cue changed")
			}
			length := rules.SequenceLengths[decision.Animation]
			if length < 1 {
				t.Fatal("death animation unavailable to caller")
			}
			for frame := 0; frame < length; frame++ {
				if image, ok := rules.Frames[decision.Animation+frame*4]; !ok || len(image.Layers) == 0 {
					t.Fatalf("native death frame %d missing", frame)
				}
			}
		})
	}
}

func TestFungusHazardMetadataAndInvalidInputs(t *testing.T) {
	bundle := testBundle(t)
	rules, err := DecodeFungusHazardRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	if cue := bundle.Audio.Cues[rules.SoundCue]; cue.Pattern != 26 || rules.RawSoundArgument != 0x104 {
		t.Fatal("native cue descriptor mapping changed")
	}
	for _, properties := range []uint16{0, 1, 0x11} {
		if rules.Enter(properties, false, -1).Applies {
			t.Fatal("nonfatal terrain triggered fungus")
		}
	}
	for _, invalid := range []int{-1, 6, 100} {
		if decision := rules.Enter(0x10, true, invalid); decision.Applies || decision.Immune {
			t.Fatal("invalid hero index produced a decision")
		}
	}
	if _, err := DecodeFungusHazardRules(nil); err == nil {
		t.Fatal("missing executable accepted")
	}
	if _, err := DecodeFungusHazardRules(&amiga.Executable{}); err == nil {
		t.Fatal("missing hazard tables accepted")
	}
}
