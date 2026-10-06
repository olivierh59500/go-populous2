package assetimport

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go-populous2/internal/amiga"
)

type Config struct {
	ADFs       []string
	Executable string
	Output     string
}

// Import reads only the required game files, checks all fingerprints, then
// writes a local installation. Existing matching files are reused; different
// files are never replaced. No original data are downloaded or executed.
func Import(config Config) (int, error) {
	if len(config.ADFs) == 0 {
		return 0, fmt.Errorf("supply at least one original disk image with -adf")
	}
	if config.Output == "" {
		return 0, fmt.Errorf("an output directory is required")
	}
	wanted := make(map[string]Fingerprint, len(Files))
	for _, entry := range Files {
		wanted[entry.Name] = entry
	}
	contents := make(map[string][]byte, len(Files))
	for _, path := range config.ADFs {
		raw, err := os.ReadFile(path)
		if err != nil {
			return 0, err
		}
		disk, err := amiga.ParseDisk(raw)
		if err != nil {
			return 0, fmt.Errorf("disk %q: %w", path, err)
		}
		for _, entry := range disk.Entries() {
			name := strings.ToLower(filepath.Base(entry.Path))
			spec, required := wanted[name]
			if entry.Directory || !required {
				continue
			}
			data, err := disk.ReadFile(entry.Path)
			if err != nil {
				return 0, err
			}
			if name == "populous.ii" && config.Executable != "" {
				continue
			}
			if name == "populous.ii" {
				data, err = prepareExecutable(data)
				if err != nil {
					continue // Another supplied disk can contain a supported revision.
				}
			} else if err := check(spec, data); err != nil {
				return 0, fmt.Errorf("disk %q: %w", path, err)
			}
			if previous, exists := contents[name]; exists && !bytes.Equal(previous, data) {
				return 0, fmt.Errorf("conflicting input for %s", name)
			}
			contents[name] = data
		}
	}
	if config.Executable != "" {
		data, err := os.ReadFile(config.Executable)
		if err != nil {
			return 0, err
		}
		data, err = prepareExecutable(data)
		if err != nil {
			return 0, err
		}
		contents["populous.ii"] = data
	}
	var missing []string
	for _, entry := range Files {
		if contents[entry.Name] == nil {
			missing = append(missing, entry.Name)
		}
	}
	if len(missing) > 0 {
		if contents["populous.ii"] == nil {
			return 0, fmt.Errorf("missing %v; supply the original supported boot disk with -adf, or your own compatible populous.ii with -executable (see docs/ASSET_SETUP.md)", missing)
		}
		return 0, fmt.Errorf("missing game files %v; supply the original data disk with -adf", missing)
	}
	if _, err := amiga.ParseExecutable(contents["populous.ii"]); err != nil {
		return 0, err
	}
	for _, entry := range Files {
		path := filepath.Join(config.Output, entry.Name)
		info, err := os.Lstat(path)
		if err == nil {
			if !info.Mode().IsRegular() {
				return 0, fmt.Errorf("output %q is not a regular file", path)
			}
			existing, err := os.ReadFile(path)
			if err != nil {
				return 0, err
			}
			if !bytes.Equal(existing, contents[entry.Name]) {
				return 0, fmt.Errorf("existing output %q differs; choose another directory", path)
			}
		} else if !os.IsNotExist(err) {
			return 0, err
		}
	}
	if err := os.MkdirAll(config.Output, 0755); err != nil {
		return 0, err
	}
	created := []string{}
	rollback := func() {
		for _, path := range created {
			_ = os.Remove(path)
		}
	}
	for _, entry := range Files {
		path := filepath.Join(config.Output, entry.Name)
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if os.IsExist(err) {
			existing, readErr := os.ReadFile(path)
			info, statErr := os.Lstat(path)
			if readErr == nil && statErr == nil && info.Mode().IsRegular() && bytes.Equal(existing, contents[entry.Name]) {
				continue
			}
		}
		if err != nil {
			rollback()
			return 0, err
		}
		created = append(created, path)
		_, writeErr := file.Write(contents[entry.Name])
		closeErr := file.Close()
		if writeErr != nil {
			rollback()
			return 0, writeErr
		}
		if closeErr != nil {
			rollback()
			return 0, closeErr
		}
	}
	return len(Files), nil
}

func check(spec Fingerprint, data []byte) error {
	if len(data) != spec.Size || fmt.Sprintf("%x", sha256.Sum256(data)) != spec.SHA256 {
		return fmt.Errorf("unsupported or damaged %s: got %d bytes, SHA-256 %x; expected %d bytes, SHA-256 %s", spec.Name, len(data), sha256.Sum256(data), spec.Size, spec.SHA256)
	}
	return nil
}

// Validate verifies a completed installation independently of the embed.
func Validate(files fs.FS) error {
	for _, entry := range Files {
		data, err := fs.ReadFile(files, entry.Name)
		if err != nil {
			return err
		}
		if entry.Name == "populous.ii" {
			if _, err := prepareExecutable(data); err != nil {
				return err
			}
		} else if err := check(entry, data); err != nil {
			return err
		}
	}
	return nil
}
