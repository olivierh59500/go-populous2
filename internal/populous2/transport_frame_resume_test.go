package populous2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type transportResumeInput struct {
	Name          string
	D             [8]uint32
	Mode          uint8
	Profile, Peer uint16
	Abort         int
	Speeds, Rules [2]uint16
	Payload       []byte
}
type transportResumeFixture struct {
	Input                                    transportResumeInput
	D                                        [8]uint32
	SR                                       uint16
	NativeReturn                             uint32
	BSSHash, CodeHash, ChipHash, PointerHash string
	Sent                                     []byte
	Switches                                 [][8]uint32
}

func TestNativeTransportResumeAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/transport_frame_resume_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []transportResumeFixture }
	if e := json.Unmarshal(data, &catalog); e != nil {
		t.Fatal(e)
	}
	if len(catalog.Cases) != 378 {
		t.Fatalf("native resume corpus changed %d", len(catalog.Cases))
	}
	exe := testBundle(t).Executable
	for _, f := range catalog.Cases {
		for _, split := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-split%v", f.Input.Name, split), func(t *testing.T) {
				p, e := NewNativeFramePresentationState(exe, 0x500000, 0x400000)
				if e != nil {
					t.Fatal(e)
				}
				for i := 0x408; i < len(p.Chip); i++ {
					p.Chip[i] = byte(i*17 + 3 + ((i-0x408)/32000)*91)
				}
				p.InterruptChain = false
				if _, e := p.Initialize(exe, NativeMouseSample{}); e != nil {
					t.Fatal(e)
				}
				raw := make([]byte, 0x11280)
				m := p.Memory(commandFrameBacking(raw))
				code := fileFrameRelocatedCode(t)
				cm := commandFrameBacking(code)
				_ = cm.Write16(0x3ea, 0)
				_ = cm.Write32(0x77a, p.CopperSelector)
				_ = cm.Write32(0x77e, p.SpritePatchPointer)
				_ = m.Write32(0xeb6a, 0x200000+0xeb56)
				_ = m.Write16(0xeb42, f.Input.Profile)
				_ = m.Write16(0xeb44, 6)
				for i, v := range []uint8{f.Input.Mode, 8, 7} {
					_ = m.Write8(0xeb5e+i*10, v)
				}
				for i := range f.Input.Speeds {
					_ = m.Write16(0xe90c+i*314, f.Input.Speeds[i])
					_ = m.Write16(0xeb2c+i*2, f.Input.Rules[i])
				}
				_ = m.Write16(0x15e, 380)
				for i, v := range f.Input.Payload {
					_ = m.Write8(0x160+i, v)
				}
				_ = m.Write16(0x156, uint16(len(f.Input.Payload)))
				if f.Input.Abort == 0 {
					_ = m.Write8(0xa7, 1)
					_ = m.Write8(0x6f, 1)
				}
				cursor, flushed := 0, false
				sent := []byte{}
				frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
				state := NativeTransportResumeState{}
				switches := 0
				resolver := func(address uint32) ([]byte, error) {
					at := int(address - p.ChipBase)
					if at < 0 || at > len(p.Chip)-32000 {
						return nil, fmt.Errorf("chip target unavailable")
					}
					return p.Chip[at : at+32000], nil
				}
				port := NativeSerialPort{Available: func() (int, error) {
					if flushed {
						return 0, nil
					}
					return len(f.Input.Payload) - cursor, nil
				}, Read: func(dst []byte) (int, error) {
					n := len(dst)
					if split {
						n = min(n, 1)
					}
					if f.Input.Abort > 0 {
						n = min(n, f.Input.Abort-cursor)
					}
					n = min(n, len(f.Input.Payload)-cursor)
					copy(dst, f.Input.Payload[cursor:cursor+n])
					cursor += n
					if f.Input.Abort > 0 && cursor >= f.Input.Abort {
						_ = m.Write8(0xa7, 1)
						_ = m.Write8(0x6f, 1)
					}
					if n < len(dst) {
						return n, ErrNativeSerialWait
					}
					return n, nil
				}, Write: func(src []byte) (int, error) {
					n := len(src)
					if split {
						n = min(n, 1)
					}
					sent = append(sent, src[:n]...)
					if n < len(src) {
						return n, ErrNativeSerialWait
					}
					return n, nil
				}, Flush: func() error { flushed = true; _ = m.Write16(0x154, 0); _ = m.Write16(0x156, 0); return nil }}
				cb := NativeTransportFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Code: cm, Memory: m, CodeBase: 0x100000, Frame: &frame, Presentation: p, Bitmap: resolver, Call: func(call NativeFileFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
					if switches >= len(f.Switches) || frame.D != f.Switches[switches] {
						return NativeCommandFrameResult{}, fmt.Errorf("native profile child D differs")
					}
					for i := range frame.D {
						frame.D[i] ^= 0x789012 + uint32(i)*0x10101
					}
					switches++
					return NativeCommandFrameResult{Complete: true}, nil
				}}, Port: port, CallerStackPointer: 0xef0000}
				var step NativeTransportFrameStep
				for resumes := 0; resumes < 128; resumes++ {
					step, e = state.Advance(cb)
					if e != nil {
						t.Fatal(e)
					}
					if step.Complete || step.CorruptReturn {
						break
					}
				}
				if frame.D != f.D {
					t.Fatalf("native resume fullD differs got%08x want%08x", frame.D, f.D)
				}
				if step.NativeStackReturn != f.NativeReturn || step.CorruptReturn != (f.NativeReturn != 0) {
					t.Fatal("actual unbalanced stack outcome changed")
				}
				if !step.CorruptReturn && (!step.Complete || !step.Zero || step.Negative) {
					t.Fatal("native successful CCR separated incorrectly")
				}
				if fileFrameHash(fileFrameMemoryBytes(t, m)) != f.BSSHash || fileFrameHash(code) != f.CodeHash || fileFrameHash(p.Chip) != f.ChipHash || fileFrameHash(p.PointerData[:15260]) != f.PointerHash {
					t.Fatalf("native resume memory differs BSS%v CODE%v chip%v pointer%v", fileFrameHash(fileFrameMemoryBytes(t, m)) == f.BSSHash, fileFrameHash(code) == f.CodeHash, fileFrameHash(p.Chip) == f.ChipHash, fileFrameHash(p.PointerData[:15260]) == f.PointerHash)
				}
				if !bytes.Equal(sent, f.Sent) || switches != len(f.Switches) {
					t.Fatal("native stream/profile operations differ")
				}
			})
		}
	}
}
