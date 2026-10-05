package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNativeWorldMenuLoadsRetainedSessionAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/world_menu_load_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			World, Mode, Profile uint16
			Registers            [8]uint32
			Hash                 string
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 1000 {
		t.Fatal("native requester world-load catalog incomplete")
	}
	b := testBundle(t)
	for _, fixture := range catalog.Cases {
		w, err := NewWorld(b, 0, false)
		if err != nil {
			t.Fatal(err)
		}
		m := w.nativeCleanupMemory()
		raw := make([]byte, 0x11280)
		for side := 0; side < 2; side++ {
			for i := 0; i < 314; i++ {
				raw[0xe8a4+side*314+i] = byte(side*71 + i*3 + 17)
			}
		}
		word := func(at int, v uint16) { binary.BigEndian.PutUint16(raw[at:], v) }
		long := func(at int, v uint32) { binary.BigEndian.PutUint32(raw[at:], v) }
		word(0xeb44, fixture.Mode)
		word(0xeb42, fixture.Profile)
		word(0xeb46, fixture.World)
		word(0xe8be, []uint16{2, 4, 18}[fixture.World%3])
		word(0xe9f8, []uint16{4, 18, 2}[fixture.World%3])
		long(0xeb24, 0x1234abcd)
		long(0xf40, 0x12345678)
		raw[0xeb57], raw[0xeb61] = 108, 110
		word(0xdc2, 0xabcd)
		for at := 0xdc2; at < len(raw); at++ {
			if err := m.Write8(at, raw[at]); err != nil {
				t.Fatal(err)
			}
		}
		w.NativeProfileSide = uint8(fixture.Profile)
		context := NativeCommandRegisterContext{}
		for i := range context.D {
			context.D[i] = 0xabcd1200 + uint32(i)
		}
		context.D[0] = 0xabcd0000 | uint32(fixture.World)
		if err := w.LoadNativeWorldMenu(b, fixture.World, &context); err != nil {
			t.Fatal(err)
		}
		for at := 0xdc2; at < len(raw); at++ {
			raw[at], err = m.Read8(at)
			if err != nil {
				t.Fatal(err)
			}
		}
		if context.D != fixture.Registers {
			t.Fatalf("world%d requester load registers differ: %x / %x", fixture.World, context.D, fixture.Registers)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(raw[0xdc2:])); got != fixture.Hash {
			t.Fatalf("world%d retained session memory differs: %s / %s", fixture.World, got, fixture.Hash)
		}
		state, err := w.NativeWorldMenuState()
		if err != nil {
			t.Fatal(err)
		}
		if state.World != fixture.World || state.RuleBits != w.Rules[fixture.Profile-1].Raw {
			t.Fatal("requester did not observe the selected live template")
		}
	}
}
