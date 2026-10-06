package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOriginalFileActionsPreserveDraftAndValidateDirectory(t *testing.T) {
	g := browserGame(t)
	if err := g.openSaveBrowser(true); err != nil {
		t.Fatal(err)
	}
	g.SaveBrowser.Files = []string{"FIRST.JSON", "SECOND.GAM"}
	g.handleFileAction("file-1")
	if g.SaveBrowser.Name != "SECOND.GAM" {
		t.Fatal("original file row did not select name")
	}
	g.handleFileAction("directory")
	if !g.SaveBrowser.EditingDirectory {
		t.Fatal("original Drawer field did not edit")
	}
	before := g.SaveBrowser.Directory
	g.SaveBrowser.DirectoryInput = filepath.Join(t.TempDir(), "absent")
	g.finishFileDirectory()
	if g.SaveBrowser.Directory != before || g.SaveBrowser.Error == "" {
		t.Fatal("invalid drawer replaced valid directory")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "valid.json"), []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	g.SaveBrowser.DirectoryInput = dir
	g.finishFileDirectory()
	if g.SaveBrowser.Directory != dir || len(g.SaveBrowser.Files) != 1 || g.SaveBrowser.EditingDirectory {
		t.Fatal("valid drawer did not refresh original list")
	}
	g.SaveBrowser.Confirm = true
	g.handleFileAction("cancel")
	if g.SaveBrowser == nil || g.SaveBrowser.Confirm {
		t.Fatal("overwrite Cancel closed the whole browser")
	}
}
