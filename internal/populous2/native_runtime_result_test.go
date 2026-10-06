package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type nativeRuntimeResultFixture struct {
	Input struct {
		Audio, Music                                 bool
		Name, Mode                                   string
		Selected, Eliminated, GameMode, World, Score uint16
		Ticks                                        uint32
		Local, Opponent                              CampaignResultStatistics
		D                                            [8]uint32
	}
	ProceedX, ProceedY int
	Frames             []struct {
		Event                                    string
		PC                                       uint32
		D                                        [8]uint32
		A                                        [7]uint32
		A7                                       uint32
		BSSHash, CodeHash, ChipHash, PointerHash string
		Score                                    uint16
	}
}

func TestNativeRuntimeResultAgainstOriginalParentCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/native_runtime_result_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativeRuntimeResultFixture }
	if err = json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 40 {
		t.Fatal("original result parent corpus incomplete", err)
	}
	frames, initialWaits, progression, reset := 0, 0, 0, 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			if len(f.Frames) != 105 && len(f.Frames) != 122 {
				t.Fatal("empty/truncated parent trace")
			}
			h := nativeRuntimeHostTest(t)
			if err = h.Memory.Code.Write16(0x3ea, 0); err != nil {
				t.Fatal(err)
			}
			if _, err = h.InitializePresentation(NativeMouseSample{}); err != nil {
				t.Fatal(err)
			}
			for _, at := range []int{0x1a, 0x1e} {
				pointer, err := h.Memory.BSS.Read32(at)
				if err != nil {
					t.Fatal(err)
				}
				bitmap, err := h.Bitmap(pointer)
				if err != nil {
					t.Fatal(err)
				}
				for i := range bitmap {
					bitmap[i] = byte(i*7 + 13)
				}
			}
			local, other := 1, 2
			if f.Input.Selected == 2 {
				local, other = other, local
			}
			for owner, s := range map[int]CampaignResultStatistics{local: f.Input.Local, other: f.Input.Opponent} {
				at := 0xe76a + owner*314
				for _, p := range []struct {
					offset int
					value  uint32
				}{{4, s.Population}, {0x3c, s.PeakPopulation}, {0x40, s.PeakMana}} {
					if err := h.Memory.BSS.Write32(at+p.offset, p.value); err != nil {
						t.Fatal(err)
					}
				}
				for _, p := range []struct {
					offset int
					value  uint16
				}{{0x18, s.Identity}, {0x44, s.Metric}, {0x46, s.LeaderLosses}, {0x48, s.BattleWins}, {0x4a, s.ScenarioOptions}, {0x138, s.WeightedPowerUse}, {0x58, s.Bolts}} {
					if err := h.Memory.BSS.Write16(at+p.offset, p.value); err != nil {
						t.Fatal(err)
					}
				}
			}
			for _, p := range []struct {
				offset int
				value  uint16
			}{{0xeb42, f.Input.Selected}, {0xeb44, f.Input.GameMode}, {0xeb46, f.Input.World}} {
				if err := h.Memory.BSS.Write16(p.offset, p.value); err != nil {
					t.Fatal(err)
				}
			}
			if err := h.Memory.BSS.Write32(0xf40, f.Input.Ticks); err != nil {
				t.Fatal(err)
			}
			var device *NativeAudioDevice
			if f.Input.Audio {
				fx := append([]byte(nil), h.Bundle.Raw["fx.dat"]...)
				if err := h.Host.MapRegion(NativeHostRegion{Name: "actual initialized FX fixture", Base: 0x700000, Bytes: fx}); err != nil {
					t.Fatal(err)
				}
				if err := h.Memory.BSS.Write32(0x3b4, 0x700000); err != nil {
					t.Fatal(err)
				}
				warm := NativeFrameRegisterContext{AddressBase: h.Memory.BSSBase}
				var err error
				device, _, err = h.InitializeAudio(&warm, 0)
				if err != nil {
					t.Fatal(err)
				}
				if f.Input.Music {
					if err := h.Memory.BSS.Write16(0x3bc, 1); err != nil {
						t.Fatal(err)
					}
					if _, err := device.MusicCommand(0x8f, 0, 0); err != nil {
						t.Fatal(err)
					}
				}
				warm.D[0] = 0x1ae
				if err := device.DirectCue(0x1ae, &warm); err != nil {
					t.Fatal(err)
				}
				for range 2 {
					if err := device.TickCIA(&warm); err != nil {
						t.Fatal(err)
					}
				}
			}
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: h.Memory.BSSBase}
			state := NativeRuntimeResultState{}
			for j := range state.A {
				state.A[j] = NativeRequesterAddress{Address: 0x800000 + uint32(j)*0x1000, Absolute: true}
			}
			if err := state.Begin(f.Input.Eliminated, &frame); err != nil {
				t.Fatal(err)
			}
			cb := NativeRuntimeResultCallbacks{Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }, Sound: func(uint16, *NativeFrameRegisterContext) error { return nil }, Child: func(call NativeStartupResetFrameCall, _ *uint32) (NativeCommandFrameResult, error) {
				if call.Routine != 0xb244 && call.Routine != 0x10a8c {
					return NativeCommandFrameResult{}, fmt.Errorf("unexpected source child%x", call.Routine)
				}
				return NativeCommandFrameResult{}, nil // Deliberately retain the genuine child boundary.
			}}
			if device != nil {
				cb.Audio = NativeRuntimeAudioOperations{Command: device.Command, MusicCommand: device.MusicCommand, DirectCue: device.DirectCue}
				cb.Sound = device.DirectCue
			}
			p := h.Session.Presentation
			irq := func(x, y uint8, left bool) {
				if _, err := p.VBlank(NativeMouseSample{CounterX: x, CounterY: y, Left: left}, h.Memory.BSS, &frame); err != nil {
					t.Fatal(err)
				}
			}
			move := func(x, y int) {
				for int(p.Input.Mouse.PositionX) != x*2 || int(p.Input.Mouse.PositionY) != y*2 {
					dx, dy := max(-100, min(100, x*2-int(p.Input.Mouse.PositionX))), max(-100, min(100, y*2-int(p.Input.Mouse.PositionY)))
					irq(uint8(int(p.Input.Mouse.CounterX)+dx), uint8(int(p.Input.Mouse.CounterY)+dy), false)
				}
				irq(uint8(p.Input.Mouse.CounterX), uint8(p.Input.Mouse.CounterY), true)
			}
			for i, want := range f.Frames {
				switch want.Event {
				case "blank", "fade":
					irq(uint8(p.Input.Mouse.CounterX), uint8(p.Input.Mouse.CounterY), false)
				case "outside":
					move(0, 0)
				case "proceed":
					move(f.ProceedX, f.ProceedY)
				case "initial", "idle":
				default:
					t.Fatal("unknown original event", want.Event)
				}
				step, err := state.Advance(h, &frame, cb)
				if err != nil {
					t.Fatalf("frame%d: %v", i, err)
				}
				pc := uint32(step.PC)
				if step.ChildRoutine != 0 {
					pc = uint32(step.ChildRoutine)
				} else if pc == 0x3868 || pc == 0x3ab0 {
					pc = 0x786
				}
				for j, a := range state.A {
					if a.Address != want.A[j] {
						t.Fatalf("frame%d native A%d differs: got%x want%x", i, j, a.Address, want.A[j])
					}
				}
				if pc != want.PC || frame.D != want.D || state.Score != want.Score || step.Complete {
					t.Fatalf("frame%d source result continuation differs: gotPC%x D%x score%d wantPC%x D%x score%d", i, pc, frame.D, state.Score, want.PC, want.D, want.Score)
				}
				bss, err := h.Memory.SnapshotBSS()
				if err != nil {
					t.Fatal(err)
				}
				for _, pair := range []struct{ name, got, want string }{{"BSS", fileFrameHash(bss), want.BSSHash}, {"CODE", fileFrameHash(nativeRuntimeResultCodeBytes(t, h)), want.CodeHash}, {"screen/Copper", fileFrameHash(p.Chip), want.ChipHash}, {"pointer RAM", fileFrameHash(p.PointerData[:15260]), want.PointerHash}} {
					if pair.got != pair.want {
						t.Fatalf("frame%d source result %s differs: got%s want%s", i, pair.name, pair.got, pair.want)
					}
				}
				if i <= 100 {
					initialWaits++
				}
				frames++
			}
			last := f.Frames[len(f.Frames)-1]
			// Original IRQs preserve the caller stack; the parent has exactly one
			// child return address left at these boundaries. Go retains its typed
			// continuation instead of claiming a physical 68000 stack emulation.
			if last.A7 != 0xeefffc {
				t.Fatal("original parent stack boundary changed")
			}
			if last.PC == 0xb244 {
				progression++
			} else if last.PC == 0x10a8c {
				reset++
			} else {
				t.Fatal("source child boundary missing")
			}

		})
	}
	if frames != 4540 || initialWaits != 4040 || progression != 20 || reset != 20 {
		t.Fatalf("result coverage changed: frames%d initial waits%d progression%d reset%d", frames, initialWaits, progression, reset)
	}
}

func nativeRuntimeResultCodeBytes(t *testing.T, h *NativeRuntimeHost) []byte {
	t.Helper()
	data := make([]byte, 0x3fa2c)
	for i := range data {
		v, err := h.Memory.Code.Read8(i)
		if err != nil {
			t.Fatal(err)
		}
		data[i] = v
	}
	return data
}
