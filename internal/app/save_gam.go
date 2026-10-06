package app

import (
	"fmt"
	"os"
	"path/filepath"

	"go-populous2/internal/engine"
	"go-populous2/internal/gamcodec"
)

func (g *Game) gamCatalog() gamcodec.Catalog {
	result := gamcodec.Catalog{Levels: g.Assets.Levels, Landscapes: g.Assets.Landscapes, AnimationRoles: make(map[uint16][]gamcodec.AnimationRole)}
	if visual := g.Assets.Visual; visual != nil {
		result.Geometry = visual.TileRasters
		if visual.FileCompatibility != nil {
			for _, token := range visual.FileCompatibility.AnimationTokens {
				result.AnimationRoles[token.Token] = append(result.AnimationRoles[token.Token], gamcodec.AnimationRole{Name: token.Animation, Frame: token.Frame})
			}
		}
	}
	return result
}

func (g *Game) loadOriginalGame() error {
	file, err := os.Open(g.SavePath)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() != gamcodec.FileSize {
		return fmt.Errorf("original GAM save must contain %d bytes", gamcodec.FileSize)
	}
	data := make([]byte, gamcodec.FileSize)
	if _, err := file.ReadAt(data, 0); err != nil {
		return err
	}
	document, err := gamcodec.Decode(data, g.gamCatalog())
	if err != nil {
		return err
	}
	meta := document.Metadata
	if meta.CameraX < 0 || meta.CameraX > 56 || meta.CameraY < 0 || meta.CameraY > 56 {
		return fmt.Errorf("original GAM camera is outside the game map")
	}
	g.World, g.Profile, g.OriginalSave = document.World, meta.Profile, document
	g.LocalSide, g.LevelIndex = meta.ProfileSide, document.World.Level.Number
	g.CameraX, g.CameraY = meta.CameraX, meta.CameraY
	g.CustomGame = meta.GameMode != 2
	g.restoreCustomSetup()
	g.Paused, g.resultApplied, g.Screen = false, false, Playing
	g.Selected, g.Direction = engine.RaiseLower, 0
	g.SelectedFollower, g.Inspecting = 0, false
	g.selectionTransferTick = g.World.Tick
	g.SelectionReturn = FollowerSelectionReturn{}
	g.AnimationSounds = AnimationSoundGate{}
	g.finishWorld()
	g.Message, g.messageUntil = "ORIGINAL GAME LOADED", g.Updates+100
	return nil
}

func (g *Game) saveOriginalGame() error {
	original := g.OriginalSave
	if original == nil {
		mode := 2
		if g.CustomGame {
			mode = 4
		}
		var err error
		original, err = gamcodec.NewDocument(g.World, g.gamCatalog(), g.Profile, g.playerSide(), mode, g.CameraX, g.CameraY)
		if err != nil {
			return err
		}
	}
	document := *original
	document.World = g.World
	document.Metadata.Profile, document.Metadata.ProfileSide = g.Profile, g.playerSide()
	document.Metadata.CameraX, document.Metadata.CameraY = g.CameraX, g.CameraY
	if g.CustomGame {
		document.Metadata.GameMode = 4
	} else {
		document.Metadata.GameMode = 2
	}
	data, err := gamcodec.Encode(&document)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(g.SavePath), ".populous2-gam-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	_, writeErr := file.Write(data)
	syncErr, closeErr := file.Sync(), file.Close()
	if writeErr != nil {
		return writeErr
	}
	if syncErr != nil {
		return syncErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(temporary, g.SavePath); err != nil {
		return err
	}
	g.OriginalSave = &document
	g.Message, g.messageUntil = "ORIGINAL GAME SAVED", g.Updates+100
	return nil
}
