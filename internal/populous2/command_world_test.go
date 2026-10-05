package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type commandNativeInput struct {
	Name, Mode                string
	Context                   [8]uint32
	Owner, Command, X, Y      uint8
	Free, Pause, GameMode     uint16
	Mana                      uint32
	XP                        uint8
	Zero                      bool
	Seed                      uint32
	Header, Tile              uint8
	Busy                      int
	Sequence                  []uint8
	FollowerState, FirstFlags uint8
	Enabled                   uint16
	Initial                   []commandNativePatch
}
type commandNativePatch struct {
	Address, Width int
	Value          uint32
}
type commandNativeCall struct {
	Routine int
	Context [8]uint32
}
type commandNativeFixture struct {
	Input     commandNativeInput
	Hash      string
	Changes   []nativeHeroChange
	Registers [8]uint32
	Calls     []commandNativeCall
}

func commandNativeInitial(c commandNativeInput) []byte {
	b := make([]byte, 131072)
	for side := 0; side < 3; side++ {
		god := 0xe76a + side*314
		binary.BigEndian.PutUint32(b[god:], c.Mana)
		binary.BigEndian.PutUint16(b[god+8:], 0x5240)
		binary.BigEndian.PutUint16(b[god+0x44:], 0xfffd)
		binary.BigEndian.PutUint16(b[god+0x138:], 0xfffe)
		for j := 0; j < 6; j++ {
			b[god+0x52+j] = c.XP
		}
	}
	for _, v := range []struct {
		at    int
		value uint16
	}{{0xf0e, c.Free}, {0xf3c, c.Pause}, {0xeb44, c.GameMode}, {0xeb2c, 4}, {0xeb2e, 0}, {0xdd2, 7}} {
		binary.BigEndian.PutUint16(b[v.at:], v.value)
	}
	b[0xeb56], b[0xeb57], b[0xeb58], b[0xeb59] = c.Owner, c.Command, c.X, c.Y
	binary.BigEndian.PutUint16(b[0xf12:], c.Enabled)
	if c.Mode == "world" || c.Mode == "lower" || c.Mode == "raise" {
		binary.BigEndian.PutUint32(b[0xeb28:], c.Seed)
		for i := 0; i < 4096; i++ {
			b[0xf44+i*4] = c.Header
			b[0xf45+i*4] = c.Tile
			b[0x4f44+i] = 0x77
		}
		for i := 0; i < 250; i++ {
			a := 0xc800 + i*32
			for j := 0; j < 32; j++ {
				b[a+j] = uint8(i*17 + j*7 + 0x21)
			}
			binary.BigEndian.PutUint32(b[a+2:], 0)
			binary.BigEndian.PutUint16(b[a+6:], 31*256+128)
			binary.BigEndian.PutUint16(b[a+8:], 31*256+128)
			b[a], b[a+22], b[a+12] = 0x22, 2, 0
			if i < c.Busy {
				b[a+12] = 1
			}
		}
		insert := func(a, grid int) {
			ref := uint16(a - 0x76c0)
			head := binary.BigEndian.Uint16(b[grid+2:])
			binary.BigEndian.PutUint16(b[a+2:], head)
			if head != 0 {
				binary.BigEndian.PutUint16(b[0x76c0+int(head)+4:], ref)
			}
			binary.BigEndian.PutUint16(b[grid+2:], ref)
		}
		for i := 0; i < 3; i++ {
			a := 0xe740 + i*14
			b[a], b[a+12] = 0x14, uint8(i)
			binary.BigEndian.PutUint16(b[a+6:], 5*256+128)
			binary.BigEndian.PutUint16(b[a+8:], 5*256+128)
			binary.BigEndian.PutUint16(b[0xe76a+i*314+10:], uint16(a-0x76c0))
			insert(a, 0xf44+5*256+5*4)
		}
		for i := 0; i < 3; i++ {
			a := 0x76f4 + i*52
			b[a] = 2
			if i == 2 {
				b[a] = 4
			}
			b[a+12] = uint8(i%2 + 1)
			b[a+22] = c.FollowerState
			if i == 0 {
				b[a+13] = c.FirstFlags
			}
			if i == 1 {
				b[a+13] = 2
			}
			binary.BigEndian.PutUint16(b[a+6:], uint16(c.X)*256+128)
			binary.BigEndian.PutUint16(b[a+8:], uint16(c.Y)*256+128)
			binary.BigEndian.PutUint32(b[a+26:], uint32(100+i*300))
			insert(a, 0xf44+int(c.Y)*256+int(c.X)*4)
		}
	}
	for _, p := range c.Initial {
		switch p.Width {
		case 1:
			b[p.Address] = uint8(p.Value)
		case 2:
			binary.BigEndian.PutUint16(b[p.Address:], uint16(p.Value))
		case 4:
			binary.BigEndian.PutUint32(b[p.Address:], p.Value)
		}
	}
	return b
}

func commandNativeMemory(b []byte) FollowerCleanupMemory {
	return FollowerCleanupMemory{
		Read8: func(a int) (uint8, error) {
			if a < 0 || a >= len(b) {
				return 0, fmt.Errorf("byte%x outside fixture", a)
			}
			return b[a], nil
		},
		Read16: func(a int) (uint16, error) {
			if a < 0 || a+2 > len(b) {
				return 0, fmt.Errorf("word%x outside fixture", a)
			}
			return binary.BigEndian.Uint16(b[a:]), nil
		},
		Read32: func(a int) (uint32, error) {
			if a < 0 || a+4 > len(b) {
				return 0, fmt.Errorf("long%x outside fixture", a)
			}
			return binary.BigEndian.Uint32(b[a:]), nil
		},
		Write8: func(a int, v uint8) error {
			if a < 0 || a >= len(b) {
				return fmt.Errorf("byte%x outside fixture", a)
			}
			b[a] = v
			return nil
		},
		Write16: func(a int, v uint16) error {
			if a < 0 || a+2 > len(b) {
				return fmt.Errorf("word%x outside fixture", a)
			}
			binary.BigEndian.PutUint16(b[a:], v)
			return nil
		},
		Write32: func(a int, v uint32) error {
			if a < 0 || a+4 > len(b) {
				return fmt.Errorf("long%x outside fixture", a)
			}
			binary.BigEndian.PutUint32(b[a:], v)
			return nil
		},
	}
}

// This corpus executes the original inner effect bodies rather than RTS
// substitutes. The enclosing dispatcher and World receive matching recycled
// pools, mixed linked followers, terrain, RNG and explicit incoming registers.
func TestWorldNativeNormalCommandsAgainstOriginalBodies(t *testing.T) {
	data, err := os.ReadFile("testdata/command_world_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []commandNativeFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 14061 {
		t.Fatal("native World command body corpus incomplete")
	}
	bundle := testBundle(t)
	rules, err := DecodeNativeCommandRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	wall, err := DecodeNativeWallRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			initial := commandNativeInitial(f.Input)
			w := aiFixtureWorld(t, initial)
			w.nativeCallDepth++
			context := NativeCommandRegisterContext{D: f.Input.Context}
			placement := NativeWallPlacementState{}
			calls := []commandNativeCall{}
			cb := w.nativeNormalCommandCallbacks(NativeCommandWorldBindings{WallRules: &wall, WallPlacement: &placement, Trace: func(call NativeCommandCall) {
				calls = append(calls, commandNativeCall{call.Routine, call.Context.D})
			}})
			sequence := f.Input.Sequence
			if len(sequence) == 0 {
				sequence = []uint8{f.Input.Command}
			}
			var err error
			for _, command := range sequence {
				if err = cb.Memory.Write8(0xeb57, command); err != nil {
					break
				}
				_, err = rules.Execute(0xeb56, &context, cb)
				if err != nil {
					break
				}
			}
			w.nativeCallDepth--
			if err != nil {
				t.Fatal(err)
			}
			if context.D != f.Registers {
				t.Fatalf("World command register continuation differs: %08x native%08x", context.D, f.Registers)
			}
			if !reflect.DeepEqual(calls, f.Calls) {
				t.Fatalf("World command source call order/context differs: %+v native%+v", calls, f.Calls)
			}
			after := aiFixtureWorldBytes(w, initial)
			if fmt.Sprintf("%x", sha256.Sum256(after)) != f.Hash {
				changes := []nativeHeroChange{}
				for at, v := range after {
					if v != initial[at] {
						changes = append(changes, nativeHeroChange{Address: at, Value: v})
					}
				}
				t.Fatalf("World command complete memory differs: %+v native%+v", changes, f.Changes)
			}
		})
	}
}

func TestNativeCommandAdmissionAndDispatcherAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/command_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []commandNativeFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 4542 {
		t.Fatalf("native command corpus incomplete: %d", len(catalog.Cases))
	}
	rules, err := DecodeNativeCommandRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			initial := commandNativeInitial(f.Input)
			b := append([]byte(nil), initial...)
			m := commandNativeMemory(b)
			context := NativeCommandRegisterContext{D: f.Input.Context}
			calls := []commandNativeCall{}
			switch f.Input.Mode {
			case "cost":
				err = rules.Cost(&context, m)
			case "admit":
				_, err = rules.Admit(&context, m)
			case "execute":
				_, err = rules.Execute(0xeb56, &context, NativeCommandCallbacks{Memory: m, Call: func(call NativeCommandCall) (bool, error) {
					calls = append(calls, commandNativeCall{call.Routine, call.Context.D})
					if call.Routine != 0x184f6 {
						for j := range call.Context.D {
							call.Context.D[j] ^= uint32((j + 1) * 0x12340000)
						}
						commandWord(call.Context, 0, 3)
					}
					return f.Input.Zero, nil
				}})
			default:
				t.Fatal("unknown command fixture mode")
			}
			if err != nil {
				t.Fatal(err)
			}
			if context.D != f.Registers {
				t.Fatalf("native command registers differ: %08x native%08x", context.D, f.Registers)
			}
			if !reflect.DeepEqual(calls, f.Calls) {
				t.Fatalf("native command inner order/context differs: %+v native%+v", calls, f.Calls)
			}
			if fmt.Sprintf("%x", sha256.Sum256(b)) != f.Hash {
				changes := []nativeHeroChange{}
				for at, v := range b {
					if v != initial[at] {
						changes = append(changes, nativeHeroChange{Address: at, Value: v})
					}
				}
				t.Fatalf("native command full BSS differs: %+v native%+v", changes, f.Changes)
			}
		})
	}
}
