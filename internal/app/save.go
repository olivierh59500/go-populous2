package app

import (
	"encoding/json"
	"fmt"
	"go-populous2/internal/engine"
	"os"
	"path/filepath"
)

type savedSession struct {
	Version          int             `json:"version"`
	World            engine.Snapshot `json:"world"`
	Profile          engine.Deity    `json:"profile"`
	CameraX, CameraY int
	LevelIndex       int
	Selected         engine.PowerID
	Direction        uint8
}

func (g *Game) saveGame() error {
	if g.Network != nil {
		return fmt.Errorf("save after leaving the two-player session")
	}
	if g.World == nil || g.SavePath == "" {
		return fmt.Errorf("no save location or game")
	}
	session := savedSession{Version: 1, World: g.World.Snapshot(), Profile: g.Profile, CameraX: g.CameraX, CameraY: g.CameraY, LevelIndex: g.LevelIndex, Selected: g.Selected, Direction: g.Direction}
	if _, err := session.World.Restore(); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(g.SavePath), ".populous2-save-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	encoder := json.NewEncoder(file)
	writeErr := encoder.Encode(session)
	syncErr := file.Sync()
	closeErr := file.Close()
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
	g.Message = "GAME SAVED"
	g.messageUntil = g.Updates + 100
	return nil
}

func (g *Game) loadGame() error {
	if g.Network != nil {
		return fmt.Errorf("load after leaving the two-player session")
	}
	if g.SavePath == "" {
		return fmt.Errorf("no save location")
	}
	file, err := os.Open(g.SavePath)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var session savedSession
	if err := decoder.Decode(&session); err != nil {
		return err
	}
	if session.Version != 1 || session.LevelIndex < 0 || session.LevelIndex >= len(g.Assets.Levels) || session.CameraX < 0 || session.CameraX > 56 || session.CameraY < 0 || session.CameraY > 56 || session.Direction > 3 {
		return fmt.Errorf("unsupported or invalid saved session")
	}
	world, err := session.World.Restore()
	if err != nil {
		return err
	}
	g.World, g.Profile, g.CameraX, g.CameraY, g.LevelIndex, g.Selected, g.Direction = world, session.Profile, session.CameraX, session.CameraY, session.LevelIndex, session.Selected, session.Direction
	g.Screen, g.resultApplied = Playing, false
	g.AnimationSounds = AnimationSoundGate{}
	g.Message = "GAME LOADED"
	g.messageUntil = g.Updates + 100
	return nil
}
