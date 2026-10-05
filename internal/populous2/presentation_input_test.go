package populous2

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type presentationInputFixture struct {
	Input struct {
		Name, Mode                string
		Initial, Code             []nativeHeroPatch
		Registers                 [8]uint32
		Buffer                    int
		ModalText                 string
		ModalD4, ModalD5, ModalD7 uint32
	}
	Registers   [8]uint32
	State, Code []nativeHeroPatch
	Sprites     []struct {
		PC     uint32
		X, Y   int16
		Height uint16
	}
	ModalCalls []int
	Hash       string
	Blit       [6]uint32
}

func preHUDInputMemory(f presentationInputFixture) *scenarioScriptMemory {
	m := &scenarioScriptMemory{}
	_ = m.write32(0x1e, 0xa10000)
	_ = m.write32(0x22, 0xa20000)
	if f.Input.Buffer != 0 {
		_ = m.write32(0x1e, 0xa20000)
		_ = m.write32(0x22, 0xa10000)
	}
	_ = m.write16(0xeb42, 1)
	_ = m.write16(0xeb44, 8)
	_ = m.write16(0xf0c, 8)
	_ = m.write32(0xeb6a, 0xeb56)
	_ = m.write32(0xf36, 0x76f4)
	m[0x76f4], m[0x76f4+12], m[0x76f4+18], m[0x76f4+22], m[0x76f4+25] = 2, 1, 20, 4, 7
	_ = m.write16(0x76f4+14, 20)
	_ = m.write32(0x76f4+26, 1000)
	m[0x76fa], m[0x76fc] = 32, 40
	_ = m.write16(0x5f44, 24)
	_ = m.write16(0x5f46, 28)
	for _, p := range f.Input.Initial {
		applyPreHUDPatch(tinyMemoryWriter(m), p)
	}
	return m
}

// The fixture mutations are byte-exact and intentionally do not clear the
// unwritten tail of the painting workspace on each compilation.
func tinyMemoryWriter(m *scenarioScriptMemory) func(int, int, uint32) {
	return func(a, w int, v uint32) {
		switch w {
		case 1:
			_ = m.write8(a, uint8(v))
		case 2:
			_ = m.write16(a, uint16(v))
		case 4:
			_ = m.write32(a, v)
		}
	}
}
func applyPreHUDPatch(write func(int, int, uint32), p nativeHeroPatch) {
	write(p.Address, p.Width, p.Value)
}

func paintingStateBytes(s NativePaintingState) ([]byte, []byte) {
	fields := make([]byte, 0x62)
	binary.BigEndian.PutUint16(fields, s.Index)
	for i := range s.Fields {
		copy(fields[0x12+i*20:], s.Fields[i][:])
	}
	return fields, s.Scratch[:]
}

func TestPreHUDInputAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/presentation_input_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []presentationInputFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 442 {
		t.Fatal("native pre-HUD corpus incomplete")
	}
	exe := testBundle(t).Executable
	r, err := DecodeNativePresentationInputRules(exe)
	if err != nil {
		t.Fatal(err)
	}
	post, err := DecodeNativePresentationContextRules(exe)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, f := range catalog.Cases {
		counts[f.Input.Mode]++
		t.Run(f.Input.Name, func(t *testing.T) {
			m := preHUDInputMemory(f)
			expectedMemory := *m
			for _, p := range f.State {
				applyPreHUDPatch(tinyMemoryWriter(&expectedMemory), p)
			}
			c := NativeHUDRegisters{f.Input.Registers[4], f.Input.Registers[5], f.Input.Registers[7]}
			bitmap := make([]byte, 32000)
			for i := range bitmap {
				bitmap[i] = byte(i*37 + 11)
			}
			s := r.NewPaintingState()
			for _, p := range f.Input.Code {
				if p.Address == 0x37bc && p.Width == 2 {
					s.Index = uint16(p.Value)
				} else {
					t.Fatal("unsupported original fixture CODE initialization")
				}
			}
			var painting NativePaintingPlan
			modalCalls := []int{}
			cb := NativePaintingCallbacks{Memory: m.callbacks(), EditNumber: func(field int, value []byte, context *NativeHUDRegisters) ([]byte, error) {
				modalCalls = append(modalCalls, field)
				*context = NativeHUDRegisters{f.Input.ModalD4, f.Input.ModalD5, f.Input.ModalD7}
				return []byte(f.Input.ModalText), nil
			}}
			switch f.Input.Mode {
			case "copy":
				var p NativePresentationCopy
				p, err = r.CopyBackground(m.callbacks(), &c)
				// The native fixture records BLTDPT then BLTAPT, followed
				// by control/size/modulo words. Those are destination/source.
				if err == nil && ([6]uint32{p.Destination, p.Source, uint32(p.Control), uint32(p.Size), 0, 0} != f.Blit || p.Bytes != 32000) {
					t.Fatalf("original bitmap blitter request differs: %+v want%x", p, f.Blit)
				}
			case "highlight":
				var p []NativePresentationHighlight
				p, err = r.Highlights(m.callbacks(), &c)
				if err == nil {
					err = r.XORHighlights(bitmap, p)
				}
			case "hit":
				err = r.SelectedHit(cb, &c)
			case "painting":
				painting, err = r.Painting(&s, cb, &c)
				if err == nil {
					err = r.PaintText(bitmap, painting)
				}
			case "composed":
				_, err = r.CopyBackground(m.callbacks(), &c)
				if err == nil {
					_, err = r.Highlights(m.callbacks(), &c)
				}
				if err == nil {
					_, err = post.NormalFrameInput(NativePresentationContextCallbacks{Memory: m.callbacks(), SelectHit: func(context *NativeHUDRegisters) error {
						return r.SelectedHit(cb, context)
					}}, &c)
				}
			default:
				t.Fatal("unknown original pre-HUD stage")
			}
			if err != nil {
				t.Fatal(err)
			}
			if c != (NativeHUDRegisters{f.Registers[4], f.Registers[5], f.Registers[7]}) {
				t.Fatalf("original input register continuation differs: got%+v want%x/%x/%x", c, f.Registers[4], f.Registers[5], f.Registers[7])
			}
			if *m != expectedMemory {
				for i, v := range m {
					if v != expectedMemory[i] {
						t.Fatalf("original BSS byte%x got%x want%x", i, v, expectedMemory[i])
					}
				}
			}
			if hash := fmt.Sprintf("%x", sha256.Sum256(bitmap)); f.Input.Mode != "composed" && hash != f.Hash {
				t.Fatalf("original software bitmap differs: got%s want%s", hash, f.Hash)
			}
			if f.Input.Mode != "painting" {
				return
			}
			expectedFields := append([]byte(nil), exe.Hunks[0].Data[0x37bc:0x381e]...)
			expectedScratch := append([]byte(nil), exe.Hunks[0].Data[0xab4e:0xab4e+2048]...)
			for _, p := range f.Code {
				if p.Address >= 0x37bc && p.Address < 0x381e {
					expectedFields[p.Address-0x37bc] = byte(p.Value)
				} else if p.Address >= 0xab4e && p.Address < 0xab4e+2048 {
					expectedScratch[p.Address-0xab4e] = byte(p.Value)
				} else {
					t.Fatal("original painting mutation outside retained scratch")
				}
			}
			fields, scratch := paintingStateBytes(s)
			// $37be holds immutable relocated parameter pointers; compare only
			// the actual index and four field buffers, not synthetic pointers.
			if !bytes.Equal(fields[:2], expectedFields[:2]) || !bytes.Equal(fields[0x12:], expectedFields[0x12:]) || !bytes.Equal(scratch, expectedScratch) {
				for i := range scratch {
					if scratch[i] != expectedScratch[i] {
						t.Fatalf("native requester scratch byte%x got%x want%x", i, scratch[i], expectedScratch[i])
					}
				}
				t.Fatalf("native painting index/field buffers differ: got%x want%x", fields, expectedFields)
			}
			if !reflect.DeepEqual(modalCalls, append([]int{}, f.ModalCalls...)) {
				t.Fatalf("native numeric-modal callback order differs: got%v want%v", modalCalls, f.ModalCalls)
			}
			if len(painting.Marker) != len(f.Sprites) {
				t.Fatalf("original painting marker count%d want%d", len(painting.Marker), len(f.Sprites))
			}
			for i, p := range painting.Marker {
				want := f.Sprites[i]
				if p.Routine != want.PC || p.X != want.X || p.Y != want.Y || uint16(p.Height) != want.Height {
					t.Fatalf("original painting marker differs: %+v want%+v", p, want)
				}
			}
		})
	}
	if !reflect.DeepEqual(counts, map[string]int{"copy": 2, "highlight": 120, "hit": 36, "painting": 252, "composed": 32}) {
		t.Fatalf("native pre-HUD stage coverage differs: %v", counts)
	}
}

func TestPreHUDInputRejectsMissingMemoryAndNumericCallback(t *testing.T) {
	r, err := DecodeNativePresentationInputRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	c := NativeHUDRegisters{D4: 0x12345678, D5: 0x23456789, D7: 0x34567890}
	if _, err := r.CopyBackground(FollowerCleanupMemory{}, &c); err == nil {
		t.Fatal("missing original pointers accepted")
	}
	if _, err := r.Highlights(FollowerCleanupMemory{}, &c); err == nil {
		t.Fatal("missing original UI words accepted")
	}
	if err := r.XORHighlights(make([]byte, 32000), []NativePresentationHighlight{{Column: -1, Row: 0, Pattern: 0x3dd68}}); err == nil {
		t.Fatal("unbounded original bitmap alias accepted")
	}
	f := presentationInputFixture{}
	m := preHUDInputMemory(f)
	_ = m.write16(0x140, 1)
	_ = m.write16(0x134, 204)
	_ = m.write16(0x136, 44)
	s := r.NewPaintingState()
	if _, err := r.Painting(&s, NativePaintingCallbacks{Memory: m.callbacks()}, &c); err == nil {
		t.Fatal("numeric modal silently supplied a fabricated value")
	}
}
