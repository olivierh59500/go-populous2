package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestIndependentRuntimeDoesNotDependOnOriginalProgram(t *testing.T) {
	command := exec.Command("go", "list", "-deps", ".")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("dependency graph: %v\n%s", err, output)
	}
	forbidden := map[string]bool{
		"go-populous2/internal/populous2": true,
		"go-populous2/internal/amiga":     true,
		"go-populous2/internal/legacy":    true,
		"go-populous2/internal/game":      true,
		"go-populous2/internal/nativeapp": true,
	}
	for _, dependency := range strings.Fields(string(output)) {
		if forbidden[dependency] {
			t.Fatalf("independent runtime imports binary-dependent subsystem %s", dependency)
		}
	}
}
