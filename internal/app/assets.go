// Package app provides the Ebitengine interface for the independent Go engine.
package app

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"

	"go-populous2/internal/engine"
	"go-populous2/internal/music"
	"go-populous2/internal/visualassets"
)

// Assets contains presentation data and decoded campaign/landscape records.
// It has no original executable, instructions, relocated memory or registers.
type Assets struct {
	Visual      *visualassets.Bundle
	StartupMenu *visualassets.StartupMenu
	Levels      []engine.Level
	Landscapes  [4]engine.Landscape
	Music       *music.Bank
	RulesID     string
}

func LoadAssets(files fs.FS) (*Assets, error) {
	visual, err := visualassets.LoadFS(files)
	if err != nil {
		return nil, fmt.Errorf("visual assets: %w", err)
	}
	campaign, err := fs.ReadFile(files, "campaign.dat")
	if err != nil {
		return nil, err
	}
	levels, err := engine.DecodeCampaign(campaign)
	if err != nil {
		return nil, err
	}
	menu, err := visualassets.LoadStartupMenu(files)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("startup menu: %w", err)
	}
	bundle := &Assets{Visual: visual, Levels: levels, StartupMenu: menu}
	for index := range bundle.Landscapes {
		data, err := fs.ReadFile(files, fmt.Sprintf("land%d.dat", index))
		if err != nil {
			return nil, err
		}
		land, err := engine.DecodeLandscape(data)
		if err != nil {
			return nil, err
		}
		bundle.Landscapes[index] = land
	}
	bank, err := music.LoadFS(files, "audio/score.json")
	if err != nil {
		return nil, fmt.Errorf("audio assets: %w", err)
	}
	bundle.Music = bank
	rules, err := json.Marshal(struct {
		Version    string
		Levels     []engine.Level
		Landscapes [4]engine.Landscape
		Powers     []engine.Power
	}{fmt.Sprintf("go-engine-4-snapshot-%d", engine.SnapshotVersion), bundle.Levels, bundle.Landscapes, engine.Powers})
	if err != nil {
		return nil, err
	}
	bundle.RulesID = fmt.Sprintf("%x", sha256.Sum256(rules))
	return bundle, nil
}
