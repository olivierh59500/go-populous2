package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type leaderFixture struct {
	Input struct {
		Name                 string
		Owner, Flags, X, Y   uint8
		SameCell, MarkerSelf bool
	}
	GodAddress                int
	MarkerReference           uint16
	BeforeSource, AfterSource [52]uint8
	BeforeGod, AfterGod       [314]uint8
	BeforeMarker, AfterMarker [14]uint8
	D0, D1, D2                uint32
	Writes                    []cleanupWrite
	Callbacks                 []string
	Hash                      string
}

func leaderFixtureMemory(t *testing.T, f leaderFixture) *cleanupMemory {
	t.Helper()
	m := &cleanupMemory{}
	for index := range 4096 {
		m.putByte(0xf44+index*4, 0x88)
		m.putByte(0xf44+index*4+1, 15)
	}
	for index, v := range f.BeforeSource {
		m.putByte(0x76f4+index, v)
	}
	if f.GodAddress+len(f.BeforeGod) <= NativeRuntimeImageEnd {
		for index, v := range f.BeforeGod {
			m.putByte(f.GodAddress+index, v)
		}
	}
	marker := cleanupRecordAddress(NativeRecordReference(f.MarkerReference))
	for index, v := range f.BeforeMarker {
		m.putByte(marker+index, v)
	}
	if !f.Input.MarkerSelf {
		if err := m.insert(NativeRecordReference(f.MarkerReference)); err != nil {
			t.Fatal(err)
		}
	}
	if f.Input.X < 64 && f.Input.Y < 64 {
		if err := m.insert(52); err != nil {
			t.Fatal(err)
		}
	}
	if m.err != nil {
		t.Fatal(m.err)
	}
	return m
}

// The original $140ae/$13fe4 and graph primitives execute with no stubs.
// Ordered writes and the full BSS hash prove that population, loss metrics,
// pressure and all unrelated bytes survive marker relocation.
func TestFollowerLeaderAgainstOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_leader_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []leaderFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 56 {
		t.Fatal("native leader catalog incomplete")
	}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			m := leaderFixtureMemory(t, fixture)
			m.record = true
			callbacks := []string{}
			memory := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
			step, err := ClearFollowerLeader(52, FollowerCleanupRegisters{D0: 0x12345678, D1: 0x89abcdef, D2: 0xfedcba98}, FollowerLeaderCallbacks{Memory: memory,
				Unlink: func(ref NativeRecordReference) error {
					callbacks = append(callbacks, fmt.Sprintf("Unlink:%04x", uint16(ref)))
					return m.unlink(ref)
				},
				Insert: func(ref NativeRecordReference) error {
					callbacks = append(callbacks, fmt.Sprintf("Insert:%04x", uint16(ref)))
					return m.insert(ref)
				},
			})
			if err != nil || m.err != nil {
				t.Fatalf("native leader failed: %v/%v", err, m.err)
			}
			if step.Registers != (FollowerCleanupRegisters{D0: fixture.D0, D1: fixture.D1, D2: fixture.D2}) || step.GodAddress != fixture.GodAddress || step.Cleared != (fixture.Input.Flags&1 != 0) {
				t.Fatalf("native registers/context differ: %+v", step)
			}
			if step.Cleared && uint16(step.MarkerReference) != fixture.MarkerReference {
				t.Fatal("native raw marker reference differs")
			}
			if !reflect.DeepEqual(m.writes, fixture.Writes) || !reflect.DeepEqual(callbacks, fixture.Callbacks) {
				t.Fatalf("native write/graph order differs\ngot%v\nwant%v", m.writes, fixture.Writes)
			}
			var whole [NativeRuntimeImageEnd]uint8
			for at := range whole {
				whole[at] = m.byte(at)
			}
			if m.err != nil {
				t.Fatal(m.err)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(whole[:])); got != fixture.Hash {
				t.Fatalf("complete native BSS differs: got%s want%s", got, fixture.Hash)
			}
			for index, v := range fixture.AfterSource {
				if m.byte(0x76f4+index) != v {
					t.Fatal("native source record differs")
				}
			}
			marker := cleanupRecordAddress(NativeRecordReference(fixture.MarkerReference))
			for index, v := range fixture.AfterMarker {
				if m.byte(marker+index) != v {
					t.Fatal("native marker record differs")
				}
			}
			if fixture.GodAddress+len(fixture.AfterGod) <= NativeRuntimeImageEnd {
				for index, v := range fixture.AfterGod {
					if m.byte(fixture.GodAddress+index) != v {
						t.Fatal("native deity/alias record differs")
					}
				}
			} else if fixture.Input.Flags&1 != 0 {
				t.Fatal("out-of-range leader fixture cannot be validated")
			}
		})
	}
}
