package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

type heroFrameInput struct {
	nativeHeroInput
	D [8]uint32
}

type heroFrameCall struct {
	Kind           string
	Source, Target uint16
	D              [8]uint32
}

type heroFrameFixture struct {
	Input            heroFrameInput
	Hash, Trap, Exit string
	Changes          []nativeHeroChange
	D                [8]uint32
	Calls            []heroFrameCall
	ReturnedA0       uint16
	RNG              uint32
}

func heroFrameMemory(input heroFrameInput) []byte {
	b := make([]byte, 0x11280)
	m := commandNativeMemory(b)
	for i := 0; i < 4096; i++ {
		b[0xf44+i*4], b[0xf45+i*4] = 1, 15
	}
	a := 0x76f4
	b[a], b[a+12], b[a+13], b[a+18], b[a+22] = 2, 1, 2, 16, 0x24
	for _, p := range []nativeHeroPatch{{a + 6, 2, 0x2087}, {a + 8, 2, 0x2093}, {a + 10, 2, 0x1c8}, {a + 14, 2, 17}, {a + 16, 2, 23}, {a + 20, 2, 23}, {a + 26, 4, 1000}, {0xeb28, 4, 4311}} {
		if p.Width == 2 {
			_ = m.Write16(p.Address, uint16(p.Value))
		} else {
			_ = m.Write32(p.Address, p.Value)
		}
	}
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
	if input.Raising {
		_ = m.Write16(0xf12, 1)
	}
	return b
}

// The committed cleanup graph primitive is replayed against the complete
// fixture backing. Its individual raw writes are applied in their source
// order, so preceding cleanup writes and native aliases remain visible.
func heroFrameGraph(b []byte, ref NativeRecordReference, insert bool) error {
	m := &cleanupMemory{record: true}
	copy(m.lower[:], b[:NativeRecordImageStart])
	copy(m.records.Bytes[:], b[NativeRecordImageStart:NativeMagnetImageStart])
	copy(m.globals.Bytes[:], b[NativeMagnetImageStart:NativeRuntimeImageEnd])
	var err error
	if insert {
		err = m.insert(ref)
	} else {
		err = m.unlink(ref)
	}
	if err != nil {
		return err
	}
	out := commandNativeMemory(b)
	for _, w := range m.writes {
		switch w.Width {
		case 1:
			err = out.Write8(w.BSSAddress, uint8(w.Value))
		case 2:
			err = out.Write16(w.BSSAddress, uint16(w.Value))
		case 4:
			err = out.Write32(w.BSSAddress, w.Value)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func TestNativeFollowerHeroFrameAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_hero_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		BSSBytes int
		Cases    []heroFrameFixture
	}
	if err = json.Unmarshal(data, &corpus); err != nil || corpus.BSSBytes != 0x11280 || len(corpus.Cases) != 2462 {
		t.Fatalf("hero frame corpus incomplete: %v", err)
	}
	bundle := testBundle(t)
	rules, err := DecodeNativeFollowerHeroFrameRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	terrain, err := DecodeNativeCommandRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	towns, err := DecodeNativeFollowerTownFrameRules(bundle.Executable, bundle.Raw["land0.dat"])
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range corpus.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			b := heroFrameMemory(f.Input)
			memory := commandNativeMemory(b)
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			state := NativeTownFrameState{}
			calls := []heroFrameCall{}
			farms := func(ref NativeRecordReference, tile uint8) error {
				return towns.ClearFarms(ref, tile, NativeFollowerTownFrameCallbacks{Memory: memory, Frame: &frame, State: &state})
			}
			cb := NativeFollowerHeroFrameCallbacks{Memory: memory, Frame: &frame,
				Raise: func(c *NativeFrameRegisterContext) error {
					calls = append(calls, heroFrameCall{Kind: "raise", D: c.D})
					context := c.CommandContext()
					_, err := terrain.DirectTerrain(true, &context, memory)
					c.SetCommandContext(context)
					return err
				},
				Contact: func(source, target NativeRecordReference, c *NativeFrameRegisterContext) (FollowerContactStep, error) {
					calls = append(calls, heroFrameCall{Kind: "contact", Source: uint16(source), Target: uint16(target), D: c.D})
					return PrepareFollowerContactWithFrame(source, target, c, FollowerContactCallbacks{Memory: memory, ClearFarms: farms, Sound: func(uint16) error { return nil }})
				},
				Cleanup: func(ref NativeRecordReference, c *NativeFrameRegisterContext) error {
					calls = append(calls, heroFrameCall{Kind: "cleanup", Source: uint16(ref), D: c.D})
					_, err := CleanupFollowerWithFrame(ref, c, FollowerCleanupCallbacks{Memory: memory, ClearFarms: farms, Unlink: func(ref NativeRecordReference) error { return heroFrameGraph(b, ref, false) }, Insert: func(ref NativeRecordReference) error { return heroFrameGraph(b, ref, true) }})
					return err
				},
			}
			step := NativeFollowerHeroFrameStep{RedispatchSource: 52}
			var err error
			switch f.Input.Mode {
			case "chase":
				step, err = rules.Chase(52, cb)
			case "select":
				_, err = rules.Select(52, cb)
			case "probe":
				frame.Word(0, uint16(b[0x76f4+8])<<8|uint16(b[0x76f4+6]))
				frame.Word(2, uint16(f.Input.DX))
				frame.Word(3, uint16(f.Input.DY))
				err = rules.Probe(52, cb)
			case "plan":
				frame.Byte(0, f.Input.TargetX)
				frame.Byte(1, f.Input.TargetY)
				err = rules.Plan(52, cb)
			default:
				step, err = rules.Tick(52, cb)
			}
			if f.Trap != "" {
				if err == nil {
					t.Fatal("native exception prefix silently continued")
				}
				if f.Trap == "zero_speed" && !errors.Is(err, ErrFollowerZeroSpeed) {
					t.Fatalf("native zero-speed exception changed: %v", err)
				}
				if f.Trap == "odd_code_word" && !strings.Contains(err.Error(), "is odd") {
					t.Fatalf("native odd-address exception changed: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if frame.D != f.D {
				t.Errorf("eight-register continuation differs: got %x, native %x", frame.D, f.D)
			}
			if !reflect.DeepEqual(calls, f.Calls) {
				t.Errorf("actual child call registers differ: got %+v, native %+v", calls, f.Calls)
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
			if f.Input.Mode == "decision24" || f.Input.Mode == "decision26" || f.Input.Mode == "chase" {
				if f.Trap == "" && fmt.Sprintf("%x", step.Continuation) != f.Exit {
					t.Errorf("native decision continuation differs: %+v, exit %s", step, f.Exit)
				}
				if uint16(step.RedispatchSource) != f.ReturnedA0 {
					t.Errorf("redispatch actor differs: got %x, native %x", step.RedispatchSource, f.ReturnedA0)
				}
			}
		})
	}
}
