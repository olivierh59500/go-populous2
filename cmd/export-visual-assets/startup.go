package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func exportStartupMenu(output string, p *populous2.NativePresentation) error {
	r := p.StartupRequester
	menu := visualassets.StartupMenu{X: r.Column * 8, Y: r.Row, Text: string(r.Text), Palette: p.StartupPalette}
	actions := map[int]visualassets.StartupAction{2: visualassets.StartupProfile, 4: visualassets.StartupConquest, 6: visualassets.StartupCustom, 8: visualassets.StartupLoad, 10: visualassets.StartupQuit}
	// Evaluate the decoded source requester offline, merging horizontal runs.
	// Runtime hit testing sees only bounded rectangles and named Go actions.
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; {
			command := p.Requesters.Click(r, x, y)
			if command == 0 {
				x++
				continue
			}
			action, ok := actions[command]
			if !ok {
				return fmt.Errorf("unexpected visible startup action%d", command)
			}
			start := x
			for x < 320 && p.Requesters.Click(r, x, y) == command {
				x++
			}
			width := x - start
			merged := false
			for i := range menu.Regions {
				previous := &menu.Regions[i]
				if previous.Action == action && previous.X == start && previous.Width == width && previous.Y+previous.Height == y {
					previous.Height++
					merged = true
					break
				}
			}
			if !merged {
				menu.Regions = append(menu.Regions, visualassets.StartupHitRegion{Action: action, X: start, Y: y, Width: width, Height: 1})
			}
		}
	}
	if err := menu.UseEnglishLabels(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(menu, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(output, visualassets.StartupMenuFile), append(data, '\n'), 0644)
}
