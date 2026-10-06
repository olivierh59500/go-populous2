package populous2

import "testing"

func TestNativeFollowerCodeAliasRetainsLiveOwnersAndSeams(t *testing.T) {
	ram := make([]byte, 0x30000)
	backing := commandFrameBacking(ram)
	for _, p := range [][2]int{{0x124a0, 0x1234}, {0x13350, 0x5678}, {0x13550, 0x9abc}, {0x136e8, 0xdef0}, {0x1374a, 0x4321}, {0x1374c, 0x789a}} {
		if err := backing.Write16(p[0], uint16(p[1])); err != nil {
			t.Fatal(err)
		}
	}
	state := NativeFollowerFrameState{}
	alias, err := NewNativeFollowerCodeAlias(&state, backing, 0)
	if err != nil {
		t.Fatal(err)
	}
	if state.Pass.MinimapVariant != 0x1234 || state.Pass.TownCacheFlag != 0x5678 || state.Town.Property13550 != 0x9abc || state.Town.Scratch136E8[0] != 0xdef0 {
		t.Fatal("actual CODE initialization was discarded")
	}
	state.Pass.MinimapVariant = 4
	state.Town.Scratch136E8[49] = 0xbeef
	if got, err := alias.Code.Read16(0x124a0); err != nil || got != 4 {
		t.Fatal("live pass word detached", got, err)
	}
	if got, err := alias.RAM.Read32(0x1374a); err != nil || got != 0xbeef789a {
		t.Fatal("scratch/backing seam detached", got, err)
	}
	if err := alias.Code.Write32(0x1374a, 0x11223344); err != nil {
		t.Fatal(err)
	}
	if state.Town.Scratch136E8[49] != 0x1122 || ram[0x1374c] != 0x33 || ram[0x1374d] != 0x44 {
		t.Fatal("long write lost either side of genuine seam")
	}
	if err := alias.RAM.Write8(0x13351, 0xee); err != nil {
		t.Fatal(err)
	}
	if state.Pass.TownCacheFlag != 0x56ee || state.Town.OuterFlag13350 != 0x56ee {
		t.Fatal("source cache byte did not reach call bridge")
	}
	if err := alias.Code.Write8(0x124a0, 0xff); err != nil {
		t.Fatal(err)
	}
	if state.Pass.MinimapVariant != 0xff04 || state.Town.MinimapVariant != 0xff04 {
		t.Fatal("source high byte did not reach marker owner")
	}
	// A register-bearing child writes physical addresses; CODE-offset callers
	// read the exact same bytes rather than a copied table.
	state2 := NativeFollowerFrameState{}
	alias2, err := NewNativeFollowerCodeAlias(&state2, commandFrameBacking(make([]byte, 0x230000)), 0x100000)
	if err != nil {
		t.Fatal(err)
	}
	if err := alias2.RAM.Write16(0x113550, 0x8081); err != nil {
		t.Fatal(err)
	}
	if got, err := alias2.Code.Read16(0x13550); err != nil || got != 0x8081 || state2.Town.Property13550 != 0x8081 {
		t.Fatal("physical/offset views detached", got, err)
	}
}
