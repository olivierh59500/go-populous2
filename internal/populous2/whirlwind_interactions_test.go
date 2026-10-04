package populous2

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type nativeInteractionInput struct {
	Name, Mode         string
	Hero               int
	State, Kind, Stage uint8
	X, Y, Followers    int
	Seed               uint32
	Mixed              bool
	ForeignRef         bool `json:"foreign_ref"`
	Animation          uint16
	Ticks              int
}

type nativeInteractionGroup struct {
	Slot int
	Raw  string
}

type nativeInteractionHead struct {
	X, Y int
	Head uint16
}

type nativeInteractionRead struct {
	Source         string
	BaseOffset     uint32 `json:"base_offset"`
	Selector, Word uint16
	RecordOffset   uint16 `json:"record_offset"`
}

type nativeInteractionFixture struct {
	Input          nativeInteractionInput
	RNG            uint32
	EffectOwner    uint8  `json:"effect_owner"`
	EffectNext     uint16 `json:"effect_next"`
	EffectPrevious uint16 `json:"effect_previous"`
	Groups         []nativeInteractionGroup
	Heads          []nativeInteractionHead
	ReleaseReads   []nativeInteractionRead `json:"release_reads"`
}

func nativeInteractionFixtures(t *testing.T) []nativeInteractionFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/whirlwind_interactions_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases    int
		Fixtures []nativeInteractionFixture
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.Cases != 92 || len(catalog.Fixtures) != 92 {
		t.Fatal("native interaction fixture catalog is incomplete")
	}
	return catalog.Fixtures
}

const interactionGroupBase, interactionEffect, interactionGrid = 0x76c0, 0xc800, 0xf44

type nativeInteractionMemory struct {
	bytes [1 << 16]byte
	RNG   uint32
}

func (m *nativeInteractionMemory) word(at int) uint16 { return binary.BigEndian.Uint16(m.bytes[at:]) }
func (m *nativeInteractionMemory) setWord(at int, value uint16) {
	binary.BigEndian.PutUint16(m.bytes[at:], value)
}
func (m *nativeInteractionMemory) setLong(at int, value uint32) {
	binary.BigEndian.PutUint32(m.bytes[at:], value)
}
func (m *nativeInteractionMemory) address(reference NativeRecordReference) int {
	return interactionGroupBase + int(reference)
}
func (m *nativeInteractionMemory) packed(at int) NativePackedTile {
	return NativePackedTile(uint16(m.bytes[at+8])<<8 | uint16(m.bytes[at+6]))
}
func interactionCell(packed NativePackedTile) int {
	return interactionGrid + (int(uint8(packed))+int(uint8(packed>>8))*64)*4
}

func newNativeInteractionMemory(input nativeInteractionInput) *nativeInteractionMemory {
	m := &nativeInteractionMemory{RNG: input.Seed}
	for index := range 4096 {
		m.bytes[interactionGrid+index*4+1] = 15
	}
	count := max(input.Followers, 1)
	addresses := []int{interactionEffect}
	for index := 1; index <= count; index++ {
		addresses = append(addresses, interactionGroupBase+index*52)
	}
	for index, at := range addresses {
		if index > 0 {
			m.setWord(at+4, uint16(addresses[index-1]-interactionGroupBase))
		}
		if index+1 < len(addresses) {
			m.setWord(at+2, uint16(addresses[index+1]-interactionGroupBase))
		}
		m.setWord(at+6, uint16(input.X*256+128))
		m.setWord(at+8, uint16(input.Y*256+128))
	}
	m.setWord(interactionCell(NativePackedTile(input.X|input.Y<<8))+2, uint16(interactionEffect-interactionGroupBase))
	m.bytes[interactionEffect], m.bytes[interactionEffect+12] = 0x20, 1
	for index, at := range addresses[1:] {
		slot := index + 1
		m.bytes[at], m.bytes[at+1] = input.Kind, input.Stage
		m.bytes[at+12] = uint8(1 + slot%2)
		m.bytes[at+22], m.bytes[at+25] = input.State, 4
		m.setLong(at+26, uint32(100+slot))
		m.setWord(at+10, 0x80)
		if input.Hero >= 0 {
			m.bytes[at+13] = 2
			m.setWord(at+40, uint16(input.Hero*2))
		}
	}
	return m
}

func (m *nativeInteractionMemory) unlink(at int) {
	previous, next := m.word(at+4), m.word(at+2)
	if previous == 0 {
		m.setWord(interactionCell(m.packed(at))+2, next)
	} else {
		m.setWord(interactionGroupBase+int(previous)+2, next)
	}
	if next != 0 {
		m.setWord(interactionGroupBase+int(next)+4, previous)
	}
	m.setLong(at+2, 0)
}

func (m *nativeInteractionMemory) move(reference NativeRecordReference, x, y uint16) bool {
	at := m.address(reference)
	old := m.packed(at)
	m.setWord(at+6, x)
	m.setWord(at+8, y)
	newTile := m.packed(at)
	if old == newTile {
		return false
	}
	previous, next := m.word(at+4), m.word(at+2)
	if previous != 0 {
		m.setWord(interactionGroupBase+int(previous)+2, next)
		if next != 0 {
			m.setWord(interactionGroupBase+int(next)+4, previous)
		}
	} else {
		m.setWord(interactionCell(old)+2, next)
		if next != 0 {
			m.setWord(interactionGroupBase+int(next)+4, 0)
		}
	}
	m.setLong(at+2, 0)
	head := m.word(interactionCell(newTile) + 2)
	if head != 0 {
		m.setWord(at+2, head)
		m.setWord(interactionGroupBase+int(head)+4, uint16(reference))
	}
	m.setWord(interactionCell(newTile)+2, uint16(reference))
	m.bytes[interactionCell(newTile)] += 8
	return true
}

func (m *nativeInteractionMemory) heads() []nativeInteractionHead {
	result := []nativeInteractionHead{}
	for y := range 64 {
		for x := range 64 {
			if head := m.word(interactionCell(NativePackedTile(x|y<<8)) + 2); head != 0 {
				result = append(result, nativeInteractionHead{X: x, Y: y, Head: head})
			}
		}
	}
	return result
}

func assertInteractionRecords(t *testing.T, m *nativeInteractionMemory, fixture nativeInteractionFixture) {
	t.Helper()
	for _, group := range fixture.Groups {
		at := interactionGroupBase + group.Slot*52
		if got := hex.EncodeToString(m.bytes[at : at+52]); got != group.Raw {
			t.Fatalf("native record %d differs\ngot  %s\nwant %s", group.Slot, got, group.Raw)
		}
	}
	if !reflect.DeepEqual(m.heads(), fixture.Heads) {
		t.Fatalf("complete native occupancy graph differs: got%+v want%+v", m.heads(), fixture.Heads)
	}
}

func TestWhirlwindLiftMatchesNativeWalkerTownAndHeroRecords(t *testing.T) {
	rules := testBundle(t).Whirlwinds.InteractionRules()
	cases := 0
	for _, fixture := range nativeInteractionFixtures(t) {
		if fixture.Input.Mode != "pickup" && fixture.Input.Mode != "town-pickup" {
			continue
		}
		cases++
		t.Run(fixture.Input.Name, func(t *testing.T) {
			m := newNativeInteractionMemory(fixture.Input)
			at := interactionGroupBase + 52
			lift, accepted := rules.Lift(m.bytes[at+22], m.bytes[at+13]&2 != 0, m.word(at+40), 20800)
			if accepted {
				m.bytes[at], m.bytes[at+22], m.bytes[at+25] = lift.Kind, lift.State, lift.Weapon
				m.setWord(at+10, lift.Animation)
				m.setWord(at+32, uint16(lift.EffectReference))
			}
			assertInteractionRecords(t, m, fixture)
			if m.RNG != fixture.RNG {
				t.Fatal("pickup consumed native randomness")
			}
		})
	}
	if cases != 28 {
		t.Fatalf("checked%d pickup fixtures, want28", cases)
	}
}

func TestWhirlwindReleaseMatchesNativeGraphRandomnessAndRegisterClobbers(t *testing.T) {
	rules := testBundle(t).Whirlwinds.InteractionRules()
	cases := 0
	for _, fixture := range nativeInteractionFixtures(t) {
		mode := fixture.Input.Mode
		if mode != "release" && mode != "edge" && !(mode == "outro" && fixture.Input.Animation == 0x6d8) {
			continue
		}
		cases++
		t.Run(fixture.Input.Name, func(t *testing.T) {
			m := newNativeInteractionMemory(fixture.Input)
			for _, group := range fixture.Groups {
				at := interactionGroupBase + group.Slot*52
				m.bytes[at], m.bytes[at+22], m.bytes[at+25] = 8, 0x14, 1
				m.setWord(at+10, 0x4d4)
				m.setWord(at+32, 20800)
				if fixture.Input.ForeignRef && group.Slot == 1 {
					m.setWord(at+32, 20832)
				}
				if fixture.Input.Mixed && group.Slot == 2 {
					m.bytes[at], m.bytes[at+22] = 2, 2
				}
			}
			reads := []nativeInteractionRead{}
			current := NativeRecordReference(0)
			callbacks := WhirlwindReleaseCallbacks{
				DeactivateAndUnlinkEffect: func(reference NativeRecordReference) { at := m.address(reference); m.bytes[at+12] = 0; m.unlink(at) },
				Head: func(tile NativePackedTile) NativeRecordReference {
					return NativeRecordReference(m.word(interactionCell(tile) + 2))
				},
				Record: func(reference NativeRecordReference) WhirlwindReleaseRecord {
					current = reference
					at := m.address(reference)
					return WhirlwindReleaseRecord{Kind: m.bytes[at], Next: NativeRecordReference(m.word(at + 2))}
				},
				ReadWord: func(source WhirlwindWordSource, selector uint16) uint16 {
					read := nativeInteractionRead{Selector: selector, RecordOffset: uint16(current)}
					switch source.Kind {
					case WhirlwindReleaseTable:
						read.Source = "release-table"
						read.BaseOffset = 0x20efa
						read.Word = rules.ReleaseOffsets[selector/2]
					case WhirlwindOccupancyPrefix:
						read.Source = "occupancy-prefix"
						read.BaseOffset = interactionGrid + 2
						read.Word = m.word(interactionGrid + 2 + int(selector))
					case WhirlwindRecordPrefix:
						read.Source = "record"
						read.BaseOffset = uint32(m.address(source.Reference))
						read.Word = m.word(m.address(source.Reference) + int(selector))
					default:
						t.Fatal("unknown native release source")
					}
					reads = append(reads, read)
					return read.Word
				},
				Random: func() int {
					if m.RNG == 0 {
						m.RNG = 0xbc614e
					}
					m.RNG *= 0xbb40e62d
					return int(m.RNG >> 8 & 0x7fff)
				},
				Move: m.move,
				Land: func(reference NativeRecordReference) {
					at := m.address(reference)
					m.bytes[at+22] = 0x1a
					m.setWord(at+10, 0x68c)
				},
				Death: func(reference NativeRecordReference) uint16 {
					at := m.address(reference)
					d1 := uint16(m.bytes[at+12]) * 0x13a
					m.bytes[at+12] = 0
					m.setLong(at+26, 0)
					m.unlink(at)
					return d1
				},
			}
			result, err := rules.Release(20800, NativePackedTile(fixture.Input.X|fixture.Input.Y<<8), callbacks)
			if err != nil {
				t.Fatal(err)
			}
			assertInteractionRecords(t, m, fixture)
			if m.RNG != fixture.RNG || !reflect.DeepEqual(reads, fixture.ReleaseReads) {
				t.Fatalf("native RNG/source trace differs: rng%x want%x reads%+v want%+v", m.RNG, fixture.RNG, reads, fixture.ReleaseReads)
			}
			if m.bytes[interactionEffect+12] != fixture.EffectOwner || m.word(interactionEffect+2) != fixture.EffectNext || m.word(interactionEffect+4) != fixture.EffectPrevious {
				t.Fatal("effect was not deactivated and unlinked before follower release")
			}
			if result.Visited != len(fixture.Groups) || result.Released+result.Removed != len(reads) {
				t.Fatal("dispatcher lost a saved next reference or visited a skipped kind")
			}
		})
	}
	if cases != 33 {
		t.Fatalf("checked%d release fixtures, want33", cases)
	}
}

func TestWhirlwindReleasePreservesCallbackDeathRegisters(t *testing.T) {
	// The leader-removal branch is outside the nonleader native fixtures. Its
	// callback must supply D1, rather than the dispatcher assuming owner*$13a.
	rules := WhirlwindInteractionRules{}
	var sources []WhirlwindWordSource
	var moved [2]uint16
	callbacks := WhirlwindReleaseCallbacks{
		DeactivateAndUnlinkEffect: func(NativeRecordReference) {}, Head: func(NativePackedTile) NativeRecordReference { return 52 },
		Record: func(ref NativeRecordReference) WhirlwindReleaseRecord {
			if ref == 52 {
				return WhirlwindReleaseRecord{Kind: 8, Next: 104}
			}
			return WhirlwindReleaseRecord{Kind: 8}
		},
		Random: func() int { return 0 }, ReadWord: func(source WhirlwindWordSource, _ uint16) uint16 {
			sources = append(sources, source)
			if source.Kind == WhirlwindReleaseTable {
				return 0xffff
			}
			return 1
		},
		Death: func(NativeRecordReference) uint16 { return 0x0101 }, Move: func(_ NativeRecordReference, x, y uint16) bool { moved = [2]uint16{x, y}; return false }, Land: func(NativeRecordReference) {},
	}
	result, err := rules.Release(20800, 0, callbacks)
	if err != nil {
		t.Fatal(err)
	}
	if moved != [2]uint16{640, 384} || result.CoordinateBase != 0x0101 || result.Source != (WhirlwindWordSource{Kind: WhirlwindRecordPrefix, Reference: 52}) || !reflect.DeepEqual(sources, []WhirlwindWordSource{{Kind: WhirlwindReleaseTable}, {Kind: WhirlwindRecordPrefix, Reference: 52}}) {
		t.Fatal("release reconstructed callback-mutated D1 or replaced a same-tile A2 source")
	}
}

func TestWhirlwindReleaseRejectsIncompleteCallbacksBeforeMutation(t *testing.T) {
	changed := false
	_, err := (WhirlwindInteractionRules{}).Release(20800, 0, WhirlwindReleaseCallbacks{DeactivateAndUnlinkEffect: func(NativeRecordReference) { changed = true }})
	if err == nil || changed {
		t.Fatal("invalid callbacks mutated the effect")
	}
}
