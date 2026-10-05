package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type paulaDMAWrite struct {
	Clock          uint64
	Address, Value uint32
	Width          uint8
}
type paulaDMAFixture struct {
	Name      string
	PAL       bool
	Channel   int
	Latch     uint16
	End       uint64
	Writes    []paulaDMAWrite
	Markers   []uint64
	Snapshots []struct {
		Clock                     uint64
		DMA, Interrupts, Requests uint16
		ChannelsSHA256            string
		Channel                   []int64
	}
}

func paulaDMAFields(c NativePaulaDMAChannelSnapshot) []int64 {
	b := func(v bool) int64 {
		if v {
			return 1
		}
		return 0
	}
	return []int64{int64(c.State), int64(c.Location), int64(c.Pointer), int64(c.Length), int64(c.WorkingLength), int64(c.Period), int64(c.Volume), int64(c.OutputVolume), int64(c.Holding), int64(c.OutputWord), int64(c.Sample), b(c.Request), b(c.Restart), b(c.InterruptOnWord), int64(c.PeriodRemaining), int64(c.InterruptCheck), b(c.DataWritten)}
}
func TestNativePaulaDMAAgainstIndependentHardwareModel(t *testing.T) {
	data, e := os.ReadFile("testdata/audio_native_dma_reference.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []paulaDMAFixture }
	if e = json.Unmarshal(data, &catalog); e != nil {
		t.Fatal(e)
	}
	if len(catalog.Cases) != 736 {
		t.Fatalf("Paula hardware model coverage changed:%d", len(catalog.Cases))
	}
	read := func(address uint32) (uint16, error) {
		v := func(a uint32) uint8 { return uint8(a*37 + (a >> 4) + 3) }
		return uint16(v(address))<<8 | uint16(v(address+1)), nil
	}
	for _, f := range catalog.Cases {
		t.Run(f.Name, func(t *testing.T) {
			cfg := NativePALPaulaDMAConfig(f.Latch)
			if !f.PAL {
				cfg = NativeNTSCPaulaDMAConfig(f.Latch)
			}
			s, e := NewNativePaulaDMAState(cfg, read)
			if e != nil {
				t.Fatal(e)
			}
			wi := 0
			for _, want := range f.Snapshots {
				for wi < len(f.Writes) && f.Writes[wi].Clock <= want.Clock {
					clock := f.Writes[wi].Clock
					if e = s.AdvanceTo(clock); e != nil {
						t.Fatal(e)
					}
					writes := []NativeFrameHardwareWrite{}
					for wi < len(f.Writes) && f.Writes[wi].Clock == clock {
						w := f.Writes[wi]
						writes = append(writes, NativeFrameHardwareWrite{Address: w.Address, Value: w.Value, Width: w.Width})
						wi++
					}
					if e = s.Apply(writes); e != nil {
						t.Fatal(e)
					}
				}
				if e = s.AdvanceTo(want.Clock); e != nil {
					t.Fatal(e)
				}
				got := s.Snapshot()
				if got.DMA != want.DMA || got.Interrupts != want.Interrupts || got.LatchedRequests != want.Requests {
					t.Fatalf("clock%d request/IRQ/DMA differs:%x/%x/%x expected%x/%x/%x", want.Clock, got.DMA, got.Interrupts, got.LatchedRequests, want.DMA, want.Interrupts, want.Requests)
				}
				fields := paulaDMAFields(got.Channels[f.Channel])
				if !reflect.DeepEqual(fields, want.Channel) {
					t.Fatalf("clock%d channel%d state differs:got%v expected%v", want.Clock, f.Channel, fields, want.Channel)
				}
				all := []int64{int64(got.DMA), int64(got.Interrupts), int64(got.LatchedRequests)}
				for _, c := range got.Channels {
					all = append(all, paulaDMAFields(c)...)
				}
				raw, e := json.Marshal(all)
				if e != nil {
					t.Fatal(e)
				}
				hash := fmt.Sprintf("%x", sha256.Sum256(raw))
				if hash != want.ChannelsSHA256 {
					t.Fatalf("clock%d full four-channel hardware state differs", want.Clock)
				}
			}
		})
	}
}

func TestNativePaulaDiscardedStartupWordRequiresRealBacking(t *testing.T) {
	missing := fmt.Errorf("configured chip word is unavailable")
	reads := 0
	s, e := NewNativePaulaDMAState(NativePALPaulaDMAConfig(0), func(address uint32) (uint16, error) {
		reads++
		if address != 0 {
			t.Fatalf("startup pipeline pointer differs:%x", address)
		}
		return 0, missing
	})
	if e != nil {
		t.Fatal(e)
	}
	if e = s.AdvanceTo(0); e != nil {
		t.Fatal(e)
	}
	if e = s.Apply([]NativeFrameHardwareWrite{{Address: 0xdff0a0, Value: 0x1000, Width: 4}, {Address: 0xdff0a4, Value: 2, Width: 2}, {Address: 0xdff0a6, Value: 16, Width: 2}, {Address: 0xdff096, Value: 0x8201, Width: 2}}); e != nil {
		t.Fatal(e)
	}
	if e = s.AdvanceTo(242); e != missing || reads != 1 {
		t.Fatal("discarded word used fabricated RAM")
	}
	before := s.Snapshot()
	if e = s.AdvanceTo(1000); e != missing || s.Snapshot() != before || reads != 1 {
		t.Fatal("failed physical fetch silently advanced or replayed")
	}
	if e = s.Apply([]NativeFrameHardwareWrite{{Address: 0xdff0a8, Value: 64, Width: 2}}); e != missing || s.Snapshot() != before {
		t.Fatal("failed physical fetch accepted later commands")
	}
}
