package populous2

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type nativeRequesterGeometry struct {
	TextHex                    string
	Column, Row, Width, Height int
	Clicks                     []struct{ X, Y, Action int }
}
type nativeInGameRequesterFixture struct {
	About     nativeRequesterGeometry
	MenuPlans []struct {
		Input NativeInGameState
		nativeRequesterGeometry
	}
	MenuActions []struct {
		Input                                 NativeInGameState
		Action                                int
		Stop                                  string
		DeferredCommand                       uint8
		SwitchProfile, ControlMode, PaintFlag uint16
		Continuation                          uint32
	}
	WorldPlans []struct {
		Input NativeWorldRequesterState
		nativeRequesterGeometry
		Icons []struct {
			SourceOffset uint16
			X, Y, Height int16
		}
	}
	Codes []struct {
		World uint16
		Code  string
	}
	Lookups []struct {
		Input, Stop, ErrorTextHex string
		World                     uint16
	}
	WorldActions []struct {
		Action int
		Stop   string
	}
	SpellHits []struct {
		X, Y, Offset uint16
		Hit          bool
	}
}

func nativeInGameFixtures(t *testing.T) nativeInGameRequesterFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/ingame_requester_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture nativeInGameRequesterFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}
func checkNativeRequesterGeometry(t *testing.T, rules *NativeInGameRequesterRules, plan *NativeRequester, fixture nativeRequesterGeometry) {
	t.Helper()
	want, err := hex.DecodeString(fixture.TextHex)
	if err != nil || !bytes.Equal(want, plan.Text) || plan.Column != fixture.Column || plan.Row != fixture.Row || plan.Width != fixture.Width || plan.Height != fixture.Height {
		t.Fatalf("requester differs from original CPU: got%q native%q", plan.Text, want)
	}
	for _, click := range fixture.Clicks {
		if got := rules.Presentation.Requesters.Click(plan, click.X, click.Y); got != click.Action {
			t.Fatalf("native pixel-cell click%d,%d: action%d native%d", click.X, click.Y, got, click.Action)
		}
	}
}

func TestNativeInGameRequesterPlansAndActionsAgainstOriginalCPU(t *testing.T) {
	fixture := nativeInGameFixtures(t)
	if len(fixture.MenuPlans) != 216 || len(fixture.MenuActions) != 2808 {
		t.Fatal("native menu fixture catalog incomplete")
	}
	rules, err := DecodeNativeInGameRequesterRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	clicks := 0
	for i, plan := range fixture.MenuPlans {
		t.Run(fmt.Sprintf("plan%d", i), func(t *testing.T) {
			got, err := rules.Plan(plan.Input)
			if err != nil {
				t.Fatal(err)
			}
			checkNativeRequesterGeometry(t, rules, got, plan.nativeRequesterGeometry)
		})
		clicks += len(plan.Clicks)
	}
	if clicks != 4000 {
		t.Fatal("native in-game click catalog incomplete")
	}
	about, err := rules.AboutPlan()
	if err != nil {
		t.Fatal(err)
	}
	if len(fixture.About.Clicks) != 1000 {
		t.Fatal("native About click catalog incomplete")
	}
	checkNativeRequesterGeometry(t, rules, about, fixture.About)
	for i, action := range fixture.MenuActions {
		t.Run(fmt.Sprintf("action%d", i), func(t *testing.T) {
			state := action.Input
			step, err := rules.Action(&state, action.Action)
			if err != nil {
				t.Fatal(err)
			}
			continuation := uint32(0)
			switch step.Continuation {
			case NativeInGameOptions:
				continuation = 0x471c
			case NativeInGameSerial:
				continuation = 0x4984
			case NativeInGameAbout:
				continuation = 0x33b2
			}
			if step.DeferredCommand != action.DeferredCommand || step.SwitchProfile != action.SwitchProfile || continuation != action.Continuation || state.ControlMode != action.ControlMode || state.PaintFlag != action.PaintFlag || step.Close != (action.Stop == "resume-check") {
				t.Fatalf("native menu action differs: state%+v step%+v native%+v", state, step, action)
			}
			if step.Close && step.ResumeCheck != 0x181c0 {
				t.Fatal("menu closed without its actual transport/status gate")
			}
			if !step.Close && step.ResumeCheck != 0 {
				t.Fatal("menu spuriously requested its return gate")
			}
			if state.Profile != action.Input.Profile || state.GameMode != action.Input.GameMode {
				t.Fatal("menu fabricated an external profile/mode switch")
			}
		})
	}
}

func TestNativeWorldRequesterAgainstOriginalCPU(t *testing.T) {
	fixture := nativeInGameFixtures(t)
	if len(fixture.WorldPlans) != 360 || len(fixture.Codes) != 1000 || len(fixture.Lookups) != 13 || len(fixture.WorldActions) != 5 || len(fixture.SpellHits) != 1000 {
		t.Fatal("native world fixture catalog incomplete")
	}
	rules, err := DecodeNativeInGameRequesterRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixture.Codes {
		if got := string(rules.NativeWorldCode(fixture.World)); got != fixture.Code {
			t.Fatalf("world%d code%q native%q", fixture.World, got, fixture.Code)
		}
	}
	for i, plan := range fixture.WorldPlans {
		t.Run(fmt.Sprintf("plan%d", i), func(t *testing.T) {
			got, err := rules.WorldPlan(plan.Input)
			if err != nil {
				t.Fatal(err)
			}
			checkNativeRequesterGeometry(t, rules, got.Requester, plan.nativeRequesterGeometry)
			if len(got.Icons) != len(plan.Icons) {
				t.Fatal("native signed-byte icon admission differs")
			}
			for i, icon := range got.Icons {
				want := plan.Icons[i]
				if icon.X != want.X || icon.Y != want.Y || icon.Height != want.Height || icon.SourceOffset != want.SourceOffset || icon.HalfWidth != 16 {
					t.Fatal("native world icon descriptor/anchor differs")
				}
			}
		})
	}
	for _, fixture := range fixture.Lookups {
		step := rules.FinishWorldCode([]byte(fixture.Input))
		if step.LoadWorld != (fixture.Stop == "load-world") || step.InvalidCode != (fixture.Stop == "invalid") || step.LoadWorld && step.World != fixture.World {
			t.Fatalf("native lookup%q differs: %+v native%+v", fixture.Input, step, fixture)
		}
		if step.InvalidCode {
			plan, err := rules.InvalidWorldCodePlan()
			want, decodeErr := hex.DecodeString(fixture.ErrorTextHex)
			if err != nil || decodeErr != nil || !bytes.Equal(plan.Text, want) {
				t.Fatal("native invalid-code message differs")
			}
		}
	}
	for _, fixture := range fixture.WorldActions {
		step, err := rules.WorldAction(fixture.Action)
		if err != nil {
			t.Fatal(err)
		}
		if step.EditCode != (fixture.Stop == "edit") || step.ShowOpponent != (fixture.Stop == "opponent") || step.Proceed != (fixture.Stop == "proceed") || step.Cancel != (fixture.Stop == "cancel") {
			t.Fatal("native world action differs")
		}
		if step.LoadWorld {
			t.Fatal("world action fabricated an actual load")
		}
	}
	for _, fixture := range fixture.SpellHits {
		offset, hit := NativeWorldSpellHit(fixture.X, fixture.Y)
		if hit != fixture.Hit || hit && offset != fixture.Offset {
			t.Fatalf("native spell hit%d,%d differs", fixture.X, fixture.Y)
		}
	}
	for slot := 0; slot < 36; slot++ {
		frame, err := rules.WorldIcon(slot, testBundle(t).Landscapes[0].Palettes[0])
		if err != nil || frame.Bounds().Dx() != 32 || frame.Bounds().Dy() != int(rules.icons[slot].Height) {
			t.Fatal("original embedded menu icon cannot be decoded")
		}
	}
}
