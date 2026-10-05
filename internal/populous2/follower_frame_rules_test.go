package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestCompleteFollowerRuleSetAgainstOriginalPasses(t *testing.T) {
	files := []struct {
		name  string
		count int
	}{
		{"follower_motion_entry_pass_native.json", 32},
		{"follower_search_hero_magnet_pass_native.json", 112},
		{"follower_combat_win_pass_native.json", 64},
		{"follower_town_frame_pass_native.json", 64},
		{"follower_terrain_pass_native.json", 72},
		{"follower_retained_pass_native.json", 32},
		{"follower_neutral_pass_native.json", 28},
		{"follower_siege_pass_native.json", 48},
	}
	b := testBundle(t)
	rules := [4]*NativeFollowerFrameRules{}
	for i := range rules {
		var err error
		rules[i], err = DecodeNativeFollowerFrameRules(b, i)
		if err != nil {
			t.Fatal(err)
		}
	}
	count := 0
	for _, file := range files {
		data, err := os.ReadFile("testdata/" + file.name)
		if err != nil {
			t.Fatal(err)
		}
		var corpus struct {
			Cases []struct {
				frameContextFixture
				Input struct {
					frameContextInput
					Landscape int
				}
			}
		}
		if err := json.Unmarshal(data, &corpus); err != nil {
			t.Fatal(err)
		}
		if len(corpus.Cases) != file.count {
			t.Fatal(file.name, "source pass coverage differs")
		}
		for _, f := range corpus.Cases {
			count++
			initial := frameContextInitial(f.Input.frameContextInput)
			w := aiFixtureWorld(t, initial)
			w.Landscape = b.Landscapes[f.Input.Landscape]
			w.FollowerWin, err = DecodeFollowerWinRules(b.Executable, w.Landscape)
			if err != nil {
				t.Fatal(err)
			}
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			state := rules[f.Input.Landscape].NewState()
			outputs := NativeFollowerFrameOutputs{MapPoint: func(_ uint16, c *NativeFrameRegisterContext) error { _, err := PlanNativeMapPoint(c); return err }}
			w.nativeCallDepth++
			err = rules[f.Input.Landscape].Tick(w, &frame, state, outputs)
			w.nativeCallDepth--
			if err != nil {
				t.Fatal(file.name, f.Input.Name, err)
			}
			want := f.Frames[0]
			if want.ErrorPC != 0 || frame.D != want.D {
				t.Fatalf("%s %s complete rule-set registers differ: %x/%x", file.name, f.Input.Name, frame.D, want.D)
			}
			all := aiFixtureWorldBytes(w, initial)
			copy(all[0xeb90:], w.NativeRedrawBytes[:])
			if got := fmt.Sprintf("%x", sha256.Sum256(all)); got != want.Hash {
				t.Fatalf("%s %s complete rule-set BSS differs: %s/%s", file.name, f.Input.Name, got, want.Hash)
			}
		}
	}
	if count != 452 {
		t.Fatalf("complete native pass total%d", count)
	}
}
