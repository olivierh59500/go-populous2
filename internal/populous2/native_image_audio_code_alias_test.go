package populous2

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"
)

func TestNativeImageAudioCodeAliasImageAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/editor_cursor_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct{ Cases []editorCursorFixture }
	if err := json.Unmarshal(data, &corpus); err != nil || len(corpus.Cases) != 4105 {
		t.Fatalf("original image corpus incomplete: %v", err)
	}
	bundle := testBundle(t)
	r, err := DecodeNativeEditorCursorRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	code, host := nativeSharedCodeTestView(t)
	for _, f := range corpus.Cases {
		if f.Error != "" {
			t.Fatal(f.Input.Name, f.Error)
		}
		image := r.NewImageState()
		audio, err := DecodeNativeFrameAudioState(bundle.Executable)
		if err != nil {
			t.Fatal(err)
		}
		alias, err := NewNativeImageAudioCodeAlias(&image, &audio, host.Memory(), code.CodeBase, NativeImageCodeOwner)
		if err != nil {
			t.Fatal(err)
		}
		if f.Input.WrapCounters {
			for i := 0; i < len(image.AudioBank); i += 2 {
				if err := alias.Code.Write16(0x185a8+i, 0xffff); err != nil {
					t.Fatal(err)
				}
			}
		}
		want := image
		for _, patch := range f.AudioChanges {
			if patch.Width != 1 || patch.Address < 0 || patch.Address >= len(want.AudioBank) {
				t.Fatal("image CPU queue delta outside bank")
			}
			want.AudioBank[patch.Address] = byte(patch.Value)
		}
		registers := f.Input.Registers
		var sprites []NativePresentationSprite
		if f.Input.Mode == "preview" {
			m := &scenarioScriptMemory{}
			_ = m.write16(0xf0e, f.Input.Edit)
			_ = m.write16(0xf10, f.Input.Tool)
			_ = m.write16(0x138, uint16(f.Input.X))
			_ = m.write16(0x13a, uint16(f.Input.Y))
			sprites, err = r.Preview(m.callbacks(), &image, &registers)
		} else {
			registers[0] = hudWord(registers[0], uint16(f.Input.X))
			registers[1] = hudWord(registers[1], uint16(f.Input.Y))
			repeat := f.Input.Repeat
			if repeat == 0 {
				repeat = 1
			}
			for i := 0; i < repeat; i++ {
				registers[2] = hudWord(registers[2], f.Input.Frame)
				var layers []NativePresentationSprite
				layers, err = r.DrawImage(&image, &registers)
				if err != nil {
					break
				}
				sprites = append(sprites, layers...)
			}
		}
		if err != nil || registers != f.Registers || image.LastY != f.LastY || image.AudioBank != want.AudioBank {
			t.Fatalf("native image owner differs for%s: %v", f.Input.Name, err)
		}
		if lastY, err := alias.RAM.Read16(int(code.CodeBase) + 0xeee0); err != nil || lastY != f.LastY {
			t.Fatal("source lastY invisible through physical callbacks", err)
		}
		for i, v := range want.AudioBank {
			if got, err := alias.Code.Read8(0x185a8 + i); err != nil || got != v {
				t.Fatalf("image queue byte%d differs for%s", i, f.Input.Name)
			}
		}
		if len(sprites) != len(f.Sprites) {
			t.Fatal("native layer count differs")
		}
		for i, p := range sprites {
			w := f.Sprites[i]
			if p.Routine != w.PC || p.X != w.X || p.Y != w.Y || uint16(p.Height) != w.Height {
				t.Fatal("native layer request changed through owner", f.Input.Name)
			}
		}
	}
}

func TestNativeImageAudioCodeAliasSchedulerAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/frame_context_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct{ Cases []frameContextFixture }
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	code, host := nativeSharedCodeTestView(t)
	count := 0
	for _, f := range corpus.Cases {
		if len(f.Input.Stages) != 1 || f.Input.Stages[0] != 0x182ce {
			continue
		}
		count++
		if len(f.Frames) != 1 {
			t.Fatal("original scheduler trace incomplete")
		}
		image := NativeImageRenderState{}
		audio, err := DecodeNativeFrameAudioState(testBundle(t).Executable)
		if err != nil {
			t.Fatal(err)
		}
		alias, err := NewNativeImageAudioCodeAlias(&image, &audio, host.Memory(), code.CodeBase, NativeAudioCodeOwner)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range f.Input.Code {
			switch p.Width {
			case 1:
				err = alias.Code.Write8(p.Address, byte(p.Value))
			case 2:
				err = alias.Code.Write16(p.Address, uint16(p.Value))
			case 4:
				err = alias.Code.Write32(p.Address, p.Value)
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		frame := NativeFrameRegisterContext{D: f.Input.D}
		if err := audio.TickAudio(&frame, NativeFrameAudioCallbacks{Command: func(_, _ uint16, input uint32) (uint32, error) { return input, nil }}); err != nil {
			t.Fatal(err)
		}
		want := f.Frames[0]
		if frame.D != want.D || !reflect.DeepEqual(audio.Entries[:], want.Audio) || audio.Channels != want.Channels {
			t.Fatal("source scheduler differs", f.Input.Name)
		}
		for i, v := range want.Audio {
			if got, err := alias.Code.Read8(0x185a8 + i); err != nil || got != v {
				t.Fatal("scheduler queue write invisible", f.Input.Name)
			}
		}
		for i, v := range want.Channels {
			if got, err := alias.RAM.Read16(int(code.CodeBase) + 0x18426 + i*2); err != nil || got != v {
				t.Fatal("scheduler channel write invisible", f.Input.Name)
			}
		}
	}
	if count != 54 {
		t.Fatalf("source scheduler cases%d", count)
	}
}

func TestNativeImageAudioCodeAliasSourceOwnerBoundaries(t *testing.T) {
	code, host := nativeSharedCodeTestView(t)
	image := NativeImageRenderState{LastY: 0xffed}
	audio := NativeFrameAudioState{}
	binary.BigEndian.PutUint16(image.AudioBank[20:], 7)
	binary.BigEndian.PutUint16(audio.Entries[20:], 3)
	alias, err := NewNativeImageAudioCodeAlias(&image, &audio, host.Memory(), code.CodeBase, NativeImageCodeOwner)
	if err != nil {
		t.Fatal(err)
	}
	if err := alias.SetOwner(NativeAudioCodeOwner); err != nil {
		t.Fatal(err)
	}
	if got, _ := alias.Code.Read16(0x185a8 + 20); got != 3 {
		t.Fatal("owner change silently copied queue")
	}
	// The real render completion copies once, then the source effects and
	// scheduler own Audio. A partial native write must not touch old Image.
	audio.Entries = image.AudioBank
	if err := alias.Code.Write16(0x185a8+20, 9); err != nil {
		t.Fatal(err)
	}
	if binary.BigEndian.Uint16(image.AudioBank[20:]) != 7 || binary.BigEndian.Uint16(audio.Entries[20:]) != 9 {
		t.Fatal("physics erased staged image authority")
	}
	// At the actual pre-swap boundary, Image receives the queue. A retained
	// deferred UI child can now add sounds without modifying old Audio.
	image.AudioBank = audio.Entries
	if err := alias.SetOwner(NativeImageCodeOwner); err != nil {
		t.Fatal(err)
	}
	if err := alias.RAM.Write8(int(code.CodeBase)+0x185a8+21, 11); err != nil {
		t.Fatal(err)
	}
	if binary.BigEndian.Uint16(image.AudioBank[20:]) != 11 || binary.BigEndian.Uint16(audio.Entries[20:]) != 9 {
		t.Fatal("deferred UI sound lost its actual owner")
	}
	if err := alias.Code.Write32(0x1842c, 0x80004e71); err != nil || audio.Channels[3] != 0x8000 {
		t.Fatal("channel/end-of-bank partial native write differs", err)
	}
	if got, _ := host.Memory().Read16(int(code.CodeBase) + 0x1842e); got != 0x4e71 {
		t.Fatal("unowned instruction bytes omitted")
	}
	if err := alias.Code.Write8(0xeee1, 0x42); err != nil || image.LastY != 0xff42 {
		t.Fatal("lastY partial WORD changed unrelated byte", err)
	}
	if err := alias.Code.Write32(0x185a6, 0x4e758076); err != nil || image.AudioBank[0] != 0x80 || image.AudioBank[1] != 0x76 {
		t.Fatal("queue start crossing changed source bytes", err)
	}
	if alias.Owner() != NativeImageCodeOwner || alias.SetOwner(2) == nil {
		t.Fatal("unavailable bank owner accepted")
	}
	if _, err := alias.Code.Read16(0x18427); err == nil {
		t.Fatal("unaligned audio WORD normalized")
	}
}

func nativeSharedAudioDeviceTest(t *testing.T) (*NativeAudioDevice, *NativeSharedCode, *NativeHostMemory) {
	t.Helper()
	code, host := nativeSharedCodeTestView(t)
	bundle := testBundle(t)
	d, err := NewNativeSharedAudioDevice(bundle.Executable, bundle.Raw["fx.dat"], code, 0x800000, 0)
	if err != nil {
		t.Fatal(err)
	}
	if &d.Code[0] != &code.Bytes[0] {
		t.Fatal("device CODE is not the canonical physical owner")
	}
	return d, code, host
}

func TestNativeSharedAudioDeviceCommandsAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/audio_native_device_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct{ Cases []nativeAudioDeviceFixture }
	if err := json.Unmarshal(data, &corpus); err != nil || len(corpus.Cases) != 465 {
		t.Fatalf("source audio device corpus incomplete: %v", err)
	}
	for _, f := range corpus.Cases {
		d, code, host := nativeSharedAudioDeviceTest(t)
		frame := NativeFrameRegisterContext{D: f.Input.D}
		writes, err := d.InitializeWithFrame(&frame)
		if err != nil || !reflect.DeepEqual(writes, f.InitialHardware) || fileFrameHash(d.Code[0x18b16:0x18ee4]) != f.InitialHash {
			t.Fatal("original shared driver initialization differs", f.Input.Name, err)
		}
		if f.Input.HasDisabled {
			d.ResourceBase = f.Input.Disabled
		}
		for _, want := range f.Frames {
			d.Hardware = nil
			if want.Step.Kind == "cue" {
				frame.Word(0, want.Step.Data)
				err = d.DirectCue(want.Step.Data, &frame)
			} else {
				frame.D[0], err = d.Command(want.Step.Control, want.Step.Data, frame.D[0])
			}
			if err != nil || frame.D != want.D || !reflect.DeepEqual(d.Hardware, want.Hardware) || fileFrameHash(d.Code[0x18b16:0x18ee4]) != want.DriverHash {
				t.Fatal("original shared driver command differs", f.Input.Name, err)
			}
			actual, err := host.Span(code.CodeBase+0x18b16, 0x18ee4-0x18b16)
			if err != nil || fileFrameHash(actual) != want.DriverHash {
				t.Fatal("native driver mutations detached from actual physical RAM", err)
			}
		}
	}
}

func TestNativeSharedAudioDeviceIRQAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/audio_native_irq_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct{ Cases []nativeAudioIRQFixture }
	if err := json.Unmarshal(data, &corpus); err != nil || len(corpus.Cases) != 142 {
		t.Fatalf("source audio IRQ corpus incomplete: %v", err)
	}
	for _, f := range corpus.Cases {
		d, _, _ := nativeSharedAudioDeviceTest(t)
		absolute := [16]byte{4: 0, 5: 0x90, 6: 0, 7: 0}
		d.ReadAbsolute8 = func(at uint32) (uint8, error) {
			if at >= uint32(len(absolute)) {
				return 0, fmt.Errorf("absolute native audio address unavailable")
			}
			return absolute[at], nil
		}
		frame := NativeFrameRegisterContext{D: f.Input.D}
		if _, err := d.InitializeWithFrame(&frame); err != nil {
			t.Fatal(err)
		}
		if f.Input.Music {
			frame.D[0], err = d.MusicCommand(0x8f, 0, frame.D[0])
			if err != nil {
				t.Fatal(err)
			}
		}
		if len(f.Frames) != f.Input.Ticks || len(f.Frames) == 0 {
			t.Fatal("original IRQ frames incomplete")
		}
		for _, want := range f.Frames {
			d.Hardware = nil
			for _, event := range f.Input.Events {
				if event.Tick != want.Tick {
					continue
				}
				switch event.Kind {
				case "cue":
					frame.Word(0, event.Data)
					err = d.DirectCue(event.Data, &frame)
				case "music":
					frame.D[0], err = d.MusicCommand(event.Control, event.Data, frame.D[0])
				default:
					frame.D[0], err = d.Command(event.Control, event.Data, frame.D[0])
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := d.TickCIA(&frame); err != nil {
				t.Fatal(err)
			}
			if frame.D != want.D || !reflect.DeepEqual(d.Hardware, want.Hardware) || fileFrameHash(d.Code[0x18b16:0x18ee4]) != want.DriverHash || fileFrameHash(d.Code[0x194e2:0x194ea]) != want.DMAHash {
				t.Fatalf("shared native IRQ differs for%s tick%d", f.Input.Name, want.Tick)
			}
		}
	}
}

func TestNativeSharedAudioDevicePreservesActualAllocation(t *testing.T) {
	code, host := nativeSharedCodeTestView(t)
	if err := host.Memory().Write16(int(code.CodeBase)+0x3f90, 0x4567); err != nil {
		t.Fatal(err)
	}
	bundle := testBundle(t)
	fx := append([]byte(nil), bundle.Raw["fx.dat"]...)
	d, err := NewNativeSharedAudioDevice(bundle.Executable, fx, code, 0x800000, 0)
	if err != nil || binary.BigEndian.Uint16(d.Code[0x3f90:]) != 0x4567 {
		t.Fatal("shared device constructor overwrote retained CODE", err)
	}
	if &d.FX[0] != &fx[0] {
		t.Fatal("decoded FX actual backing copied")
	}
	if err := host.Memory().Write8(int(code.CodeBase)+0x18b30, 0xa5); err != nil || d.Code[0x18b30] != 0xa5 {
		t.Fatal("physical driver mutation hidden from device", err)
	}
	if _, err := NewNativeSharedAudioDevice(bundle.Executable, nil, code, 0x800000, 0); err == nil {
		t.Fatal("missing actual FX backing accepted")
	}
}

func TestNativeSharedAudioDeviceUsesActualStartupFXAllocation(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	frame := NativeFrameRegisterContext{AddressBase: h.Memory.BSSBase}
	complete, err := h.AdvanceAllocations(0x1a43e, &frame, NativeErrorFrameCallbacks{})
	if err != nil || !complete {
		t.Fatal("actual encoded FX allocation/load failed", err)
	}
	address, err := h.Memory.BSS.Read32(0x3b4)
	if err != nil {
		t.Fatal(err)
	}
	fx, err := h.Host.Span(address, NativeStartupAudioBytes)
	if err != nil {
		t.Fatal(err)
	}
	d, err := NewNativeSharedAudioDevice(h.Bundle.Executable, fx, h.Code, address, 0)
	if err != nil {
		t.Fatal("actual larger FX allocation was rejected", err)
	}
	if len(d.FX) != NativeStartupAudioBytes || &d.FX[0] != &fx[0] || uint64(binary.BigEndian.Uint32(fx[:4]))+4 != uint64(len(h.Bundle.Raw["fx.dat"])) {
		t.Fatal("actual native allocation/header or owner differs")
	}
	if _, err := d.InitializeWithFrame(&frame); err != nil {
		t.Fatal(err)
	}
	if len(d.Code) != len(h.Code.RawData()) || &d.Code[0] != &h.Code.RawData()[0] {
		t.Fatal("actual startup driver detached from physical CODE")
	}
}

func TestNativeSharedAudioDeviceWithSerializedCallerOwnership(t *testing.T) {
	d, code, host := nativeSharedAudioDeviceTest(t)
	if _, err := d.Initialize(); err != nil {
		t.Fatal(err)
	}
	var owner sync.Mutex
	var work sync.WaitGroup
	errors := make(chan error, 2)
	work.Add(2)
	go func() {
		defer work.Done()
		for i := 0; i < 64; i++ {
			owner.Lock()
			_, err := d.Command(0x2004, uint16(i), 0)
			owner.Unlock()
			if err != nil {
				errors <- err
				return
			}
		}
	}()
	go func() {
		defer work.Done()
		for i := 0; i < 64; i++ {
			owner.Lock()
			value, err := host.Memory().Read8(int(code.CodeBase) + 0x18b6a + 2*80 + 320 + 47)
			owner.Unlock()
			if err != nil {
				errors <- err
				return
			}
			if value > 63 {
				errors <- fmt.Errorf("serialized shared volume outside source byte range")
				return
			}
		}
	}()
	work.Wait()
	close(errors)
	for err := range errors {
		t.Fatal(err)
	}
}
