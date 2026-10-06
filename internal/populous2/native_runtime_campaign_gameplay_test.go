package populous2

import (
	"encoding/json"
	"os"
	"testing"
)

// Six worlds are selected through the actual campaign password modal.
// Complete E94 main frames include genuine protection requesters. IRQ/poll boundaries
// come from the independent CPU, not a Go-derived gameplay schedule.
func TestNativeRuntimeCampaignGameplayAgainstOriginalMain(t *testing.T) {
	data, err := os.ReadFile("testdata/native_runtime_campaign_gameplay_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Input struct {
				Name     string
				World    int
				Password string
			}
			Startup      nativeStockSnapshot
			StartupPolls []int
			Frames       []nativeStockSnapshot
			FrameInputs  []struct {
				Tick    int
				Samples []nativeStockSample
				Polls   []int
			}
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 6 {
		t.Fatal("campaign main corpus incomplete", err)
	}
	landscapes := map[uint16]bool{}
	worlds := map[int]bool{}
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			if len(f.Frames) != 15 || len(f.FrameInputs) != 1000 || len(f.StartupPolls) == 0 {
				t.Fatal("campaign main checkpoints or inputs truncated")
			}
			g := nativeRuntimeGameplayStart(t, f.Startup, f.StartupPolls)
			h, c, apply := g.Host, g.Frame, g.Apply
			check := func(want nativeStockSnapshot) { nativeRuntimeGameplayCheck(t, h, want, c.D) }
			check(f.Startup)
			worlds[f.Input.World] = true
			land, err := h.Memory.BSS.Read16(0xeb22)
			if err != nil {
				t.Fatal(err)
			}
			landscapes[land] = true
			if int(land) != h.Bundle.Levels[f.Input.World].Terrain {
				t.Fatal("original world landscape differs", land)
			}

			if world, err := h.Memory.BSS.Read16(0xeb46); err != nil || int(world) != f.Input.World {
				t.Fatal("genuine password selected wrong world", world, err)
			}
			if mode, err := h.Memory.BSS.Read16(0xeb44); err != nil || mode != 2 {
				t.Fatal("actual campaign menu did not select conquest", mode, err)
			}

			frame := g.newFrame(t, nil)
			checkpoint := 0
			for tick, events := range f.FrameInputs {
				if events.Tick != tick || len(events.Polls) == 0 {
					t.Fatal("original frame input sequence incomplete")
				}
				if err := h.Session.BeginRaw(h.World, c); err != nil {
					t.Fatal(err)
				}
				pos, complete := 0, false
				for j, end := range events.Polls {
					if end < pos || end > len(events.Samples) {
						t.Fatal("invalid original main poll")
					}
					for ; pos < end; pos++ {
						apply(events.Samples[pos], &h.Session.Frame)
					}
					complete, err = frame.Advance()
					if err != nil {
						t.Fatalf("campaign tick%d poll%d: %v", tick, j, err)
					}
					if complete && j != len(events.Polls)-1 {
						t.Fatalf("campaign tick%d returned before original wait boundary", tick)
					}
				}
				if !complete || pos != len(events.Samples) {
					t.Fatalf("campaign tick%d source frame remained pending: phase%d render%d protection%x", tick, h.Session.Phase, frame.RenderState.Step, frame.RenderChildren.ProtectionStep.PC)
				}
				c = h.Session.Frame
				if checkpoint < len(f.Frames) && f.Frames[checkpoint].Tick == tick {
					check(f.Frames[checkpoint])
					checkpoint++
				}
			}
			if checkpoint != 15 || h.World.nativeCallDepth != 0 || h.Session.Phase != NativeFrameSessionIdle {
				t.Fatal("campaign main checkpoints/borrow incomplete")
			}
		})
	}
	if len(landscapes) != 4 || len(worlds) != 6 {
		t.Fatal("campaign landscape/profile coverage incomplete", landscapes, worlds)
	}
}
