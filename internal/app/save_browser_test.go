package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"go-populous2/internal/engine"
)

func browserGame(t *testing.T) *Game {
	t.Helper()
	return &Game{Assets: &Assets{Levels: make([]engine.Level, 1000)}, World: controllerWorld(t), Screen: Playing, Profile: engine.NewDeity("PLAYER"), SavePath: filepath.Join(t.TempDir(), "game.json"), Selected: engine.RaiseLower}
}

func TestSaveBrowserRequiresConfirmationBeforeReplacingFile(t *testing.T) {
	g := browserGame(t)
	original := []byte("existing file")
	if err := os.WriteFile(g.SavePath, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := g.openSaveBrowser(true); err != nil {
		t.Fatal(err)
	}
	g.acceptSaveBrowser()
	current, err := os.ReadFile(g.SavePath)
	if err != nil || !bytes.Equal(current, original) || !g.SaveBrowser.Confirm {
		t.Fatal("existing file was replaced before confirmation", err)
	}
	g.acceptSaveBrowser()
	if g.SaveBrowser != nil || g.Screen != Playing {
		t.Fatal("confirmed save did not return to the game")
	}
	if err := g.loadGame(); err != nil {
		t.Fatal("confirmed file is not a valid independent save", err)
	}
}

func TestSaveBrowserFailedLoadPreservesLiveWorldAndPath(t *testing.T) {
	g := browserGame(t)
	world, path := g.World, g.SavePath
	if err := os.WriteFile(filepath.Join(filepath.Dir(path), "broken.json"), []byte(`{"version":99}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := g.openSaveBrowser(false); err != nil {
		t.Fatal(err)
	}
	g.SaveBrowser.Name = "broken.json"
	g.acceptSaveBrowser()
	if g.World != world || g.SavePath != path || g.SaveBrowser.Error == "" || g.Screen != SaveBrowserScreen {
		t.Fatal("failed load changed the live session or hid its error")
	}
	g.SaveBrowser.Name = "../outside.json"
	g.acceptSaveBrowser()
	if g.SavePath != path || g.World != world {
		t.Fatal("filename escaped the selected save directory")
	}
	g.closeSaveBrowser()
	if g.Screen != Playing || g.SaveBrowser != nil {
		t.Fatal("cancel did not restore the previous screen")
	}
}

func TestSavedSessionRejectsTrailingDataWithoutMutation(t *testing.T) {
	g := browserGame(t)
	if err := g.saveGame(); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(g.SavePath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.WriteString("{}")
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	world := g.World
	if err := g.loadGame(); err == nil || g.World != world {
		t.Fatal("trailing JSON accepted or changed the live world")
	}
}
