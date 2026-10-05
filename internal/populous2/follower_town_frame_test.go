package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

type townFrameInput struct {
	Name                                string
	D                                   [8]uint32
	PropertyCache                       uint16
	Scratch                             [50]uint16
	FrameClock                          uint16
	Terrain                             byte
	Obstruction                         byte
	Land, Slot, FreeSlot                int
	Stage, Owner, Flags                 byte
	Work                                uint16
	Population                          int32
	Mana                                uint32
	PoolBlocked                         uint16
	Clock, Deadline                     uint32
	Selected, SupportFailure, ExtraFree bool
}
type townFrameFixture struct {
	Input                         townFrameInput
	D                             [8]uint32
	PropertyCache, MinimapVariant uint16
	Scratch                       [50]uint16
	RawHash, Trap, Exit           string
	RNG                           uint32
}

func townFrameFixtureMemory(t *testing.T, c townFrameInput) (FollowerCleanupMemory, []byte) {
	t.Helper()
	m, b := serialFixtureMemory(t, serialNativeInput{})
	clear(b)
	_ = m.Write32(0xf40, c.Clock)
	_ = m.Write32(0xdd8, c.Deadline)
	_ = m.Write16(0xdc2, c.PoolBlocked)
	_ = m.Write32(0xeb28, 4311)
	_ = m.Write16(0xeb44, 8)
	_ = m.Write16(0xf42, c.FrameClock)
	for i := 0; i < 4096; i++ {
		b[0xf44+i*4] = 0xa8
		tile := c.Terrain
		if c.SupportFailure {
			tile = 0
		}
		b[0xf45+i*4] = tile
		b[0x4f44+i] = 0x77
	}
	for slot := 1; slot < 400; slot++ {
		if slot == c.FreeSlot || slot == c.Slot || c.ExtraFree && slot == 251 {
			continue
		}
		at := 0x76c0 + slot*52
		b[at], b[at+12], b[at+22] = 4, 1, 0x30
		_ = m.Write16(at+20, 32767)
		_ = m.Write16(at+6, 128)
		_ = m.Write16(at+8, 128)
		_ = m.Write16(at+10, 0x2a8)
	}
	if c.FreeSlot > 0 && c.FreeSlot != c.Slot {
		at := 0x76c0 + c.FreeSlot*52
		for i := 0; i < 52; i++ {
			b[at+i] = byte(0xa0 + i)
		}
		b[at+12] = 0
	}
	at := 0x76c0 + c.Slot*52
	god := 0xe76a + int(c.Owner)*314
	b[at], b[at+1], b[at+12], b[at+13] = 4, c.Stage, c.Owner, c.Flags
	for _, v := range []struct {
		Offset int
		Value  uint16
	}{{6, 0x2080}, {8, 0x2080}, {10, 0x744}, {14, 123}, {16, 456}, {20, c.Work}, {40, 2}, {46, 0x4321}, {48, 0x1234}, {50, 8}} {
		_ = m.Write16(at+v.Offset, v.Value)
	}
	b[at+18], b[at+19], b[at+22], b[at+23], b[at+24], b[at+25] = 40, 0xaa, 6, 2, 2, 7
	_ = m.Write32(at+26, uint32(c.Population))
	_ = m.Write16(0xf44+(32+32*64)*4+2, uint16(c.Slot*52))
	if c.Obstruction != 0 {
		other := 0x76c0 + 3*52
		b[other], b[other+1], b[other+12], b[other+22] = c.Obstruction, 18, 2, 6
		_ = m.Write16(other+6, 0x2080)
		_ = m.Write16(other+8, 0x2080)
		_ = m.Write16(at+2, 3*52)
	}
	_ = m.Write32(god, c.Mana)
	_ = m.Write16(god+8, uint16(c.Slot*52))
	_ = m.Write16(god+0x20, 0xffff)
	_ = m.Write16(god+0x24, 0xffff)
	if c.Selected {
		_ = m.Write32(0xf36, 0x200000+uint32(at))
	}
	return m, b
}
func townFrameRawInsert(ref NativeRecordReference, m FollowerCleanupMemory) error {
	at := cleanupRecordAddress(ref)
	if err := m.Write32(at+2, 0); err != nil {
		return err
	}
	x, err := m.Read8(at + 6)
	if err != nil {
		return err
	}
	y, err := m.Read16(at + 8)
	if err != nil {
		return err
	}
	grid := 0xf44 + int(int16(y&0xff00|uint16(x*4)))
	head, err := m.Read16(grid + 2)
	if err != nil {
		return err
	}
	if head != 0 {
		if err := m.Write16(at+2, head); err != nil {
			return err
		}
		if err := m.Write16(cleanupRecordAddress(NativeRecordReference(head))+4, uint16(ref)); err != nil {
			return err
		}
	}
	return m.Write16(grid+2, uint16(ref))
}

func TestNativeFollowerTownFrameAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_town_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		BSSBytes int
		Cases    []townFrameFixture
	}
	if err := json.Unmarshal(data, &corpus); err != nil || corpus.BSSBytes != 0x11280 || len(corpus.Cases) != 820 {
		t.Fatalf("native town frame corpus incomplete: %v", err)
	}
	bundle := testBundle(t)
	for _, fixture := range corpus.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			input := fixture.Input
			m, b := townFrameFixtureMemory(t, input)
			rules, err := DecodeNativeFollowerTownFrameRules(bundle.Executable, bundle.Raw[fmt.Sprintf("land%d.dat", input.Land)])
			if err != nil {
				t.Fatal(err)
			}
			state := NativeTownFrameState{Property13550: input.PropertyCache, Scratch136E8: input.Scratch}
			frame := NativeFrameRegisterContext{D: input.D, AddressBase: 0x200000}
			callbacks := NativeFollowerTownFrameCallbacks{Memory: m, Frame: &frame, State: &state, Insert: func(ref NativeRecordReference) error { return townFrameRawInsert(ref, m) }, LandAI: func(frame *NativeFrameRegisterContext) error {
				_, err := CreateNativeNeutral(FollowerCleanupRegisters{D0: frame.D[0], D1: frame.D[1], D2: frame.D[2]}, NativeNeutralCallbacks{Memory: m, Insert: func(ref NativeRecordReference) error { return townFrameRawInsert(ref, m) }})
				return err
			}}
			step, err := rules.Tick(NativeRecordReference(input.Slot*52), callbacks)
			if fixture.Trap != "" {
				if err == nil || !strings.Contains(err.Error(), "DIVU zero") {
					t.Fatalf("native zero-divisor trap hidden: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if frame.D != fixture.D {
				t.Errorf("native town eight-register continuation differs: got %x, native %x", frame.D, fixture.D)
			}
			if state.Property13550 != fixture.PropertyCache || state.Scratch136E8 != fixture.Scratch || state.MinimapVariant != fixture.MinimapVariant {
				t.Error("native town CODE cache/scratch/marker differs")
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(b[:0x11280])); got != fixture.RawHash {
				t.Errorf("native town complete BSS differs: %s, native %s", got, fixture.RawHash)
			}
			if binary.BigEndian.Uint32(b[0xeb28:]) != fixture.RNG {
				t.Error("native rare-birth RNG differs")
			}
			if fixture.Trap == "" && step.Redispatch != (fixture.Exit == "1131c") {
				t.Errorf("native town continuation differs: %+v, exit %s", step, fixture.Exit)
			}
		})
	}
}
