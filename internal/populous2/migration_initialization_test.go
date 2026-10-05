package populous2

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

func prototypeInitializationSnapshot(t *testing.T) Snapshot {
	t.Helper()
	w, err := NewWorld(testBundle(t), 27, false)
	if err != nil {
		t.Fatal(err)
	}
	s := w.Snapshot()
	s.Version = 26
	for player := range 2 {
		god, _ := NativeDeityAddress(uint8(player + 1))
		offset := god - NativeMagnetImageStart
		clear(s.NativeGlobals.Bytes[offset+0x5a : offset+0x11c])
		clear(s.NativeGlobals.Bytes[offset+0x1a : offset+0x1c])
	}
	clear(s.NativeCommandBytes[:])
	return s
}

func TestSnapshot27PrototypeMigrationPreservesLiveState(t *testing.T) {
	s := prototypeInitializationSnapshot(t)
	s.Experience = [2][6]uint8{{1, 2, 3, 4, 5, 6}, {11, 12, 13, 14, 15, 16}}
	s.Deity.Name = "A LONG OLDER JSON NAME"
	s.Deity.Experience = s.Experience[0]
	s.Deity.Bolts = 17
	s.NativeClock = 12345
	s.Core.GameTurn = 12345
	s.NativeRaiseEnabled = 1
	s.NativeControlBytes[0xf0a-0xdc4+1] = 18
	s.NativeControlBytes[0xdde-0xdc4] = 0x12
	s.NativeControlBytes[0xddf-0xdc4] = 0x34
	s.NativeCommandBytes[0xeb57-0xeb18] = 64
	s.NativeCommandBytes[0xeb58-0xeb18] = 12
	s.NativeCommandBytes[0xeb59-0xeb18] = 13
	s.NativeCommandBytes[0xeb61-0xeb18] = 46
	s.NativeCommandBytes[0xeb62-0xeb18] = 21
	s.NativeCommandBytes[0xeb63-0xeb18] = 22
	s.NativeCommandBytes[0xeb6e-0xeb18+1] = 42
	for player := range 2 {
		s.Core.Magnets[player].Mana = 9000 + player
		offset := 0xe8a4 + player*314 - NativeMagnetImageStart
		binary.BigEndian.PutUint16(s.NativeGlobals.Bytes[offset+12:], uint16(16+player*2))
		binary.BigEndian.PutUint16(s.NativeGlobals.Bytes[offset+14:], 0x5140)
		binary.BigEndian.PutUint16(s.NativeGlobals.Bytes[offset+0x26:], 73)
		binary.BigEndian.PutUint16(s.NativeGlobals.Bytes[offset+0x44:], 77)
		binary.BigEndian.PutUint16(s.NativeGlobals.Bytes[offset+0x46:], 9)
		binary.BigEndian.PutUint16(s.NativeGlobals.Bytes[offset+0x48:], 31)
	}
	beforeGlobals := s.NativeGlobals
	w, err := Restore(testBundle(t), s)
	if err != nil {
		t.Fatal(err)
	}
	if w.Experience != s.Experience || w.Deity.Bolts != 17 || w.Deity.Name != s.Deity.Name || w.NativeClock != 12345 || w.Core.RandomState() != s.Core.RNG || w.NativeRaiseEnabled != 1 {
		t.Fatal("prototype initialization replayed live profile/clock/RNG state")
	}
	if w.NativeControlBytes != s.NativeControlBytes {
		t.Fatal("prototype migration reset live script or cursor")
	}
	for player := range 2 {
		offset := 0xe8a4 + player*314 - NativeMagnetImageStart
		if !bytes.Equal(w.NativeGlobals.Bytes[offset:offset+0x1a], beforeGlobals.Bytes[offset:offset+0x1a]) || !bytes.Equal(w.NativeGlobals.Bytes[offset+0x1c:offset+0x5a], beforeGlobals.Bytes[offset+0x1c:offset+0x5a]) {
			t.Fatal("prototype migration changed live deity stats/modes/XP/pending state")
		}
		if value, _ := w.runtimeMemory().Read16(0xe8a4 + player*314 + 0x94); value == 0 {
			t.Fatal("missing prototype policy choices were not compiled")
		}
	}
	for _, address := range []int{0xeb57, 0xeb58, 0xeb59, 0xeb61, 0xeb62, 0xeb63, 0xeb6f} {
		if w.NativeCommandBytes[address-0xeb18] != s.NativeCommandBytes[address-0xeb18] {
			t.Fatal("prototype migration reset queued payload/render bytes")
		}
	}
	if w.NativeCommandBytes[0xeb56-0xeb18] != 1 || w.NativeCommandBytes[0xeb60-0xeb18] != 2 || w.NativeCommandBytes[0xeb5e-0xeb18] != 2 || w.NativeCommandBytes[0xeb68-0xeb18] != 4 {
		t.Fatal("prototype command defaults remain uninitialized")
	}
	if profile, _ := w.nativeCleanupMemory().Read16(0xeb42); profile != 1 {
		t.Fatal("prototype profile header was not bound")
	}
}

func TestSnapshot27MigrationPreservesExistingTemplatesAndPolicies(t *testing.T) {
	s := prototypeInitializationSnapshot(t)
	offset := 0xe8a4 - NativeMagnetImageStart
	s.NativeGlobals.Bytes[offset+0x5a] = 0xa5
	s.NativeGlobals.Bytes[offset+0x9c] = 0xb6
	before := s.NativeGlobals
	w, err := Restore(testBundle(t), s)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(w.NativeGlobals.Bytes[offset+0x5a:offset+0x11c], before.Bytes[offset+0x5a:offset+0x11c]) {
		t.Fatal("prototype migration normalized a nonzero raw template/policy")
	}
	newer := s
	newer.Version = 27
	w, err = Restore(testBundle(t), newer)
	if err != nil {
		t.Fatal(err)
	}
	if w.NativeCommandBytes != newer.NativeCommandBytes || w.NativeGlobals != newer.NativeGlobals {
		t.Fatal("current-version save was subjected to prototype migration")
	}
}

func TestSnapshot27MigrationExemptsNativeGAMState(t *testing.T) {
	data, err := os.ReadFile("../../.local/native-audit/extracted-pop2-b/ARNY 1.GAM")
	if os.IsNotExist(err) {
		t.Skip("supplied original GAM remains private")
	}
	if err != nil {
		t.Fatal(err)
	}
	w, err := ImportNativeGAM(testBundle(t), data)
	if err != nil {
		t.Fatal(err)
	}
	s := w.Snapshot()
	s.Version = 26
	loaded, err := Restore(testBundle(t), s)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := loaded.ExportNativeGAM()
	if err != nil {
		t.Fatal(err)
	}
	assertNativeGAMBytes(t, actual, data)
}
