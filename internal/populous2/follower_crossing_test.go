package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNativeCrossingAdmissionAgainstOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_crossing_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Input struct {
				Name                                             string
				Owner, Header, Tile, Kind, OtherOwner, XP, Stage uint8
				Population                                       uint32
				X, Y                                             uint16
			}
			Exit, SHA256 string
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 2528 {
		t.Fatalf("native crossing catalog incomplete: count%d error%v", len(catalog.Cases), err)
	}
	rules, err := DecodeFollowerCrossingRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			var records NativeRecordImage
			var globals NativeGlobalImage
			var grid NativeOccupancyState
			memory := NativeRuntimeMemory{Records: &records, Globals: &globals}
			in := fixture.Input
			a, b := 0x76f4, 0x7728
			god, _ := NativeDeityAddress(in.Owner)
			memory.Write8(a, 2)
			memory.Write8(a+12, in.Owner)
			memory.Write8(a+18, 20)
			memory.Write8(a+22, 4)
			memory.Write32(a+26, in.Population)
			memory.Write16(a+6, 0x2080)
			memory.Write16(a+8, 0x2080)
			memory.Write16(a+14, 20)
			memory.Write8(god+0x54, in.XP)
			pos := int(in.X>>8) + int(in.Y>>8)*64
			grid.Cells[pos] = NativeOccupancyCell{Header: in.Header, Tile: in.Tile}
			if in.Kind != 0 {
				memory.Write8(b, in.Kind)
				memory.Write8(b+1, in.Stage)
				memory.Write8(b+12, in.OtherOwner)
				grid.Cells[pos].Head = 104
			}
			read8 := func(at int) (uint8, error) {
				if at >= 0xf44 && at < 0x4f44 {
					v, _ := grid.Byte(at - 0xf44)
					return v, nil
				}
				return memory.Read8(at)
			}
			cb := FollowerCrossingCallbacks{Memory: FollowerCleanupMemory{Read8: read8, Read16: func(at int) (uint16, error) {
				if at >= 0xf44 && at < 0x4f44-1 {
					a, _ := read8(at)
					b, _ := read8(at + 1)
					return uint16(a)<<8 | uint16(b), nil
				}
				return memory.Read16(at)
			}, Read32: memory.Read32,
				Write8: func(at int, v uint8) error { _, e := memory.Write8(at, v); return e }, Write16: func(at int, v uint16) error { _, e := memory.Write16(at, v); return e }, Write32: func(at int, v uint32) error { _, e := memory.Write32(at, v); return e }}}
			step, err := rules.Admit(52, in.X, in.Y, cb)
			if err != nil {
				t.Fatal(err)
			}
			exit := "116cc"
			if step.Admitted {
				exit = "11730"
			}
			if step.WallBroken {
				exit = "11ce8"
			}
			if exit != fixture.Exit {
				t.Fatalf("native admission result got%s want%s", exit, fixture.Exit)
			}
			image := append(append([]byte(nil), records.Bytes[:]...), globals.Bytes[:]...)
			if fmt.Sprintf("%x", sha256.Sum256(image)) != fixture.SHA256 {
				t.Fatal("complete native actor/global admission writes differ")
			}
		})
	}
}
