package populous2

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
)

type followerDecisionFrameFixture struct {
	Input struct {
		Name      string
		Initial   []nativeHeroPatch
		Registers [8]uint32
		Seed      uint32
		Tile      uint8
	}
	Registers [8]uint32
	Changes   []nativeHeroPatch
	Calls     []struct {
		PC        uint32
		Reference uint16
		God       uint32
		Registers [8]uint32
	}
	Boundary uint32
	Trap     bool
}

func decisionFrameFixtureMemory(f followerDecisionFrameFixture) *scenarioScriptMemory {
	m := &scenarioScriptMemory{}
	for i := 0; i < 4096; i++ {
		m[0xf44+i*4], m[0xf45+i*4] = 0xa8, f.Input.Tile
	}
	at := 0x76f4
	m[at], m[at+12], m[at+18], m[at+22] = 2, 1, 20, 2
	_ = m.write16(at+6, 0x2080)
	_ = m.write16(at+8, 0x2080)
	_ = m.write16(at+14, 20)
	_ = m.write32(at+26, 1000)
	_ = m.write16(0xf46+32*256+32*4, 52)
	_ = m.write32(0xeb28, f.Input.Seed)
	_ = m.write16(0xe76a+314+12, 14)
	for _, p := range f.Input.Initial {
		applyPreHUDPatch(tinyMemoryWriter(m), p)
	}
	return m
}

// This fixture child executes the already proven raw cleanup body. It models
// only $130e4's own assignments around that child, not a recorded-delta replay.
func decisionFrameFixtureAttrition(m *scenarioScriptMemory, ref NativeRecordReference, god int, c *NativeFrameRegisterContext, cleanup func(NativeRecordReference, *NativeFrameRegisterContext) error) (bool, error) {
	at := cleanupRecordAddress(ref)
	population, err := m.read32(at + 26)
	if err != nil {
		return false, err
	}
	amount, err := m.read32(god + 20)
	if err != nil {
		return false, err
	}
	c.D[0] = population - amount
	if err := m.write32(at+26, c.D[0]); err != nil {
		return false, err
	}
	if int32(c.D[0]) > 0 {
		c.D[0] = 0
		return false, nil
	}
	c.D[0] = 1
	if err := cleanup(ref, c); err != nil {
		return false, err
	}
	if err := m.write16(at+10, 0x7f4); err != nil {
		return false, err
	}
	if m[at+13]&2 != 0 {
		if err := m.write16(at+10, 0x9d4); err != nil {
			return false, err
		}
		m[at+22] = 0x40
	} else {
		m[at+22] = 0x2c
	}
	c.D[0] = 1
	return true, nil
}

func TestFollowerSearchFullFrameAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_decision_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []followerDecisionFrameFixture
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 835 {
		t.Fatal("native search full-frame corpus incomplete")
	}
	r, err := DecodeFollowerDecisionFrameRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[uint32]int{}
	traps := 0
	for _, f := range catalog.Cases {
		counts[f.Boundary]++
		if f.Trap {
			traps++
		}
		t.Run(f.Input.Name, func(t *testing.T) {
			m := decisionFrameFixtureMemory(f)
			expected := *m
			for _, p := range f.Changes {
				applyPreHUDPatch(tinyMemoryWriter(&expected), p)
			}
			c := NativeFrameRegisterContext{D: f.Input.Registers, AddressBase: 0x200000}
			calls := 0
			checkCall := func(pc uint32, ref NativeRecordReference, god int, c *NativeFrameRegisterContext) {
				if calls >= len(f.Calls) {
					t.Fatal("unexpected native child")
				}
				want := f.Calls[calls]
				calls++
				if want.PC != pc || want.Reference != uint16(ref) || want.Registers != c.D || (pc == 0x130e4 && want.God != uint32(god)) {
					t.Fatalf("native child input differs: got%x/%x/%x/%x want%+v", pc, ref, god, c.D, want)
				}
			}
			cb := FollowerDecisionFrameCallbacks{Memory: m.callbacks(), Frame: &c, AttritionFrame: func(ref NativeRecordReference, god int, c *NativeFrameRegisterContext) (bool, error) {
				checkCall(0x130e4, ref, god, c)
				return decisionFrameFixtureAttrition(m, ref, god, c, func(ref NativeRecordReference, c *NativeFrameRegisterContext) error {
					checkCall(0x124a2, ref, god, c)
					_, err := CleanupFollowerWithFrame(ref, c, FollowerCleanupCallbacks{Memory: m.callbacks(), Unlink: func(ref NativeRecordReference) error {
						t.Fatal("mode1 attrition must not unlink")
						return nil
					}, Insert: func(ref NativeRecordReference) error {
						t.Fatal("ordinary attrition must not insert")
						return nil
					}, ClearFarms: func(ref NativeRecordReference, tile uint8) error {
						t.Fatal("ordinary search attrition must not clear farms")
						return nil
					}})
					return err
				})
			}}
			step, err := r.Search(52, cb)
			trap := errors.Is(err, ErrFollowerZeroSpeed) || errors.Is(err, ErrFollowerStationarySearch)
			if trap != f.Trap || (err != nil && !trap) {
				t.Fatalf("native search trap differs: got%v want%v", err, f.Trap)
			}
			if step.Boundary != f.Boundary || calls != len(f.Calls) {
				t.Fatalf("native search continuation differs: got%x/%d want%x/%d", step.Boundary, calls, f.Boundary, len(f.Calls))
			}
			if c.D != f.Registers {
				t.Fatalf("native search full registers differ: got%x want%x", c.D, f.Registers)
			}
			if *m != expected {
				for i := range m {
					if m[i] != expected[i] {
						t.Fatalf("native search BSS differs at%#x: got%02x want%02x", i, m[i], expected[i])
					}
				}
			}
		})
	}
	for _, b := range []uint32{0x112b8, 0x12044, 0x11bb4, 0x123b4, 0x1156c} {
		if counts[b] == 0 {
			t.Fatalf("native search boundary%#x untested", b)
		}
	}
	if traps == 0 {
		t.Fatal("native search DIVU0 traps untested")
	}
}

func TestFollowerSearchLeavesUnprovedChildrenExplicit(t *testing.T) {
	r, err := DecodeFollowerDecisionFrameRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	m := decisionFrameFixtureMemory(followerDecisionFrameFixture{})
	c := NativeFrameRegisterContext{D: [8]uint32{1, 2, 0x13572468, 4, 5, 6, 7, 8}}
	before := *m
	step, err := r.Search(52, FollowerDecisionFrameCallbacks{Memory: m.callbacks(), Frame: &c})
	if err != nil || step.Boundary != 0x130e4 || *m != before || c.D[2] != 314 {
		t.Fatalf("missing attrition child must retain exact source boundary: %+v %v %x", step, err, c.D)
	}
}

func TestFollowerSearchRejectsCorruptChainsAndOddTablesWithoutNormalization(t *testing.T) {
	r, err := DecodeFollowerDecisionFrameRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"odd-table", "cyclic-chain"} {
		t.Run(mode, func(t *testing.T) {
			m := decisionFrameFixtureMemory(followerDecisionFrameFixture{Input: struct {
				Name      string
				Initial   []nativeHeroPatch
				Registers [8]uint32
				Seed      uint32
				Tile      uint8
			}{Tile: 1}})
			if mode == "odd-table" {
				m[0x76f4+24] = 1
			} else {
				_ = m.write16(0xf46+31*256+31*4, 104)
				m[0x7728], m[0x7728+12] = 22, 3
				_ = m.write16(0x7728+2, 104)
			}
			c := NativeFrameRegisterContext{}
			step, err := r.Search(52, FollowerDecisionFrameCallbacks{Memory: m.callbacks(), Frame: &c, AttritionFrame: func(ref NativeRecordReference, god int, c *NativeFrameRegisterContext) (bool, error) {
				return decisionFrameFixtureAttrition(m, ref, god, c, nil)
			}})
			if err == nil || step.Boundary != 0 || m[0x76f4+22] != 2 || m[0x76f4+24] != map[string]uint8{"odd-table": 1, "cyclic-chain": 0}[mode] {
				t.Fatalf("corrupt raw input silently normalized: %+v %v", step, err)
			}
		})
	}
}
