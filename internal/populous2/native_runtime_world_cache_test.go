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
