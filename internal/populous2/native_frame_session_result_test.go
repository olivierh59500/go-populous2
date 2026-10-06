package populous2

import "testing"

func TestNativeFrameSessionRetainsResultAfterRecountWithoutRepeatingFollowers(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	if _, err := h.InitializePresentation(NativeMouseSample{}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []nativeHeroPatch{{0x138, 2, 20}, {0x3b0, 2, 1}, {0xeb44, 2, 4}, {0xe8bc, 2, 1}, {0xe9f6, 2, 2}} {
		renderFramePatch(h.Memory.BSS, p)
	}
	if err := h.Session.BeginRaw(h.World, NativeFrameRegisterContext{AddressBase: 0x200000}); err != nil {
		t.Fatal(err)
	}
	h.Session.Presentation.Input.setWord(0xa, 1)
	calls := 0
	cb := NativeFrameSessionCallbacks{Render: func(FollowerCleanupMemory, *NativeFrameRegisterContext, *NativeImageRenderState, []byte, *uint32) (bool, error) {
		return true, nil
	}, ResultAdvance: func(identity uint16, c *NativeFrameRegisterContext) (bool, error) {
		calls++
		if identity != 1 || h.World.nativeCallDepth != 1 {
			t.Fatal("native result identity/borrow changed", identity)
		}
		if calls == 1 {
			c.D[2] = 0xcafebabe
			return false, nil
		}
		if c.D[2] != 0xcafebabe {
			t.Fatal("result wait lost data registers")
		}
		return false, nil
	}}
	if done, err := h.Session.Advance(cb); err != nil || done || calls != 1 || h.Session.Pass.Stage != NativeFrameFollowers {
		t.Fatal("native result wait not retained", done, err, calls)
	}
	if err := h.Memory.BSS.Write32(0xe8a8, 0x12345678); err != nil {
		t.Fatal(err)
	}
	if done, err := h.Session.Advance(cb); err != nil || done || calls != 2 {
		t.Fatal("result did not resume", done, err, calls)
	}
	if value, err := h.Memory.BSS.Read32(0xe8a8); err != nil || value != 0x12345678 {
		t.Fatal("pending result reran follower population recount", value, err)
	}
	h.Session.finish(nil)
}
