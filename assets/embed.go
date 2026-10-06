// Package assets embeds locally imported game data at build time. The source
// repository contains a placeholder only; original assets are not distributed.
package assets

import (
	"embed"
	"io/fs"
	"os"
)

//go:embed amiga/*
var Files embed.FS

// DataFS selects an external installation when configured, otherwise the
// locally imported resources embedded by the current build.
func DataFS() (fs.FS, error) {
	if dir := os.Getenv("POPULOUS2_DATA_DIR"); dir != "" {
		return os.DirFS(dir), nil
	}
	return fs.Sub(Files, "amiga")
}
