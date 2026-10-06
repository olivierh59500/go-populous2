package populous2

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	embedded "go-populous2/assets"
	"go-populous2/internal/amiga"
)

func TestResourceCatalogContainsDataOnlyAndReturnsIndependentEntries(t *testing.T) {
	resources := AssetResourceCatalog()
	if len(resources) != 26 {
		t.Fatalf("catalog size %d, want 26", len(resources))
	}
	seen := make(map[string]bool, len(resources))
	for index, resource := range resources {
		name := strings.ToLower(resource.Name)
		if resource.Index != index || seen[name] || !fs.ValidPath(resource.Name) {
			t.Fatalf("invalid catalog entry: %+v", resource)
		}
		seen[name] = true
		if resource.ReadLimit != 1<<20-1 || resource.TableOffset != 0 || resource.Destination != 0 || resource.Invalidate != 0 || resource.PlanarTable != 0 {
			t.Fatalf("catalog retains executable metadata: %+v", resource)
		}
		wantPacked := !strings.HasPrefix(name, "land") && name != "fx.dat" && !strings.HasSuffix(name, ".dif")
		if resource.Packed != wantPacked {
			t.Fatalf("%s compression %t, want %t", name, resource.Packed, wantPacked)
		}
	}
	resources[0].Name = "changed"
	if AssetResourceCatalog()[0].Name != "BLOCK0.PAK" {
		t.Fatal("catalog storage is shared between callers")
	}
}

// noExecutableFS rejects access to the original program even when the local
// installation happens to contain it. The asset-only path must not read it.
type noExecutableFS struct{ fs.FS }

func (files noExecutableFS) Open(name string) (fs.File, error) {
	if strings.EqualFold(name, "populous.ii") {
		return nil, fmt.Errorf("asset-only loader attempted to open an executable")
	}
	return files.FS.Open(name)
}

func TestResourceSetLoadsAndDecodesAssetsWithoutExecutable(t *testing.T) {
	files, err := embedded.DataFS()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(files, "block0.pak"); errors.Is(err, fs.ErrNotExist) {
		t.Skip("original data files are imported locally")
	} else if err != nil {
		t.Fatal(err)
	}
	// Copy only the catalog's data files into a filesystem that contains no
	// executable at all. This also prevents a directory scan from relying on it.
	dataOnly := make(fstest.MapFS, len(AssetResourceCatalog()))
	for _, resource := range AssetResourceCatalog() {
		name := strings.ToLower(resource.Name)
		data, err := fs.ReadFile(files, name)
		if err != nil {
			t.Fatal(err)
		}
		dataOnly[name] = &fstest.MapFile{Data: data}
	}
	bank, err := LoadResourceSetFS(noExecutableFS{dataOnly})
	if err != nil {
		t.Fatal(err)
	}
	if len(bank) != 26 {
		t.Fatalf("decoded resource count %d, want 26", len(bank))
	}
	for name, want := range map[string]string{
		"block0.pak":   "68d6312a48e8aac0d95f246a05818d0f914ecdfdeffdad7052c227e14ee8812e",
		"s16-0.pak":    "d96d8137f653eda79f96f9f867009b529fc1c8574f1eacc32111c043e0417cfe",
		"conquest.pak": "e85d474b65a5e185a375a69c7059837a275fcabce53a8fe5d8825accf6393652",
		"end.pak":      "a4bb6992a3169c8b88d3431d06e4d2b55a329fe6d0111a7596f8a024990df696",
	} {
		if got := amiga.Digest(bank[name]); got != want {
			t.Fatalf("%s decoded hash %s, want %s", name, got, want)
		}
	}
	for index := range 4 {
		landscape, err := DecodeLandscape(bank[fmt.Sprintf("land%d.dat", index)])
		if err != nil {
			t.Fatal(err)
		}
		tiles, err := DecodeTiles(bank[fmt.Sprintf("block%d.pak", index)], landscape.Palettes[0])
		if err != nil || len(tiles) != TileCount {
			t.Fatalf("landscape %d tiles: count %d, error %v", index, len(tiles), err)
		}
		if index == 0 {
			if _, err := DecodeScreen(bank["qaz.pak"], landscape.Palettes[0]); err != nil {
				t.Fatal(err)
			}
		}
		for _, size := range []int{16, 32} {
			diff := fmt.Sprintf("s%d-%d.dif", size, index)
			if size == 32 && index != 0 {
				diff = fmt.Sprintf("s32-%d.pif", index)
			}
			base := bank[fmt.Sprintf("s%d-0.pak", size)]
			before := append([]byte(nil), base...)
			if _, err := ApplySpriteDifference(base, bank[diff]); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(base, before) {
				t.Fatal("sprite variation changed its shared base")
			}
		}
	}
	levels, err := DecodeCampaign(bank["conquest.pak"])
	if err != nil || len(levels) != 1000 {
		t.Fatalf("campaign count %d, error %v", len(levels), err)
	}
}

func TestResourceSetReportsMissingDataWithoutRequiringProgram(t *testing.T) {
	_, err := LoadResourceSetFS(fstest.MapFS{})
	if err == nil || !strings.Contains(err.Error(), "BLOCK0.PAK") || strings.Contains(strings.ToLower(err.Error()), "populous.ii") {
		t.Fatalf("unexpected missing-data diagnostic: %v", err)
	}
}
