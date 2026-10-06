package app

import (
	"encoding/binary"
	"go-populous2/internal/engine"
	"go-populous2/internal/populous2"
	"os"
	"testing"
)

// The actual HUD dispatcher chooses whether its icon calls the FX scanner;
// its separate table chooses the retained drawing identity. Both must match.
func TestPrivateEffectInspectionMatchesOriginalIconDispatch(t *testing.T) {
	path := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	if path == "" {
		t.Skip("set original interface data directory")
	}
	source, err := populous2.LoadFS(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	code := source.Executable.Hunks[0].Data
	fileKinds := [...]int{0, 32, 34, 36, 38, 40, 42, 44, 46, 48, 50, 52, 54, 56, 58}
	for _, power := range engine.Powers {
		branch := 0x2528 + int(int16(binary.BigEndian.Uint16(code[0x2528+int(power.ID)*2:])))
		scanner := false
		switch branch {
		case 0x267e, 0x26c6, 0x270c, 0x2722, 0x2738, 0x2784, 0x279a, 0x27ca, 0x27e0, 0x27f6:
			scanner = true
		}
		kind := int(int16(binary.BigEndian.Uint16(code[0x21066+int(power.ID)*2:])))
		wantScan := scanner && kind >= 0
		class, gotScan := effectIconInspection(power.ID)
		if gotScan != wantScan || gotScan && fileKinds[class] != kind {
			t.Fatalf("icon %s inspection differs from its original dispatcher: scan %v/%v class %d/%d", power.Name, gotScan, wantScan, fileKinds[class], kind)
		}
	}
}
