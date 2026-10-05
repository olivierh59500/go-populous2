package populous2

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ResolveNativeHostPath gives the original case-insensitive DOS name a host
// identity. Exact names take precedence; ambiguous folded names are errors.
// Only the final component may be absent when preparing a new save.
func ResolveNativeHostPath(path []byte, create bool) (string, error) {
	name := string(nativeFileCString(path))
	if name == "" {
		name = "."
	}
	name = filepath.Clean(name)
	if _, err := os.Stat(name); err == nil {
		return name, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	volume := filepath.VolumeName(name)
	rest := strings.TrimPrefix(name, volume)
	current := "."
	if filepath.IsAbs(name) {
		current = volume + string(filepath.Separator)
		rest = strings.TrimLeft(rest, string(filepath.Separator))
	}
	parts := strings.Split(rest, string(filepath.Separator))
	for index, part := range parts {
		entries, err := os.ReadDir(current)
		if err != nil {
			return "", err
		}
		matched := ""
		exact := false
		for _, entry := range entries {
			if entry.Name() == part {
				matched, exact = part, true
				break
			}
		}
		if !exact {
			for _, entry := range entries {
				if !strings.EqualFold(entry.Name(), part) {
					continue
				}
				if matched != "" {
					return "", fmt.Errorf("ambiguous native filename %q in %q", part, current)
				}
				matched = entry.Name()
			}
		}
		if matched == "" {
			if !create || index != len(parts)-1 {
				return "", &os.PathError{Op: "open", Path: name, Err: os.ErrNotExist}
			}
			matched = part
		}
		current = filepath.Join(current, matched)
	}
	return current, nil
}

// ListNativeHostFiles retains filesystem enumeration order; the requester
// itself applies the original uppercase .GAM filter and byte conversion.
func ListNativeHostFiles(drawer []byte) ([]NativeFileEntry, error) {
	path, err := ResolveNativeHostPath(drawer, false)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(-1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	files := make([]NativeFileEntry, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		files = append(files, NativeFileEntry{Name: []byte(entry.Name()), File: info.Mode().IsRegular()})
	}
	return files, nil
}

func NativeHostFileExists(path []byte) (bool, error) {
	_, err := ResolveNativeHostPath(path, false)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}
