package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
)

func TestNativeAudioPCMAgainstOriginalCPUHardwareTraces(t *testing.T) {
	data, err := os.ReadFile("testdata/audio_native_pcm_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Scenario         string
			PAL              bool
			SampleRate       int
			TimerLow         uint8
			Seconds          int
			SHA256           string
			Peak, IRQUpdates int
		}
	}
	if err = json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 5 {
		t.Fatalf("native hardware PCM corpus incomplete: %v", err)
	}
	data, err = os.ReadFile("testdata/audio_native_irq_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var traces struct{ Cases []nativeAudioIRQFixture }
	if err = json.Unmarshal(data, &traces); err != nil {
		t.Fatal(err)
	}
	byname := map[string]nativeAudioIRQFixture{}
	for _, f := range traces.Cases {
		byname[f.Input.Name] = f
	}
	bundle := testBundle(t)
	for _, f := range catalog.Cases {
		t.Run(fmt.Sprintf("%s-pal%t-rate%d-low%d", f.Scenario, f.PAL, f.SampleRate, f.TimerLow), func(t *testing.T) {
			for _, chunk := range []int{f.SampleRate * f.Seconds * 4, 1, 127, 4097} {
				d, err := NewNativeAudioDevice(bundle.Executable, bundle.Raw["fx.dat"], 0x100000, 0x800000, f.TimerLow)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = d.Initialize(); err != nil {
					t.Fatal(err)
				}
				absolute := [16]byte{5: 0x90}
				d.ReadAbsolute8 = func(address uint32) (uint8, error) {
					if address >= uint32(len(absolute)) {
						return 0, fmt.Errorf("native absolute address %x not supplied", address)
					}
					return absolute[address], nil
				}
				timing := NativePALAudioTiming(f.SampleRate, f.TimerLow)
				if !f.PAL {
					timing = NativeNTSCAudioTiming(f.SampleRate, f.TimerLow)
				}
				p, err := NewNativeAudioPCM(d, timing)
				if err != nil {
					t.Fatal(err)
				}
				source := byname[f.Scenario]
				if source.Input.Music {
					if _, err = p.MusicCommand(0x8f, 0, 0); err != nil {
						t.Fatal(err)
					}
				}
				for _, e := range source.Input.Events {
					if err = p.ScheduleCIAEvent(NativeAudioTimedEvent{CIAUpdate: uint64(e.Tick), Kind: e.Kind, Control: e.Control, Data: e.Data}); err != nil {
						t.Fatal(err)
					}
				}
				pcm := make([]byte, f.SampleRate*f.Seconds*4)
				for at := 0; at < len(pcm); {
					end := min(len(pcm), at+chunk)
					if _, err = io.ReadFull(p, pcm[at:end]); err != nil {
						t.Fatal(err)
					}
					at = end
				}
				if got := fmt.Sprintf("%x", sha256.Sum256(pcm)); got != f.SHA256 {
					t.Fatalf("chunk%d original hardware-derived PCM differs: %s/%s", chunk, got, f.SHA256)
				}
				if got := int(p.nextCIA/p.ciaPeriod) - 1; got != f.IRQUpdates {
					t.Fatalf("exact native CIA schedule differs: %d/%d", got, f.IRQUpdates)
				}
			}
		})
	}
}

func TestNativeAudioPCMConcurrentReadAndCommands(t *testing.T) {
	bundle := testBundle(t)
	d, err := NewNativeAudioDevice(bundle.Executable, bundle.Raw["fx.dat"], 0x100000, 0x800000, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = d.Initialize(); err != nil {
		t.Fatal(err)
	}
	absolute := [16]byte{5: 0x90}
	d.ReadAbsolute8 = func(address uint32) (uint8, error) {
		if address >= uint32(len(absolute)) {
			return 0, fmt.Errorf("absolute%08x notsupplied", address)
		}
		return absolute[address], nil
	}
	p, err := NewNativeAudioPCM(d, NativePALAudioTiming(44100, 0))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errors := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		buffer := make([]byte, 1027)
		for i := 0; i < 50; i++ {
			if _, e := p.Read(buffer); e != nil {
				errors <- e
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			if _, e := p.Command(0x84, uint16(1+i%10), 0); e != nil {
				errors <- e
				return
			}
			if _, e := p.Command(0x2004, 63, 0); e != nil {
				errors <- e
				return
			}
			if _, e := p.Command(0x14, 0, 0); e != nil {
				errors <- e
				return
			}
		}
	}()
	wg.Wait()
	close(errors)
	for e := range errors {
		t.Fatal(e)
	}
}
