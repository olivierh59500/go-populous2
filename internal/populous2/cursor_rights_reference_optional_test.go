package populous2

import (
	"go-populous2/internal/engine"
	"os"
	"testing"
)

// This optional oracle executes the original actor renderer. Production rights
// remain pure Go view and actor rules; no renderer addresses enter the engine.
func TestIndependentCursorRightsActorReferenceOptional(t *testing.T) {
	if os.Getenv("POPULOUS2_REFERENCE_COMPARE") != "1" {
		t.Skip("optional local reference comparison")
	}
	bundle := testBundle(t)
	rules, err := DecodeNativeActorRenderRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	bank, err := DecodeNativeSpriteBitmapBank(bundle, 0)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name        string
		sourceState uint8
		follower    engine.Follower
	}{
		{"search", 2, engine.Follower{State: engine.Walking}},
		{"moving", 4, engine.Follower{State: engine.Walking}},
		{"contact", 12, engine.Follower{State: engine.Walking, ContactWith: 2}},
		{"waiting", 10, engine.Follower{State: engine.Walking, ContactWaiting: true}},
		{"walking battle", 14, engine.Follower{State: engine.Fighting}},
		{"defending town", 16, engine.Follower{State: engine.Fighting, BattleWasTown: true, Stage: 1}},
		{"swimmer", 22, engine.Follower{State: engine.Drowning}},
		{"hero pursuit", 38, engine.Follower{State: engine.Walking, Hero: engine.HeroState{Kind: engine.HeroPerseus}}},
		{"carried", 20, engine.Follower{State: engine.Airborne}},
		{"conversion", 54, engine.Follower{State: engine.Converting, Conversion: engine.ConversionState{Active: true}}},
		{"dead", 24, engine.Follower{State: engine.Ruin, CleanupPrepared: true}},
	}
	for _, rawState := range []uint8{30, 28, 32, 42, 46, 48, 50, 56, 58, 60, 62, 64, 66, 70} {
		follower := engine.Follower{State: engine.Ruin, CleanupPrepared: true}
		if rawState == 30 {
			follower = engine.Follower{State: engine.Town}
		}
		cases = append(cases, struct {
			name        string
			sourceState uint8
			follower    engine.Follower
		}{"retained continuation", rawState, follower})
	}
	for stage := uint8(0); stage < 19; stage++ {
		cases = append(cases, struct {
			name        string
			sourceState uint8
			follower    engine.Follower
		}{"town", 6, engine.Follower{State: engine.Town, Stage: stage}})
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			fixture := renderActorFixture{}
			raw := renderActorInitial(fixture)
			m := commandNativeMemory(raw)
			at := 0x76f4
			raw[at+22] = test.sourceState
			raw[at+1] = test.follower.Stage
			if test.follower.State == engine.Town || test.follower.BattleWasTown {
				raw[at] = 4
			}
			_ = m.Write16(0xeb18, 2)
			_ = m.Write16(0x3b8, 1)
			_ = m.Write16(0x3b0, 0)
			_ = m.Write16(at+10, 0)
			_ = m.Write16(at+20, 5)
			frame := NativeFrameRegisterContext{D: [8]uint32{192, 100}, AddressBase: 0x200000}
			image := rules.Frames.Images.NewImageState()
			state := NativeActorRenderState{}
			cb := NativeRenderFrameCallbacks{Memory: m, Frame: &frame, Image: &image, Bitmap: make([]byte, 32000), Sprite: bank.Paint}
			if _, err := rules.Follower(at, cb, &state, NativeActorRenderChildren{}); err != nil {
				t.Fatal(err)
			}
			expected := raw[0xe76a+314+0x4b] & 3
			world := &engine.World{}
			f := test.follower
			f.Owner = 0
			f.X = 22
			f.Y = 22
			f.Population = 100
			world.Followers[1] = f
			world.Actors.Link(engine.ActorRef{Kind: engine.ActorFollower, Index: 1}, 22*256+128, 22*256+128)
			rights := world.CursorTerrainRights(0, engine.Viewport{X: 20, Y: 20, Size: 8})
			actual := uint8(0)
			if rights.BuildAnywhere {
				actual |= 1
			}
			if rights.SeaLevelOnly {
				actual |= 2
			}
			if actual != expected {
				t.Fatalf("source state%d stage%d rights%d reference%d", test.sourceState, f.Stage, actual, expected)
			}
		})
	}
}
