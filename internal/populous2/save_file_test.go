package populous2

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeAndGoSaveFilesResumeCompleteWorld(t *testing.T) {
	for _, extension := range []string{".GAM", ".gam", ".sav"} {
		t.Run(extension, func(t *testing.T) {
			w, err := NewWorld(testBundle(t), 27, false)
			if err != nil {
				t.Fatal(err)
			}
			w.SetEffectView(8, 4)
			for range 25 {
				w.Tick()
			}
			path := filepath.Join(t.TempDir(), "world"+extension)
			if err := w.WriteGameFile(path); err != nil {
				t.Fatal(err)
			}
			restored, err := ReadGameFile(testBundle(t), path)
			if err != nil {
				t.Fatal(err)
			}
			if IsNativeGAMFile(path) {
				data, err := os.ReadFile(path)
				if err != nil || len(data) != NativeGAMSize {
					t.Fatal("native file does not contain the original exact transfer size")
				}
				if x, y := restored.EffectView(); x != 8 || y != 4 {
					t.Fatal("native camera origin did not resume")
				}
			}
			for range 80 {
				w.Tick()
				restored.Tick()
			}
			if IsNativeGAMFile(path) {
				actual, err := restored.ExportNativeGAM()
				if err != nil {
					t.Fatal(err)
				}
				expected, err := w.ExportNativeGAM()
				if err != nil {
					t.Fatal(err)
				}
				assertNativeGAMBytes(t, actual, expected)
			} else if !bytes.Equal(encodeSnapshot(t, w), encodeSnapshot(t, restored)) {
				t.Fatal("Go JSON file continuation differs")
			}
		})
	}
}

func TestFailedNativeExportPreservesPreviousFile(t *testing.T) {
	w, err := NewWorld(testBundle(t), 0, false)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "world.GAM")
	if err := w.WriteGameFile(path); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	w.Deity.Name = "NAME EXCEEDS THE NATIVE FIELD"
	if err := w.WriteGameFile(path); err == nil {
		t.Fatal("unrepresentable native name was silently truncated")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed native export replaced the existing saved game")
	}
}

func TestNativeCameraUsesTheSharedViewMemory(t *testing.T) {
	w, err := NewWorld(testBundle(t), 0, false)
	if err != nil {
		t.Fatal(err)
	}
	w.SetEffectView(8, 4)
	if binary.BigEndian.Uint16(w.NativeViewBytes[:]) != 8 || binary.BigEndian.Uint16(w.NativeViewBytes[2:]) != 4 {
		t.Fatal("camera origin did not enter native view memory")
	}
	if err := w.nativeCleanupMemory().Write16(0x5f44, 9); err != nil {
		t.Fatal(err)
	}
	if x, y := w.EffectView(); x != 9 || y != 4 {
		t.Fatal("view memory alias did not reach the presentation camera")
	}
}

func TestNativeFileReadsOnlyOriginalTransferBlock(t *testing.T) {
	w, err := NewWorld(testBundle(t), 0, false)
	if err != nil {
		t.Fatal(err)
	}
	data, err := w.ExportNativeGAM()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "trailing.GAM")
	if err := os.WriteFile(path, append(data, []byte("ignored native file suffix")...), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadGameFile(testBundle(t), path)
	if err != nil {
		t.Fatal(err)
	}
	exported, err := loaded.ExportNativeGAM()
	if err != nil {
		t.Fatal(err)
	}
	assertNativeGAMBytes(t, exported, data)
}
