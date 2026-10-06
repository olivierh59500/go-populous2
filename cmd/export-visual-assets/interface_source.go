package main

import (
	"fmt"
	"os"
	"strings"

	"go-populous2/internal/amiga"
	"go-populous2/internal/populous2"
)

// interfaceSource selects an optional original English presentation revision
// during local export only. The resulting runtime data contains glyphs, image
// metadata and named actions, never instructions or executable state.
func interfaceSource(source *populous2.Bundle) (*populous2.Bundle, error) {
	path := os.Getenv("POPULOUS2_INTERFACE_EXECUTABLE")
	if path == "" {
		if err := validateEnglishInterface(source); err != nil {
			return nil, err
		}
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
	if err := validateEnglishInterface(&copy); err != nil {
		return nil, err
	}
	return &copy, nil
}

// The supported English edition has stable English labels in independent
// startup and result resources. Reject mixed-language exports explicitly.
func validateEnglishInterface(source *populous2.Bundle) error {
	if source == nil || source.Executable == nil {
		return fmt.Errorf("interface executable is missing")
	}
	p, err := populous2.DecodeNativePresentation(source.Executable)
	if err != nil {
		return err
	}
	r, err := p.Result(1, 2, 0, populous2.CampaignResultStatistics{}, populous2.CampaignResultStatistics{}, 0)
	if err != nil {
		return err
	}
	if !strings.Contains(string(p.StartupRequester.Text), "CREATE YOUR") || !strings.Contains(string(r.Parameters[1]), "DAYS") {
		return fmt.Errorf("English interface assets are required; supply a compatible original English executable with -interface-executable or POPULOUS2_INTERFACE_EXECUTABLE")
	}
	return nil
}
