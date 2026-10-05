package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type nativeFramePresentationFixture struct {
	Input struct {
		Name       string
		D          [8]uint32
		Initial    []nativeHeroPatch
		Deadline   uint32
		MouseImage int16
		Chain      bool
	}
	Frames []struct {
		Step struct {
			Kind        string
			X, Y        uint8
			Left, Right bool
		}
		D                                                          [8]uint32
		BSSHash, ChipHash, PointerHash, PixelHash                  string
		Hardware                                                   []NativeFrameHardwareWrite
		CopperSelector, SpritePatchPointer, ActiveCopper, Deadline uint32
		Mouse                                                      [6]uint16
		Pending                                                    string
	}
}

func TestNativeFramePresentationAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/native_frame_presentation_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		BSSBytes, ChipBytes, PointerBytes int
		Cases                             []nativeFramePresentationFixture
	}
	if err = json.Unmarshal(data, &corpus); err != nil || len(corpus.Cases) != 189 || corpus.BSSBytes != 0x11280 || corpus.ChipBytes != NativeFrameChipBytes || corpus.PointerBytes != 15260 {
		t.Fatalf("native frame presentation corpus incomplete: %v", err)
	}
	exe := testBundle(t).Executable
	for _, f := range corpus.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			s, err := NewNativeFramePresentationState(exe, 0x500000, 0x400000)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0x408; i < len(s.Chip); i++ {
				s.Chip[i] = uint8(i*17 + 3 + ((i-0x408)/32000)*91)
			}
			b := make([]byte, 0x11280)
			m := s.Memory(commandNativeMemory(b))
			for _, p := range f.Input.Initial {
				switch p.Width {
				case 1:
					err = m.Write8(p.Address, uint8(p.Value))
				case 2:
					err = m.Write16(p.Address, uint16(p.Value))
				case 4:
					err = m.Write32(p.Address, p.Value)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			s.Deadline1117C = f.Input.Deadline
			s.Input.Mouse.Image = uint16(f.Input.MouseImage)
			s.InterruptChain = f.Input.Chain
			c := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			for _, frame := range f.Frames {
				sample := NativeMouseSample{CounterX: frame.Step.X, CounterY: frame.Step.Y, Left: frame.Step.Left, Right: frame.Step.Right}
				var hardware []NativeFrameHardwareWrite
				pending := ""
				switch frame.Step.Kind {
				case "init":
					hardware, err = s.Initialize(exe, sample)
				case "swap":
					hardware, err = s.Swap(&c)
				case "vblank":
					var p NativeFrameVBlankResult
					p, err = s.VBlank(sample, m, &c)
					hardware = p.Hardware
					if p.ChainOriginalInterrupt != f.Input.Chain {
						t.Error("original IRQ chain gate changed")
					}
				case "clock":
					var complete bool
					palette := false
					complete, err = s.AdvanceClock(&c, NativeFrameClockCallbacks{Memory: m, Palette: func(*NativeFrameRegisterContext) (bool, error) { palette = true; return false, nil }})
					if !complete {
						pending = "idle"
						if palette {
							pending = "palette"
						}
					}
				default:
					t.Fatal("unknown native producer stage")
				}
				if err != nil {
					t.Fatal(err)
				}
				if c.D != frame.D {
					t.Errorf("stage%s all eight registers differ: got %x, native %x", frame.Step.Kind, c.D, frame.D)
				}
				if pending != frame.Pending {
					t.Errorf("native wait differs: %s/%s", pending, frame.Pending)
				}
				if !reflect.DeepEqual(hardware, frame.Hardware) {
					t.Errorf("ordered hardware PC/address/width/value differs: got %+v, native %+v", hardware, frame.Hardware)
				}
				if s.CopperSelector != frame.CopperSelector || s.SpritePatchPointer != frame.SpritePatchPointer || s.ActiveCopper != frame.ActiveCopper || s.Deadline1117C != frame.Deadline {
					t.Error("mutable CODE/Copper selection differs")
				}
				mouse := s.Input.Mouse
				if [6]uint16{mouse.Image, mouse.CounterX, mouse.CounterY, mouse.PositionX, mouse.PositionY, mouse.MaximumY} != frame.Mouse {
					t.Error("native mutable mouse CODE differs")
				}
				all := append([]byte(nil), b...)
				copy(all[:0x14c], s.Input.Low[:])
				copy(all[0x14c:0xdc2], s.LowTail[:])
				for name, pair := range map[string]struct {
					Bytes []byte
					Hash  string
				}{"BSS": {all, frame.BSSHash}, "HUNK4 chip": {s.Chip, frame.ChipHash}, "HUNK3 pointer": {s.PointerData, frame.PointerHash}} {
					if got := fmt.Sprintf("%x", sha256.Sum256(pair.Bytes)); got != pair.Hash {
						t.Errorf("stage%s complete%s differs: %s/%s", frame.Step.Kind, name, got, pair.Hash)
					}
				}
				img, err := s.Image()
				if err != nil {
					t.Fatal(err)
				}
				if got := fmt.Sprintf("%x", sha256.Sum256(img.Pix)); got != frame.PixelHash {
					t.Errorf("actual Copper-selected bitmap pixels differ: %s/%s", got, frame.PixelHash)
				}
			}
		})
	}
}
