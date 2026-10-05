package populous2

import (
	"encoding/json"
	"os"
	"testing"
)

type followerCombatFrameFixture struct {
	Input struct {
		Name, Mode string
		Initial    []nativeHeroPatch
		Registers  [8]uint32
		Seed       uint32
	}
	Registers [8]uint32
	Changes   []nativeHeroPatch
	Calls     []struct {
		PC                       uint32
		Reference, Winner, Loser uint16
		Registers                [8]uint32
	}
	Boundary       uint32
	MinimapVariant uint16
}

func combatFrameFixtureMemory(f followerCombatFrameFixture) *scenarioScriptMemory {
	m := &scenarioScriptMemory{}
	_ = m.write32(0xeb28, f.Input.Seed)
	for i := 0; i < 2; i++ {
		a := 0x76f4 + i*52
		m[a], m[a+12], m[a+22], m[a+25] = 2, uint8(i+1), 14+uint8(i)*2, 3+uint8(i)*4
		_ = m.write16(a+10, 0x1c8)
		_ = m.write16(a+6, 0x2080)
		_ = m.write16(a+8, 0x2080)
		_ = m.write32(a+26, 1000)
		_ = m.write16(a+30, uint16(104-i*52))
	}
	for _, p := range f.Input.Initial {
		applyPreHUDPatch(tinyMemoryWriter(m), p)
	}
	return m
}

func combatFrameEmptyGraphUnlink(m *scenarioScriptMemory, ref NativeRecordReference) error {
	var grid NativeOccupancyState
	for i := range grid.Cells {
		a := 0xf44 + i*4
		h, _ := m.read16(a + 2)
		grid.Cells[i] = NativeOccupancyCell{Header: m[a], Tile: m[a+1], Head: NativeRecordReference(h)}
	}
	access := NativeRecordAccess{
		Record: func(ref NativeRecordReference) (NativeOccupancyRecord, bool) {
			a := cleanupRecordAddress(ref)
			if a < 0 || a+10 > len(m) {
				return NativeOccupancyRecord{}, false
			}
			next, _ := m.read16(a + 2)
			prev, _ := m.read16(a + 4)
			x, _ := m.read16(a + 6)
			y, _ := m.read16(a + 8)
			return NativeOccupancyRecord{Next: NativeRecordReference(next), Previous: NativeRecordReference(prev), X: x, Y: y}, true
		},
		SetLinks: func(ref, next, prev NativeRecordReference) {
			a := cleanupRecordAddress(ref)
			_ = m.write16(a+2, uint16(next))
			_ = m.write16(a+4, uint16(prev))
		},
		SetPosition: func(ref NativeRecordReference, x, y uint16) {
			a := cleanupRecordAddress(ref)
			_ = m.write16(a+6, x)
			_ = m.write16(a+8, y)
		},
	}
	if err := grid.Remove(ref, access); err != nil {
		return err
	}
	for i, cell := range grid.Cells {
		a := 0xf44 + i*4
		m[a], m[a+1] = cell.Header, cell.Tile
		_ = m.write16(a+2, uint16(cell.Head))
	}
	return nil
}

func TestFollowerCombatFullFrameAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_combat_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []followerCombatFrameFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 773 {
		t.Fatal("native combat frame corpus incomplete")
	}
	r, err := DecodeFollowerCombatFrameRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[uint32]int{}
	for _, f := range catalog.Cases {
		counts[f.Boundary]++
		t.Run(f.Input.Name, func(t *testing.T) {
			m := combatFrameFixtureMemory(f)
			expected := *m
			for _, p := range f.Changes {
				applyPreHUDPatch(tinyMemoryWriter(&expected), p)
			}
			frame := NativeFrameRegisterContext{D: f.Input.Registers, AddressBase: 0x200000}
			variant := uint16(0)
			calls := 0
			cb := FollowerCombatFrameCallbacks{Memory: m.callbacks(), Frame: &frame, SetMinimapVariant: func(v uint16) error { variant = v; return nil }, CleanupFrame: func(ref NativeRecordReference, c *NativeFrameRegisterContext) error {
				if calls >= len(f.Calls) {
					t.Fatal("unexpected native cleanup callback")
				}
				want := f.Calls[calls]
				calls++
				if want.PC != 0x124a2 || want.Reference != uint16(ref) || want.Registers != c.D {
					t.Fatalf("native cleanup full context differs: got%x/%x want%x/%x", ref, c.D, want.Reference, want.Registers)
				}
				_, err := CleanupFollowerWithFrame(ref, c, FollowerCleanupCallbacks{Memory: m.callbacks(), Unlink: func(ref NativeRecordReference) error { return combatFrameEmptyGraphUnlink(m, ref) }, Insert: func(ref NativeRecordReference) error {
					t.Fatal("unexpected native insertion during ordinary cleanup")
					return nil
				}, ClearFarms: func(ref NativeRecordReference, tile uint8) error {
					t.Fatal("unexpected farm callback in ordinary combat fixture")
					return nil
				}})
				return err
			}}
			var step FollowerCombatFrameStep
			if f.Input.Mode == "defender" {
				step, err = r.Defender(52, cb)
			} else {
				step, err = r.Aggressor(52, cb)
			}
			if err != nil {
				t.Fatal(err)
			}
			if step.Boundary != f.Boundary || variant != f.MinimapVariant {
				t.Fatalf("native combat continuation/variant differs: got%x/%d want%x/%d", step.Boundary, variant, f.Boundary, f.MinimapVariant)
			}
			if step.Boundary == 0x1298c {
				if calls >= len(f.Calls) {
					t.Fatal("native winner boundary missing")
				}
				want := f.Calls[calls]
				calls++
				if want.PC != 0x1298c || want.Winner != uint16(step.Winner) || want.Loser != uint16(step.Loser) || frame.D != want.Registers {
					t.Fatalf("native winner input differs: %+v got%x want%+v", step, frame.D, want)
				}
			}
			if calls != len(f.Calls) {
				t.Fatal("native child callback count differs")
			}
			if frame.D != f.Registers {
				t.Fatalf("native combat full registers differ: got%x want%x", frame.D, f.Registers)
			}
			if *m != expected {
				for a, v := range m {
					if v != expected[a] {
						t.Fatalf("native combat BSS byte%x got%x want%x", a, v, expected[a])
					}
				}
			}
		})
	}
	if counts[0x1298c] == 0 || counts[0x12462] == 0 || counts[0x1131c] == 0 || counts[0x123b4] == 0 {
		t.Fatalf("native combat outcome coverage incomplete: %v", counts)
	}
}

func TestFollowerCombatFrameUsesActualReturnedWinnerA3(t *testing.T) {
	r, err := DecodeFollowerCombatFrameRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, returned := range []NativeRecordReference{52, 104, 156} {
		f := followerCombatFrameFixture{}
		f.Input.Seed = 59500
		m := combatFrameFixtureMemory(f)
		_ = m.write32(0x7728+26, 1)
		frame := NativeFrameRegisterContext{}
		step, err := r.Aggressor(52, FollowerCombatFrameCallbacks{Memory: m.callbacks(), Frame: &frame, SetMinimapVariant: func(uint16) error { return nil }, WinFrame: func(w, l, originalA0 NativeRecordReference, c *NativeFrameRegisterContext) (NativeRecordReference, error) {
			if originalA0 != 52 {
				t.Fatal("winner callback lost originalA0")
			}
			return returned, nil
		}})
		if err != nil {
			t.Fatal(err)
		}
		want := uint32(0x123b4)
		if returned == 52 {
			want = 0x12462
		}
		if step.Boundary != want {
			t.Fatal("winner continuation was inferred from initial winner/loser instead of actualA3")
		}
	}
}
