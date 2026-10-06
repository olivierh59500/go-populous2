package populous2

import (
	"fmt"
	"io/fs"
	"strings"
)

// AssetResourceCatalog describes the data files read by the Go asset loaders. Its
// entries contain no executable offsets, relocations, or emulated addresses.
// A fresh slice is returned so callers can safely keep their own inventory.
func AssetResourceCatalog() []Resource {
	const readLimit = 1<<20 - 1
	entries := []struct {
		name   string
		packed bool
	}{
		{"BLOCK0.PAK", true}, {"BLOCK1.PAK", true},
		{"BLOCK2.PAK", true}, {"BLOCK3.PAK", true},
		{"LAND0.DAT", false}, {"LAND1.DAT", false},
		{"LAND2.DAT", false}, {"LAND3.DAT", false},
		{"FACES.PAK", true}, {"JUDGE.PAK", true},
		{"AWARDS.PAK", true}, {"HANDS.PAK", true},
		{"CONQUEST.PAK", true}, {"fx.dat", false},
		{"qaz.pak", true}, {"s16-0.dif", false},
		{"s16-1.dif", false}, {"s16-2.dif", false},
		{"s16-3.dif", false}, {"s32-0.dif", false},
		{"s32-1.pif", true}, {"s32-2.pif", true},
		{"s32-3.pif", true}, {"s16-0.pak", true},
		{"s32-0.pak", true}, {"end.pak", true},
	}
	resources := make([]Resource, len(entries))
	for index, entry := range entries {
		resources[index] = Resource{
			Index: index, Name: entry.name, ReadLimit: readLimit, Packed: entry.packed,
		}
	}
	return resources
}

// LoadResourceSetFS loads the complete data-file bank without opening the
// original executable. Keys are lowercase Amiga filenames. Graphics, campaign
// and audio decoders can consume this bank independently of the reference host.
func LoadResourceSetFS(files fs.FS) (map[string][]byte, error) {
	resources := AssetResourceCatalog()
	status, err := Inventory(files, resources)
	if err != nil {
		return nil, err
	}
	if missing := MissingResources(status); len(missing) != 0 {
		return nil, fmt.Errorf("missing Populous II data files: %v", missing)
	}
	decoded := make(map[string][]byte, len(resources))
	for _, resource := range resources {
		data, err := LoadResource(files, resource)
		if err != nil {
			return nil, err
		}
		decoded[strings.ToLower(resource.Name)] = data
	}
	return decoded, nil
}
