package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type entryFixtureInput struct {
	Phase, Name                       string
	Timer                             int16
	Animation                         uint16
	Tile, Flags, FractionX, FractionY uint8
	Mode, Hero                        uint16
	Leader                            bool
	Others                            []struct {
		Pool                                     string
		Slot                                     int
		Kind, Owner, Flags, Stage, State, Weapon uint8
		Population, AliasPopulation              int32
		Hero, Word48, X, Y                       uint16
	}
}
type entryFixtureRecord struct {
	Reference uint16
	Raw       string
}
type entryFixtureExternal struct {
	Name                string
	Reference, Argument uint16
	Result              int32
	BeforeGridSHA256    string
	BeforeRecords       []entryFixtureRecord
	Changes             []struct {
		Offset int
		Value  uint8
	}
}
type entryFixture struct {
	Input                                    entryFixtureInput
	Exit                                     string
	Records                                  []entryFixtureRecord
	Events                                   []entryFixtureExternal
	GridSHA256                               string
	Gods                                     []string
	RNG                                      uint32
	Sound                                    []uint32
	LeaderReferences                         [2]uint16
	SelectedReference, WallObserved          uint16
	SettlementCalls, MergeCalls, BattleCalls int
}
type entryFixtureMemory struct {
	bytes    [65536]byte
	selected NativeRecordReference
	sounds   []uint32
	event    int
}

func (m *entryFixtureMemory) word(at int) uint16       { return binary.BigEndian.Uint16(m.bytes[at:]) }
func (m *entryFixtureMemory) putWord(at int, v uint16) { binary.BigEndian.PutUint16(m.bytes[at:], v) }
func (m *entryFixtureMemory) long(at int) uint32       { return binary.BigEndian.Uint32(m.bytes[at:]) }
func (m *entryFixtureMemory) putLong(at int, v uint32) { binary.BigEndian.PutUint32(m.bytes[at:], v) }
func entryAddress(ref NativeRecordReference, offset uint16, size int) (int, error) {
	at := 0x76c0 + int(int16(ref)) + int(offset)
	if at < 0 || at+size > 65536 {
		return 0, fmt.Errorf("native entry alias exceeds bounded memory")
	}
	return at, nil
}
func (m *entryFixtureMemory) read(ref NativeRecordReference) (FollowerEntryActor, error) {
	at, err := entryAddress(ref, 0, 52)
	if err != nil {
		return FollowerEntryActor{}, err
	}
	a := FollowerEntryActor{Owner: m.bytes[at+12], Byte1: m.bytes[at+1], Byte19: m.bytes[at+19], Weapon: m.bytes[at+25], Contact30: m.word(at + 30), Association34: m.word(at + 34), AssociationBack36: m.word(at + 36), Hero40: m.word(at + 40), Captive42: m.word(at + 42), CaptiveBack44: m.word(at + 44), Founded46: m.word(at + 46), Extra48: m.word(at + 48)}
	a.Motion = FollowerMotionActor{Kind: m.bytes[at], Player: m.bytes[at+12] - 1, Flags: m.bytes[at+13], Next: m.word(at + 2), Previous: m.word(at + 4), X: int16(m.word(at + 6)), Y: int16(m.word(at + 8)), Animation: int(m.word(at + 10)), VX: int16(m.word(at + 14)), VY: int16(m.word(at + 16)), Speed: m.bytes[at+18], Timer: int16(m.word(at + 20)), State: m.bytes[at+22], ReturnState: m.bytes[at+23], Population: int32(m.long(at + 26)), Variant: m.word(at + 50)}
	return a, nil
}
func (m *entryFixtureMemory) write(ref NativeRecordReference, a FollowerEntryActor) error {
	at, err := entryAddress(ref, 0, 52)
	if err != nil {
		return err
	}
	m.bytes[at], m.bytes[at+1], m.bytes[at+12], m.bytes[at+13] = a.Motion.Kind, a.Byte1, a.Owner, a.Motion.Flags
	m.putWord(at+2, a.Motion.Next)
	m.putWord(at+4, a.Motion.Previous)
	m.putWord(at+6, uint16(a.Motion.X))
	m.putWord(at+8, uint16(a.Motion.Y))
	m.putWord(at+10, uint16(a.Motion.Animation))
	m.putWord(at+14, uint16(a.Motion.VX))
	m.putWord(at+16, uint16(a.Motion.VY))
	m.bytes[at+18], m.bytes[at+19] = a.Motion.Speed, a.Byte19
	m.putWord(at+20, uint16(a.Motion.Timer))
	m.bytes[at+22], m.bytes[at+23], m.bytes[at+25] = a.Motion.State, a.Motion.ReturnState, a.Weapon
	m.putLong(at+26, uint32(a.Motion.Population))
	for _, v := range []struct {
		off  int
		word uint16
	}{{30, a.Contact30}, {34, a.Association34}, {36, a.AssociationBack36}, {40, a.Hero40}, {42, a.Captive42}, {44, a.CaptiveBack44}, {46, a.Founded46}, {48, a.Extra48}, {50, a.Motion.Variant}} {
		m.putWord(at+v.off, v.word)
	}
	return nil
}
func (m *entryFixtureMemory) insert(at int) {
	x, y := m.word(at+6)>>8, m.word(at+8)>>8
	cell := 0xf44 + (int(x)+int(y)*64)*4
	head := m.word(cell + 2)
	ref := uint16(at - 0x76c0)
	m.putWord(at+2, head)
	m.putWord(at+4, 0)
	if head != 0 {
		m.putWord(0x76c0+int(int16(head))+4, ref)
	}
	m.putWord(cell+2, ref)
}
func entryMemory(input entryFixtureInput) *entryFixtureMemory {
	m := &entryFixtureMemory{}
	for i := range 4096 {
		m.bytes[0xf44+i*4+1] = 15
	}
	m.bytes[0xf44+(32+32*64)*4+1] = input.Tile
	m.putLong(0xeb28, 4311)
	m.putWord(0xf42, 17)
	for owner := 1; owner <= 2; owner++ {
		at := 0xe76a + owner*314
		m.putWord(at+12, input.Mode)
		m.putWord(at+8, 52)
		m.putLong(at, 1000)
	}
	for _, o := range input.Others {
		at := 0x76c0 + o.Slot*52
		if o.Pool == "wall" {
			at = 0x5f50 + o.Slot*16
		} else if o.Pool == "scenery" {
			at = 0x6bd0 + o.Slot*14
		}
		m.bytes[at], m.bytes[at+1], m.bytes[at+12], m.bytes[at+13] = o.Kind, o.Stage, o.Owner, o.Flags
		m.putWord(at+6, o.X)
		m.putWord(at+8, o.Y)
		m.putWord(at+10, 0x80)
		if o.Pool == "" {
			m.bytes[at+18], m.bytes[at+22], m.bytes[at+23], m.bytes[at+25] = 20, o.State, 2, o.Weapon
			m.putLong(at+26, uint32(o.Population))
			m.putWord(at+40, o.Hero)
			m.putWord(at+48, o.Word48)
		} else if o.AliasPopulation != 0 {
			m.putLong(at+26, uint32(o.AliasPopulation))
		}
		m.insert(at)
	}
	at := 0x76f4
	m.bytes[at], m.bytes[at+12], m.bytes[at+13], m.bytes[at+18] = 2, 1, input.Flags, 20
	m.bytes[at+22], m.bytes[at+23], m.bytes[at+24], m.bytes[at+25] = 4, 2, 2, 7
	m.putLong(at+26, 1000)
	m.putWord(at+40, input.Hero)
	m.putWord(at+48, 1234)
	m.putWord(at+6, 8192+uint16(input.FractionX))
	m.putWord(at+8, 8192+uint16(input.FractionY))
	m.putWord(at+10, 8)
	if input.Phase == "complete" {
		m.bytes[at+22] = 12
	}
	if input.Phase == "waiting" {
		m.bytes[at+22] = 10
		m.putWord(at+10, input.Animation)
		m.putWord(at+20, uint16(input.Timer))
	}
	if input.Leader {
		m.selected = 52
	}
	m.insert(at)
	return m
}
func (m *entryFixtureMemory) callbacks(t *testing.T, f entryFixture) FollowerEntryCallbacks {
	t.Helper()
	external := func(name string, ref NativeRecordReference, arg uint16) (int, error) {
		if m.event >= len(f.Events) {
			return 0, fmt.Errorf("unexpected native external callback %s", name)
		}
		e := f.Events[m.event]
		m.event++
		if e.Name != name || e.Reference != uint16(ref) || e.Argument != arg {
			return 0, fmt.Errorf("native callback ordering/argument differs: got%s/%04x/%d, native%+v", name, uint16(ref), arg, e)
		}
		for _, r := range e.BeforeRecords {
			raw, err := hex.DecodeString(r.Raw)
			if err != nil {
				return 0, err
			}
			at, err := entryAddress(NativeRecordReference(r.Reference), 0, len(raw))
			if err != nil {
				return 0, err
			}
			if got := hex.EncodeToString(m.bytes[at : at+len(raw)]); got != r.Raw {
				return 0, fmt.Errorf("record%04x before%s differs: got%s native%s", r.Reference, name, got, r.Raw)
			}
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(m.bytes[0xf44:0x4f44])); got != e.BeforeGridSHA256 {
			return 0, fmt.Errorf("native grid before%s differs", name)
		}
		// Exact numeric deltas from the original CPU callback, not invented town
		// success. Their ordering proves the entry boundary; production must supply
		// the independently translated evaluator/farm/hero-link routines.
		for _, d := range e.Changes {
			if d.Offset < 0xf44 || d.Offset >= 0xeb18 {
				return 0, fmt.Errorf("native callback delta outside proven window")
			}
			m.bytes[d.Offset] = d.Value
		}
		return int(int16(e.Result)), nil
	}
	return FollowerEntryCallbacks{
		Read: m.read, Write: m.write,
		Head: func(tile NativePackedTile) (NativeRecordReference, error) {
			x, y := int(uint8(tile)), int(uint8(tile>>8))
			if x >= 64 || y >= 64 {
				return 0, fmt.Errorf("entry fixture head outside map")
			}
			return NativeRecordReference(m.word(0xf44 + (x+y*64)*4 + 2)), nil
		},
		Node: func(ref NativeRecordReference) (FollowerEntryNode, error) {
			at, err := entryAddress(ref, 0, 14)
			if err != nil {
				return FollowerEntryNode{}, err
			}
			return FollowerEntryNode{Kind: m.bytes[at], Owner: m.bytes[at+12], Next: NativeRecordReference(m.word(at + 2))}, nil
		},
		ReadWord: func(ref NativeRecordReference, off uint16) (uint16, error) {
			at, e := entryAddress(ref, off, 2)
			if e != nil {
				return 0, e
			}
			return m.word(at), nil
		}, ReadLong: func(ref NativeRecordReference, off uint16) (uint32, error) {
			at, e := entryAddress(ref, off, 4)
			if e != nil {
				return 0, e
			}
			return m.long(at), nil
		}, WriteWord: func(ref NativeRecordReference, off, value uint16) error {
			at, e := entryAddress(ref, off, 2)
			if e != nil {
				return e
			}
			m.putWord(at, value)
			return nil
		},
		Tile: func(tile NativePackedTile) (uint8, error) {
			return m.bytes[0xf44+(int(uint8(tile))+int(uint8(tile>>8))*64)*4+1], nil
		}, GodMode: func(owner uint8) (uint16, error) { return m.word(0xe76a + int(owner)*314 + 12), nil }, Tick: func() uint16 { return 17 },
		SetLeader: func(owner uint8, ref NativeRecordReference) error {
			m.putWord(0xe76a+int(owner)*314+8, uint16(ref))
			return nil
		}, Selected: func() NativeRecordReference { return m.selected }, Select: func(ref NativeRecordReference) error { m.selected = ref; return nil }, Founded: func(owner uint8) error { at := 0xe76a + int(owner)*314 + 68; m.putWord(at, m.word(at)+1); return nil },
		Unlink: func(ref NativeRecordReference) error {
			at, _ := entryAddress(ref, 0, 52)
			previous, next := m.word(at+4), m.word(at+2)
			if previous != 0 {
				m.putWord(0x76c0+int(int16(previous))+2, next)
			} else {
				cell := 0xf44 + (int(m.word(at+6)>>8)+int(m.word(at+8)>>8)*64)*4
				m.putWord(cell+2, next)
			}
			if next != 0 {
				m.putWord(0x76c0+int(int16(next))+4, previous)
			}
			m.putLong(at+2, 0)
			return nil
		},
		EvaluateTown: func(ref NativeRecordReference) (int, error) { return external("EvaluateTown", ref, 0) }, ClearFarms: func(ref NativeRecordReference, tile uint8) error {
			_, err := external("ClearFarms", ref, uint16(tile))
			return err
		}, ClearHeroLinks: func(ref NativeRecordReference) error { _, err := external("ClearHeroLinks", ref, 0); return err }, Sound: func(raw uint16) error { m.sounds = append(m.sounds, uint32(raw)); return nil },
	}
}
func TestFollowerEntryAgainstOriginal68000Phases(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_entry_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases    int
		Fixtures []entryFixture
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.Cases != 72 || len(catalog.Fixtures) != 72 {
		t.Fatal("native entry fixture catalog is incomplete")
	}
	rules, err := DecodeFollowerEntryRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range catalog.Fixtures {
		t.Run(f.Input.Name, func(t *testing.T) {
			if len(f.Records) == 0 || len(f.Gods) != 2 || len(f.GridSHA256) != 64 {
				t.Fatal("native entry fixture snapshot is incomplete")
			}
			m := entryMemory(f.Input)
			callbacks := m.callbacks(t, f)
			var step FollowerEntryStep
			var err error
			switch f.Input.Phase {
			case "complete":
				step, err = rules.CompleteContact(52, callbacks)
			case "waiting":
				step, err = rules.TickWaiting(52, callbacks)
			default:
				step, err = rules.Enter(52, callbacks)
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range f.Records {
				raw, err := hex.DecodeString(r.Raw)
				if err != nil || len(raw) == 0 {
					t.Fatal("native entry record missing")
				}
				at, _ := entryAddress(NativeRecordReference(r.Reference), 0, len(raw))
				if got := hex.EncodeToString(m.bytes[at : at+len(raw)]); got != r.Raw {
					t.Fatalf("record%04x differs:\ngot %s\nnative %s", r.Reference, got, r.Raw)
				}
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(m.bytes[0xf44:0x4f44])); got != f.GridSHA256 {
				t.Fatal("complete native grid differs")
			}
			for owner, raw := range f.Gods {
				at := 0xe76a + (owner+1)*314
				if got := hex.EncodeToString(m.bytes[at : at+314]); got != raw {
					t.Fatal("native deity record differs")
				}
			}
			if m.event != len(f.Events) || m.long(0xeb28) != f.RNG || m.selected != NativeRecordReference(f.SelectedReference) {
				t.Fatal("native callbacks/randomness/selected reference differs")
			}
			if len(m.sounds) != len(f.Sound) {
				t.Fatal("native sound events differ")
			}
			for i := range m.sounds {
				if m.sounds[i] != f.Sound[i] {
					t.Fatal("native raw sound argument differs")
				}
			}
			exit := "render"
			if step.NextFollower {
				exit = "next-follower"
			} else if step.ContinueTownUpdate {
				exit = "new-town-update"
			} else if step.RedispatchHero {
				exit = "hero-search"
			} else if step.RedispatchSearch {
				exit = "search"
			}
			if exit != f.Exit {
				t.Fatalf("native control boundary differs: got%s native%s", exit, f.Exit)
			}
			if step.WallObserved != (f.WallObserved != 0) {
				t.Fatal("native aliased wall-population guard differs")
			}
		})
	}
}
func TestFollowerEntryRejectsMissingCallbacksAndCyclicGraph(t *testing.T) {
	rules, err := DecodeFollowerEntryRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rules.Enter(52, FollowerEntryCallbacks{}); err == nil {
		t.Fatal("missing entry callbacks accepted")
	}
	if _, err := rules.CompleteContact(52, FollowerEntryCallbacks{}); err == nil {
		t.Fatal("missing delayed callbacks accepted")
	}
	if _, err := rules.TickWaiting(52, FollowerEntryCallbacks{}); err == nil {
		t.Fatal("missing waiting callbacks accepted")
	}
	m := entryMemory(entryFixtureInput{Tile: 15, Mode: 16, FractionX: 128, FractionY: 128})
	m.putWord(0x76f4+2, 52)
	callbacks := m.callbacks(t, entryFixture{})
	before := m.bytes
	if _, err := rules.Enter(52, callbacks); err == nil {
		t.Fatal("entry cycle accepted")
	}
	if m.bytes != before {
		t.Fatal("cycle changed entry state")
	}
	if _, err := entryAddress(0x7fff, 65535, 4); err == nil {
		t.Fatal("unbounded native alias accepted")
	}
}
