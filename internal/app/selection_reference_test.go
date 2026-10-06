package app

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	"os"
	"testing"

	"go-populous2/internal/engine"
	"go-populous2/internal/populous2"
)

// This optional comparison invokes the actual selected-panel parent and its
// actor child. Both sides receive the same owner, population, state and phase;
// it checks complete pixels, rather than just the actor's panel anchor.
func TestPrivateSelectedPanelAnimationMatchesOriginalPixels(t *testing.T) {
	original, portable := os.Getenv("POPULOUS2_EXPORT_TEST_DIR"), os.Getenv("POPULOUS2_ENDING_TEST_DIR")
	if original == "" || portable == "" {
		t.Skip("set private original and portable asset directories")
	}
	source, err := populous2.LoadFS(os.DirFS(original))
	if err != nil {
		t.Fatal(err)
	}
	assets, err := LoadAssets(os.DirFS(portable))
	if err != nil {
		t.Fatal(err)
	}
	rules, err := populous2.DecodeNativeActorRenderRules(source.Executable)
	if err != nil {
		t.Fatal(err)
	}
	bank, err := populous2.DecodeNativeSpriteBitmapBank(source, 0)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name               string
		follower           engine.Follower
		kind, state, flags uint8
		token              uint16
		leader             bool
		phases             int
	}{
		{"walking", engine.Follower{State: engine.Walking}, 2, 4, 0, 0, false, 4},
		{"walking leader", engine.Follower{State: engine.Walking}, 2, 4, 1, 0, true, 4},
		{"town", engine.Follower{State: engine.Town, Stage: 18}, 4, 6, 0, 0, false, 2},
		{"town leader", engine.Follower{State: engine.Town, Stage: 18}, 4, 6, 1, 0, true, 2},
		{"hero", engine.Follower{State: engine.Walking, Hero: engine.HeroState{Kind: engine.HeroPerseus}}, 2, 38, 2, 0, false, 4},
		{"battle aggressor", engine.Follower{State: engine.Fighting, BattleAggressor: true}, 2, 14, 0, 0x1c8, false, 2},
		{"battle town defender", engine.Follower{State: engine.Fighting, BattleWasTown: true, Stage: 18}, 4, 16, 0, 0, false, 2},
		{"waiting group", engine.Follower{State: engine.Walking, ContactWaiting: true}, 2, 10, 0, 0xccc, false, 2},
	}
	// Building silhouettes and the flag's population height vary by stage.
	// Check each distinct source composition, not just the largest settlement.
	for stage := uint8(0); stage < engine.TownStages-1; stage++ {
		cases = append(cases, struct {
			name               string
			follower           engine.Follower
			kind, state, flags uint8
			token              uint16
			leader             bool
			phases             int
		}{fmt.Sprintf("town leader stage %d", stage), engine.Follower{State: engine.Town, Stage: stage}, 4, 6, 1, 0, true, 2})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for phase := 0; phase < tc.phases; phase++ {
				world := &engine.World{Tick: uint64(phase)}
				f := tc.follower
				f.Owner, f.X, f.Y, f.Population, f.Weapons, f.Frame = 0, 12, 12, 0x123, 7, uint16(phase)
				world.Followers[1] = f
				if tc.leader {
					world.Players[0].Leader = 1
				}
				g := &Game{World: world, SelectedFollower: 1, Assets: assets, framebuffer: image.NewRGBA(image.Rect(0, 0, 320, 200))}
				draw.Draw(g.framebuffer, g.framebuffer.Bounds(), image.NewUniform(assets.Visual.Palettes[0][0]), image.Point{}, draw.Src)
				before := world.Snapshot()
				g.drawSelectionPanel()
				if world.Snapshot() != before {
					t.Fatal("panel drawing changed simulation")
				}
				raw := make([]byte, 0x11280)
				const actor = 0x76f4
				raw[actor], raw[actor+1], raw[actor+12], raw[actor+13], raw[actor+22], raw[actor+25] = tc.kind, f.Stage, 1, tc.flags, tc.state, uint8(f.Weapons)
				binary.BigEndian.PutUint16(raw[actor+10:], tc.token+uint16(phase*4))
				binary.BigEndian.PutUint16(raw[actor+16:], 0xffec)
				binary.BigEndian.PutUint32(raw[actor+26:], uint32(f.Population))
				binary.BigEndian.PutUint16(raw[0xf42:], uint16(phase))
				binary.BigEndian.PutUint32(raw[0xf36:], 0x200000+actor)
				binary.BigEndian.PutUint16(raw[0xeb18:], 12)
				binary.BigEndian.PutUint16(raw[0x3b8:], 1)
				memory := selectedPanelReferenceMemory(raw)
				bitmap := make([]byte, 32000)
				frame := populous2.NativeFrameRegisterContext{AddressBase: 0x200000}
				imageState := rules.Frames.Images.NewImageState()
				cb := populous2.NativeRenderFrameCallbacks{Memory: memory, Frame: &frame, Image: &imageState, Bitmap: bitmap, Sprite: bank.Paint}
				_, err := rules.Frames.Selected(cb, populous2.NativeRenderFrameChildren{DrawActor: func(at int, _ *populous2.NativeFrameRegisterContext) error {
					_, e := rules.Actor(at, populous2.NativeActorEffectsCallbacks{NativeRenderFrameCallbacks: cb}, &populous2.NativeActorRenderState{}, populous2.NativeActorRenderChildren{})
					return e
				}})
				if err != nil {
					t.Fatal(err)
				}
				want, err := populous2.DecodeScreen(bitmap, assets.Visual.Palettes[0])
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(g.framebuffer.Pix, want.Pix) {
					for y := range 200 {
						for x := range 320 {
							if g.framebuffer.RGBAAt(x, y) != want.RGBAAt(x, y) {
								t.Fatalf("selected phase %d pixel %d,%d differs: got %v source %v", phase, x, y, g.framebuffer.RGBAAt(x, y), want.RGBAAt(x, y))
							}
						}
					}
				}
			}
		})
	}
}

func selectedPanelReferenceMemory(raw []byte) populous2.FollowerCleanupMemory {
	span := func(at, n int) error {
		if at < 0 || at > len(raw)-n {
			return fmt.Errorf("selected-panel reference access outside backing")
		}
		return nil
	}
	return populous2.FollowerCleanupMemory{
		Read8: func(at int) (uint8, error) {
			if err := span(at, 1); err != nil {
				return 0, err
			}
			return raw[at], nil
		},
		Read16: func(at int) (uint16, error) {
			if err := span(at, 2); err != nil {
				return 0, err
			}
			return binary.BigEndian.Uint16(raw[at:]), nil
		},
		Read32: func(at int) (uint32, error) {
			if err := span(at, 4); err != nil {
				return 0, err
			}
			return binary.BigEndian.Uint32(raw[at:]), nil
		},
		Write8: func(at int, v uint8) error {
			if err := span(at, 1); err != nil {
				return err
			}
			raw[at] = v
			return nil
		},
		Write16: func(at int, v uint16) error {
			if err := span(at, 2); err != nil {
				return err
			}
			binary.BigEndian.PutUint16(raw[at:], v)
			return nil
		},
		Write32: func(at int, v uint32) error {
			if err := span(at, 4); err != nil {
				return err
			}
			binary.BigEndian.PutUint32(raw[at:], v)
			return nil
		},
	}
}
