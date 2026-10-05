package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
)

func TestNativeAudioDMAPCMFromIndependentPaulaReference(t *testing.T) {
	data, e := os.ReadFile("testdata/audio_native_dma_pcm.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct {
		Cases []struct {
			Scenario         string
			PAL              bool
			SampleRate       int
			TimerLow         uint8
			Seconds          int
			RequestClock     uint16
			SHA256           string
			Peak, IRQUpdates int
		}
	}
	if e = json.Unmarshal(data, &catalog); e != nil || len(catalog.Cases) != 8 {
		t.Fatalf("DMA PCM reference coverage differs:%v", e)
	}
	data, e = os.ReadFile("testdata/audio_native_irq_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var traces struct{ Cases []nativeAudioIRQFixture }
	if e = json.Unmarshal(data, &traces); e != nil {
		t.Fatal(e)
	}
	byname := map[string]nativeAudioIRQFixture{}
	for _, f := range traces.Cases {
		byname[f.Input.Name] = f
	}
	bundle := testBundle(t)
	for _, f := range catalog.Cases {
		t.Run(fmt.Sprintf("%s-pal%v-latch%d", f.Scenario, f.PAL, f.RequestClock), func(t *testing.T) {
			for _, chunk := range []int{f.SampleRate * f.Seconds * 4, 1, 127, 4097} {
				d, e := NewNativeAudioDevice(bundle.Executable, bundle.Raw["fx.dat"], 0x100000, 0x800000, f.TimerLow)
				if e != nil {
					t.Fatal(e)
				}
				if _, e = d.Initialize(); e != nil {
					t.Fatal(e)
				}
				absolute := [16]byte{5: 0x90}
				d.ReadAbsolute8 = func(address uint32) (uint8, error) {
					if address >= uint32(len(absolute)) {
						return 0, fmt.Errorf("physical word byte%x missing", address)
					}
					return absolute[address], nil
				}
				timing, config := NativePALAudioTiming(f.SampleRate, f.TimerLow), NativePALPaulaDMAConfig(f.RequestClock)
				if !f.PAL {
					timing, config = NativeNTSCAudioTiming(f.SampleRate, f.TimerLow), NativeNTSCPaulaDMAConfig(f.RequestClock)
				}
				p, e := NewNativeAudioPCMWithDMA(d, timing, config)
				if e != nil {
					t.Fatal(e)
				}
				source, ok := byname[f.Scenario]
				if !ok {
					t.Fatal("native IRQ input missing")
				}
				if source.Input.Music {
					if _, e = p.MusicCommand(0x8f, 0, 0); e != nil {
						t.Fatal(e)
					}
				}
				for _, event := range source.Input.Events {
					if e = p.ScheduleCIAEvent(NativeAudioTimedEvent{CIAUpdate: uint64(event.Tick), Kind: event.Kind, Control: event.Control, Data: event.Data}); e != nil {
						t.Fatal(e)
					}
				}
				pcm := make([]byte, f.SampleRate*f.Seconds*4)
				for at := 0; at < len(pcm); {
					end := min(len(pcm), at+chunk)
					if _, e = io.ReadFull(p, pcm[at:end]); e != nil {
						t.Fatal(e)
					}
					at = end
				}
				if got := fmt.Sprintf("%x", sha256.Sum256(pcm)); got != f.SHA256 {
					t.Fatalf("chunk%d independent DMA PCM differs:%s/%s", chunk, got, f.SHA256)
				}
				if int(p.nextCIA/p.ciaPeriod)-1 != f.IRQUpdates {
					t.Fatal("DMA transport changed native CIA schedule")
				}
				snapshot, ok := p.DMASnapshot()
				if !ok || snapshot.Clock != p.clock {
					t.Fatal("actual DMA transport snapshot unavailable")
				}
			}
		})
	}
}

func TestNativePaulaStarvationRepeatsBothBytesWithoutPeriodClamp(t *testing.T) {
	reads := []uint32{}
	s, e := NewNativePaulaDMAState(NativePALPaulaDMAConfig(0), func(a uint32) (uint16, error) {
		reads = append(reads, a)
		switch a {
		case 0:
			return 0xdead, nil
		case 0x1000:
			return 0x7f81, nil
		case 0x1002:
			return 0x20e0, nil
		}
		return 0, fmt.Errorf("unexpected physical DMA word%x", a)
	})
	if e != nil {
		t.Fatal(e)
	}
	if e = s.AdvanceTo(0); e != nil {
		t.Fatal(e)
	}
	if e = s.Apply([]NativeFrameHardwareWrite{{Address: 0xdff0a0, Value: 0x1000, Width: 4}, {Address: 0xdff0a4, Value: 2, Width: 2}, {Address: 0xdff0a6, Value: 16, Width: 2}, {Address: 0xdff0a8, Value: 64, Width: 2}, {Address: 0xdff096, Value: 0x8201, Width: 2}}); e != nil {
		t.Fatal(e)
	}
	for _, f := range []struct {
		clock  uint64
		sample int8
	}{{469, 127}, {485, -127}, {501, 127}, {517, -127}} {
		if e = s.AdvanceTo(f.clock); e != nil {
			t.Fatal(e)
		}
		c := s.Snapshot().Channels[0]
		if c.Sample != f.sample || c.Period != 16 || c.OutputWord != 0x7f81 || c.Pointer != 0x1002 {
			t.Fatalf("starved two-byte waveform clock%d differs:%+v", f.clock, c)
		}
	}
	if len(reads) != 2 || reads[0] != 0 || reads[1] != 0x1000 {
		t.Fatal("period expiries fabricated additional DMA word fetches")
	}
	if e = s.AdvanceTo(725); e != nil {
		t.Fatal(e)
	}
	c := s.Snapshot().Channels[0]
	if c.Sample != 32 || c.OutputWord != 0x20e0 || len(reads) != 3 || reads[2] != 0x1002 {
		t.Fatal("next raster fetch did not replace the holding word")
	}
}
