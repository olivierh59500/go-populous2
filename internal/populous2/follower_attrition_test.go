package populous2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestFollowerAttritionAgainstCompleteOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_attrition_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Input struct {
				Name         string
				Owner, Flags uint8
				Population   int32
				Amount       uint32
			}
			Hash, SourceRaw string
			D0              uint32
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 180 {
		t.Fatal("native attrition catalog incomplete")
	}
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			m := &cleanupMemory{}
			c := f.Input
			at := 0x76f4
			god := 0xe76a + int(c.Owner)*314
			marker := 0xe740 + int(c.Owner)*14
			ref := NativeRecordReference(marker - 0x76c0)
			for i := range 4096 {
				m.putByte(0xf44+i*4, 0x88)
				m.putByte(0xf44+i*4+1, 15)
			}
			m.putByte(at, 2)
			m.putByte(at+12, c.Owner)
			m.putByte(at+13, c.Flags)
			m.putByte(at+19, 0xaa)
			m.putByte(at+22, 0x24)
			m.putWord(at+6, 0x2011)
			m.putWord(at+8, 0x202b)
			m.putWord(at+10, 8)
			m.write32(at+26, uint32(c.Population))
			m.putWord(at+40, 0)
			m.write32(god+20, c.Amount)
			m.putWord(god+68, 30)
			m.putWord(god+8, 52)
			m.putWord(god+10, uint16(ref))
			m.putByte(marker, 20)
			m.putByte(marker+12, c.Owner)
			m.putWord(marker+6, 0x0880)
			m.putWord(marker+8, 0x0980)
			if err := m.insert(ref); err != nil {
				t.Fatal(err)
			}
			if err := m.insert(52); err != nil {
				t.Fatal(err)
			}
			read := func(ref NativeRecordReference) (FollowerEntryActor, error) { return m.records.ReadFollowerEntry(ref) }
			write := func(ref NativeRecordReference, a FollowerEntryActor) error {
				old, err := read(ref)
				if err != nil {
					return err
				}
				_, err = m.records.PatchFollowerEntry(ref, old, a)
				return err
			}
			mem := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
			step, err := ApplyFollowerAttrition(52, c.Amount, FollowerAttritionCallbacks{Read: read, Write: write, Cleanup: func(ref NativeRecordReference, mode uint16) error {
				_, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: uint32(mode)}, FollowerCleanupCallbacks{Memory: mem, Unlink: m.unlink, Insert: m.insert, ClearFarms: func(NativeRecordReference, uint8) error { return fmt.Errorf("ordinary attrition cannot clear farms") }})
				return err
			}})
			if err != nil {
				t.Fatal(err)
			}
			if step.D0 != f.D0 || step.Died != (f.D0 != 0) {
				t.Fatal("native attrition branch/return differs")
			}
			offset := at - NativeRecordImageStart
			if hex.EncodeToString(m.records.Bytes[offset:offset+52]) != f.SourceRaw {
				t.Fatal("native attrition source fields differ")
			}
			bytes := append([]byte(nil), m.lower[:]...)
			bytes = append(bytes, m.records.Bytes[:]...)
			bytes = append(bytes, m.globals.Bytes[:]...)
			if fmt.Sprintf("%x", sha256.Sum256(bytes)) != f.Hash {
				t.Fatal("complete native attrition retainedBSS/globals/grid differ")
			}
		})
	}
}
