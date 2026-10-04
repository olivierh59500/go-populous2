package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestNativeNeutralRuntimeAgainstOriginalEffectsAndMovement(t *testing.T) {
	data, err := os.ReadFile("testdata/neutral_runtime_native.json")
	if err != nil {
		t.Fatal(err)
	}
	type call struct {
		Name       string
		D0, D1, D2 uint32
	}
	var catalog struct {
		Cases []struct {
			Input struct {
				Name      string
				Selector  uint16
				X, Y      uint16
				VX, VY    int16
				Seed      uint32
				Tile      uint8
				Animation uint16
			}
			Calls                   []call
			ImageSHA256, GridSHA256 string
			RNG                     uint32
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 252 {
		t.Fatalf("neutral runtime catalog incomplete: %v", err)
	}
	rules, err := DecodeNativeNeutralRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Input.Name, func(t *testing.T) {
			var records NativeRecordImage
			var globals NativeGlobalImage
			var grid NativeOccupancyState
			memory := NativeRuntimeMemory{Records: &records, Globals: &globals}
			at := 0x76f4
			in := fixture.Input
			memory.Write8(at, 0x3c)
			memory.Write8(at+12, 3)
			memory.Write8(at+22, 0x44)
			memory.Write16(at+40, in.Selector)
			memory.Write16(at+6, in.X)
			memory.Write16(at+8, in.Y)
			memory.Write16(at+14, uint16(in.VX))
			memory.Write16(at+16, uint16(in.VY))
			memory.Write16(at+10, in.Animation)
			for pos := range grid.Cells {
				grid.Cells[pos] = NativeOccupancyCell{Header: 0xa8, Tile: in.Tile}
			}
			grid.Cells[int(in.X>>8)+int(in.Y>>8)*64].Head = 52
			rng := in.Seed
			calls := []call{}
			mem := FollowerCleanupMemory{Read8: memory.Read8, Read16: memory.Read16, Read32: memory.Read32,
				Write8: func(a int, v uint8) error { _, e := memory.Write8(a, v); return e }, Write16: func(a int, v uint16) error { _, e := memory.Write16(a, v); return e }, Write32: func(a int, v uint32) error { _, e := memory.Write32(a, v); return e }}
			cb := NativeNeutralActorCallbacks{Memory: mem,
				Move: func(ref NativeRecordReference, x, y uint16) error {
					_, e := grid.Move(ref, x, y, memory.RecordAccess())
					return e
				},
				Unlink: func(ref NativeRecordReference) error { return grid.Remove(ref, memory.RecordAccess()) },
				Tile: func(p NativePackedTile) (uint8, error) {
					return grid.Cells[int(uint8(p))+int(uint8(uint16(p)>>8))*64].Tile, nil
				},
				WriteTile: func(p NativePackedTile, v uint8) error {
					grid.Cells[int(uint8(p))+int(uint8(uint16(p)>>8))*64].Tile = v
					return nil
				},
				Head: func(p NativePackedTile) (NativeRecordReference, error) {
					return grid.Cells[int(uint8(p))+int(uint8(uint16(p)>>8))*64].Head, nil
				},
				Random: func() int {
					if rng == 0 {
						rng = 0xbc614e
					}
					rng *= 0xbb40e62d
					return int(rng >> 8 & 0x7fff)
				},
				Lower: func(x, y uint8) error {
					calls = append(calls, call{"lower", uint32(x), uint32(y), 0})
					return nil
				},
				CreateWhirlwind: func(x, y uint8, owner uint16) error {
					calls = append(calls, call{"ignite", uint32(x), uint32(y), uint32(owner)})
					return nil
				},
				PlantTree: func(x, y uint8, owner uint16) error {
					calls = append(calls, call{"plant", uint32(x), uint32(y), uint32(owner)})
					return nil
				},
				PlantFungus: func(x, y uint8, owner uint16) error {
					calls = append(calls, call{"fungus", uint32(x), uint32(y), uint32(owner)})
					return nil
				},
				Cleanup: func(NativeRecordReference, uint16) error { return fmt.Errorf("unexpected empty-neighbor cleanup") }}
			if _, err := rules.Tick(52, cb); err != nil {
				t.Fatal(err)
			}
			// The callback fixtures retain complete original D0/D1/D2 values.
			// Compare semantic coordinate/owner arguments, while raw register
			// clobbers remain at the explicitly external primitive boundary.
			for index := range fixture.Calls {
				fixture.Calls[index].D0 &= 255
				fixture.Calls[index].D1 &= 255
				fixture.Calls[index].D2 &= 65535
			}
			if !reflect.DeepEqual(calls, fixture.Calls) {
				t.Fatalf("effect callback arguments differ: got%+v want%+v", calls, fixture.Calls)
			}
			image := append(append([]byte(nil), records.Bytes[:]...), globals.Bytes[:]...)
			var mapBytes [16384]byte
			for offset := range mapBytes {
				mapBytes[offset], _ = grid.Byte(offset)
			}
			if fmt.Sprintf("%x", sha256.Sum256(image)) != fixture.ImageSHA256 || fmt.Sprintf("%x", sha256.Sum256(mapBytes[:])) != fixture.GridSHA256 || rng != fixture.RNG {
				t.Fatal("native neutral movement/image/grid/RNG differs")
			}
		})
	}
}
