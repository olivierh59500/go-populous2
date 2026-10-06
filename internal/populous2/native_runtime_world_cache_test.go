package populous2

import (
	"bytes"
	"fmt"
	"testing"
)

func TestNativeRuntimeWorldCachesChangeWithoutRegeneratingRawState(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	for land := 0; land < 4; land++ {
		index := 0
		for i, level := range h.Bundle.Levels {
			if level.Terrain == land {
				index = i
				break
			}
		}
		if err := h.World.retainNativeLAND(h.Bundle.Raw[fmt.Sprintf("land%d.dat", land)]); err != nil {
			t.Fatal(err)
		}
		if err := h.Memory.BSS.Write16(0xeb46, uint16(index)); err != nil {
			t.Fatal(err)
		}
		if err := h.Memory.BSS.Write16(0xeb22, uint16(land)); err != nil {
			t.Fatal(err)
		}
		if err := h.Memory.BSS.Write16(0xeb44, 6); err != nil {
			t.Fatal(err)
		}
		if err := h.Memory.BSS.Write16(0xeb42, 2); err != nil {
			t.Fatal(err)
		}
		if err := h.Memory.BSS.Write32(0x76f4+26, 0x12345678); err != nil {
			t.Fatal(err)
		}
		before, err := h.Memory.SnapshotBSS()
		if err != nil {
			t.Fatal(err)
		}
		code := append([]byte(nil), h.Code.RawData()...)
		if err := h.RefreshWorldCaches(); err != nil {
			t.Fatal(err)
		}
		after, err := h.Memory.SnapshotBSS()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) || !bytes.Equal(code, h.Code.RawData()) {
			t.Fatal("cache update rewrote source-owned world or CODE")
		}
		if h.World.Level.Terrain != land || h.World.Landscape != h.Bundle.Landscapes[land] || h.Session.followerRules.landscape != h.World.Landscape {
			t.Fatal("loaded LAND rules remained stale")
		}
		if h.World.NativeGameMode != 6 || h.World.NativeProfileSide != 2 || !h.World.Custom {
			t.Fatal("cache refresh retained stale native session metadata")
		}
		if err := h.Session.BeginRaw(h.World, NativeFrameRegisterContext{AddressBase: 0x200000}); err != nil {
			t.Fatal(err)
		}
		if err := h.RefreshWorldCaches(); err == nil {
			t.Fatal("cache update admitted a borrowed frame")
		}
		h.Session.finish(nil)
	}
}

func TestNativeRuntimeWorldCachesRequireRealLoadedLand(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	if err := h.Memory.BSS.Write16(0xeb22, 2); err != nil {
		t.Fatal(err)
	}
	if err := h.RefreshWorldCaches(); err == nil {
		t.Fatal("unloaded LAND cached as if startup completed")
	}
}

func TestNativeRuntimeResultCacheRefreshPreservesBorrowedRawWorld(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	if err := h.RefreshResultWorldCaches(); err == nil {
		t.Fatal("idle runtime admitted the retained result boundary")
	}
	if err := h.Session.BeginRaw(h.World, NativeFrameRegisterContext{AddressBase: h.Memory.BSSBase}); err != nil {
		t.Fatal(err)
	}
	defer h.Session.finish(nil)
	if err := h.RefreshResultWorldCaches(); err == nil {
		t.Fatal("unfinished follower frame admitted a result refresh")
	}
	land := 1
	if err := h.World.retainNativeLAND(h.Bundle.Raw[fmt.Sprintf("land%d.dat", land)]); err != nil {
		t.Fatal(err)
	}
	for _, patch := range []nativeHeroPatch{{0xeb46, 2, 1}, {0xeb22, 2, uint32(land)}, {0xeb44, 2, 2}, {0xeb42, 2, 1}, {0x76f4 + 26, 4, 0x12345678}} {
		renderFramePatch(h.Memory.BSS, patch)
	}
	h.Session.Phase = NativeFrameSessionPhysics
	h.Session.Pass.Stage = NativeFrameFollowers
	h.Session.followersCompleted, h.Session.resultPending = true, true
	before, err := h.Memory.SnapshotBSS()
	if err != nil {
		t.Fatal(err)
	}
	code := append([]byte(nil), h.Code.RawData()...)
	if err := h.RefreshResultWorldCaches(); err != nil {
		t.Fatal(err)
	}
	after, err := h.Memory.SnapshotBSS()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || !bytes.Equal(code, h.Code.RawData()) {
		t.Fatal("result cache update rewrote source-owned BSS/CODE")
	}
	if h.World.nativeCallDepth != 1 || h.Session.world != h.World || !h.Session.resultPending || h.Session.Pass.Stage != NativeFrameFollowers {
		t.Fatal("result cache update released or advanced the retained frame")
	}
	if h.World.Landscape != h.Bundle.Landscapes[land] || h.Session.followerRules.landscape != h.World.Landscape || h.World.Level.Terrain != land {
		t.Fatal("following native AI/effects would use the old LAND rules")
	}
}
