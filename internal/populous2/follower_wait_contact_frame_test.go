package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

type waitContactFrameInput struct {
	Name, Mode string
	Initial    []nativeHeroPatch
	D          [8]uint32
}
type waitContactFrameFixture struct {
	Input            waitContactFrameInput
	Hash, Trap, Exit string
	D                [8]uint32
	Calls            []heroFrameCall
	Changes          []nativeHeroChange
	ReturnedA0       uint16
	RNG              uint32
}

func waitContactFrameMemory(input waitContactFrameInput) []byte {
	state := uint32(12)
	if input.Mode == "wait" {
		state = 10
	}
	b := heroFrameMemory(heroFrameInput{nativeHeroInput: nativeHeroInput{Initial: []nativeHeroPatch{{0x76f4 + 13, 1, 0}, {0x76f4 + 10, 2, 0xccc}, {0x76f4 + 20, 2, 5}, {0x76f4 + 22, 1, state}}}})
	for i := 0; i < 4096; i++ {
		b[0xf44+i*4], b[0xf45+i*4], b[0x4f44+i] = 0xa8, 15, 0x77
	}
	m := commandNativeMemory(b)
	for _, p := range input.Initial {
		switch p.Width {
		case 1:
			_ = m.Write8(p.Address, uint8(p.Value))
		case 2:
			_ = m.Write16(p.Address, uint16(p.Value))
		case 4:
			_ = m.Write32(p.Address, p.Value)
		}
	}
	return b
}

// The existing complete merge body uses typed field views over the same raw
// bytes. Each operation refreshes its view; original graph and hero-link
// helpers perform their actual mutations instead of fixture-delta replay.
func waitContactFrameMerge(t *testing.T, b []byte, entry FollowerEntryRules, source, target NativeRecordReference, c *NativeFrameRegisterContext) error {
	t.Helper()
	m := &entryFixtureMemory{}
	refresh := func() { copy(m.bytes[:], b) }
	cb := FollowerEntryCallbacks{
		Read: func(ref NativeRecordReference) (FollowerEntryActor, error) { refresh(); return m.read(ref) },
		Write: func(ref NativeRecordReference, a FollowerEntryActor) error {
			refresh()
			if err := m.write(ref, a); err != nil {
				return err
			}
			copy(b, m.bytes[:])
			return nil
		},
		ClearHeroLinks: func(ref NativeRecordReference) error {
			var image NativeRecordImage
			copy(image.Bytes[:], b[NativeRecordImageStart:NativeRecordImageEnd])
			_, err := ClearEntryHeroLinks(&image, ref)
			copy(b[NativeRecordImageStart:NativeRecordImageEnd], image.Bytes[:])
			return err
		},
		SetLeader: func(owner uint8, ref NativeRecordReference) error {
			return commandNativeMemory(b).Write16(heroGodAddress(owner)+8, uint16(ref))
		},
		Selected: func() NativeRecordReference {
			v := binary.BigEndian.Uint32(b[0xf36:])
			if v == 0 {
				return 0
			}
			return NativeRecordReference(uint16(v - c.AddressBase - 0x76c0))
		},
		Select: func(ref NativeRecordReference) error {
			return commandNativeMemory(b).Write32(0xf36, c.AddressBase+uint32(cleanupRecordAddress(ref)))
		},
		Unlink: func(ref NativeRecordReference) error { return heroFrameGraph(b, ref, false) },
	}
	return entry.MergeWithFrame(source, target, c, cb)
}

func TestNativeFollowerWaitContactFrameAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_wait_contact_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		BSSBytes int
		Cases    []waitContactFrameFixture
	}
	if err = json.Unmarshal(data, &corpus); err != nil || corpus.BSSBytes != 0x11280 || len(corpus.Cases) != 947 {
		t.Fatalf("waiting/contact frame corpus incomplete: %v", err)
	}
	bundle := testBundle(t)
	rules, err := DecodeNativeFollowerWaitContactFrameRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	entry, err := DecodeFollowerEntryRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	town, err := DecodeNativeFollowerTownFrameRules(bundle.Executable, bundle.Raw["land0.dat"])
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range corpus.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			b := waitContactFrameMemory(f.Input)
			memory := commandNativeMemory(b)
			c := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			state := NativeTownFrameState{}
			calls := []heroFrameCall{}
			cb := NativeFollowerWaitContactFrameCallbacks{Memory: memory, Frame: &c,
				Merge: func(source, target NativeRecordReference, c *NativeFrameRegisterContext) error {
					calls = append(calls, heroFrameCall{Kind: "merge", Source: uint16(source), Target: uint16(target), D: c.D})
					return waitContactFrameMerge(t, b, entry, source, target, c)
				},
				Contact: func(source, target NativeRecordReference, c *NativeFrameRegisterContext) (FollowerContactStep, error) {
					calls = append(calls, heroFrameCall{Kind: "contact", Source: uint16(source), Target: uint16(target), D: c.D})
					return PrepareFollowerContactWithFrame(source, target, c, FollowerContactCallbacks{Memory: memory, ClearFarms: func(ref NativeRecordReference, tile uint8) error {
						return town.ClearFarms(ref, tile, NativeFollowerTownFrameCallbacks{Memory: memory, Frame: c, State: &state})
					}, Sound: func(uint16) error { return nil }})
				},
			}
			var step NativeFollowerWaitContactFrameStep
			var err error
			if f.Input.Mode == "wait" {
				step, err = rules.TickWaiting(52, cb)
			} else {
				step, err = rules.CompleteContact(52, cb)
			}
			if f.Trap != "" {
				if err == nil || !strings.Contains(err.Error(), "is odd") {
					t.Fatalf("native animation address exception changed: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if c.D != f.D {
				t.Errorf("eight-register continuation differs: got %x, native %x", c.D, f.D)
			}
			if !reflect.DeepEqual(calls, f.Calls) {
				t.Errorf("child call registers differ: got %+v, native %+v", calls, f.Calls)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(b)); got != f.Hash {
				for _, p := range f.Changes {
					if b[p.Address] != p.Value {
						t.Logf("native changed byte %x: got %x, native %x", p.Address, b[p.Address], p.Value)
					}
				}
				t.Errorf("complete BSS differs: got %s, native %s", got, f.Hash)
			}
			if binary.BigEndian.Uint32(b[0xeb28:]) != f.RNG {
				t.Error("native RNG changed")
			}
			if f.Trap == "" && fmt.Sprintf("%x", step.Continuation) != f.Exit {
				t.Errorf("native tail differs: %+v, exit %s", step, f.Exit)
			}
			if f.ReturnedA0 != 52 {
				t.Errorf("native source was not restored: %x", f.ReturnedA0)
			}
		})
	}
}

func TestNativeFollowerCompletedContactRejectsCyclicChain(t *testing.T) {
	rules, err := DecodeNativeFollowerWaitContactFrameRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	b := waitContactFrameMemory(waitContactFrameInput{Initial: []nativeHeroPatch{{0xf44 + (32+32*64)*4 + 2, 2, 52}, {0x76f4 + 2, 2, 52}}})
	before := append([]byte(nil), b...)
	c := NativeFrameRegisterContext{}
	_, err = rules.CompleteContact(52, NativeFollowerWaitContactFrameCallbacks{Memory: commandNativeMemory(b), Frame: &c})
	if err == nil || !strings.Contains(err.Error(), "cyclic") {
		t.Fatalf("unbounded native cycle accepted: %v", err)
	}
	if !reflect.DeepEqual(b, before) {
		t.Fatal("cycle detection mutated native memory")
	}
}
