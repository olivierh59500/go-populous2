package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type nativeAudioIRQEvent struct {
	Tick          int
	Kind          string
	Control, Data uint16
}
type nativeAudioIRQFixture struct {
	Input struct {
		Name   string
		D      [8]uint32
		Music  bool
		Ticks  int
		Events []nativeAudioIRQEvent
	}
	Frames []struct {
		Tick                int
		D                   [8]uint32
		DriverHash, DMAHash string
		Hardware            []NativeFrameHardwareWrite
	}
}

func TestNativeAudioIRQAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/audio_native_irq_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct{ Cases []nativeAudioIRQFixture }
	if err = json.Unmarshal(data, &corpus); err != nil || len(corpus.Cases) != 142 {
		t.Fatalf("native IRQ corpus incomplete: %v", err)
	}
	bundle := testBundle(t)
	for _, f := range corpus.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			d, err := NewNativeAudioDevice(bundle.Executable, bundle.Raw["fx.dat"], 0x100000, 0x800000, 0)
			if err != nil {
				t.Fatal(err)
			}
			absolute := [16]byte{4: 0, 5: 0x90, 6: 0, 7: 0} // Original oracle's ExecBase at address four.
			d.ReadAbsolute8 = func(address uint32) (uint8, error) {
				if address >= uint32(len(absolute)) {
					return 0, fmt.Errorf("absolute audio read %x outside supplied oracle memory", address)
				}
				return absolute[address], nil
			}
			c := NativeFrameRegisterContext{D: f.Input.D}
			if _, err = d.InitializeWithFrame(&c); err != nil {
				t.Fatal(err)
			}
			if f.Input.Music {
				c.D[0], err = d.MusicCommand(0x80f, 0, c.D[0])
				if err != nil {
					t.Fatal(err)
				}
			}
			if len(f.Frames) != f.Input.Ticks {
				t.Fatal("native IRQ trace incomplete")
			}
			for _, frame := range f.Frames {
				d.Hardware = nil
				for _, e := range f.Input.Events {
					if e.Tick != frame.Tick {
						continue
					}
					switch e.Kind {
					case "cue":
						c.Word(0, e.Data)
						err = d.DirectCue(e.Data, &c)
					case "music":
						c.D[0], err = d.MusicCommand(e.Control, e.Data, c.D[0])
					default:
						c.D[0], err = d.Command(e.Control, e.Data, c.D[0])
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				if err = d.TickCIA(&c); err != nil {
					t.Fatal(frame.Tick, err)
				}
				if c.D != frame.D {
					t.Fatalf("tick%d full IRQ register boundary differs: got %x, native %x", frame.Tick, c.D, frame.D)
				}
				if !reflect.DeepEqual(d.Hardware, frame.Hardware) {
					t.Fatalf("tick%d actual Paula write order differs: got %+v, native %+v", frame.Tick, d.Hardware, frame.Hardware)
				}
				if got := fmt.Sprintf("%x", sha256.Sum256(d.Code[0x18b16:0x18ee4])); got != frame.DriverHash {
					t.Fatalf("tick%d complete mutable driver differs: %s/%s", frame.Tick, got, frame.DriverHash)
				}
				if got := fmt.Sprintf("%x", sha256.Sum256(d.Code[0x194e2:0x194ea])); got != frame.DMAHash {
					t.Fatalf("tick%d complete DMAphase state differs: %s/%s", frame.Tick, got, frame.DMAHash)
				}
			}
		})
	}
}
