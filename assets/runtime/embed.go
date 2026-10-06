// Package runtimeassets embeds locally exported presentation and game data.
// It contains no original executable or reference-machine state.
package runtimeassets

import (
	"embed"
	"io/fs"
)

//go:embed data
var files embed.FS

// FS returns the imported asset root embedded in the current build. A clean
// source checkout contains only a placeholder, so export assets before use.
func FS() (fs.FS, error) { return fs.Sub(files, "data") }
