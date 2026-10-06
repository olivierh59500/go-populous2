package populous2

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNativeRuntimeFileStoreExplicitRootIdentityAndScope(t *testing.T) {
	root := t.TempDir()
	if _, e := NewNativeRuntimeFileStore("", nil, false); e == nil {
		t.Fatal("implicitrootaccepted")
	}
	if e := os.WriteFile(filepath.Join(root, "lower.GAM"), []byte{1, 2, 3}, 0600); e != nil {
		t.Fatal(e)
	}
	store, e := NewNativeRuntimeFileStore(root, []string{"SAVES"}, false)
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	path, e := store.ResolvePath([]byte("saves:LOWER.GAM"))
	if e != nil || path != filepath.Join(store.Root, "lower.GAM") {
		t.Fatal("native ASCII identity changed", path, e)
	}
	for _, name := range []string{"../outside.GAM", "OTHER:WORLD.GAM", "/tmp/outside.GAM", "SAVES:../outside.GAM"} {
		if _, e = store.ResolvePath([]byte(name)); e == nil {
			t.Fatalf("configuredsaveboundaryescaped%s", name)
		}
	}
	outside := t.TempDir()
	if e = os.Symlink(outside, filepath.Join(root, "link")); e == nil {
		if _, e = store.ResolvePath([]byte("link/WORLD.GAM")); e == nil {
			t.Fatal("symlinksaveescapedconfiguredroot")
		}
	}
	if e = store.Delete([]byte("LOWER.GAM")); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("actualdeletefailed")
	}
}

func TestNativeRuntimeFilesSaveBorrowedRawGAMWithoutRegeneration(t *testing.T) {
	for _, async := range []bool{false, true} {
		t.Run(map[bool]string{false: "sync", true: "pending"}[async], func(t *testing.T) {
			h := nativeRuntimeHostTest(t)
			root := t.TempDir()
			store, e := NewNativeRuntimeFileStore(root, []string{"SAVES"}, async)
			if e != nil {
				t.Fatal(e)
			}
			defer store.Close()
			if _, e = h.InitializePresentation(NativeMouseSample{}); e != nil {
				t.Fatal(e)
			}
			frame := NativeFrameRegisterContext{D: [8]uint32{0x12340000, 0x56780001, 0x9abc0002, 0xdef00003, 4, 5, 6, 7}, AddressBase: 0x200000}
			for at := NativeGAMStart; at < NativeGAMEnd; at++ {
				if e = h.Memory.BSS.Write8(at, byte(at*37+11)); e != nil {
					t.Fatal(e)
				}
			}
			_ = h.Memory.BSS.Write32(0x14c, 0xd00000)
			_ = h.Memory.Code.Write16(0xa2a, 0x120)
			path := []byte("SAVES:RAW.GAM\x00")
			for i, v := range path {
				_ = h.Memory.Code.Write8(0x43c0+i, v)
			}
			before, e := h.Memory.SnapshotBSS()
			if e != nil {
				t.Fatal(e)
			}
			state := NativeRuntimeFilesState{}
			a := [7]NativeRequesterAddress{}
			a[0] = NativeRequesterAddress{Address: 0x1043c0, Code: true}
			phase := uint32(0)
			deadline := time.Now().Add(time.Second)
			var result NativeCommandFrameResult
			for {
				result, e = state.AdvanceChild(h, store, NativeStartupResetFrameCall{Routine: 0x19afc, Frame: &frame, A: &a}, &phase, NativeRuntimeFilesCallbacks{})
				if e != nil {
					t.Fatal(e)
				}
				if result.Complete {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("actualsavependingoperationstalled")
				}
				time.Sleep(time.Millisecond)
			}
			saved, e := os.ReadFile(filepath.Join(root, "RAW.GAM"))
			if e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(saved, before[NativeGAMStart:NativeGAMEnd]) {
				t.Fatal("rawsavewasregeneratedorreencoded")
			}
			after, e := h.Memory.SnapshotBSS()
			if e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(after[NativeGAMStart:NativeGAMEnd], before[NativeGAMStart:NativeGAMEnd]) {
				t.Fatal("sourceDOSsaveflushedtypedactorstate")
			}
			if state.RefreshPending {
				t.Fatal("savespuriousloadcache-refresh")
			}
			if uint16(frame.D[0]) != 1 {
				t.Fatalf("actualsavereturnD0%x", frame.D[0])
			}
			if len(store.DOS.handles) != 0 {
				t.Fatal("saveleakedactualhosthandle")
			}
		})
	}
}
