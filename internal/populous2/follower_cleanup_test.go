package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type cleanupWrite struct {
	BSSAddress, Width int
	Value             uint32
}
type cleanupFixture struct {
	Input struct {
		Name                                                    string
		Owner, Kind, Flags, Stage, Byte19                       uint8
		Mode                                                    uint32
		X, Y, Popularity, LeaderLoss, SourceHero, GodHero, Head uint16
	}
	D0, D1, D2, A0, A1, A2  uint32
	Writes                  []cleanupWrite
	Hash, SourceRaw, GodRaw string
	MarkerReference         uint16
	Callbacks               []string
}
type cleanupMemory struct {
	lower   [NativeRecordImageStart]byte
	records NativeRecordImage
	globals NativeGlobalImage
	writes  []cleanupWrite
	record  bool
	err     error
}

func (m *cleanupMemory) runtime() NativeRuntimeMemory {
	return NativeRuntimeMemory{Records: &m.records, Globals: &m.globals}
}
func (m *cleanupMemory) read8(a int) (uint8, error) {
	if a < 0 {
		return 0, fmt.Errorf("negative BSS address")
	}
	if a < NativeRecordImageStart {
		return m.lower[a], nil
	}
	return m.runtime().Read8(a)
}
func (m *cleanupMemory) read16(a int) (uint16, error) {
	if a < 0 {
		return 0, fmt.Errorf("negative BSS address")
	}
	if a+2 <= NativeRecordImageStart {
		return binary.BigEndian.Uint16(m.lower[a:]), nil
	}
	return m.runtime().Read16(a)
}
func (m *cleanupMemory) read32(a int) (uint32, error) {
	if a < 0 {
		return 0, fmt.Errorf("negative BSS address")
	}
	if a+4 <= NativeRecordImageStart {
		return binary.BigEndian.Uint32(m.lower[a:]), nil
	}
	return m.runtime().Read32(a)
}
func (m *cleanupMemory) write8(a int, v uint8) error {
	if m.record {
		m.writes = append(m.writes, cleanupWrite{a, 1, uint32(v)})
	}
	if a < 0 {
		return fmt.Errorf("negative BSS address")
	}
	if a < NativeRecordImageStart {
		m.lower[a] = v
		return nil
	}
	_, err := m.runtime().Write8(a, v)
	return err
}
func (m *cleanupMemory) write16(a int, v uint16) error {
	if m.record {
		m.writes = append(m.writes, cleanupWrite{a, 2, uint32(v)})
	}
	if a < 0 {
		return fmt.Errorf("negative BSS address")
	}
	if a+2 <= NativeRecordImageStart {
		binary.BigEndian.PutUint16(m.lower[a:], v)
		return nil
	}
	_, err := m.runtime().Write16(a, v)
	return err
}
func (m *cleanupMemory) write32(a int, v uint32) error {
	if m.record {
		m.writes = append(m.writes, cleanupWrite{a, 4, v})
	}
	if a < 0 {
		return fmt.Errorf("negative BSS address")
	}
	if a+4 <= NativeRecordImageStart {
		binary.BigEndian.PutUint32(m.lower[a:], v)
		return nil
	}
	_, err := m.runtime().Write32(a, v)
	return err
}
func (m *cleanupMemory) word(a int) uint16 {
	v, err := m.read16(a)
	if err != nil {
		m.err = err
	}
	return v
}
func (m *cleanupMemory) byte(a int) uint8 {
	v, err := m.read8(a)
	if err != nil {
		m.err = err
	}
	return v
}
func (m *cleanupMemory) putWord(a int, v uint16) {
	if err := m.write16(a, v); err != nil {
		m.err = err
	}
}
func (m *cleanupMemory) putByte(a int, v uint8) {
	if err := m.write8(a, v); err != nil {
		m.err = err
	}
}
func cleanupCell(m *cleanupMemory, address int) int {
	packed := m.word(address+8)&0xff00 | uint16(m.byte(address+6))
	offset := int(int16(packed&0xff00 | uint16(uint8(packed)<<2)))
	return 0xf44 + offset
}
func (m *cleanupMemory) insert(ref NativeRecordReference) error {
	a := cleanupRecordAddress(ref)
	if err := m.write32(a+2, 0); err != nil {
		return err
	}
	cell := cleanupCell(m, a)
	head := m.word(cell + 2)
	if head != 0 {
		m.putWord(a+2, head)
		m.putWord(cleanupRecordAddress(NativeRecordReference(head))+4, uint16(ref))
	}
	m.putWord(cell+2, uint16(ref))
	return m.err
}
func (m *cleanupMemory) unlink(ref NativeRecordReference) error {
	a := cleanupRecordAddress(ref)
	cell := cleanupCell(m, a)
	current := NativeRecordReference(m.word(cell + 2))
	previous := NativeRecordReference(0)
	visits := 0
	for current != 0 {
		if visits > 1053 {
			return fmt.Errorf("cleanup fixture cycle")
		}
		visits++
		at := cleanupRecordAddress(current)
		next := m.word(at + 2)
		if current == ref {
			if previous == 0 {
				m.putWord(cell+2, next)
				if next != 0 {
					m.putWord(cleanupRecordAddress(NativeRecordReference(next))+4, 0)
				}
			} else {
				m.putWord(cleanupRecordAddress(previous)+2, next)
				if next != 0 {
					m.putWord(cleanupRecordAddress(NativeRecordReference(next))+4, m.word(at+4))
				}
			}
			break
		}
		previous, current = current, NativeRecordReference(next)
	}
	if err := m.write32(a+2, 0); err != nil {
		return err
	}
	return m.err
}
func (m *cleanupMemory) town(t *testing.T) NativeTownCallbacks {
	t.Helper()
	return NativeTownCallbacks{
		Record: func(ref NativeRecordReference) (NativeTownRecord, bool) {
			a := cleanupRecordAddress(ref)
			return NativeTownRecord{Kind: m.byte(a), Owner: m.byte(a + 12), Stage: m.byte(a + 1), State: m.byte(a + 22), Animation: m.word(a + 10), Next: NativeRecordReference(m.word(a + 2)), X: m.word(a + 6), Y: m.word(a + 8)}, m.err == nil
		}, SetRecord: func(NativeRecordReference, NativeTownRecord) {
			t.Fatal("ClearFarms unexpectedly changed town metadata")
		}, Head: func(x, y int) NativeRecordReference { return NativeRecordReference(m.word(0xf44 + (x+y*64)*4 + 2)) }, ReadTile: func(x, y int) uint8 { return m.byte(0xf44 + (x+y*64)*4 + 1) }, WriteTile: func(x, y int, tile uint8) { m.putByte(0xf44+(x+y*64)*4+1, tile) }, WriteOverlay: func(x, y int, code uint8) { m.putByte(0x4f44+x+y*64, code) }}
}
func cleanupFixtureMemory(t *testing.T, f cleanupFixture) *cleanupMemory {
	t.Helper()
	m := &cleanupMemory{}
	c := f.Input
	at := 0x76f4
	god := 0xe76a + int(c.Owner)*314
	marker := 0xe740 + int(c.Owner)*14
	for i := range 4096 {
		m.putByte(0xf44+i*4, 0x88)
		m.putByte(0xf44+i*4+1, 15)
	}
	m.putByte(at, c.Kind)
	m.putByte(at+1, c.Stage)
	m.putByte(at+12, c.Owner)
	m.putByte(at+13, c.Flags)
	m.putByte(at+19, c.Byte19)
	m.putWord(at+6, c.X)
	m.putWord(at+8, c.Y)
	m.write32(at+26, 1000)
	m.putWord(at+34, 104)
	m.putWord(at+36, 156)
	m.putWord(at+40, c.SourceHero)
	m.putWord(at+42, 104)
	m.putWord(0x76c0+104+36, 52)
	m.putWord(0x76c0+156+34, 52)
	m.putWord(0x76c0+104+42, 0)
	m.putByte(0x76c0+104+13, 8)
	m.putByte(0x76c0+104+22, 52)
	m.putWord(god+68, c.Popularity)
	m.putWord(god+70, c.LeaderLoss)
	m.putWord(god+8, 52)
	m.putWord(god+10, f.MarkerReference)
	m.putWord(god+34, 0xbeef)
	m.putWord(god+40, c.GodHero)
	m.putWord(god+42, c.Head)
	m.putByte(marker, 20)
	m.putByte(marker+12, c.Owner)
	m.putWord(marker+6, 0x0880)
	m.putWord(marker+8, 0x0980)
	if err := m.insert(NativeRecordReference(f.MarkerReference)); err != nil {
		t.Fatal(err)
	}
	if err := m.insert(52); err != nil {
		t.Fatal(err)
	}
	if c.Kind == 4 {
		for y := 30; y <= 36; y++ {
			for x := 29; x <= 35; x++ {
				p := x + y*64
				m.putByte(0xf44+p*4+1, 47+16*(c.Owner-1))
				m.putByte(0x4f44+p, 7)
			}
		}
	}
	if m.err != nil {
		t.Fatal(m.err)
	}
	m.record = true
	return m
}
func TestFollowerCleanupAgainstCompleteOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_cleanup_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []cleanupFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 49 {
		t.Fatal("native cleanup fixture catalog incomplete")
	}
	town, err := DecodeNativeTownEvaluator(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			m := cleanupFixtureMemory(t, f)
			cb := FollowerCleanupCallbacks{Memory: FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}, Unlink: m.unlink, Insert: m.insert, ClearFarms: func(ref NativeRecordReference, tile uint8) error { return town.ClearFarms(ref, tile, m.town(t)) }}
			step, err := CleanupFollower(52, FollowerCleanupRegisters{D0: f.Input.Mode, D1: 0xaabbccdd, D2: 0x98765432}, cb)
			if err != nil {
				t.Fatal(err)
			}
			if step.Registers != (FollowerCleanupRegisters{D0: f.D0, D1: f.D1, D2: f.D2}) {
				t.Fatalf("native output registers differ: %+v want%08x/%08x/%08x", step.Registers, f.D0, f.D1, f.D2)
			}
			if !reflect.DeepEqual(m.writes, f.Writes) {
				at := 0
				for at < len(m.writes) && at < len(f.Writes) && m.writes[at] == f.Writes[at] {
					at++
				}
				t.Fatalf("native write order differs at%d: got%d want%d\ngot%+v\nnative%+v", at, len(m.writes), len(f.Writes), m.writes, f.Writes)
			}
			bytes := append([]byte(nil), m.lower[:]...)
			bytes = append(bytes, m.records.Bytes[:]...)
			bytes = append(bytes, m.globals.Bytes[:]...)
			if got := fmt.Sprintf("%x", sha256.Sum256(bytes)); got != f.Hash {
				t.Fatal("full native BSS image/grid/overlay/globals differs")
			}
			source := cleanupRecordAddress(52) - NativeRecordImageStart
			if hex.EncodeToString(m.records.Bytes[source:source+52]) != f.SourceRaw {
				t.Fatal("source native record differs")
			}
			god := 0xe76a + int(f.Input.Owner)*314 - NativeMagnetImageStart
			if hex.EncodeToString(m.globals.Bytes[god:god+314]) != f.GodRaw {
				t.Fatal("native deity record differs")
			}
			if step.Removed != (uint8(f.Input.Mode) == 0) || step.Leader != (f.Input.Flags&1 != 0) || step.Owner != f.Input.Owner {
				t.Fatal("cleanup ownership/mode summary differs")
			}
			context := 0xe76a + int(f.Input.Owner)*314
			if f.Input.Flags&1 != 0 {
				context = 0x76f4
			}
			if step.HeroContextAddress != context {
				t.Fatal("native A0 hero context differs")
			}
			if f.A0 != 0x123456 || f.A1 != 0x234567 || f.A2 != 0x2076f4 {
				t.Fatal("native A0-A2 preservation proof differs")
			}
		})
	}
}
func TestFollowerCleanupRejectsMissingNativeCallbacks(t *testing.T) {
	if _, err := CleanupFollower(52, FollowerCleanupRegisters{}, FollowerCleanupCallbacks{}); err == nil {
		t.Fatal("incomplete cleanup callback accepted")
	}
}
