package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type nativeAudioDeviceFixture struct {
	Input struct {
		Name        string
		D           [8]uint32
		Disabled    uint32
		HasDisabled bool
	}
	InitialHash     string
	InitialHardware []NativeFrameHardwareWrite
	Frames          []struct {
		Step struct {
			Kind          string
			Control, Data uint16
		}
		InputD, D  [8]uint32
		DriverHash string
		Hardware   []NativeFrameHardwareWrite
	}
}

func TestNativeAudioDeviceInitializationAndCommandsAgainstCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/audio_native_device_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct{ Cases []nativeAudioDeviceFixture }
	if err = json.Unmarshal(data, &corpus); err != nil || len(corpus.Cases) != 465 {
		t.Fatalf("initialized audio corpus incomplete: %v", err)
	}
	bundle := testBundle(t)
	for _, f := range corpus.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			d, err := NewNativeAudioDevice(bundle.Executable, bundle.Raw["fx.dat"], 0x100000, 0x800000, 0)
			if err != nil {
				t.Fatal(err)
			}
			c := NativeFrameRegisterContext{D: f.Input.D}
			hardware, err := d.InitializeWithFrame(&c)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(hardware, f.InitialHardware) {
				t.Errorf("initial hardware requests differ: got %+v, native %+v", hardware, f.InitialHardware)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(d.Code[0x18b16:0x18ee4])); got != f.InitialHash {
				t.Errorf("complete initialized driver differs: %s/%s", got, f.InitialHash)
			}
			if f.Input.HasDisabled {
				d.ResourceBase = f.Input.Disabled
			}
			for _, frame := range f.Frames {
				d.Hardware = nil
				if frame.Step.Kind == "cue" {
					c.Word(0, frame.Step.Data)
					err = d.DirectCue(frame.Step.Data, &c)
				} else {
					c.D[0], err = d.Command(frame.Step.Control, frame.Step.Data, c.D[0])
				}
				if err != nil {
					t.Fatal(err)
				}
				if c.D != frame.D {
					t.Errorf("device caller/status registers differ: got %x, native %x", c.D, frame.D)
				}
				if !reflect.DeepEqual(d.Hardware, frame.Hardware) {
					t.Errorf("device hardware request order differs: got %+v, native %+v", d.Hardware, frame.Hardware)
				}
				if got := fmt.Sprintf("%x", sha256.Sum256(d.Code[0x18b16:0x18ee4])); got != frame.DriverHash {
					t.Errorf("complete device state differs: %s/%s", got, frame.DriverHash)
				}
			}
		})
	}
}
