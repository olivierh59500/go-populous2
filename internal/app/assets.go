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
	AboutLayout                                  *visualassets.RequesterLayout
	EditorLayout                                 *visualassets.RequesterLayout
	EditorPreview                                map[string]visualassets.Frame
	NetworkLayout                                *visualassets.RequesterLayout
	SpellHelp                                    *visualassets.SpellHelpArt
	Levels                                       []engine.Level
	Landscapes                                   [4]engine.Landscape
	Music                                        *music.Bank
	RulesID                                      string
}

// A playable build must carry the restored interface package. Older exports
// remain readable for reference tools, but must not silently select substitute
// controls in the ordinary launcher.
func (a *Assets) requireOriginalInterface() error {
	for _, item := range []struct {
		name    string
		present bool
	}{
		{"startup-menu.json", a.StartupMenu != nil},
		{"pointers.json", a.Pointers != nil},
		{"selected-panel.json", a.SelectionPanel != nil},
		{"deity-layout.json", a.DeityLayout != nil},
		{"deity-widgets.json", a.DeityWidgets != nil},
		{"hud.json", a.HUD != nil},
		{"conquest.json", a.Conquest != nil},
		{"options.json", a.OptionsArt != nil},
		{"result-layout.json", a.Result != nil},
		{"in-game-layout.json", a.InGameLayout != nil},
		{"about-layout.json", a.AboutLayout != nil},
		{"editor-layout.json", a.EditorLayout != nil},
		{"editor-preview.json", a.EditorPreview != nil},
		{"network-layout.json", a.NetworkLayout != nil},
		{"spell-help.json", a.SpellHelp != nil},
		{"files-layout.json", a.FileLayout != nil},
		{"overwrite-layout.json", a.OverwriteLayout != nil},
		{"file-error-layout.json", a.FileErrorLayout != nil},
	} {
		if !item.present {
			return fmt.Errorf("original interface asset %s is missing; regenerate assets with cmd/export-visual-assets", item.name)
		}
	}
	return nil
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
	bundle.AboutLayout, err = visualassets.LoadRequesterLayout(files, "about-layout.json")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("about window: %w", err)
	}
	bundle.EditorLayout, err = visualassets.LoadRequesterLayout(files, "editor-layout.json")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("paint window: %w", err)
	}
	bundle.EditorPreview, err = visualassets.LoadEditorPreview(files)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("editor preview: %w", err)
	}
	bundle.NetworkLayout, err = visualassets.LoadRequesterLayout(files, "network-layout.json")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("network window: %w", err)
	}
	bundle.SpellHelp, err = visualassets.LoadSpellHelpArt(files)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("spell help: %w", err)
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
	}{fmt.Sprintf("go-engine-7-snapshot-%d-scenarios-%d", engine.SnapshotVersion, engine.ScenarioEventCapacity), bundle.Levels, bundle.Landscapes, engine.Powers})
	if err != nil {
		return nil, err
	}
	bundle.RulesID = fmt.Sprintf("%x", sha256.Sum256(rules))
	return bundle, nil
}
