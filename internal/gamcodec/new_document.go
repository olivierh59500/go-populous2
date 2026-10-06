package gamcodec

import (
	"fmt"

	"go-populous2/internal/engine"
)

// NewDocument constructs a GAM file from named Go game fields. Reserved file
// fields start at zero; no original executable or memory image is consulted.
// Encode then rejects any active state for which the catalog lacks an exact
// artwork token, so fresh files are never accepted through a no-change shortcut.
func NewDocument(world *engine.World, catalog Catalog, profile engine.Deity, side, gameMode, cameraX, cameraY int) (*Document, error) {
	if world == nil {
		return nil, fmt.Errorf("GAM world is missing")
	}
	if _, err := world.Snapshot().Restore(); err != nil {
		return nil, err
	}
	metadata := Metadata{Original: make([]byte, FileSize), Profile: profile, ProfileSide: side, GameMode: gameMode, CameraX: cameraX, CameraY: cameraY, Catalog: catalog}
	document := &Document{World: world, Metadata: metadata}
	data, err := Encode(document)
	if err != nil {
		return nil, err
	}
	decoded, err := Decode(data, catalog)
	if err != nil {
		return nil, fmt.Errorf("new GAM file did not reimport: %w", err)
	}
	// Retain the caller's complete world rather than silently replacing fields
	// not representable by the original file with the imported view.
	decoded.World = world
	decoded.Metadata.initial = engine.Snapshot{}
	return decoded, nil
}
