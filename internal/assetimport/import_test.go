package assetimport

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestImportRejectsMissingInputsWithoutChangingOutput(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "assets")
	if _, err := Import(Config{Output: out}); err == nil {
		t.Fatal("no disk inputs were accepted")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("failed import created output", err)
	}
	bad := filepath.Join(root, "bad.adf")
	if err := os.WriteFile(bad, []byte("not a disk"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(Config{ADFs: []string{bad}, Output: out}); err == nil {
		t.Fatal("corrupt disk was accepted")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatal("failed import created output", err)
	}
}

func TestManifestContainsOnlyFingerprintsAndRequiresAllFiles(t *testing.T) {
	if len(Files) != 27 {
		t.Fatal("resource manifest is incomplete", len(Files))
	}
	names := map[string]bool{}
	for _, spec := range Files {
		if spec.Name == "" || spec.Name != filepath.Base(spec.Name) || spec.Size <= 0 || len(spec.SHA256) != 64 || names[spec.Name] {
			t.Fatal("invalid manifest entry", spec)
		}
		names[spec.Name] = true
		if err := check(spec, []byte("different bytes")); err == nil {
			t.Fatal("unverified payload was accepted", spec.Name)
		}
	}
	if err := Validate(fstest.MapFS{}); err == nil {
		t.Fatal("empty installation was accepted")
	}
}
