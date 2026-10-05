package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

type renderActorFixture struct {
	Input struct {
		Name, Mode string
		Initial    []nativeHeroPatch
		D          [8]uint32
		Pattern    bool
		GridCursor int
	}
	D                    [8]uint32
	Changes, BankChanges []nativeHeroPatch
	Sprites              []struct {
		Sprite  int
		X, Y    int16
		Height  uint16
		Routine uint32
	}
	LastY, TownHitHeight       uint16
	BSSHash, BitmapHash, Error string
	Child                      uint32
}

func renderActorInitial(f renderActorFixture) []byte {
	b := make([]byte, 0x11280)
	m := commandNativeMemory(b)
	_ = m.Write32(0x1e, 0xa10000)
	_ = m.Write32(0xf36, 0x200000+0x76f4)
	b[0x76f4], b[0x76f4+12], b[0x76f4+22], b[0x76f4+25] = 2, 1, 4, 7
	_ = m.Write16(0x76f4+14, 20)
	_ = m.Write32(0x76f4+26, 1000)
	for i := 0; i < 4096; i++ {
		b[0xf44+i*4], b[0xf45+i*4] = 0xa8, 15
	}
	for _, p := range f.Input.Initial {
		renderFramePatch(m, p)
	}
	return b
}

func TestNativeActorRenderingFullFrameAndPixelsAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/render_frame_actor_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []renderActorFixture }
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 774 {
		t.Fatalf("native actor renderer corpus incomplete: %v", err)
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
	counts := map[string]int{}
	sprites, permissions := 0, 0
	for _, f := range catalog.Cases {
		counts[f.Input.Mode]++
		t.Run(f.Input.Name, func(t *testing.T) {
			if f.Error != "" {
				t.Fatal("original renderer did not finish", f.Error)
			}
			raw := renderActorInitial(f)
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			image := rules.Frames.Images.NewImageState()
			expectedImage := image
			for _, p := range f.BankChanges {
				if p.Address < 0 || p.Address >= len(expectedImage.AudioBank) || p.Width != 1 {
					t.Fatal("native image bank mutation malformed")
				}
				expectedImage.AudioBank[p.Address] = byte(p.Value)
			}
			bitmap := make([]byte, 32000)
			if f.Input.Pattern {
				for i := range bitmap {
					bitmap[i] = byte(i*7 + 13)
				}
			}
			cb := NativeRenderFrameCallbacks{Memory: commandNativeMemory(raw), Frame: &frame, Image: &image, Bitmap: bitmap, Sprite: bank.Paint}
			state := NativeActorRenderState{}
			var plan NativeRenderFramePlan
			if f.Input.Mode == "angle" {
				err = rules.Angle(&frame)
			} else if f.Input.Mode == "selected" {
				plan, err = rules.Frames.Selected(cb, NativeRenderFrameChildren{DrawActor: func(at int, c *NativeFrameRegisterContext) error {
					_, err := rules.Follower(at, cb, &state, NativeActorRenderChildren{})
					return err
				}})
			} else {
				plan, err = rules.Follower(0x76f4, cb, &state, NativeActorRenderChildren{})
			}
			if f.Child != 0 {
				if err == nil || f.Child != 0x314a || !strings.Contains(err.Error(), "314a") {
					t.Fatalf("native actor child boundary differs: %x/%v", f.Child, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if frame.D != f.D {
				t.Errorf("native actor all8D differ: got%x want%x", frame.D, f.D)
			}
			if image.LastY != f.LastY || image.AudioBank != expectedImage.AudioBank || state.TownHitHeight != f.TownHitHeight {
				t.Errorf("native actor mutable CODE differs: Y%x/%x town%x/%x", image.LastY, f.LastY, state.TownHitHeight, f.TownHitHeight)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != f.BSSHash {
				expected := renderActorInitial(f)
				for _, p := range f.Changes {
					renderFramePatch(commandNativeMemory(expected), p)
				}
				shown := 0
				for i := range raw {
					if raw[i] != expected[i] && shown < 10 {
						t.Logf("BSS%#x differs: got%x native%x", i, raw[i], expected[i])
						shown++
					}
				}
				t.Errorf("native actor full BSS differs: got%s want%s", got, f.BSSHash)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(bitmap)); got != f.BitmapHash {
				t.Errorf("native actor actual sprite/software bitmap differs: got%s want%s", got, f.BitmapHash)
			}
			if f.Input.Mode != "selected" {
				if len(plan.Sprites) != len(f.Sprites) {
					t.Fatalf("native actor sprite request count differs: got%d want%d", len(plan.Sprites), len(f.Sprites))
				}
				for i, s := range plan.Sprites {
					want := f.Sprites[i]
					if s.Sprite != want.Sprite || s.X != want.X || s.Y != want.Y || s.Height != int16(want.Height) || s.Routine != want.Routine {
						t.Errorf("native actor sprite request differs: got%+v want%+v", s, want)
					}
				}
			}
			sprites += len(f.Sprites)
			if raw[0xe76a+314+0x4b]&3 != 0 || raw[0xe76a+628+0x4b]&3 != 0 {
				permissions++
			}
		})
	}
	if counts["actor"] != 633 || counts["angle"] != 81 || counts["selected"] != 60 || sprites == 0 || permissions == 0 {
		t.Fatalf("native actor render coverage incomplete: modes%v sprites%d permissions%d", counts, sprites, permissions)
	}
}
