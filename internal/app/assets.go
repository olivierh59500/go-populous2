// Package app provides the Ebitengine interface for the independent Go engine.
package app

import (
	"fmt"
	"io/fs"

	"go-populous2/internal/engine"
	"go-populous2/internal/music"
	"go-populous2/internal/visualassets"
)

// Assets contains presentation data and decoded campaign/landscape records.
// It has no original executable, instructions, relocated memory or registers.
type Assets struct {
	Visual     *visualassets.Bundle
	Levels     []engine.Level
	Landscapes [4]engine.Landscape
	Music      *music.Bank
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
	bundle := &Assets{Visual: visual, Levels: levels}
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
	return bundle, nil
}
