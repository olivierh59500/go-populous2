package app

import (
	"image"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"go-populous2/internal/engine"
)

func TestMobileFrontTargetsFitLandscapeAndPortraitWithoutOverlap(t *testing.T) {
	g := menuTestGame(t)
	g.mobile = &MobileState{Width: 540, Height: 240}
	g.Profile = engine.NewDeity("OLYMPIAN")
	g.SaveBrowser = &SaveBrowser{Files: []string{"one.json", "two.json", "three.json", "four.json", "five.json", "six.json", "seven.json"}, Name: "one.json"}
	g.Options = &OptionsState{Draft: g.Assets.Levels[0]}
	g.Editor = &EditorState{Draft: g.World, Population: 100}
	for _, width := range []int{320, 540, 960} {
		g.mobile.Width = width
		for _, screen := range []Screen{MainMenu, ConquestBriefing, DeityProfile, OptionsScreen, SaveBrowserScreen, NetworkSetup, InGameMenuScreen, HelpScreen, PowerHelpScreen, CampaignResult, EndingScreen, AboutScreen, EditorScreen} {
			g.Screen = screen
			for _, page := range []int{0, 1, 2} {
				g.mobile.FrontPage = page
				for _, overlay := range []string{"", "native", "spells"} {
					g.mobile.Overlay = overlay
					assertMobileFrontTargets(t, g, width, screen)
				}
			}
		}
		g.mobile.TextField = "profile-name"
		assertMobileFrontTargets(t, g, width, DeityProfile)
		g.mobile.TextField = ""
	}
}

func assertMobileFrontTargets(t *testing.T, g *Game, width int, screen Screen) {
	t.Helper()
	buttons := g.mobileFrontButtons()
	if len(buttons) == 0 {
		t.Fatalf("screen %d has no touch actions", screen)
	}
	bounds := image.Rect(0, 0, width, 240)
	for i, b := range buttons {
		if b.Rect.Dx() < 25 || b.Rect.Dy() < 30 || !b.Rect.In(bounds) || b.Action == "" {
			t.Fatalf("screen %d width %d has unusable target %#v", screen, width, b)
		}
		for _, previous := range buttons[:i] {
			if !b.Rect.Intersect(previous.Rect).Empty() {
				t.Fatalf("screen %d width %d overlaps %s and %s", screen, width, b.Action, previous.Action)
			}
		}
		selected, ok := mobileButtonAt(buttons, b.Rect.Min.X+b.Rect.Dx()/2, b.Rect.Min.Y+b.Rect.Dy()/2)
		if !ok || selected.Action != b.Action {
			t.Fatalf("rendered %s target does not hit its action", b.Action)
		}
	}
}

func TestMobileProfileKeyboardUsesExistingInsertionAndPasswordCodec(t *testing.T) {
	g := &Game{Screen: DeityProfile, Profile: engine.NewDeity("BLUE"), mobile: &MobileState{Width: 540, Height: 240}}
	if err := g.handleMobileFront("profile-name"); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"clear", "key-a", "key-T", "key-H", "key-E", "key-N", "key-X", "backspace", "key-A", "accept"} {
		if err := g.handleMobileFront(action); err != nil {
			t.Fatal(err)
		}
	}
	if g.Profile.Name != "ATHENA" || g.mobile.TextField != "" || g.editingProfileName {
		t.Fatal("touch input did not share the deity name editor", g.Profile.Name)
	}
	g.Profile.Bolts = 1
	if err := g.handleMobileFront("experience-5"); err != nil {
		t.Fatal(err)
	}
	if g.Profile.Experience[engine.Water] != 1 || g.Profile.Bolts != 0 {
		t.Fatal("mobile experience allocation bypassed the profile rules")
	}
	_ = g.handleMobileFront("experience-5")
	if g.Profile.Experience[engine.Water] != 1 {
		t.Fatal("touch allocation created an unearned experience point")
	}
	_ = g.handleMobileFront("face-1-prev")
	if g.Profile.FaceParts[1] != 7 {
		t.Fatal("face selection did not use the original wraparound")
	}
	want := engine.NewDeity("IGNORED")
	want.Experience = [6]uint8{2, 4, 6, 8, 10, 12}
	want.FaceParts = [3]uint8{1, 3, 5}
	code, err := want.Password()
	if err != nil {
		t.Fatal(err)
	}
	_ = g.handleMobileFront("profile-code")
	for _, char := range code {
		_ = g.handleMobileFront("key-" + string(char))
	}
	if err := g.handleMobileFront("accept"); err != nil {
		t.Fatal(err)
	}
	if g.Profile.Name != "ATHENA" || g.Profile.Experience != want.Experience || g.Profile.FaceParts != want.FaceParts || g.Profile.Bolts != want.Bolts {
		t.Fatal("touch password did not round-trip the original profile format")
	}
}

func TestMobileWorldNavigationAndCodeDoNotChangeTheLiveSimulation(t *testing.T) {
	g := menuTestGame(t)
	g.mobile = &MobileState{Width: 540, Height: 240}
	g.Screen = ConquestBriefing
	g.Assets.Levels = make([]engine.Level, 35)
	for i := range g.Assets.Levels {
		g.Assets.Levels[i].Code = engine.CodeForLevel(i)
	}
	before := g.World.Snapshot()
	for _, action := range []string{"world-plus-10", "world-plus-10", "world-plus-10", "world-plus-10"} {
		_ = g.handleMobileFront(action)
	}
	if g.LevelIndex != 34 {
		t.Fatal("world navigation did not clamp to the imported campaign")
	}
	_ = g.handleMobileFront("world-code")
	for _, char := range engine.CodeForLevel(7) {
		_ = g.handleMobileFront("key-" + string(char))
	}
	if err := g.handleMobileFront("accept"); err != nil || g.LevelIndex != 7 || g.mobile.TextField != "" {
		t.Fatal("valid world code did not select its campaign world", err)
	}
	_ = g.handleMobileFront("world-code")
	_ = g.handleMobileFront("key-2")
	_ = g.handleMobileFront("key-Q")
	if g.worldCodeInput != "Q" {
		t.Fatal("world-code keyboard accepted non-letter characters")
	}
	if err := g.handleMobileFront("accept"); err == nil || g.mobile.TextField == "" || g.LevelIndex != 7 {
		t.Fatal("invalid world code dismissed the editor or changed the world")
	}
	_ = g.handleMobileFront("cancel-keyboard")
	if g.editingWorldCode || g.mobile.TextField != "" || g.World.Snapshot() != before {
		t.Fatal("touch briefing navigation mutated live gameplay")
	}
}

func TestMobileOptionsKeepLockedCampaignRulesAndApplyAudioExplicitly(t *testing.T) {
	g := menuTestGame(t)
	g.mobile = &MobileState{Width: 540, Height: 240}
	g.Screen = Playing
	if err := g.openOptions(); err != nil {
		t.Fatal(err)
	}
	before := g.World.Snapshot()
	rules := g.Options.Draft.Players[0].Scenario
	_ = g.handleMobileFront("rule-0")
	_ = g.handleMobileFront("reaction-increase")
	if !g.Options.RulesLocked || g.Options.Draft.Players[0].Scenario != rules || g.Options.Draft.Players[0].ReactionDelay != before.World.Level.Players[0].ReactionDelay {
		t.Fatal("mobile options changed locked conquest rules")
	}
	_ = g.handleMobileFront("sound")
	_ = g.handleMobileFront("special-code")
	for _, char := range "MUSIC" {
		_ = g.handleMobileFront("key-" + string(char))
	}
	_ = g.handleMobileFront("accept")
	if g.Options.Music || g.Options.Sound {
		t.Fatal("audio draft and special code did not share the existing option model")
	}
	_ = g.handleMobileFront("back")
	if g.Screen != Playing || g.Options != nil || g.World.Snapshot() != before {
		t.Fatal("cancelling mobile options committed the draft")
	}
}

func TestMobileSaveSelectionKeyboardAndConfirmationReuseBrowser(t *testing.T) {
	g := &Game{Screen: SaveBrowserScreen, mobile: &MobileState{Width: 540, Height: 240}, SaveBrowser: &SaveBrowser{Files: []string{"a.json", "b.json", "c.json", "d.json", "e.json", "f.json", "g.json", "h.json"}, Name: "a.json", Return: InGameMenuScreen}}
	_ = g.handleMobileFront("file-page-down")
	_ = g.handleMobileFront("file-row-5")
	if g.SaveBrowser.Offset != 2 || g.SaveBrowser.Name != "h.json" {
		t.Fatal("touch paging selected the wrong save")
	}
	_ = g.handleMobileFront("save-name")
	for _, action := range []string{"clear", "key-M", "key-Y", "key-.", "key-J", "key-S", "key-O", "key-N", "accept"} {
		_ = g.handleMobileFront(action)
	}
	if g.SaveBrowser.Name != "MY.JSON" || g.mobile.TextField != "" {
		t.Fatal("save keyboard did not retain the chosen filename")
	}
	g.SaveBrowser.Confirm, g.SaveBrowser.Error = true, "Replace the existing save?"
	buttons := g.mobileFrontButtons()
	foundReplace := false
	for _, b := range buttons {
		foundReplace = foundReplace || b.Action == "file-replace" && b.Enabled
	}
	if !foundReplace {
		t.Fatal("overwrite confirmation lacks an explicit replacement action")
	}
	_ = g.handleMobileFront("back")
	if g.Screen != SaveBrowserScreen || g.SaveBrowser.Confirm || g.SaveBrowser.Error != "" {
		t.Fatal("first Back did not cancel only the overwrite modal")
	}
	_ = g.handleMobileFront("back")
	if g.Screen != InGameMenuScreen || g.SaveBrowser != nil {
		t.Fatal("file browser did not return to its caller")
	}
}

func TestMobileHelpLibraryCoversEveryPowerWithoutChangingWorld(t *testing.T) {
	g := menuTestGame(t)
	g.mobile = &MobileState{Width: 540, Height: 240, Overlay: "spells"}
	g.Screen, g.helpReturn = HelpScreen, MainMenu
	before := g.World.Snapshot()
	seen := map[string]bool{}
	for page := 0; page < 3; page++ {
		g.mobile.FrontPage = page
		for _, b := range g.mobileFrontButtons() {
			if strings.HasPrefix(b.Action, "preview-") {
				seen[b.Action] = b.Enabled
			}
		}
	}
	if len(seen) != len(engine.Powers) {
		t.Fatal("not all original powers are reachable in the touch help library", len(seen))
	}
	for _, p := range engine.Powers {
		if !seen["preview-"+strconv.Itoa(int(p.ID))] {
			t.Fatal("power is missing from touch help", p.Name)
		}
	}
	if !reflect.DeepEqual(g.World.Snapshot(), before) {
		t.Fatal("browsing touch spell descriptions changed live gameplay")
	}
}
