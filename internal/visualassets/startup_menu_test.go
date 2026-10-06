package visualassets

import (
	"encoding/json"
	"testing"
	"testing/fstest"
)

func TestStartupMenuRejectsUnknownActionsAndUnboundedRegions(t *testing.T) {
	menu := StartupMenu{X: 80, Y: 88, Text: "ABC"}
	for i, action := range []StartupAction{StartupProfile, StartupConquest, StartupCustom, StartupLoad, StartupQuit} {
		menu.Regions = append(menu.Regions, StartupHitRegion{Action: action, X: 80, Y: 88 + i*8, Width: 152, Height: 8})
	}
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
