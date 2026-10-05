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

type frameContextInput struct {
	Name          string
	Actor         uint16
	StopPC        uint32
	CaptureAudio  bool
	MainPhysics   bool
	D             [8]uint32
	Stages        []uint32
	Initial, Code []commandNativePatch
	Links         []uint16
	Header, Tile  uint8
	Seed, Clock   uint32
}

func TestNativeFrameAudioRegistersAgainstCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/frame_context_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []frameContextFixture }
	if e := json.Unmarshal(data, &catalog); e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, f := range catalog.Cases {
		if len(f.Input.Stages) != 1 || f.Input.Stages[0] != 0x182ce {
			continue
		}
		count++
		t.Run(f.Input.Name, func(t *testing.T) {
			s, e := DecodeNativeFrameAudioState(testBundle(t).Executable)
			if e != nil {
				t.Fatal(e)
			}
			for _, p := range f.Input.Code {
				if p.Address >= 0x185a8 && p.Address < 0x18ada {
					at := p.Address - 0x185a8
					switch p.Width {
					case 1:
						s.Entries[at] = uint8(p.Value)
					case 2:
						binary.BigEndian.PutUint16(s.Entries[at:], uint16(p.Value))
					case 4:
						binary.BigEndian.PutUint32(s.Entries[at:], p.Value)
					}
				} else if p.Address >= 0x18426 && p.Address < 0x1842e {
					s.Channels[(p.Address-0x18426)/2] = uint16(p.Value)
				}
			}
			c := NativeFrameRegisterContext{D: f.Input.D}
			e = s.TickAudio(&c, NativeFrameAudioCallbacks{Command: func(control, data uint16, input uint32) (uint32, error) { return input, nil }})
			if e != nil {
				t.Fatal(e)
			}
			want := f.Frames[0]
			if c.D != want.D {
				t.Fatalf("audio registers got%08x native%08x", c.D, want.D)
			}
			if !reflect.DeepEqual(s.Entries[:], want.Audio) || s.Channels != want.Channels {
				t.Fatal("native mutable audio software queue differs")
			}
		})
	}
	if count != 54 {
		t.Fatalf("native audio frame coverage%d", count)
	}
}

type frameContextFixture struct {
	Input  frameContextInput
	Frames []struct {
		Routine  uint32
		Input, D [8]uint32
		Hash     string
		RNG      uint32
		Changes  []nativeHeroChange
		ErrorPC  uint32
		Audio    []uint8
		Channels [4]uint16
	}
}

func frameContextInitial(c frameContextInput) []byte {
	b := make([]byte, 0x11280)
	binary.BigEndian.PutUint32(b[0xeb28:], c.Seed)
	binary.BigEndian.PutUint32(b[0xf40:], c.Clock)
	binary.BigEndian.PutUint16(b[0xeb44:], 8)
	binary.BigEndian.PutUint16(b[0xf0c:], 8)
	binary.BigEndian.PutUint16(b[0xeb42:], 1)
	for i := 0; i < 4096; i++ {
		b[0xf44+i*4], b[0xf45+i*4] = c.Header, c.Tile
	}
	for side := 0; side < 3; side++ {
		god, marker := 0xe76a+side*314, 0xe740+side*14
		binary.BigEndian.PutUint32(b[god:], 10000)
		binary.BigEndian.PutUint16(b[god+0x18:], uint16(side))
		binary.BigEndian.PutUint16(b[god+12:], 14)
		binary.BigEndian.PutUint16(b[god+10:], uint16(marker-0x76c0))
		b[marker], b[marker+12] = 20, uint8(side)
		binary.BigEndian.PutUint16(b[marker+6:], 0x1080)
		binary.BigEndian.PutUint16(b[marker+8:], 0x1080)
	}
	for side := 0; side < 2; side++ {
		b[0xeb56+side*10], b[0xeb5e+side*10] = uint8(side+1), uint8(2+side*2)
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
	for _, ref := range append([]uint16{0x7080, 0x708e, 0x709c}, c.Links...) {
		at := cleanupRecordAddress(NativeRecordReference(ref))
		binary.BigEndian.PutUint32(b[at+2:], 0)
		packed := uint16(b[at+8])<<8 | uint16(b[at+6])
		grid := tsunamiGrid(packed)
		head := binary.BigEndian.Uint16(b[grid+2:])
		binary.BigEndian.PutUint16(b[at+2:], head)
		if head != 0 {
			binary.BigEndian.PutUint16(b[cleanupRecordAddress(NativeRecordReference(head))+4:], ref)
		}
		binary.BigEndian.PutUint16(b[grid+2:], ref)
	}
	return b
}

func TestNativeFrameWallAndSceneryRegistersAgainstCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/frame_context_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []frameContextFixture }
	if e := json.Unmarshal(data, &catalog); e != nil {
		t.Fatal(e)
	}
	wall, e := DecodeNativeWallRules(testBundle(t).Executable)
	if e != nil {
		t.Fatal(e)
	}
	forest := testBundle(t).ForestNative
	count := 0
	for _, f := range catalog.Cases {
		if len(f.Input.Stages) != 1 || f.Input.Stages[0] != 0x161cc && f.Input.Stages[0] != 0xde36 {
			continue
		}
		count++
		t.Run(f.Input.Name, func(t *testing.T) {
			initial := frameContextInitial(f.Input)
			w := aiFixtureWorld(t, initial)
			w.nativeCallDepth++
			c := NativeFrameRegisterContext{D: f.Input.D}
			if f.Input.Stages[0] == 0x161cc {
				cb := w.nativeWallCallbacks(0)
				cb.Frame = &c
				_, e = wall.TickPool(cb)
			} else {
				cb := w.nativeForestCallbacks()
				cb.Frame = &c
				for i := 0; i < SceneryCapacity; i++ {
					if _, e = forest.Tick(nativeActorReference(NativeSceneryPool, i), uint16(f.Input.Clock), cb); e != nil {
						break
					}
				}
			}
			w.nativeCallDepth--
			if e != nil {
				t.Fatal(e)
			}
			want := f.Frames[0]
			if c.D != want.D {
				t.Fatalf("frame registers got%08x native%08x", c.D, want.D)
			}
			b := aiFixtureWorldBytes(w, initial)
			copy(b[0xeb90:], w.NativeRedrawBytes[:])
			if fmt.Sprintf("%x", sha256.Sum256(b)) != want.Hash {
				t.Fatal("frame boundary complete BSS/RNG differs")
			}
		})
	}
	if count != 1232 {
		t.Fatalf("frame wall/scenery coverage%d", count)
	}
}

func TestNativeFrameCompleteAIRegistersAgainstCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/command_ai_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct {
		Cases []struct {
			Input           aiNativeInput
			PolicyRegisters [8]uint32
			PolicyHash      string
		}
	}
	if e := json.Unmarshal(data, &catalog); e != nil {
		t.Fatal(e)
	}
	rules, e := DecodeNativeAIRules(testBundle(t).Executable)
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			memory := aiNativeFixtureMemory(f.Input)
			initial := make([]byte, 131072)
			copy(initial, memory[:])
			initial[0xeb5e], initial[0xeb68] = 2, 4
			w := aiFixtureWorld(t, initial)
			context := NativeFrameRegisterContext{D: [8]uint32{0, 0, 0, 0, 0xaabbccdd, 0x11226778, 0, 0x33445566}}
			cb := w.nativeAICallbacks(nil)
			_, e := rules.TickFrame(&context, cb)
			if e != nil {
				t.Fatal(e)
			}
			if context.D != f.PolicyRegisters {
				t.Fatalf("full AI registers got%08x native%08x", context.D, f.PolicyRegisters)
			}
		})
	}
}

func TestNativeFrameFireColumnRegistersAgainstFullFXPass(t *testing.T) {
	data, e := os.ReadFile("testdata/frame_context_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []frameContextFixture }
	if e := json.Unmarshal(data, &catalog); e != nil {
		t.Fatal(e)
	}
	count := 0
	for _, f := range catalog.Cases {
		if len(f.Input.Stages) != 1 || f.Input.Stages[0] != 0x1482e {
			continue
		}
		state := uint8(0)
		for _, p := range f.Input.Initial {
			if p.Address == 0xc816 {
				state = uint8(p.Value)
			}
		}
		if state != 2 && state != 4 && state != 6 && state != 8 && state != 10 && state != 12 && state != 0x12 && state != 0x14 {
			continue
		}
		count++
		t.Run(f.Input.Name, func(t *testing.T) {
			initial := frameContextInitial(f.Input)
			w := aiFixtureWorld(t, initial)
			w.nativeCallDepth++
			c := NativeFrameRegisterContext{D: f.Input.D}
			e := w.tickNativeFrameFX(&c, NativeFrameWorldBindings{})
			w.nativeCallDepth--
			if e != nil {
				t.Fatal(e)
			}
			want := f.Frames[0]
			if c.D != want.D {
				t.Fatalf("FX registers got%08x native%08x", c.D, want.D)
			}
			b := aiFixtureWorldBytes(w, initial)
			copy(b[0xeb90:], w.NativeRedrawBytes[:])
			if fmt.Sprintf("%x", sha256.Sum256(b)) != want.Hash {
				t.Fatal("FX complete BSS/RNG differs")
			}
		})
	}
	if count != 48 {
		t.Fatalf("full fire-column frame coverage%d", count)
	}
}
