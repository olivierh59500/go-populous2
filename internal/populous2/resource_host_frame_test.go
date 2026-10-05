package populous2

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	embedded "go-populous2/assets"
	"io"
	"io/fs"
	"os"
	"testing"
)

type resourceHostFaultFilesystem struct {
	fs.FS
	Failure                string
	openFailed, readFailed bool
}

func (f *resourceHostFaultFilesystem) Open(name string) (fs.File, error) {
	if f.Failure == "open" && !f.openFailed {
		f.openFailed = true
		return nil, fs.ErrNotExist
	}
	file, err := f.FS.Open(name)
	if err != nil {
		return nil, err
	}
	return &resourceHostFaultFile{File: file, owner: f}, nil
}

type resourceHostFaultFile struct {
	fs.File
	owner *resourceHostFaultFilesystem
}

func (f *resourceHostFaultFile) Read(data []byte) (int, error) {
	if f.owner.Failure == "read" && !f.owner.readFailed {
		f.owner.readFailed = true
		return 0, io.EOF
	}
	return f.File.Read(data)
}

type resourceHostFrameFixture struct {
	Input struct {
		Name, Mode, Failure string
		Index, Land, Gate   uint16
		D                   [8]uint32
		Events              []struct {
			Action int
			VBlank bool
		}
	}
	CallerA [7]uint32
	Frames  []struct {
		PC                                                              int
		Waiting, Complete                                               bool
		D                                                               [8]uint32
		A                                                               [7]uint32
		BSSHash, CodeHash, ChipHash, PointerHash, BankHash, SpritesHash string
		Selector, Patch, Copper                                         uint32
		Dialog                                                          uint32
		Sounds                                                          []uint16
		IO                                                              []struct {
			Operation, Name string
			D               [8]uint32
			Value           uint32
		}
	}
}

func TestNativeResourceHostFrameActualFailureDialogRetry(t *testing.T) {
	data, err := os.ReadFile("testdata/resource_host_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []resourceHostFrameFixture }
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 12 {
		t.Fatalf("native error corpus incomplete: %v", err)
	}
	bundle := testBundle(t)
	base := resourceFrameInitialRAM(t)
	rules, err := DecodeNativeResourceFrameRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	files, err := fs.Sub(embedded.Files, "amiga")
	if err != nil {
		t.Fatal(err)
	}
	frames, waiting, finished := 0, 0, 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			if len(f.Frames) == 0 {
				t.Fatal("empty original options trace")
			}
			p, err := NewNativeFramePresentationState(bundle.Executable, 0x500000, 0x400000)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0x408; i < len(p.Chip); i++ {
				p.Chip[i] = byte(i*17 + 3 + ((i-0x408)/32000)*91)
			}
			p.InterruptChain = false
			if _, err = p.Initialize(bundle.Executable, NativeMouseSample{}); err != nil {
				t.Fatal(err)
			}
			ram := append([]byte(nil), base...)
			m := p.Memory(commandFrameBacking(ram[0x200000:0x211280]))
			code := ram[0x100000 : 0x100000+0x3fa2c]
			cm := commandFrameBacking(code)
			_ = cm.Write16(0x3ea, 0)
			_ = cm.Write32(0x77a, p.CopperSelector)
			_ = cm.Write32(0x77e, p.SpritePatchPointer)
			_ = m.Write16(0x3b0, f.Input.Gate)
			_ = m.Write32(0x14c, 0xd00000)
			_ = m.Write16(0xeb22, f.Input.Land)
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			frame.Word(0, f.Input.Index)
			state := NativeResourceHostFrameState{Landscape: f.Input.Mode == "landscape"}

			expected, sounds := 0, 0
			resolver := func(address uint32) ([]byte, error) {
				at := int(int64(address) - int64(p.ChipBase))
				if at < 0 || at > len(p.Chip)-32000 {
					return nil, fmt.Errorf("native error target %#x unavailable", address)
				}
				return p.Chip[at : at+32000], nil
			}
			physical := commandFrameBacking(ram)
			rawRead, rawWrite := physical.Read8, physical.Write8
			physical.Read8 = func(at int) (byte, error) {
				if at >= 0x200000 && at < 0x211280 {
					return m.Read8(at - 0x200000)
				}
				return rawRead(at)
			}
			physical.Write8 = func(at int, value byte) error {
				if at >= 0x200000 && at < 0x211280 {
					return m.Write8(at-0x200000, value)
				}
				return rawWrite(at, value)
			}
			physical.Read16 = func(at int) (uint16, error) {
				hi, e := physical.Read8(at)
				if e != nil {
					return 0, e
				}
				lo, e := physical.Read8(at + 1)
				return uint16(hi)<<8 | uint16(lo), e
			}
			physical.Read32 = func(at int) (uint32, error) {
				hi, e := physical.Read16(at)
				if e != nil {
					return 0, e
				}
				lo, e := physical.Read16(at + 2)
				return uint32(hi)<<16 | uint32(lo), e
			}
			physical.Write16 = func(at int, v uint16) error {
				if e := physical.Write8(at, byte(v>>8)); e != nil {
					return e
				}
				return physical.Write8(at+1, byte(v))
			}
			physical.Write32 = func(at int, v uint32) error {
				if e := physical.Write16(at, uint16(v>>16)); e != nil {
					return e
				}
				return physical.Write16(at+2, uint16(v))
			}
			filesystem, err := NewNativeResourceFilesystem(files)
			if err != nil {
				t.Fatal(err)
			}
			filesystem.next = 37
			// The real filesystem adapter sees a genuine failing file operation
			// once, then the same encoded resource becomes available for retry.
			filesystem.Files = &resourceHostFaultFilesystem{FS: files, Failure: f.Input.Failure}
			ioCalls := 0
			cb := NativeResourceHostFrameCallbacks{NativeErrorFrameCallbacks: NativeErrorFrameCallbacks{RAM: physical, NativeFileFrameCallbacks: NativeFileFrameCallbacks{Code: cm, Memory: m, CodeBase: 0x100000, Frame: &frame, Presentation: p, Bitmap: resolver, Sound: func(cue uint16, c *NativeFrameRegisterContext) error {
				want := f.Frames[expected]
				if sounds >= len(want.Sounds) || cue != want.Sounds[sounds] {
					return fmt.Errorf("frame%d source sound differs", expected)
				}
				sounds++
				return nil
			}},
			}}
			for i, a := range f.CallerA {
				cb.ResourceCallerA[i] = NativeRequesterAddress{Address: a, Absolute: true}
			}
			cb.IO = func(call NativeResourceFrameIOCall, phase *uint32) (NativeResourceFrameIOResult, error) {
				want := f.Frames[expected]
				if ioCalls >= len(want.IO) || want.IO[ioCalls].Operation != call.Operation || call.Frame.D != want.IO[ioCalls].D {
					return NativeResourceFrameIOResult{}, fmt.Errorf("frame%d actualIO%s D differs", expected, call.Operation)
				}
				original := want.IO[ioCalls]
				ioCalls++
				result, err := filesystem.IO(call, phase)
				if err == nil && uint32(result.Value) != original.Value {
					return result, fmt.Errorf("realencodedIO count differs")
				}
				return result, err
			}
			check := func() {
				step, err := state.Advance(&rules, cb)
				if err != nil {
					t.Fatalf("frame%d: %v", expected, err)
				}
				want := f.Frames[expected]
				if step.Complete != want.Complete || step.Waiting != !want.Complete || frame.D != want.D {
					t.Fatalf("frame%d native poll/modal context differs: got%+v D%x wantPC%x wait%v done%v D%x", expected, step, frame.D, want.PC, want.Waiting, want.Complete, want.D)
				}
				values := []uint16{p.Input.Mouse.Image, p.Input.Mouse.CounterX, p.Input.Mouse.CounterY, p.Input.Mouse.PositionX, p.Input.Mouse.PositionY, p.Input.Mouse.MaximumY}
				for i, v := range values {
					binary.BigEndian.PutUint16(code[0xa2a+i*2:], v)
				}
				for _, pair := range []struct{ name, got, want string }{{"BSS", fileFrameHash(fileFrameMemoryBytes(t, m)), want.BSSHash}, {"CODE", fileFrameHash(code), want.CodeHash}, {"chip pixels/Copper", fileFrameHash(p.Chip), want.ChipHash}, {"pointer RAM", fileFrameHash(p.PointerData[:15260]), want.PointerHash}} {
					if pair.got != pair.want {
						t.Fatalf("frame%d native error %s differs: got%s want%s", expected, pair.name, pair.got, pair.want)
					}
				}
				if fileFrameHash(ram[0x300000:0x30cf58]) != want.BankHash || fileFrameHash(ram[0x600000:0x6263d8]) != want.SpritesHash {
					t.Fatalf("frame%d actualresourcebanks differ", expected)
				}
				if p.CopperSelector != want.Selector || p.SpritePatchPointer != want.Patch || p.ActiveCopper != want.Copper || sounds != len(want.Sounds) || ioCalls != len(want.IO) {
					t.Fatal("native error retained video/sound metadata differs")
				}
				frames++
				if step.Waiting {
					waiting++
				}
				if step.Complete {
					finished++
				}
				sounds, ioCalls = 0, 0
			}
			check()
			irq := func(x, y uint8, left bool) {
				if _, err = p.VBlank(NativeMouseSample{CounterX: x, CounterY: y, Left: left}, m, &frame); err != nil {
					t.Fatal(err)
				}
			}
			find := func(action int) (int, int) {
				start := 0xab4e + int(int16(binary.BigEndian.Uint16(code[0xab4e:])))
				width := int(binary.BigEndian.Uint16(code[0xab54:]))
				count := 0
				for i := 0; code[start+i] != 0; i++ {
					v := code[start+i]
					if int8(v) > 0x5a && int8(code[0x4e92+int(v)-0x5b]) > 0 {
						count += 2
						if count == action {
							return (int(binary.BigEndian.Uint16(code[0xab50:])) + i%(width+1)) * 8, int(binary.BigEndian.Uint16(code[0xab52:])) + i/(width+1)*8
						}
					}
				}
				t.Fatalf("source action%d unavailable", action)
				return 0, 0
			}
			move := func(x, y int) {
				for int(p.Input.Mouse.PositionX) != x*2 || int(p.Input.Mouse.PositionY) != y*2 {
					dx, dy := max(-100, min(100, x*2-int(p.Input.Mouse.PositionX))), max(-100, min(100, y*2-int(p.Input.Mouse.PositionY)))
					irq(uint8(int(p.Input.Mouse.CounterX)+dx), uint8(int(p.Input.Mouse.CounterY)+dy), false)
				}
				irq(uint8(p.Input.Mouse.CounterX), uint8(p.Input.Mouse.CounterY), true)
			}
			for i, event := range f.Input.Events {
				expected = i + 1
				if event.Action > 0 {
					x, y := find(event.Action)
					move(x, y)
				} else if event.VBlank {
					irq(uint8(p.Input.Mouse.CounterX), uint8(p.Input.Mouse.CounterY), false)
				}

				check()
			}
			if (!state.Landscape && state.Resource.Finished) || (state.Landscape && state.Graphics.Finished) {
				before, registers := fileFrameHash(p.Chip), frame.D
				step, err := state.Advance(&rules, cb)
				if err != nil || !step.Complete || before != fileFrameHash(p.Chip) || frame.D != registers {
					t.Fatal("completed options requester replayed its prefix")
				}
			}
		})
	}
	if frames != 228 || waiting != 216 || finished != 12 {
		t.Fatalf("native error coverage changed: frames%d waits%d exits%d", frames, waiting, finished)
	}
}
