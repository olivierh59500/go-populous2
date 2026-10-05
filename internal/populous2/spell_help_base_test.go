package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNativeSpellHelpBaseAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/spell_help_base_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Slot, Available, Background int
			Visible                     bool
			RGBAHash                    string
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 144 {
		t.Fatal("native spell help base corpus incomplete")
	}
	b := testBundle(t)
	r, err := DecodeNativeInGameRequesterRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Cases {
		state := NativeWorldRequesterState{}
		state.PowerFlags[fixture.Slot] = int8(fixture.Available)
		background := make([]byte, 320*200)
		if fixture.Background != 0 {
			for y := range 200 {
				for x := range 320 {
					background[y*320+x] = byte((x*3 + y*5 + 7) & 15)
				}
			}
		}
		frame, visible, err := r.SpellHelpBase(b, state, uint16(fixture.Slot*2), background)
		if err != nil {
			t.Fatal(err)
		}
		if visible != fixture.Visible {
			t.Fatalf("slot%d availability%d help admission differs", fixture.Slot, fixture.Available)
		}
		if visible {
			if got := fmt.Sprintf("%x", sha256.Sum256(frame.Image.Pix)); got != fixture.RGBAHash {
				t.Fatalf("slot%d background%d help base differs: %s / %s", fixture.Slot, fixture.Background, got, fixture.RGBAHash)
			}
		}
	}
}
