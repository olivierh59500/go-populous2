package visualassets

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io"
	"io/fs"
)

const StartupMenuFile = "startup-menu.json"

type StartupAction string

const (
	StartupProfile  StartupAction = "profile"
	StartupConquest StartupAction = "conquest"
	StartupCustom   StartupAction = "custom"
	StartupLoad     StartupAction = "load"
	StartupQuit     StartupAction = "quit"
)

type StartupHitRegion struct {
	Action              StartupAction `json:"action"`
	X, Y, Width, Height int
}

// StartupMenu contains decoded glyph cells and named hit regions. It carries
// no original template grammar, instruction offsets or dispatch addresses.
type StartupMenu struct {
	X, Y    int
	Text    string             `json:"text"`
	Palette [16]color.RGBA     `json:"palette"`
	Regions []StartupHitRegion `json:"regions"`
}

func (m *StartupMenu) ActionAt(x, y int) StartupAction {
	if m == nil {
		return ""
	}
	for _, r := range m.Regions {
		if image.Pt(x, y).In(image.Rect(r.X, r.Y, r.X+r.Width, r.Y+r.Height)) {
			return r.Action
		}
	}
	return ""
}

func (m *StartupMenu) Draw(dst *image.RGBA, font *Font) {
	if m == nil || font == nil {
		return
	}
	font.Draw(dst, m.Text, m.X, m.Y, m.Palette)
}

func LoadStartupMenu(files fs.FS) (*StartupMenu, error) {
	data, err := readLimited(files, StartupMenuFile, 16384)
	if err != nil {
		return nil, err
	}
	var m StartupMenu
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&m); err != nil {
		return nil, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("startup menu has trailing data")
	}
	if m.X < 0 || m.X >= 320 || m.Y < 0 || m.Y >= 200 || len(m.Text) > 2048 || len(m.Regions) > 64 {
		return nil, fmt.Errorf("startup menu dimensions exceed supported range")
	}
	for _, glyph := range []byte(m.Text) {
		if glyph != '\n' && (glyph < 32 || glyph > 127) {
			return nil, fmt.Errorf("startup menu contains an unsupported glyph")
		}
	}
	seen := map[StartupAction]bool{}
	for _, r := range m.Regions {
		if r.Action != StartupProfile && r.Action != StartupConquest && r.Action != StartupCustom && r.Action != StartupLoad && r.Action != StartupQuit {
			return nil, fmt.Errorf("unknown startup action")
		}
		if r.Width <= 0 || r.Height <= 0 || r.X < 0 || r.Y < 0 || r.X+r.Width > 320 || r.Y+r.Height > 200 {
			return nil, fmt.Errorf("invalid startup hit region")
		}
		seen[r.Action] = true
	}
	if len(seen) != 5 {
		return nil, fmt.Errorf("startup menu does not expose its five actions")
	}
	return &m, nil
}
