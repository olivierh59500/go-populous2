package populous2

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"go-populous2/internal/amiga"
)

type Resource struct {
	Index       int    `json:"index"`
	Name        string `json:"name"`
	TableOffset int    `json:"hunk0_table_offset"`
	Destination uint32 `json:"destination"`
	Invalidate  uint32 `json:"invalidate_mask"`
	ReadLimit   uint32 `json:"read_limit"`
	PlanarTable uint32 `json:"planar_table"`
	Packed      bool   `json:"packed"`
}

// ResourceTable decodes the original 48-byte load descriptors used by the
// routine at 0x19cd0. It finds the table by its first filename, then validates
// all names and fields. The Challenge executable uses a different table.
func ResourceTable(exe *amiga.Executable) ([]Resource, error) {
	if exe == nil || len(exe.Hunks) == 0 || exe.Hunks[0].Index != 0 || exe.Hunks[0].Kind != "code" {
		return nil, fmt.Errorf("Populous II main code hunk is absent")
	}
	code := exe.Hunks[0].Data
	start := bytes.Index(code, []byte("BLOCK0.PAK\x00")) - 18
	const recordSize = 48
	names := []string{
		"BLOCK0.PAK", "BLOCK1.PAK", "BLOCK2.PAK", "BLOCK3.PAK",
		"LAND0.DAT", "LAND1.DAT", "LAND2.DAT", "LAND3.DAT",
		"FACES.PAK", "JUDGE.PAK", "AWARDS.PAK", "HANDS.PAK", "CONQUEST.PAK",
		"fx.dat", "qaz.pak", "s16-0.dif", "s16-1.dif", "s16-2.dif", "s16-3.dif",
		"s32-0.dif", "s32-1.pif", "s32-2.pif", "s32-3.pif", "s16-0.pak", "s32-0.pak", "end.pak",
	}
	if start < 0 || start+len(names)*recordSize > len(code) {
		return nil, fmt.Errorf("Populous II resource table is missing or truncated")
	}
	result := make([]Resource, 0, len(names))
	for i, expected := range names {
		offset := start + i*recordSize
		record := code[offset : offset+recordSize]
		name := string(bytes.TrimRight(record[18:], "\x00"))
		if !strings.EqualFold(name, expected) || !fs.ValidPath(name) {
			return nil, fmt.Errorf("resource %d at 0x%x has unexpected name %q", i, offset, name)
		}
		packed := binary.BigEndian.Uint16(record[16:18])
		if packed > 1 {
			return nil, fmt.Errorf("resource %q has unknown compression flag %d", name, packed)
		}
		r := Resource{
			Index: i, Name: name, TableOffset: offset,
			Destination: binary.BigEndian.Uint32(record[0:4]),
			Invalidate:  binary.BigEndian.Uint32(record[4:8]),
			ReadLimit:   binary.BigEndian.Uint32(record[8:12]),
			PlanarTable: binary.BigEndian.Uint32(record[12:16]),
			Packed:      packed == 1,
		}
		if r.ReadLimit == 0 || r.ReadLimit > MaxDecodedBytes {
			return nil, fmt.Errorf("resource %q has unsupported read limit %d", name, r.ReadLimit)
		}
		result = append(result, r)
	}
	return result, nil
}

type ResourceStatus struct {
	Resource
	Path    string `json:"path,omitempty"`
	Present bool   `json:"present"`
}

// Inventory compares the executable's own load table to extracted data. Lookup
// is case-insensitive, as on AmigaDOS. Ambiguous filenames are rejected.
func Inventory(files fs.FS, resources []Resource) ([]ResourceStatus, error) {
	paths, err := resourcePaths(files)
	if err != nil {
		return nil, err
	}
	result := make([]ResourceStatus, 0, len(resources))
	for _, resource := range resources {
		p, found := paths[strings.ToUpper(resource.Name)]
		result = append(result, ResourceStatus{Resource: resource, Path: p, Present: found})
	}
	return result, nil
}

func resourcePaths(files fs.FS) (map[string]string, error) {
	if files == nil {
		return nil, fmt.Errorf("nil Populous II data filesystem")
	}
	paths := make(map[string]string)
	err := fs.WalkDir(files, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return fmt.Errorf("unsupported nonregular data file %q", p)
		}
		parts := strings.Split(p, "/")
		key := strings.ToUpper(parts[len(parts)-1])
		if previous, exists := paths[key]; exists {
			return fmt.Errorf("ambiguous Amiga filename %q: %q and %q", key, previous, p)
		}
		paths[key] = p
		return nil
	})
	return paths, err
}

// LoadResource selects compression according to the executable, including for
// files whose suffix does not advertise it. It returns raw decoded bytes; sprite
// tables and palettes are interpreted by the separate graphics loader.
func LoadResource(files fs.FS, resource Resource) ([]byte, error) {
	paths, err := resourcePaths(files)
	if err != nil {
		return nil, err
	}
	p, found := paths[strings.ToUpper(resource.Name)]
	if !found {
		return nil, fmt.Errorf("Populous II resource %q: %w", resource.Name, fs.ErrNotExist)
	}
	data, err := fs.ReadFile(files, p)
	if err != nil {
		return nil, err
	}
	if uint64(len(data)) > uint64(resource.ReadLimit) {
		return nil, fmt.Errorf("resource %q exceeds original read limit %d", resource.Name, resource.ReadLimit)
	}
	if resource.Packed {
		data, err = DecodePacked(data)
		if err != nil {
			return nil, fmt.Errorf("resource %q: %w", resource.Name, err)
		}
	}
	return data, nil
}

func MissingResources(status []ResourceStatus) []string {
	var missing []string
	for _, r := range status {
		if !r.Present {
			missing = append(missing, r.Name)
		}
	}
	sort.Strings(missing)
	return missing
}
