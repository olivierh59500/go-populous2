package populous2

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
)

type followerMotionFrameFixture struct {
	Input struct {
		Name, Mode            string
		Initial               []nativeHeroPatch
		Registers             [8]uint32
		Ref, TargetX, TargetY uint16
	}
	Registers [8]uint32
	Changes   []nativeHeroPatch
	Calls     []struct {
		PC        uint32
		Registers [8]uint32
		X, Y      uint16
	}
	Boundary uint32
	Trap     bool
}

func motionFrameFixtureMemory(f followerMotionFrameFixture) *scenarioScriptMemory {
	m := &scenarioScriptMemory{}
	for i := 0; i < 4096; i++ {
		m[0xf44+i*4], m[0xf45+i*4] = 0xa8, 15
	}
	at := cleanupRecordAddress(NativeRecordReference(f.Input.Ref))
	m[at], m[at+12], m[at+18], m[at+22], m[at+23] = 2, 1, 20, 4, 2
	_ = m.write16(at+6, 0x2080)
	_ = m.write16(at+8, 0x2080)
	_ = m.write16(at+14, 20)
	_ = m.write16(at+20, 5)
	_ = m.write32(at+26, 1000)
	_ = m.write16(0xf46+32*256+32*4, f.Input.Ref)
	for _, p := range f.Input.Initial {
		applyPreHUDPatch(tinyMemoryWriter(m), p)
	}
	return m
}

func motionFrameFixtureGraphMove(m *scenarioScriptMemory, ref NativeRecordReference, x, y uint16, c *NativeFrameRegisterContext) error {
	if err := c.ObserveMove(ref, x, y, m.callbacks()); err != nil {
		return err
	}
	var grid NativeOccupancyState
	for i := range grid.Cells {
		a := 0xf44 + i*4
		head, _ := m.read16(a + 2)
		grid.Cells[i] = NativeOccupancyCell{Header: m[a], Tile: m[a+1], Head: NativeRecordReference(head)}
	}
	access := NativeRecordAccess{
		Record: func(ref NativeRecordReference) (NativeOccupancyRecord, bool) {
			if _, ok := LocateNativeRecord(ref); !ok {
				return NativeOccupancyRecord{}, false
			}
			a := cleanupRecordAddress(ref)
			next, _ := m.read16(a + 2)
			prev, _ := m.read16(a + 4)
			xx, _ := m.read16(a + 6)
			yy, _ := m.read16(a + 8)
			return NativeOccupancyRecord{Next: NativeRecordReference(next), Previous: NativeRecordReference(prev), X: xx, Y: yy}, true
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
	if _, err := grid.Move(ref, x, y, access); err != nil {
		return err
	}
	for i, cell := range grid.Cells {
		a := 0xf44 + i*4
		m[a], m[a+1] = cell.Header, cell.Tile
		_ = m.write16(a+2, uint16(cell.Head))
	}
	return nil
}

func TestFollowerMotionFullFrameAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_motion_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []followerMotionFrameFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 332 {
		t.Fatal("full native motion corpus incomplete")
	}
	b := testBundle(t)
	r, err := DecodeFollowerMotionFrameRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	leg, err := DecodeFollowerMotionRules(b.Executable)
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
			m := motionFrameFixtureMemory(f)
			expected := *m
			for _, p := range f.Changes {
				applyPreHUDPatch(tinyMemoryWriter(&expected), p)
			}
			ref := NativeRecordReference(f.Input.Ref)
			at := cleanupRecordAddress(ref)
			c := NativeFrameRegisterContext{D: f.Input.Registers, AddressBase: 0x200000}
			calls := 0
			if f.Input.Mode == "leg" {
				a := FollowerMotionActor{X: int16(uint16(m[at+6])<<8 | uint16(m[at+7])), Y: int16(uint16(m[at+8])<<8 | uint16(m[at+9])), Speed: m[at+18]}
				v, _ := m.read16(at + 14)
				a.VX = int16(v)
				v, _ = m.read16(at + 16)
				a.VY = int16(v)
				v, _ = m.read16(at + 20)
				a.Timer = int16(v)
				err = leg.BeginLegWithFrame(&a, int16(f.Input.TargetX), int16(f.Input.TargetY), &c)
				if errors.Is(err, ErrFollowerZeroSpeed) != f.Trap {
					t.Fatalf("native leg trap differs: %v", err)
				}
				_ = m.write16(at+14, uint16(a.VX))
				_ = m.write16(at+16, uint16(a.VY))
				_ = m.write16(at+20, uint16(a.Timer))
			} else {
				step, e := r.Movement(ref, FollowerMotionFrameCallbacks{Memory: m.callbacks(), Frame: &c, Move: func(ref NativeRecordReference, x, y uint16, frame *NativeFrameRegisterContext) error {
					if calls >= len(f.Calls) {
						t.Fatal("unexpected native graph callback")
					}
					want := f.Calls[calls]
					calls++
					if frame.D != want.Registers || x != want.X || y != want.Y || want.PC != 0x12518 {
						t.Fatalf("native12518 entry context differs: got%x/%x,%x want%x/%x,%x", frame.D, x, y, want.Registers, want.X, want.Y)
					}
					return motionFrameFixtureGraphMove(m, ref, x, y, frame)
				}})
				if e != nil {
					t.Fatal(e)
				}
				if step.Boundary != f.Boundary {
					t.Fatalf("native motion boundary%x want%x", step.Boundary, f.Boundary)
				}
			}
			if c.D != f.Registers {
				t.Fatalf("native full motion registers differ: got%x want%x", c.D, f.Registers)
			}
			if calls != len(f.Calls) {
				t.Fatal("native graph callback count differs")
			}
			if *m != expected {
				for a, v := range m {
					if v != expected[a] {
						t.Fatalf("native motion BSS byte%x got%x want%x", a, v, expected[a])
					}
				}
			}
		})
	}
	if traps != 11 || counts[0x11ce8] != 24 || counts[0x112b8] != 18 || counts[0] != 77 {
		t.Fatalf("native motion branch coverage differs: %v traps%d", counts, traps)
	}
}

func TestFollowerBeginLegFrameNilPreservesLegacyZeroSpeed(t *testing.T) {
	r, err := DecodeFollowerMotionRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	a := FollowerMotionActor{X: 0x2080, Y: 0x2080, VX: 3, VY: -4, Timer: 77}
	before := a
	if err := r.BeginLegWithFrame(&a, 0x2081, 0x2081, nil); !errors.Is(err, ErrFollowerZeroSpeed) || a != before {
		t.Fatal("nil frame changed legacy zero-speed behavior")
	}
}
