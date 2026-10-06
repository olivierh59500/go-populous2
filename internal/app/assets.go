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
	Visual                                       *visualassets.Bundle
	StartupMenu                                  *visualassets.StartupMenu
	Pointers                                     *visualassets.PointerArt
	SelectionPanel                               *visualassets.SelectionPanel
	DeityLayout                                  *visualassets.RequesterLayout
	DeityWidgets                                 *visualassets.DeityWidgets
	HUD                                          *visualassets.HUDArt
	Conquest                                     *visualassets.ConquestArt
	OptionsArt                                   *visualassets.OptionsArt
	Result                                       *visualassets.ResultDescriptor
	FileLayout, OverwriteLayout, FileErrorLayout *visualassets.RequesterLayout
	InGameLayout                                 *visualassets.RequesterLayout
	Levels                                       []engine.Level
	Landscapes                                   [4]engine.Landscape
	Music                                        *music.Bank
	RulesID                                      string
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
	pointers, err := visualassets.LoadPointers(files)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("pointer artwork: %w", err)
	}
	bundle := &Assets{Visual: visual, Levels: levels, StartupMenu: menu, Pointers: pointers}
	panel, err := visualassets.LoadSelectionPanel(files)
	if err != nil {
		return nil, fmt.Errorf("selected group panel: %w", err)
	}
	bundle.SelectionPanel = panel
	bundle.DeityLayout, err = visualassets.LoadRequesterLayout(files, "deity-layout.json")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("deity layout: %w", err)
	}
	bundle.DeityWidgets, err = visualassets.LoadDeityWidgets(files)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("deity artwork: %w", err)
	}
	bundle.HUD, err = visualassets.LoadHUD(files)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("game HUD: %w", err)
	}
	bundle.InGameLayout, err = visualassets.LoadRequesterLayout(files, "in-game-layout.json")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("in-game menu: %w", err)
	}
	bundle.Conquest, err = visualassets.LoadConquestArt(files)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("conquest interface: %w", err)
	}
	bundle.OptionsArt, err = visualassets.LoadOptionsArt(files)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("game options: %w", err)
	}
	bundle.Result, err = visualassets.LoadResult(files)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("campaign result: %w", err)
	}
	for name, target := range map[string]**visualassets.RequesterLayout{"files-layout.json": &bundle.FileLayout, "overwrite-layout.json": &bundle.OverwriteLayout, "file-error-layout.json": &bundle.FileErrorLayout} {
		*target, err = visualassets.LoadRequesterLayout(files, name)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("file interface: %w", err)
		}
	}
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
