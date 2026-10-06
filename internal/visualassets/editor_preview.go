package visualassets

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
)

const EditorPreviewFile = "editor-preview.json"

func LoadEditorPreview(files fs.FS) (map[string]Frame, error) {
	f, err := files.Open(EditorPreviewFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, 1<<16))
	decoder.DisallowUnknownFields()
	var frames map[string]Frame
	if err := decoder.Decode(&frames); err != nil {
		return nil, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("editor preview has trailing data")
	}
	if len(frames) != 4 {
		return nil, fmt.Errorf("editor preview brush artwork incomplete")
	}
	for _, name := range []string{"blue", "red", "tree", "rock"} {
		frame, ok := frames[name]
		if !ok || len(frame.Layers) < 1 || len(frame.Layers) > 16 {
			return nil, fmt.Errorf("editor brush preview missing")
		}
		for _, layer := range frame.Layers {
			if layer.Sprite < 0 || layer.Sprite >= 4096 || layer.X < -320 || layer.X > 320 || layer.Y < -200 || layer.Y > 200 {
				return nil, fmt.Errorf("editor brush layer invalid")
			}
		}
	}
	return frames, nil
}
