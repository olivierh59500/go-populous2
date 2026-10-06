package game

import (
	"strings"
	"testing"

	"go-populous2/internal/populous2"
)

func TestNativeShowcasePilotToursMenusAndStartsPlayableConquest(t *testing.T) {
	bundle, err := populous2.Load()
	if err != nil {
		t.Fatal(err)
	}
	g, err := NewNativeOffline(bundle)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	p := NewNativeShowcasePilot()
	seen := make(map[int]bool)
	for tick := 0; tick < 85*50; tick++ {
		input, err := p.Next(g)
		if err != nil {
			t.Fatalf("input tick %d: %v", tick, err)
		}
		seen[p.phase] = true
		if _, err := g.StepNative(input); err != nil {
			t.Fatalf("game tick %d: %v", tick, err)
		}
		if tick == 200 && !strings.HasPrefix(p.Caption, "Populous II Go\n") {
			t.Fatalf("opening title disappeared before it could be read: %q", p.Caption)
		}
	}
	for phase := 1; phase <= 6; phase++ {
		if !seen[phase] {
			t.Errorf("menu tour skipped phase %d", phase)
		}
	}
	if g.Frame == nil || p.gameAt == 0 || p.gameAt > 70*50 {
		t.Fatalf("tour did not begin a conquest promptly: gameplay tick %d", p.gameAt)
	}
	if p.player.Towns < 2 || p.player.TerrainActions < 5 {
		t.Fatalf("opening play failed to develop settlements: %s", p.Status(g))
	}
	if g.Director.Children.Help.Started && !g.Director.Children.Help.Finished {
		t.Fatal("the power help requester remained open")
	}
	if g.Result.Result != nil {
		t.Fatal("the opening presentation unexpectedly finished the campaign world")
	}
}

func TestNativeShowcasePilotActionCaptionRemainsReadable(t *testing.T) {
	p := NewNativeShowcasePilot()
	p.gameAt = 3000
	g := &NativeGame{Updates: 9600}
	p.player.Stage = "Rallying a strong expedition"
	p.observeAction(g)
	caption := p.gameCaption(g.Updates - p.gameAt)
	if !strings.Contains(caption, "magnet") {
		t.Fatalf("actual rally action was not described: %q", caption)
	}
	p.player.Stage = "Moving to the next settlement"
	g.Updates += 250
	p.observeAction(g)
	if got := p.gameCaption(g.Updates - p.gameAt); got != caption {
		t.Fatalf("a transient planner stage interrupted the explanation: %q", got)
	}
	g.Updates += 400
	if got := p.gameCaption(g.Updates - p.gameAt); got == caption {
		t.Fatal("action caption remained after its twelve-second reading period")
	}
}
