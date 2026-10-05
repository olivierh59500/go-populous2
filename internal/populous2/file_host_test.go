package populous2

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeHostRequesterTransfersAndOverwrite(t *testing.T) {
	b := testBundle(t)
	p, err := DecodeNativePresentation(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "Saved Games")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	w, err := NewWorld(b, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	actual := filepath.Join(dir, "MixedCase.GAM")
	if err := w.WriteGameFile(actual); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(actual)
	if err != nil {
		t.Fatal(err)
	}
	var loaded *World
	cb := NativeFileRequesterCallbacks{
		List: ListNativeHostFiles, Exists: NativeHostFileExists,
		Load: func(raw []byte) error {
			path, err := ResolveNativeHostPath(raw, false)
			if err != nil {
				return err
			}
			loaded, err = ReadGameFile(b, path)
			return err
		},
		Save: func(raw []byte) error {
			path, err := ResolveNativeHostPath(raw, true)
			if err != nil {
				return err
			}
			return w.WriteGameFile(path)
		},
	}
	r, err := NewNativeFileRequester(p, NativeFileLoad, []byte(dir), nil, cb)
	if err != nil || len(r.Files) != 1 || string(r.Files[0]) != "MIXEDCASE.GAM" {
		t.Fatal("native browser did not retain host identity", err)
	}
	if err := r.Action(4); err != nil {
		t.Fatal(err)
	}
	if err := r.Action(34); err != nil || !r.Done || loaded == nil || loaded.Level.Number != 0 {
		t.Fatal("selected uppercase name did not load mixed-case host file", err)
	}
	r, err = NewNativeFileRequester(p, NativeFileSave, []byte(dir), []byte("MIXEDCASE.GAM"), cb)
	if err != nil {
		t.Fatal(err)
	}
	w.SetEffectView(17, 21)
	if err := r.Action(34); err != nil || r.Modal != NativeFileOverwrite || r.Done {
		t.Fatal("existing host file bypassed overwrite confirmation", err)
	}
	if err := r.Action(4); err != nil {
		t.Fatal(err)
	}
	unchanged, err := os.ReadFile(actual)
	if err != nil || !bytes.Equal(before, unchanged) {
		t.Fatal("declined overwrite changed saved data", err)
	}
	if err := r.Action(34); err != nil {
		t.Fatal(err)
	}
	if err := r.Action(2); err != nil || !r.Done {
		t.Fatal("confirmed overwrite did not complete", err)
	}
	loaded, err = ReadGameFile(b, actual)
	if err != nil {
		t.Fatal(err)
	}
	if x, y := loaded.EffectView(); x != 17 || y != 21 {
		t.Fatal("overwrite did not transfer current camera")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 || entries[0].Name() != "MixedCase.GAM" {
		t.Fatal("save created a case-variant duplicate", err)
	}
	r, err = NewNativeFileRequester(p, NativeFileLoad, []byte(dir), []byte("missing"), cb)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Action(34); err != nil || r.Modal != NativeFileMessage || r.Done || !errors.Is(r.LastError, os.ErrNotExist) {
		t.Fatal("missing transfer did not show the native error dialog", err)
	}
}

func TestNativeHostPathCaseComponentsAndMissingParent(t *testing.T) {
	dir := t.TempDir()
	actual := filepath.Join(dir, "Drawer")
	if err := os.Mkdir(actual, 0755); err != nil {
		t.Fatal(err)
	}
	path, err := ResolveNativeHostPath([]byte(filepath.Join(dir, "DRAWER", "NEW.GAM")), true)
	if err != nil || path != filepath.Join(actual, "NEW.GAM") {
		t.Fatal("case-insensitive parent not resolved", path, err)
	}
	if _, err := ResolveNativeHostPath([]byte(filepath.Join(dir, "Missing", "NEW.GAM")), true); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("save silently created a missing parent", err)
	}
}
