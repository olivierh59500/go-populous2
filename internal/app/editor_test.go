package app

import (
	"image"
	"image/png"
	"os"
	"testing"

	"go-populous2/internal/engine"
	"go-populous2/internal/music"
)

func TestEditorPaintAndCancelKeepLiveWorldAndCamera(t *testing.T) {
	g := menuTestGame(t)
	g.CameraX, g.CameraY = 12, 13
	before := g.World.Snapshot()
	if err := g.openEditor(); err != nil {
		t.Fatal(err)
	}
	g.Editor.Tool = EditorLevel
	g.Editor.Height = 4
	if err := g.Editor.paint(32, 32); err != nil {
		t.Fatal(err)
	}
	if g.World.Heights[32+32*engine.CornerSize] != before.World.Heights[32+32*engine.CornerSize] {
		t.Fatal("editor paint changed live world")
	}
	g.CameraX, g.CameraY = 40, 41
	g.cancelEditor()
	if g.World.Snapshot() != before || g.CameraX != 12 || g.CameraY != 13 || g.Screen != Playing {
		t.Fatal("editor cancel lost continuation or camera")
	}
}

func TestEditorApplyAdoptsValidatedDetachedTerrainAndObjects(t *testing.T) {
	g := menuTestGame(t)
	if err := g.openEditor(); err != nil {
		t.Fatal(err)
	}
	live := g.World
	g.Editor.Tool = EditorLevel
	g.Editor.Height = 3
	if err := g.Editor.paint(32, 32); err != nil {
		t.Fatal(err)
	}
	g.Editor.Tool = EditorRed
	g.Editor.Population = 250
	if err := g.Editor.paint(32, 32); err != nil {
		t.Fatal(err)
	}
	if err := g.applyEditor(); err != nil {
		t.Fatal(err)
	}
	if g.World == live || g.World.Editor || g.Screen != Playing || g.World.Heights[32+32*engine.CornerSize] != 3 || g.CustomLevel == nil {
		t.Fatal("editor apply did not adopt validated detached world")
	}
	var ids [engine.FollowerCapacity]int
	count := g.World.FollowersAt(32, 32, ids[:])
	found := false
	for _, id := range ids[:count] {
		f := g.World.Followers[id]
		if f.Owner == 1 && f.Population == 250 {
			found = true
		}
	}
	if !found {
		t.Fatal("edited red group did not reach play")
	}
}

func TestOriginalEditorManaRadiosAndEventFieldsStayInsideDraft(t *testing.T) {
	g := menuTestGame(t)
	before := g.World.Snapshot()
	if err := g.openEditor(); err != nil {
		t.Fatal(err)
	}
	if err := g.applyEditorAction("blue"); err != nil || g.Editor.Tool != EditorBlue {
		t.Fatal("original blue radio did not select brush", err)
	}
	if err := g.applyEditorAction("blue"); err != nil || g.Editor.Tool != EditorRaise {
		t.Fatal("selected radio did not toggle back to terrain", err)
	}
	mana := g.Editor.Draft.Players[0].Mana
	if err := g.applyEditorAction("local-mana-add"); err != nil || g.Editor.Draft.Players[0].Mana != mana+8000 {
		t.Fatal("original mana increment differs", err)
	}
	for field, text := range map[string]string{"time": "200", "x": "31", "y": "32", "effect": "40"} {
		g.Editor.EditingField, g.Editor.NumberInput = field, text
		if err := g.Editor.applyNumber(); err != nil {
			t.Fatal(err)
		}
	}
	event := g.Editor.Draft.Scenario.Events[0]
	if event.Time != 200 || event.X != 31 || event.Y != 32 || event.Kind != engine.ScenarioEarthquake {
		t.Fatal("original event fields did not reach typed scenario", event)
	}
	snapshot := g.Editor.Draft.Snapshot()
	g.Editor.EditingField, g.Editor.NumberInput = "effect", "124"
	if err := g.Editor.applyNumber(); err == nil || g.Editor.Draft.Snapshot() != snapshot {
		t.Fatal("unsupported event partially changed editor map")
	}
	if g.World.Snapshot() != before {
		t.Fatal("original editor controls changed live world")
	}
}

func TestPrivateOriginalEditorApplicationFrame(t *testing.T) {
	path := os.Getenv("POPULOUS2_GENERATED_ASSETS_TEST_DIR")
	if path == "" {
		t.Skip("set portable original editor assets")
	}
	assets, err := LoadAssets(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	world, err := engine.NewWorld(assets.Levels[0], assets.Landscapes[assets.Levels[0].Landscape])
	if err != nil {
		t.Fatal(err)
	}
	g := &Game{Assets: assets, World: world, Screen: Playing, CameraX: 28, CameraY: 28, framebuffer: image.NewRGBA(image.Rect(0, 0, 320, 200))}
	if err := g.openEditor(); err != nil {
		t.Fatal(err)
	}
	g.Editor.Tool = EditorBlue
	g.Editor.EventIndex = 49
	if err := g.Editor.Draft.EditorSetScenarioEvent(49, engine.ScenarioEvent{Time: 300, Kind: engine.ScenarioFireColumn, X: 31, Y: 32}); err != nil {
		t.Fatal(err)
	}
	before := g.World.Snapshot()
	g.drawEditor()
	g.drawEditorPointer(188, 119)
	if g.World.Snapshot() != before {
		t.Fatal("original editor presentation mutated live map")
	}
	if capture := os.Getenv("POPULOUS2_EDITOR_CAPTURE"); capture != "" {
		file, err := os.Create(capture)
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(file, g.framebuffer)
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			t.Fatal(err, closeErr)
		}
	}
}

func TestDetachedEditorPhysicsKeepsPreStepFrameAndPausesNumericInput(t *testing.T) {
	g := menuTestGame(t)
	before := g.World.Snapshot()
	if err := g.openEditor(); err != nil {
		t.Fatal(err)
	}
	g.Editor.Draft.Scenario.Events[0] = engine.ScenarioEvent{Time: 1, Kind: engine.ScenarioWhirlwind, X: 32, Y: 32}
	for i := 0; i < 4; i++ {
		g.advanceEditorPresentation()
	}
	if g.Editor.Draft.Tick != 1 || !g.Editor.Presentation.Ready || g.Editor.Presentation.World.Tick != 1 {
		t.Fatal("editor did not use main presentation cadence")
	}
	if !g.Editor.Draft.Air.Whirlwinds[0].Active || g.Editor.Presentation.World.Air.Whirlwinds[0].Active {
		t.Fatal("editor drew post-physics actors instead of source pre-step frame")
	}
	if g.Editor.Draft.Air.Whirlwinds[0].Owner != 1 {
		t.Fatal("paint-mode event did not use original red faction")
	}
	g.Editor.EditingField = "time"
	frozen := g.Editor.Draft.Snapshot()
	for i := 0; i < 20; i++ {
		g.advanceEditorPresentation()
	}
	if g.Editor.Draft.Snapshot() != frozen {
		t.Fatal("numeric editor modal did not suspend simulation")
	}
	if g.World.Snapshot() != before {
		t.Fatal("editor physics changed live game")
	}
	g.cancelEditor()
	if g.World.Snapshot() != before {
		t.Fatal("cancel adopted simulated draft")
	}
}

func TestPrivateEditorCombatCueHasIndependentPersistentSoundGate(t *testing.T) {
	path := os.Getenv("POPULOUS2_ENDING_TEST_DIR")
	if path == "" {
		t.Skip("set portable original artwork/audio directory")
	}
	assets, err := LoadAssets(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	replay, err := music.NewPlayer(assets.Music, 44100)
	if err != nil {
		t.Fatal(err)
	}
	world := &engine.World{Editor: true}
	var heights [engine.CornerSize * engine.CornerSize]uint8
	for id := range heights {
		heights[id] = 1
	}
	if err := world.EditorSetTerrain(heights); err != nil {
		t.Fatal(err)
	}
	if err := world.EditorPlaceFollower(0, 32, 32, 1000); err != nil {
		t.Fatal(err)
	}
	world.Followers[1].State, world.Followers[1].BattleAggressor = engine.Fighting, true
	live := &engine.World{}
	g := &Game{World: live, Assets: assets, Screen: EditorScreen, CameraX: 28, CameraY: 28, music: replay, framebuffer: image.NewRGBA(image.Rect(0, 0, 320, 200)), Editor: &EditorState{Draft: world}}
	g.AnimationSounds.Admit(41, 78)
	before := g.AnimationSounds
	view := *g
	view.World = world
	g.Editor.Presentation.Capture(&view, true)
	cue := assets.Visual.Animations["combat/attack"].Frames[0].SoundCue
	if cue <= 0 {
		t.Fatal("original combat frame has no real cue")
	}
	for repeat := 0; repeat < 4; repeat++ {
		g.drawEditor()
		gate := g.Editor.AnimationSounds
		if !gate.Started || !gate.Played[cue] || gate.Tick != 1 {
			t.Fatal("editor renderer discarded its cue gate", repeat)
		}
		if g.Editor.AnimationSounds.Admit(1, cue) {
			t.Fatal("editor redraw can restart combat sound at50Hz", repeat)
		}
		if g.AnimationSounds != before {
			t.Fatal("editor sound admission polluted live game")
		}
	}
	world.Tick = 1
	g.Editor.Presentation.Reset()
	g.drawEditor()
	if g.Editor.AnimationSounds.Tick != 1 {
		t.Fatal("editing invalidation reset sound admission within a pass")
	}
}
