package populous2

import (
	"encoding/binary"
	"errors"
	"testing"
)

func nativeSessionTestSetup(t *testing.T) (*World, *NativeFrameSession) {
	t.Helper()
	b := testBundle(t)
	w, err := NewWorld(b, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewNativeFrameSession(b, 0, 0x500000, 0x400000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Presentation.Initialize(b.Executable, NativeMouseSample{}); err != nil {
		t.Fatal(err)
	}
	s.Presentation.Input.setWord(0x138, 20)
	s.Presentation.LowTail[0x3b0-0x14c+1] = 1
	if err := w.nativeCleanupMemory().Write16(0xf3c, 1); err != nil {
		t.Fatal(err)
	}
	return w, s
}

func TestNativeFrameSessionKeepsRawBorrowAcrossPendingCommands(t *testing.T) {
	w, s := nativeSessionTestSetup(t)
	m := w.nativeCleanupMemory()
	for side := 0; side < 2; side++ {
		at := 0xeb56 + side*10
		_ = m.Write8(at+8, 2)
		_ = m.Write8(at+1, 108)
		_ = m.Write16(at+2, 0x1234)
	}
	initial := NativeFrameRegisterContext{D: [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}}
	if err := s.Begin(w, initial); err != nil {
		t.Fatal(err)
	}
	if w.nativeCallDepth != 1 {
		t.Fatal("native frame did not borrow raw World")
	}
	beforeClock := w.NativeClock
	renders, commands := 0, 0
	cb := NativeFrameSessionCallbacks{
		// This callback validates the suspension contract at the still
		// external main-render boundary; it is not native-render pixel proof.
		Render: func(_ FollowerCleanupMemory, c *NativeFrameRegisterContext, image *NativeImageRenderState, bitmap []byte, phase *uint32) (bool, error) {
			renders++
			if *phase == 0 {
				*phase = 1
				bitmap[100] = 0xa5
				c.D[4] = 0x12345678
				return false, nil
			}
			if bitmap[100] != 0xa5 || c.D[4] != 0x12345678 {
				t.Fatal("render state lost on resume")
			}
			binary.BigEndian.PutUint16(image.AudioBank[20:], 7)
			return true, nil
		},
		Execute: func(at int, c *NativeCommandRegisterContext, phase *uint32) (bool, error) {
			commands++
			if at == 0xeb56 && *phase == 0 {
				*phase = 1
				_ = m.Write32(0xdd8, 0xcafebabe)
				c.D[6] = 0x87654321
				binary.BigEndian.PutUint16(s.Image.AudioBank[20:], 9)
				return false, nil
			}
			if at == 0xeb56 && (c.D[6] != 0x87654321 || w.nativeCallDepth != 1) {
				t.Fatal("command borrow/context lost")
			}
			return true, nil
		},
	}
	if done, err := s.Advance(cb); err != nil || done || renders != 0 || s.Phase != NativeFrameSessionVBlank {
		t.Fatal("native frame bypassed real VBlank wait", done, err)
	}
	s.Presentation.Input.setWord(0xa, 1)
	if done, err := s.Advance(cb); err != nil || done || renders != 1 || s.Phase != NativeFrameSessionRender {
		t.Fatal("render suspension missing", done, err)
	}
	frontBefore := s.Presentation.Input.long(0x1a)
	if done, err := s.Advance(cb); err != nil || done || s.Pass.Stage != NativeFrameCommands {
		t.Fatal("command suspension missing", done, err)
	}
	if s.Presentation.Input.long(0x1a) == frontBefore || s.Presentation.CopperSelector != 4 {
		t.Fatal("actual native swap missing")
	}
	if binary.BigEndian.Uint16(s.Audio.Entries[20:]) != 7 || binary.BigEndian.Uint16(s.Image.AudioBank[20:]) != 9 {
		t.Fatal("shared-bank stage authority lost")
	}
	if w.nativeCallDepth != 1 {
		t.Fatal("pending frame released raw state")
	}
	if value, _ := m.Read32(0xdd8); value != 0xcafebabe {
		t.Fatal("pending raw child mutation lost")
	}
	if done, err := s.Advance(cb); err != nil || !done {
		t.Fatal("frame did not resume", done, err)
	}
	if renders != 2 || commands != 3 || s.Presentation.CopperSelector != 4 || w.nativeCallDepth != 0 || s.Phase != NativeFrameSessionIdle {
		t.Fatal("completed work replayed or borrow leaked")
	}
	if binary.BigEndian.Uint16(s.Image.AudioBank[20:]) != 9 {
		t.Fatal("late command drawing bank overwritten")
	}
	if w.NativeClock != beforeClock {
		t.Fatal("paused source clock advanced")
	}
	if s.Frame.D[4] != 0x12345678 || s.Frame.D[6] != 7 {
		t.Fatal("outer deferred MOVEM continuation lost")
	}
	for _, at := range []int{0xeb56, 0xeb60} {
		command, _ := m.Read8(at + 1)
		xy, _ := m.Read16(at + 2)
		if command != 0 || xy != 0 {
			t.Fatal("completed command not cleared")
		}
	}
}

func TestNativeFrameSessionFailurePreservesPrefixAndClosesBorrow(t *testing.T) {
	w, s := nativeSessionTestSetup(t)
	s.Presentation.Input.setWord(0xa, 1)
	if err := s.Begin(w, NativeFrameRegisterContext{}); err != nil {
		t.Fatal(err)
	}
	want := errors.New("native renderer failure")
	_, err := s.Advance(NativeFrameSessionCallbacks{Render: func(m FollowerCleanupMemory, _ *NativeFrameRegisterContext, _ *NativeImageRenderState, _ []byte, _ *uint32) (bool, error) {
		_ = m.Write32(0xdd8, 0x12345678)
		return false, want
	}})
	if !errors.Is(err, want) || w.nativeCallDepth != 0 || s.Phase != NativeFrameSessionIdle {
		t.Fatal("failed frame borrow not closed")
	}
	if value, _ := w.nativeCleanupMemory().Read32(0xdd8); value != 0x12345678 {
		t.Fatal("failed source prefix rolled back")
	}
	if err := s.Begin(w, NativeFrameRegisterContext{}); !errors.Is(err, want) {
		t.Fatal("failed frame was silently replayed")
	}
}

func TestNativeFrameSessionUsesRealPaletteWaits(t *testing.T) {
	w, s := nativeSessionTestSetup(t)
	m := s.Presentation.Memory(w.nativeCleanupMemory())
	_ = m.Write16(0x3b0, 0)
	_ = m.Write32(0xf40, 123)
	s.Presentation.Deadline1117C = 123
	s.Presentation.Input.setWord(0xa, 1)
	if err := s.Begin(w, NativeFrameRegisterContext{D: [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}}); err != nil {
		t.Fatal(err)
	}
	defer s.finish(nil)
	renders := 0
	cb := NativeFrameSessionCallbacks{Render: func(_ FollowerCleanupMemory, _ *NativeFrameRegisterContext, _ *NativeImageRenderState, _ []byte, _ *uint32) (bool, error) {
		renders++
		return false, nil
	}}
	for phase := 1; phase <= 17; phase++ {
		if phase > 1 {
			s.Presentation.Input.setWord(0xa, 1)
		}
		done, err := s.Advance(cb)
		if err != nil || done {
			t.Fatal("native fade frame unexpectedly completed", done, err)
		}
		if s.palette == nil || int(s.palette.Phase) != phase || s.Presentation.Input.word(0xa) != 0 {
			t.Fatal("real palette wait lost", phase)
		}
		if phase < 17 && renders != 0 {
			t.Fatal("render ran before native fade completed")
		}
	}
	if renders != 1 || s.Phase != NativeFrameSessionRender || s.Frame.D[7] != 17 || w.NativeClock != 123 {
		t.Fatal("native fade/paused-clock continuation differs")
	}
}

func TestNativeFrameSessionTerrainUsesSeparatePersistentTarget(t *testing.T) {
	w, s := nativeSessionTestSetup(t)
	if err := s.Begin(w, NativeFrameRegisterContext{}); err != nil {
		t.Fatal(err)
	}
	defer s.finish(nil)
	m := s.Presentation.Memory(w.nativeCleanupMemory())
	_ = m.Write32(0x22, 0xa00000)
	background := make([]byte, 32000)
	s.bitmapResolver = func(address uint32) ([]byte, error) {
		if address != 0xa00000 {
			t.Fatal("wrong terrain target", address)
		}
		return background, nil
	}
	bitmap, err := s.Presentation.BackBuffer()
	if err != nil {
		t.Fatal(err)
	}
	context := NativeCommandRegisterContext{D: [8]uint32{32, 32, 0, 0, 0, 0, 0, 0}}
	if _, err := w.commandDirectTerrain(NativeCommandCall{Routine: 0xd81e, Context: &context}, true); err != nil {
		t.Fatal(err)
	}
	changed := false
	for i, v := range background {
		if v != 0 {
			changed = true
		}
		if bitmap[i] != 0 {
			t.Fatal("terrain drew into overview target instead of22")
		}
	}
	if !changed {
		t.Fatal("actual terrain primitive did not paint persistent target22")
	}
}

func TestNativeFrameSessionRequiresInitializedDirectSoundBackend(t *testing.T) {
	w, s := nativeSessionTestSetup(t)
	if err := s.Begin(w, NativeFrameRegisterContext{}); err != nil {
		t.Fatal(err)
	}
	defer s.finish(nil)
	m := s.Presentation.Memory(w.nativeCleanupMemory())
	_ = m.Write32(0x3b4, 0x800000)
	if err := w.nativeEntryCallbacks().Sound(0x35c); err == nil {
		t.Fatal("initialized audio silently skipped actual direct cue")
	}
	device, err := NewNativeAudioDevice(testBundle(t).Executable, testBundle(t).Raw["fx.dat"], 0x100000, 0x800000, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := device.Initialize(); err != nil {
		t.Fatal(err)
	}
	calls := 0
	s.directSound = func(raw uint16) error { calls++; return device.DirectCue(raw, &s.Frame) }
	if err := w.nativeEntryCallbacks().Sound(0x35c); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("native direct cue not delivered exactly once")
	}
}
