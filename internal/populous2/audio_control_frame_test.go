package populous2

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type audioControlFrameFixture struct {
	Input struct {
		Name      string
		Routine   int
		Gate      uint32
		MusicFlag uint16
		Music     bool
		Warm      int
		Cue       uint16
		D         [8]uint32
	}
	Calls []struct {
		Routine       int
		Control, Data uint16
		D             [8]uint32
	}
	D                            [8]uint32
	A, CallerA                   [7]uint32
	BSSHash, DriverHash, DMAHash string
	Hardware                     []NativeFrameHardwareWrite
}

func TestNativeAudioControlFramesAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/audio_control_frame_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []audioControlFrameFixture }
	if e = json.Unmarshal(data, &catalog); e != nil {
		t.Fatal(e)
	}
	if len(catalog.Cases) != 480 {
		t.Fatalf("native audio control coverage changed:%d", len(catalog.Cases))
	}
	b := testBundle(t)
	r, e := DecodeNativeAudioControlFrameRules(b.Executable)
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range catalog.Cases {
		for _, pcm := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-pcm%v", f.Input.Name, pcm), func(t *testing.T) {
				d, e := NewNativeAudioDevice(b.Executable, b.Raw["fx.dat"], 0x100000, 0x800000, 0)
				if e != nil {
					t.Fatal(e)
				}
				if _, e = d.Initialize(); e != nil {
					t.Fatal(e)
				}
				absolute := [16]byte{5: 0x90}
				d.ReadAbsolute8 = func(address uint32) (uint8, error) {
					if address >= uint32(len(absolute)) {
						return 0, fmt.Errorf("native absolute%x unavailable", address)
					}
					return absolute[address], nil
				}
				if f.Input.Music {
					if _, e = d.MusicCommand(0x8f, 0, 0); e != nil {
						t.Fatal(e)
					}
				}
				warm := NativeFrameRegisterContext{}
				warm.Word(0, f.Input.Cue)
				if e = d.DirectCue(f.Input.Cue, &warm); e != nil {
					t.Fatal(e)
				}
				for i := 0; i < f.Input.Warm; i++ {
					if e = d.TickCIA(&warm); e != nil {
						t.Fatal(e)
					}
				}
				raw := make([]byte, 0x11280)
				binary.BigEndian.PutUint32(raw[0x3b4:], f.Input.Gate)
				binary.BigEndian.PutUint16(raw[0x3bc:], f.Input.MusicFlag)
				m := commandFrameBacking(raw)
				frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
				cb := NativeAudioControlDeviceCallbacks(d, m, &frame)
				if pcm {
					p, e := NewNativeAudioPCMWithDMA(d, NativePALAudioTiming(44100, 0), NativePALPaulaDMAConfig(0))
					if e != nil {
						t.Fatal(e)
					}
					cb = NativeAudioControlPCMCallbacks(p, m, &frame)
				}
				d.Hardware = nil
				step, e := r.Run(f.Input.Routine, cb)
				if e != nil {
					t.Fatal(e)
				}
				if !step.Complete || frame.D != f.D {
					t.Fatalf("source register result differs:got%08x want%08x", frame.D, f.D)
				}
				if len(step.Calls) != len(f.Calls) {
					t.Fatal("native call count differs")
				}
				for i, call := range step.Calls {
					want := f.Calls[i]
					if call.Routine != want.Routine || call.Control != want.Control || call.Data != want.Data || call.D != want.D {
						t.Fatalf("native child%d ABI differs:got%+v want%+v", i, call, want)
					}
					if call.Disabled != (call.Routine == 0x190e4 && int32(f.Input.Gate) <= 0) {
						t.Fatal("native disabled gate applied to wrong child")
					}
				}
				if fileFrameHash(raw) != f.BSSHash || fileFrameHash(d.Code[0x18b16:0x18ee4]) != f.DriverHash || fileFrameHash(d.Code[0x194e2:0x194ea]) != f.DMAHash {
					t.Fatal("native complete BSS/driver/DMA state differs")
				}
				if !reflect.DeepEqual(d.Hardware, f.Hardware) {
					t.Fatalf("native hardware writes differ:got%+v want%+v", d.Hardware, f.Hardware)
				}
				for i, a := range f.A {
					if i == 0 && f.Input.Routine == 0x18474 && f.Input.MusicFlag == 0 {
						if !step.A0Assigned || step.A0.Address != a || !step.A0.Code {
							t.Fatal("source resume descriptorA0 differs")
						}
					} else if a != f.CallerA[i] {
						t.Fatal("source address register restoration differs")
					}
				}
			})
		}
	}
}

func TestNativeAudioControlDisabledGateDoesNotSkipPrimaryChild(t *testing.T) {
	r, e := DecodeNativeAudioControlFrameRules(testBundle(t).Executable)
	if e != nil {
		t.Fatal(e)
	}
	raw := make([]byte, 0x11280)
	binary.BigEndian.PutUint16(raw[0x3bc:], 1)
	c := NativeFrameRegisterContext{D: [8]uint32{0xdeadbeef, 0x12340000, 2, 3, 4, 5, 6, 7}}
	cb := NativeAudioControlFrameCallbacks{Memory: commandFrameBacking(raw), Frame: &c}
	step, e := r.Run(0x1842e, cb)
	if e == nil || step.Complete || len(step.Calls) != 4 || c.D[0] != 0xdead2008 || c.D[1] != 0x12340010 {
		t.Fatal("disabled secondary gate fabricated primary completion or lost prefix registers")
	}
}
