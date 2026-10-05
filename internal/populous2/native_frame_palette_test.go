package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type nativeFramePaletteFixture struct {
	Input struct {
		Name           string
		D              [8]uint32
		Source, Target NativeFramePaletteBank
		Clock          bool
		Pause          uint16
	}
	Frames []struct {
		Phase                          int
		D                              [8]uint32
		BSSHash, ChipHash, PointerHash string
		Palette                        [16]uint16
		Mouse                          [6]uint16
		Complete                       bool
	}
}

func TestNativeFramePaletteAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/native_frame_palette_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct{ Cases []nativeFramePaletteFixture }
	if err = json.Unmarshal(data, &corpus); err != nil || len(corpus.Cases) != 24 {
		t.Fatalf("native palette corpus incomplete: %v", err)
	}
	exe := testBundle(t).Executable
	for _, f := range corpus.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			p, err := NewNativeFramePresentationState(exe, 0x500000, 0x400000)
			if err != nil {
				t.Fatal(err)
			}
			p.InterruptChain = false
			for i := 0x408; i < len(p.Chip); i++ {
				p.Chip[i] = uint8(i*17 + 3 + ((i-0x408)/32000)*91)
			}
			if _, err = p.Initialize(exe, NativeMouseSample{}); err != nil {
				t.Fatal(err)
			}
			b := make([]byte, 0x11280)
			m := p.Memory(commandNativeMemory(b))
			if err = m.Write32(0xf40, 123); err != nil {
				t.Fatal(err)
			}
			c := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			fade := NewNativeFramePaletteState(f.Input.Source, f.Input.Target, 0x100000)
			if f.Input.Clock {
				p.Deadline1117C = 123
				_ = m.Write16(0x138, 1)
				_ = m.Write16(0xf3c, f.Input.Pause)
				_ = m.Write16(0x5f44, 0x8000)
				_ = m.Write16(0x5f46, 56)
				_ = m.Write16(0xeb2c, 0x1234)
				_ = m.Write16(0xeb2e, 0x5678)
			}
			if len(f.Frames) != 18 {
				t.Fatal("native fade did not capture seventeen real phases")
			}
			for _, frame := range f.Frames {
				if frame.Phase > 0 {
					_, err = p.VBlank(NativeMouseSample{CounterX: uint8(frame.Phase * 7), CounterY: uint8(frame.Phase * 9)}, m, &c)
					if err != nil {
						t.Fatal(err)
					}
				}
				var complete bool
				if f.Input.Clock {
					complete, err = p.AdvanceClock(&c, NativeFrameClockCallbacks{Memory: m, Palette: func(c *NativeFrameRegisterContext) (bool, error) { return fade.Advance(p, c, m) }})
				} else {
					complete, err = fade.Advance(p, &c, m)
				}
				if err != nil {
					t.Fatal(err)
				}
				if complete != frame.Complete || int(fade.Phase) != frame.Phase {
					t.Errorf("native wait/completion differs: phase%d/%d complete%t/%t", fade.Phase, frame.Phase, complete, frame.Complete)
				}
				if c.D != frame.D {
					t.Errorf("phase%d all eight registers differ: got %x, native %x", frame.Phase, c.D, frame.D)
				}
				all := append([]byte(nil), b...)
				copy(all[:0x14c], p.Input.Low[:])
				copy(all[0x14c:0xdc2], p.LowTail[:])
				for name, pair := range map[string]struct {
					Bytes []byte
					Hash  string
				}{"BSS": {all, frame.BSSHash}, "chip": {p.Chip, frame.ChipHash}, "pointer": {p.PointerData, frame.PointerHash}} {
					if got := fmt.Sprintf("%x", sha256.Sum256(pair.Bytes)); got != pair.Hash {
						t.Errorf("phase%d complete%s differs: %s/%s", frame.Phase, name, got, pair.Hash)
					}
				}
				for i, w := range frame.Palette {
					if binary.BigEndian.Uint16(p.Chip[0x36+i*4:]) != w || binary.BigEndian.Uint16(p.Chip[0x236+i*4:]) != w {
						t.Errorf("phase%d dual Copper palette word%d differs", frame.Phase, i)
					}
				}
				mouse := p.Input.Mouse
				if [6]uint16{mouse.Image, mouse.CounterX, mouse.CounterY, mouse.PositionX, mouse.PositionY, mouse.MaximumY} != frame.Mouse {
					t.Error("native mouse/VBlank state differs during fade")
				}
			}
		})
	}
}
