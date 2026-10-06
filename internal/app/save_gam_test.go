package app

import (
	"os"
	"path/filepath"
	"testing"

	"go-populous2/internal/engine"
)

func TestPrivateOriginalSaveLoadsThroughTheIndependentInterface(t *testing.T) {
	assetPath := os.Getenv("POPULOUS2_GENERATED_ASSETS_TEST_DIR")
	savePath := os.Getenv("POPULOUS2_GAM_TEST_FILE")
	if assetPath == "" || savePath == "" {
		t.Skip("provide private generated assets and an original GAM save")
	}
	assets, err := LoadAssets(os.DirFS(assetPath))
	if err != nil {
		t.Fatal(err)
	}
	g := &Game{Assets: assets, SavePath: savePath}
	if err := g.loadGame(); err != nil {
		t.Fatal(err)
	}
	if g.Screen != Playing || g.LevelIndex != 27 || g.Profile.Name != "DAMOCLES" || g.LocalSide != 0 || g.CustomGame || g.CameraX != 8 || g.CameraY != 4 {
		t.Fatal("original game did not restore its session", g.Screen, g.LevelIndex, g.Profile, g.LocalSide, g.CustomGame, g.CameraX, g.CameraY)
	}
	for range 10 {
		g.World.Step()
	}
	g.SavePath = filepath.Join(t.TempDir(), "continued.GAM")
	if err := g.saveGame(); err != nil {
		t.Fatal(err)
	}
	tick, mana := g.World.Tick, g.World.Players[0].Mana
	if err := g.loadGame(); err != nil {
		t.Fatal(err)
	}
	if g.World.Tick != tick || g.World.Players[0].Mana != mana {
		t.Fatal("continued original save lost simulation state")
	}
	g.SavePath = filepath.Join(t.TempDir(), "continued.json")
	g.Selected = engine.Basalt
	if err := g.saveGame(); err != nil {
		t.Fatal(err)
	}
	g.OriginalSave = nil
	if err := g.loadGame(); err != nil || g.OriginalSave == nil || g.Selected != engine.Basalt {
		t.Fatal("independent session lost original-save interoperability", err)
	}
}

func TestPrivateNewIndependentGameExportsAsOriginalSave(t *testing.T) {
	path := os.Getenv("POPULOUS2_GENERATED_ASSETS_TEST_DIR")
	if path == "" {
		t.Skip("provide private generated assets")
	}
	assets, err := LoadAssets(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	world, err := engine.NewWorld(assets.Levels[0], assets.Landscapes[0])
	if err != nil {
		t.Fatal(err)
	}
	for range 20 {
		world.Step()
	}
	g := &Game{Assets: assets, World: world, Profile: engine.NewDeity("GO PLAYER"), SavePath: filepath.Join(t.TempDir(), "new.GAM"), CameraX: 8, CameraY: 4}
	g.SelectedFollower = world.Players[0].Leader
	g.SelectionReturn = FollowerSelectionReturn{BackupFollower: world.Players[1].Leader, FramesLeft: 23}
	selection, returnState := g.SelectedFollower, g.SelectionReturn
	if err := g.saveGame(); err != nil {
		t.Fatal(err)
	}
	if g.OriginalSave == nil {
		t.Fatal("fresh GAM export lost its file metadata")
	}
	tick := world.Tick
	if err := g.loadGame(); err != nil {
		t.Fatal(err)
	}
	if g.World.Tick != tick || g.Profile.Name != "GO PLAYER" || g.CustomGame || g.CameraX != 8 || g.CameraY != 4 || g.SelectedFollower != selection || g.SelectionReturn != returnState {
		t.Fatal("new original-format save lost its session")
	}
}
