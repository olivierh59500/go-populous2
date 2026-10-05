package populous2

import (
	"encoding/binary"
	"testing"
)

func TestNativeRuntimeImageAudioOwnerChangesAtSourceBoundaries(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	s := h.Session
	if _, err := h.InitializePresentation(NativeMouseSample{}); err != nil {
		t.Fatal(err)
	}
	for _, patch := range []struct {
		at    int
		value uint16
	}{{0x138, 20}, {0x3b0, 1}, {0xf3c, 1}} {
		if err := h.Memory.BSS.Write16(patch.at, patch.value); err != nil {
			t.Fatal(err)
		}
	}
	for _, at := range []int{0xeb56, 0xeb60} {
		if err := h.Memory.BSS.Write8(at+8, 2); err != nil {
			t.Fatal(err)
		}
		if err := h.Memory.BSS.Write8(at+1, 108); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.BeginRaw(h.World, NativeFrameRegisterContext{AddressBase: 0x200000}); err != nil {
		t.Fatal(err)
	}
	commands := 0
	cb := NativeFrameSessionCallbacks{Render: func(_ FollowerCleanupMemory, _ *NativeFrameRegisterContext, image *NativeImageRenderState, _ []byte, phase *uint32) (bool, error) {
		if h.ImageAudioCode.Owner() != NativeImageCodeOwner {
			t.Fatal("render did not own shared image queue")
		}
		binary.BigEndian.PutUint16(image.AudioBank[20:], 7)
		if *phase == 0 {
			*phase = 1
			return false, nil
		}
		return true, nil
	}, Execute: func(at int, _ *NativeCommandRegisterContext, phase *uint32) (bool, error) {
		commands++
		if h.ImageAudioCode.Owner() != NativeImageCodeOwner {
			t.Fatal("deferred UI retained audio authority after swap")
		}
		if at == 0xeb56 && *phase == 0 {
			*phase = 1
			if err := h.Memory.Code.Write16(0x185a8+20, 9); err != nil {
				return false, err
			}
			return false, nil
		}
		return true, nil
	}}
	s.Presentation.Input.setWord(0xa, 1)
	if done, err := s.Advance(cb); err != nil || done || s.Phase != NativeFrameSessionRender {
		t.Fatal("render wait did not retain source frame", done, err)
	}
	if got, err := h.Memory.Code.Read16(0x185a8 + 20); err != nil || got != 7 {
		t.Fatal("physical queue did not see pending image mutation", got, err)
	}
	if done, err := s.Advance(cb); err != nil || done {
		t.Fatal("deferred UI wait missing", done, err)
	}
	if binary.BigEndian.Uint16(s.Audio.Entries[20:]) != 7 || binary.BigEndian.Uint16(s.Image.AudioBank[20:]) != 9 {
		t.Fatal("source queue copies lost phase authority")
	}
	if done, err := s.Advance(cb); err != nil || !done || commands != 3 {
		t.Fatal("retained command completion failed", done, commands, err)
	}
	if got, err := h.Memory.Code.Read16(0x185a8 + 20); err != nil || got != 9 {
		t.Fatal("completion erased deferred image sound", got, err)
	}
}
