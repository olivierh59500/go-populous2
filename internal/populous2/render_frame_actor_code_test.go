package populous2

import (
	"errors"
	"testing"
)

func TestNativeActorTownHitHeightUsesCanonicalCodeAcrossCallers(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	r, err := DecodeNativeActorRenderRules(h.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Frames.BindCode(h.Memory.Code, h.Code.Logical().Read32); err != nil {
		t.Fatal(err)
	}
	selected, world := NativeActorRenderState{TownHitHeight: 3}, NativeActorRenderState{TownHitHeight: 7}
	if err := r.setTownHitHeight(&selected, 20); err != nil {
		t.Fatal(err)
	}
	if got, err := h.Memory.RAM.Read16(int(h.Memory.CodeBase) + 0xe8ce); err != nil || got != 20 {
		t.Fatal("native producer detached from physical CODE", got, err)
	}
	// A second caller and a retained/raw CODE write must be visible without
	// copying either actor state or rebuilding its rules at a frame boundary.
	if err := h.Memory.RAM.Write16(int(h.Memory.CodeBase)+0xe8ce, 0xfffe); err != nil {
		t.Fatal(err)
	}
	if got, err := r.townHitHeight(&world); err != nil || got != 0xfffe || world.TownHitHeight != got || selected.TownHitHeight != 20 {
		t.Fatal("native consumer used stale caller state", got, err)
	}
	if err := r.setTownHitHeight(&world, 12); err != nil {
		t.Fatal(err)
	}
	if got, err := r.townHitHeight(&selected); err != nil || got != 12 {
		t.Fatal("selected/world children lost shared CODE", got, err)
	}
}

func TestNativeActorTownHitHeightPropagatesUnavailableLiveCode(t *testing.T) {
	b := testBundle(t)
	r, err := DecodeNativeActorRenderRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	want := errors.New("explicit unavailable E8CE")
	m := commandFrameBacking(make([]byte, 0x3fa2c))
	m.Write16 = func(int, uint16) error { return want }
	if err := r.Frames.BindCode(m, m.Read32); err != nil {
		t.Fatal(err)
	}
	s := NativeActorRenderState{}
	if err := r.setTownHitHeight(&s, 20); !errors.Is(err, want) || s.TownHitHeight != 20 {
		t.Fatal("source write prefix/error was discarded", s, err)
	}
	m.Write16 = nil
	if err := r.Frames.BindCode(m, m.Read32); err != nil {
		t.Fatal(err)
	}
	if err := r.setTownHitHeight(&s, 24); err == nil {
		t.Fatal("read-only live CODE silently accepted source write")
	}
	m.Read16 = func(int) (uint16, error) { return 0, want }
	if err := r.Frames.BindCode(m, m.Read32); err != nil {
		t.Fatal(err)
	}
	if _, err := r.townHitHeight(&s); !errors.Is(err, want) || s.TownHitHeight != 24 {
		t.Fatal("source consumer discarded read failure", s, err)
	}
}
