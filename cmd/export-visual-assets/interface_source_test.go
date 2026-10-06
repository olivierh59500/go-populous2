package main

import (
	"os"
	"strings"
	"testing"

	"go-populous2/internal/populous2"
)

func TestPrivateEnglishInterfaceExportRejectsImplicitFrenchAndAcceptsOverride(t *testing.T) {
	path := os.Getenv("POPULOUS2_EXPORT_TEST_DIR")
	english := os.Getenv("POPULOUS2_INTERFACE_EXECUTABLE")
	if path == "" || english == "" {
		t.Skip("set private reference directory and English interface executable")
	}
	source, err := populous2.LoadFS(os.DirFS(path))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("POPULOUS2_INTERFACE_EXECUTABLE", "")
	p, err := populous2.DecodeNativePresentation(source.Executable)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(p.StartupRequester.Text), "CREATE YOUR") {
		if _, err := interfaceSource(source); err != nil {
			t.Fatal("supported English import rejected", err)
		}
	} else {
		if _, err := interfaceSource(source); err == nil || !strings.Contains(err.Error(), "-interface-executable") {
			t.Fatal("French source silently exported mixed-language UI", err)
		}
	}
	t.Setenv("POPULOUS2_INTERFACE_EXECUTABLE", english)
	selected, err := interfaceSource(source)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateEnglishInterface(selected); err != nil {
		t.Fatal("explicit original English interface override rejected", err)
	}
}
