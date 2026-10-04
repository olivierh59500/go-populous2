// Package assets embeds the original Populous II resources extracted from the
// supplied Amiga disk B and the supplied French HD executable.
package assets

import "embed"

//go:embed amiga/*
var Files embed.FS
