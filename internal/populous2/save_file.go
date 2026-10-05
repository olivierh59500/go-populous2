package populous2

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func IsNativeGAMFile(path string) bool { return strings.EqualFold(filepath.Ext(path), ".gam") }

// ReadGameFile accepts native .GAM files and the retained Go JSON save format.
// Format selection is explicit; malformed native data never falls back to JSON.
func ReadGameFile(bundle *Bundle, path string) (*World, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if IsNativeGAMFile(path) {
		data, err := io.ReadAll(io.LimitReader(f, NativeGAMSize))
		if err != nil {
			return nil, err
		}
		return ImportNativeGAM(bundle, data)
	}
	return ReadSave(bundle, f)
}

// WriteGameFile replaces a complete save atomically within its destination
// directory. Native export errors leave any existing file untouched.
func (w *World) WriteGameFile(path string) error {
	var data []byte
	var err error
	if IsNativeGAMFile(path) {
		data, err = w.ExportNativeGAM()
	} else {
		data, err = json.Marshal(w.Snapshot())
	}
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".populous2-save-*.tmp")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	_, err = f.Write(data)
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}
