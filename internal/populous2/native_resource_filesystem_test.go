package populous2

import (
	"bytes"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	embedded "go-populous2/assets"
)

func TestNativeResourceFilesystemReadsOriginalEncodedAssets(t *testing.T) {
	files, err := fs.Sub(embedded.Files, "amiga")
	if err != nil {
		t.Fatal(err)
	}
	disk, err := NewNativeResourceFilesystem(files)
	if err != nil {
		t.Fatal(err)
	}
	defer disk.Close()
	for _, resource := range testBundle(t).Resources {
		open, err := disk.IO(NativeResourceFrameIOCall{Operation: "open", Name: resource.Name}, nil)
		if err != nil || open.Value <= 0 {
			t.Fatal("original encoded resource did not open", resource.Name, open, err)
		}
		read, err := disk.IO(NativeResourceFrameIOCall{Operation: "read", Handle: uint32(open.Value), Limit: resource.ReadLimit}, nil)
		if err != nil {
			t.Fatal(err)
		}
		want, err := fs.ReadFile(files, disk.Paths[strings.ToUpper(resource.Name)])
		if err != nil {
			t.Fatal(err)
		}
		if read.Value != int32(len(want)) || !bytes.Equal(read.Data, want) {
			t.Fatal("native encoded read count/payload changed", resource.Name, read.Value, len(want))
		}
		close, err := disk.IO(NativeResourceFrameIOCall{Operation: "close", Handle: uint32(open.Value)}, nil)
		if err != nil || close.Value != -1 {
			t.Fatal("native close result changed", close, err)
		}
		if resource.Packed && bytes.Equal(read.Data, testBundle(t).Raw[strings.ToLower(resource.Name)]) {
			t.Fatal("decoded resource was substituted for encoded source", resource.Name)
		}
	}
	if len(disk.handles) != 0 {
		t.Fatal("resource handles leaked")
	}
}

func TestNativeResourceFilesystemReturnsShortCountsAndNativeFailures(t *testing.T) {
	disk, err := NewNativeResourceFilesystem(fstest.MapFS{"Folder/Test.PAK": &fstest.MapFile{Data: []byte{1, 2, 3, 4, 5}}})
	if err != nil {
		t.Fatal(err)
	}
	open, err := disk.IO(NativeResourceFrameIOCall{Operation: "open", Name: "test.pak"}, nil)
	if err != nil || open.Value <= 0 {
		t.Fatal(open, err)
	}
	read, err := disk.IO(NativeResourceFrameIOCall{Operation: "read", Handle: uint32(open.Value), Limit: 3}, nil)
	if err != nil || read.Value != 3 || !bytes.Equal(read.Data, []byte{1, 2, 3}) {
		t.Fatal(read, err)
	}
	read, err = disk.IO(NativeResourceFrameIOCall{Operation: "read", Handle: uint32(open.Value), Limit: 3}, nil)
	if err != nil || read.Value != 2 || !bytes.Equal(read.Data, []byte{4, 5}) {
		t.Fatal("actual short transfer changed", read, err)
	}
	missing, err := disk.IO(NativeResourceFrameIOCall{Operation: "open", Name: "missing.pak"}, nil)
	if err != nil || missing.Value != 0 || disk.LastError == nil {
		t.Fatal("native open failure became success", missing, err)
	}
	invalid, err := disk.IO(NativeResourceFrameIOCall{Operation: "read", Handle: 999, Limit: 3}, nil)
	if err != nil || invalid.Value != -1 || disk.LastError == nil {
		t.Fatal("native read failure became success", invalid, err)
	}
	if err := disk.Close(); err != nil || len(disk.handles) != 0 {
		t.Fatal("resource cleanup failed", err)
	}
	if _, err := NewNativeResourceFilesystem(fstest.MapFS{"A/Test.PAK": &fstest.MapFile{}, "B/test.pak": &fstest.MapFile{}}); err == nil {
		t.Fatal("ambiguous Amiga filename accepted")
	}
}
