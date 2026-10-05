package populous2

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"testing"

	embedded "go-populous2/assets"
)

func TestNativeHostEncodedLoaderMatchesOriginalPhysicalReferences(t *testing.T) {
	data, err := os.ReadFile("testdata/native_host_resource_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []resourceFrameFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	bundle := testBundle(t)
	rules, err := DecodeNativeResourceFrameRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	files, err := fs.Sub(embedded.Files, "amiga")
	if err != nil {
		t.Fatal(err)
	}
	executed := 0
	for _, f := range catalog.Cases {
		// FX/QAZ descriptors require source runtime allocations. Their host
		// allocation paths are a separate startup operation, not padded gaps.
		if f.Input.Mode != "resource" || f.Input.Flags != 0 || f.Input.Failure != "" || f.Input.Index == 13 || f.Input.Index == 14 {
			continue
		}
		t.Run(f.Input.Name, func(t *testing.T) {
			if f.Error != "" {
				t.Fatal("original resource capture failed", f.Error)
			}
			memory := nativeHostTestMemory(t)
			m := memory.Memory()
			_ = m.Write32(0x20014c, 0xd00000)
			// Match the reference's declared descriptor pointers. Their target
			// allocations stay absent, so these cases cannot access FX/QAZ RAM.
			_ = m.Write32(0x100000+0x19e4a+13*48, 0x900000)
			_ = m.Write32(0x100000+0x19e4a+14*48, 0x920000)
			_ = m.Write16(0x200000+0xeb22, f.Input.Land)
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			frame.Word(0, f.Input.Index)
			input, err := NewNativeInputState(bundle.Executable)
			if err != nil {
				t.Fatal(err)
			}
			disk, err := NewNativeResourceFilesystem(files)
			if err != nil {
				t.Fatal(err)
			}
			defer disk.Close()
			state := NativeResourceFrameState{}
			step, err := state.Advance(&rules, NativeResourceFrameCallbacks{RAM: m, CodeBase: 0x100000, Frame: &frame, Input: &input, IO: disk.IO})
			if err != nil || !step.Complete || frame.D != f.D {
				t.Fatal("host encoded loader differs from native source", step, err, frame.D, f.D)
			}
			for _, span := range f.Spans {
				bytes, err := memory.Span(span.Start, int(span.Length))
				if err != nil {
					t.Fatal("reference region was not actually allocated", err)
				}
				if got := fileFrameHash(bytes); got != span.Hash {
					t.Fatalf("host physical resource span%x differs: %s/%s", span.Start, got, span.Hash)
				}
			}
			count, err := m.Read32(0x100000 + 0x19e46)
			if err != nil || count != f.ReadCount {
				t.Fatal("source encoded read count differs", count, f.ReadCount, err)
			}
			if len(disk.handles) != 0 {
				t.Fatal("native resource host leaked a file handle")
			}
			executed++
		})
	}
	if executed != 24 {
		t.Fatal(fmt.Sprintf("original physical resource host cases:%d", executed))
	}
}
