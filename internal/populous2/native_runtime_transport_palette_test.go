package populous2

import (
	"encoding/json"
	"net"
	"os"
	"testing"
)

func TestNativeRuntimeTransportPaletteAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/native_frame_palette_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct{ Cases []nativeFramePaletteFixture }
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	covered := 0
	for _, f := range corpus.Cases {
		if f.Input.Clock {
			continue
		}
		t.Run(f.Input.Name, func(t *testing.T) {
			h := nativeRuntimeHostTest(t)
			p := h.Session.Presentation
			for i := 0x408; i < len(p.Chip); i++ {
				p.Chip[i] = uint8(i*17 + 3 + ((i-0x408)/32000)*91)
			}
			if err := h.Memory.Code.Write16(0x3ea, 0); err != nil {
				t.Fatal(err)
			}
			if _, err := h.InitializePresentation(NativeMouseSample{}); err != nil {
				t.Fatal(err)
			}
			if err := h.Memory.BSS.Write32(0xf40, 123); err != nil {
				t.Fatal(err)
			}
			for _, bank := range []NativeFramePaletteBank{f.Input.Source, f.Input.Target} {
				for i, v := range bank.Words {
					if err := h.Memory.RAM.Write16(int(bank.Address)+i*2, v); err != nil {
						t.Fatal(err)
					}
				}
			}
			left, right := net.Pipe()
			defer right.Close()
			conn, err := NewNativeSerialConn(left)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			transport, err := h.NewTransport(conn, NativeTransportFrameCallbacks{}, func(bool, *NativeFrameRegisterContext) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			c := NativeFrameRegisterContext{AddressBase: h.Memory.BSSBase, D: f.Input.D}
			phase := uint32(0)
			cb, err := transport.controlCallbacks(&c)
			if err != nil {
				t.Fatal(err)
			}
			call := NativeFileFrameCall{Routine: 0x102e4, Frame: &c, Arguments: 1<<2 | 1<<3}
			call.A[2] = NativeRequesterAddress{Address: f.Input.Source.Address, Code: true}
			call.A[3] = NativeRequesterAddress{Address: f.Input.Target.Address, Code: true}
			for _, want := range f.Frames {
				if want.Phase > 0 {
					if _, err := p.VBlank(NativeMouseSample{CounterX: uint8(want.Phase * 7), CounterY: uint8(want.Phase * 9)}, h.Memory.BSS, &c); err != nil {
						t.Fatal(err)
					}
				}
				step, err := cb.CallTransport(call, &phase)
				if err != nil || step.Complete != want.Complete || c.D != want.D {
					t.Fatal("transport fade differs from actual source", want.Phase, step, c.D, want.D, err)
				}
				if step.Complete && (!step.Zero || step.Negative) {
					t.Fatal("palette lost source CMP terminal flags")
				}
				raw, err := h.Memory.SnapshotBSS()
				if err != nil {
					t.Fatal(err)
				}
				if fileFrameHash(raw) != want.BSSHash || fileFrameHash(p.Chip) != want.ChipHash || fileFrameHash(p.PointerData) != want.PointerHash {
					t.Fatal("transport palette detached native memory", want.Phase)
				}
			}
			if transport.palette != nil {
				t.Fatal("completed transport fade remained retained")
			}
			covered++
		})
	}
	if covered == 0 {
		t.Fatal("native palette corpus omitted direct transport fades")
	}
}
