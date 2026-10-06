package main

import (
	"fmt"
	"os"

	"go-populous2/internal/amiga"
	"go-populous2/internal/populous2"
)

// interfaceSource selects an optional original English presentation revision
// during local export only. The resulting runtime data contains glyphs, image
// metadata and named actions, never instructions or executable state.
func interfaceSource(source *populous2.Bundle) (*populous2.Bundle, error) {
	path := os.Getenv("POPULOUS2_INTERFACE_EXECUTABLE")
	if path == "" {
		return source, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("interface source: %w", err)
	}
	executable, err := amiga.ParseExecutable(data)
	if err != nil {
		return nil, fmt.Errorf("interface source executable: %w", err)
	}
	copy := *source
	copy.Executable = executable
	return &copy, nil
}
