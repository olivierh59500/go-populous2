package visualassets

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

const originalFrenchStartup = "x CREER VOTRE DIEU \nx JEU DE CONQUETE  \nx JEU PERSONNALISE \nx CHARGER JEU      \nx QUITTER VERS DOS "
const englishStartup = "x CREATE YOUR DEITY\nx CONQUEST GAME    \nx CUSTOM GAME      \nx LOAD GAME        \nx QUIT             "

func startupMenuFixture() StartupMenu {
	menu := StartupMenu{X: 80, Y: 88, Text: originalFrenchStartup}
	for i, action := range []StartupAction{StartupProfile, StartupConquest, StartupCustom, StartupLoad, StartupQuit} {
		menu.Regions = append(menu.Regions, StartupHitRegion{Action: action, X: 80, Y: 88 + i*8, Width: 152, Height: 8})
	}
	return menu
}

func TestOlderStartupExportUsesEnglishWithoutMovingActions(t *testing.T) {
	original := startupMenuFixture()
	original.Palette[3].R = 127
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	got, err := LoadStartupMenu(fstest.MapFS{StartupMenuFile: &fstest.MapFile{Data: data}})
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != englishStartup {
		t.Fatalf("startup actions are not English: %q", got.Text)
	}
	if got.X != original.X || got.Y != original.Y || got.Palette != original.Palette || !reflect.DeepEqual(got.Regions, original.Regions) {
		t.Fatal("English labels changed the original presentation geometry")
	}
	for i, row := range strings.Split(got.Text, "\n") {
		if len(row) != 19 || got.ActionAt(88, 92+i*8) != original.Regions[i].Action {
			t.Fatal("English label changed its cells or action binding", i)
		}
	}
	if err := got.UseEnglishLabels(); err != nil || got.Text != englishStartup {
		t.Fatal("English label normalization is not stable", err)
	}
}

func TestEnglishStartupLabelsRejectTruncationAtomically(t *testing.T) {
	menu := startupMenuFixture()
	menu.Regions[0].Width = 144
	original := menu.Text
	if err := menu.UseEnglishLabels(); err == nil || menu.Text != original {
		t.Fatal("a truncated English action was accepted or partially applied")
	}
}

func TestStartupMenuRejectsUnknownActionsAndUnboundedRegions(t *testing.T) {
	menu := startupMenuFixture()
	load := func(m StartupMenu) error {
		b, _ := json.Marshal(m)
		_, err := LoadStartupMenu(fstest.MapFS{StartupMenuFile: &fstest.MapFile{Data: b}})
		return err
	}
	if err := load(menu); err != nil {
		t.Fatal(err)
	}
	menu.Regions[0].Action = "program-offset"
	if load(menu) == nil {
		t.Fatal("unknown action admitted")
	}
	menu.Regions[0].Action = StartupProfile
	menu.Regions[0].Width = 999
	if load(menu) == nil {
		t.Fatal("outside hit rectangle admitted")
	}
}
