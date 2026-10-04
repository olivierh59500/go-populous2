package populous2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type lavaFixture struct {
	Input struct {
		Name, Mode, Behind                                  string
		Owner, Tile, TargetKind, Flags, State, Stage        uint8
		Hero, Direction, Timer, Life, X, Y, Fraction, Clock uint16
		Occupied                                            int
		Seed                                                uint32
		Duplicate                                           bool
	}
	Result        int16
	Before, After lavaSnapshot
}
type lavaSnapshot struct {
	GridHash, RecordsHash, GlobalsHash, OverlayHash, SourceRaw, TargetRaw string
	RNG, Boundary                                                         uint32
}

func lavaFixtureMemory(t *testing.T, f lavaFixture) (*volcanoTestMemory, NativeRecordReference, NativeRecordReference) {
	t.Helper()
	var input volcanoFixture
	input.Input.Terrain, input.Input.Occupied, input.Input.Seed = "flat", f.Input.Occupied, f.Input.Seed
	v := newVolcanoTestMemory(t, input)
	m, c := v.m, f.Input
	for i := range 4096 {
		m.putByte(0x4f44+i, 0x77)
	}
	for owner := 0; owner < 3; owner++ {
		god, marker := 0xe76a+owner*314, 0xe740+owner*14
		m.putWord(god+68, 30)
		m.putWord(god+8, 52)
		m.putWord(god+10, uint16(marker-0x76c0))
		m.putByte(marker, 20)
		m.putByte(marker+12, uint8(owner))
		m.putWord(marker+6, 0x0880)
		m.putWord(marker+8, 0x0980)
		if err := m.insert(NativeRecordReference(marker - 0x76c0)); err != nil {
			t.Fatal(err)
		}
	}
	target := NativeRecordReference(0)
	if c.X < 64 && c.Y < 64 {
		m.putByte(0xf44+(int(c.X)+int(c.Y)*64)*4+1, c.Tile)
		if c.Behind == "vent" {
			packed := c.Y<<8 | c.X
			back := packed + v.lava.Behind[c.Direction/2]
			if back&0xc0c0 == 0 {
				m.putByte(0xf44+(int(uint8(back))+int(uint8(back>>8))*64)*4+1, 0xdc)
			}
		}
		if c.TargetKind != 0 {
			at := 0x76f4
			if c.TargetKind == 20 {
				at = 0xe74e
				if err := m.unlink(NativeRecordReference(at - 0x76c0)); err != nil {
					t.Fatal(err)
				}
			}
			if c.TargetKind == 22 || c.TargetKind == 24 || c.TargetKind == 30 {
				at = 0x6bd0
			}
			if c.TargetKind == 26 || c.TargetKind == 28 {
				at = 0x5f50
			}
			if c.TargetKind >= 32 {
				at = 0xc820
				m.putByte(0xc80c, 2)
			}
			m.putByte(at, c.TargetKind)
			m.putByte(at+1, c.Stage)
			m.putByte(at+12, 1)
			m.putByte(at+13, c.Flags)
			m.putWord(at+6, c.X<<8|c.Fraction)
			m.putWord(at+8, c.Y<<8|c.Fraction)
			if c.TargetKind < 20 {
				m.putByte(at+22, c.State)
				m.putWord(at+40, c.Hero)
				if err := m.write32(at+26, 100); err != nil {
					t.Fatal(err)
				}
			}
			target = NativeRecordReference(uint16(at - 0x76c0))
			if err := m.insert(target); err != nil {
				t.Fatal(err)
			}
		}
		if c.Duplicate {
			at := 0xc800
			m.putByte(at, 56)
			m.putByte(at+12, 1)
			m.putWord(at+6, c.X<<8|128)
			m.putWord(at+8, c.Y<<8|128)
			if err := m.insert(NativeRecordReference(at - 0x76c0)); err != nil {
				t.Fatal(err)
			}
		}
	}
	m.putWord(0xf42, c.Clock)
	at := 0xc800 + c.Occupied*32
	if c.TargetKind >= 32 {
		at += 32
	}
	if c.Duplicate {
		at += 32
	}
	source := NativeRecordReference(0)
	if at < 0xe740 {
		source = NativeRecordReference(at - 0x76c0)
	}
	if m.err != nil {
		t.Fatal(m.err)
	}
	return v, source, target
}

func (v *volcanoTestMemory) move(ref NativeRecordReference, x, y uint16) error {
	var graph NativeOccupancyState
	for i := range graph.Cells {
		at := 0xf44 + i*4
		graph.Cells[i] = NativeOccupancyCell{Header: v.m.byte(at), Tile: v.m.byte(at + 1), Head: NativeRecordReference(v.m.word(at + 2))}
	}
	_, err := graph.Move(ref, x, y, v.m.runtime().RecordAccess())
	if err != nil {
		return err
	}
	for i, cell := range graph.Cells {
		at := 0xf44 + i*4
		v.m.putByte(at, cell.Header)
		v.m.putWord(at+2, uint16(cell.Head))
	}
	return v.m.err
}

func lavaCompleteCallbacks(t *testing.T, v *volcanoTestMemory, clock uint16) NativeLavaCallbacks {
	t.Helper()
	exe := testBundle(t).Executable
	prepass, err := DecodeCommonPrepassRules(exe)
	if err != nil {
		t.Fatal(err)
	}
	town, err := DecodeTownCombatRules(exe)
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := DecodeNativeTownEvaluator(exe)
	if err != nil {
		t.Fatal(err)
	}
	read := func(ref NativeRecordReference) (FollowerEntryActor, error) { return v.m.records.ReadFollowerEntry(ref) }
	write := func(ref NativeRecordReference, a FollowerEntryActor) error {
		old, err := read(ref)
		if err != nil {
			return err
		}
		_, err = v.m.records.PatchFollowerEntry(ref, old, a)
		return err
	}
	farms := func(ref NativeRecordReference, tile uint8) error { return evaluator.ClearFarms(ref, tile, v.m.town(t)) }
	cb := v.lavaCallbacks()
	cb.Clock = clock
	cb.Move = v.move
	cb.Scorch = func(ref NativeRecordReference) error {
		at := cleanupRecordAddress(ref)
		p := NativePackedTile(v.m.word(at+8)&0xff00 | v.m.word(at+6)>>8)
		cell, err := v.cell(p)
		if err != nil {
			return err
		}
		if v.lava.Geometry[cell.Tile]&15 == 15 && prepass.Properties[cell.Tile]&0x400 == 0 {
			return v.writeTile(p, 95)
		}
		return nil
	}
	cb.DestroyTown = func(ref NativeRecordReference) error {
		_, err := town.Destroy(ref, TownCombatCallbacks{Read: read, Write: write, ClearFarms: farms, Cleanup: func(ref NativeRecordReference, mode uint16) error {
			_, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: uint32(mode)}, FollowerCleanupCallbacks{Memory: v.memory(), Unlink: v.m.unlink, Insert: v.m.insert, ClearFarms: farms})
			return err
		}})
		return err
	}
	return cb
}

func lavaVerify(t *testing.T, v *volcanoTestMemory, source, target NativeRecordReference, kind uint8, want lavaSnapshot) {
	t.Helper()
	if source != 0 {
		at := cleanupRecordAddress(source) - NativeRecordImageStart
		if hex.EncodeToString(v.m.records.Bytes[at:at+32]) != want.SourceRaw {
			t.Fatalf("native lava source differs: got%s want%s", hex.EncodeToString(v.m.records.Bytes[at:at+32]), want.SourceRaw)
		}
	}
	if target != 0 {
		at, length := cleanupRecordAddress(target), 52
		if kind >= 20 {
			length = 14
		}
		raw := make([]byte, length)
		for i := range raw {
			raw[i] = v.m.byte(at + i)
		}
		if hex.EncodeToString(raw) != want.TargetRaw {
			t.Fatalf("native lava target differs: got%s want%s", hex.EncodeToString(raw), want.TargetRaw)
		}
	}
	for _, field := range []struct{ name, got, want string }{
		{"grid", fmt.Sprintf("%x", sha256.Sum256(v.m.lower[0xf44:0x4f44])), want.GridHash},
		{"overlay", fmt.Sprintf("%x", sha256.Sum256(v.m.lower[0x4f44:0x5f44])), want.OverlayHash},
		{"records", fmt.Sprintf("%x", sha256.Sum256(v.m.records.Bytes[:])), want.RecordsHash},
		{"globals", fmt.Sprintf("%x", sha256.Sum256(v.m.globals.Bytes[:])), want.GlobalsHash},
	} {
		if field.got != field.want {
			t.Fatalf("complete native lava %s differs", field.name)
		}
	}
	if v.rng != want.RNG {
		t.Fatalf("native lava RNG got%x want%x", v.rng, want.RNG)
	}
}

func TestLavaAgainstCompleteOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/lava_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []lavaFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 356 {
		t.Fatal("native lava fixture catalog incomplete")
	}
	created, updated := 0, 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			v, source, target := lavaFixtureMemory(t, f)
			cb := lavaCompleteCallbacks(t, v, f.Input.Clock)
			if f.Input.Mode == "create" {
				created++
				lavaVerify(t, v, source, target, f.Input.TargetKind, f.Before)
			}
			result, err := v.lava.Create(f.Input.Owner, NativePackedTile(f.Input.Y<<8|f.Input.X), f.Input.Direction, cb)
			if err != nil {
				t.Fatal(err)
			}
			if result != f.Result {
				t.Fatalf("original lava admission got%d want%d", result, f.Result)
			}
			if f.Input.Mode == "tick" {
				updated++
				at := cleanupRecordAddress(source)
				v.m.putWord(at+20, f.Input.Timer)
				v.m.putWord(at+24, f.Input.Life)
				lavaVerify(t, v, source, target, f.Input.TargetKind, f.Before)
				_, err := v.lava.Tick(source, cb)
				if err != nil {
					t.Fatal(err)
				}
				if f.After.Boundary != 0x15ad4 {
					t.Fatal("unexpected original lava continuation")
				}
			}
			lavaVerify(t, v, source, target, f.Input.TargetKind, f.After)
		})
	}
	if created != 84 || updated != 272 {
		t.Fatalf("native lava coverage differs: %d/%d", created, updated)
	}
}
