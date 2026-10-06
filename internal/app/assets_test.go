package app

import (
	"io/fs"
	"os"
	"strings"
	"testing"
)

type assetsOnlyFS struct{ fs.FS }

func (f assetsOnlyFS) Open(name string) (fs.File, error) {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "populous.ii") || strings.Contains(lower, "hunk") || strings.Contains(lower, "code.bin") {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}
	return f.FS.Open(name)
}

func TestIndependentAssetsLoadWithoutOriginalExecutable(t *testing.T) {
	path := os.Getenv("POPULOUS2_GENERATED_ASSETS_TEST_DIR")
	if path == "" {
		t.Skip("locally exported original artwork is not distributed")
	}
	files := assetsOnlyFS{os.DirFS(path)}
	assets, err := LoadAssets(files)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets.Levels) != 1000 || len(assets.Visual.Tiles[0]) != 255 || len(assets.Visual.Sprites[0]) != 830 || assets.Music == nil {
		t.Fatal("portable asset catalog incomplete")
	}
	if err := assets.requireOriginalInterface(); err != nil {
		t.Fatal("portable export cannot launch the restored original interface", err)
	}
}

func TestLoadAssetsRequiresAVisualCatalog(t *testing.T) {
	_, err := LoadAssets(os.DirFS(t.TempDir()))
	if err == nil {
		t.Fatal("missing generated assets accepted")
	}
}
