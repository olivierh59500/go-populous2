package populous2

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeGameplayScreenExportFilesystemActualFileAndFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.SCR")
	port := NativeGameplayScreenExportFilesystem{Resolve: func(name string) (string, error) {
		if name != "POPA.SCR" {
			t.Fatal("nativefilenamechanged")
		}
		return path, nil
	}}
	phase := uint32(0)
	opened, e := port.IO(NativeGameplayScreenExportCall{Operation: "open", Name: "POPA.SCR", Mode: 1006}, &phase)
	if e != nil || !opened.Complete || opened.Value <= 0 {
		t.Fatal(opened, e)
	}
	payload := []byte{0x46, 0x4f, 0x52, 0x4d, 0, 1, 2, 3}
	written, e := port.IO(NativeGameplayScreenExportCall{Operation: "write", Handle: uint32(opened.Value), Data: payload}, &phase)
	if e != nil || written.Value != int32(len(payload)) {
		t.Fatal(written, e)
	}
	closed, e := port.IO(NativeGameplayScreenExportCall{Operation: "close", Handle: uint32(opened.Value)}, &phase)
	if e != nil || closed.Value != -1 {
		t.Fatal(closed, e)
	}
	actual, e := os.ReadFile(path)
	if e != nil || !bytes.Equal(actual, payload) {
		t.Fatal("actualfilebyteschanged", e)
	}
	again, e := port.IO(NativeGameplayScreenExportCall{Operation: "open", Name: "POPA.SCR", Mode: 1006}, &phase)
	if e != nil || again.Value != 0 || port.LastError == nil {
		t.Fatal("existingfilefailurefabricatedsuccess")
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(after, payload) {
		t.Fatal("unapprovedreplacechangedexistingfile")
	}
	port.AllowReplace = true
	replace, e := port.IO(NativeGameplayScreenExportCall{Operation: "open", Name: "POPA.SCR", Mode: 1006}, &phase)
	if e != nil || replace.Value <= 0 {
		t.Fatal(replace, e)
	}
	if e = port.Close(); e != nil {
		t.Fatal(e)
	}
	after, _ = os.ReadFile(path)
	if len(after) != 0 {
		t.Fatal("configuredsource1006didnottruncate")
	}
}
