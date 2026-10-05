package populous2

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type nativeModalFixture struct {
	Input struct {
		Name, Text string
		Field      int
		Registers  [8]uint32
		Events     []struct {
			Keys        []uint8
			Left, Right bool
			X, Y, Phase uint16
		}
	}
	ClickEnd  int
	Snapshots []struct {
		Finished             bool
		Registers            [8]uint32
		Caret                int
		Fields, Scratch, Low []byte
		Toggle               uint32
		Hashes               [2]string
		Draws                []struct {
			Buffer      uint32
			Column, Row uint16
			Text        []byte
		}
		Audio []uint16
	}
}

func TestNativeNumericModalAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/modal_input_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativeModalFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 156 {
		t.Fatal("native modal corpus incomplete")
	}
	exe := testBundle(t).Executable
	r, err := DecodeNativePresentationInputRules(exe)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := DecodeNativeInputRules(exe)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			painting := r.NewPaintingState()
			for i := range painting.Fields {
				painting.Fields[i][0], painting.Fields[i][1] = '0', 0
			}
			copy(painting.Fields[f.Input.Field][:], f.Input.Text)
			painting.Fields[f.Input.Field][len(f.Input.Text)] = 0
			temporary := [8]uint32{}
			_, err := r.compilePainting(&painting, &NativeHUDRegisters{D4: temporary[4]})
			if err != nil {
				t.Fatal(err)
			}
			input, err := NewNativeInputState(exe)
			if err != nil {
				t.Fatal(err)
			}
			input.setLong(0, 0xa00100)
			input.setLong(4, 0xa00300)
			input.setLong(0x1a, 0xa10000)
			input.setLong(0x1e, 0xa20000)
			modal, err := BeginNativeNumericModal(&painting, f.Input.Field, f.ClickEnd, f.Input.Registers, 0)
			if err != nil {
				t.Fatal(err)
			}
			buffers := map[uint32][]byte{0xa10000: make([]byte, 32000), 0xa20000: make([]byte, 32000)}
			if len(f.Snapshots) != len(f.Input.Events)+1 {
				t.Fatal("native modal frame snapshots incomplete")
			}
			for frame, want := range f.Snapshots {
				count++
				if frame != 0 {
					e := f.Input.Events[frame-1]
					for _, wire := range e.Keys {
						if err := input.KeyboardInterrupt(wire); err != nil {
							t.Fatal(err)
						}
					}
					input.Mouse.PositionX, input.Mouse.PositionY = e.X*2, e.Y*2
					input.setWord(0xe, e.Phase)
					if _, err := input.VBlank(NativeMouseSample{Left: e.Left, Right: e.Right}, 10, 0x400000); err != nil {
						t.Fatal(err)
					}
				}
				step, err := modal.Advance(&r, &painting, &input, keys)
				if err != nil {
					t.Fatal(err)
				}
				if modal.Registers != want.Registers {
					t.Fatalf("native modal frame%d full registers differ: got%x want%x", frame, modal.Registers, want.Registers)
				}
				if step.Finished != want.Finished || modal.Active == want.Finished || modal.BufferToggle != want.Toggle {
					t.Fatalf("native modal frame%d lifetime/swap differs: %+v wantdone%v/toggle%d", frame, modal, want.Finished, want.Toggle)
				}
				if !want.Finished && modal.Caret != want.Caret {
					t.Fatalf("native modal frame%d caret%d want%d", frame, modal.Caret, want.Caret)
				}
				if len(want.Low) != len(input.Low) || !bytes.Equal(input.Low[:], want.Low) {
					for a, v := range input.Low {
						if a >= len(want.Low) || v != want.Low[a] {
							t.Fatalf("native modal frame%d low byte%x got%x want%x", frame, a, v, want.Low[a])
						}
					}
				}
				fields := make([]byte, 0, 80)
				for _, field := range painting.Fields {
					fields = append(fields, field[:]...)
				}
				if !bytes.Equal(fields, want.Fields) {
					t.Fatalf("native modal frame%d numeric buffers differ: got%x want%x", frame, fields, want.Fields)
				}
				if !bytes.Equal(painting.Scratch[:], want.Scratch) {
					for a, v := range painting.Scratch {
						if a >= len(want.Scratch) || v != want.Scratch[a] {
							t.Fatalf("native modal frame%d requester byte%x got%x want%x", frame, a, v, want.Scratch[a])
						}
					}
				}
				if len(step.Draws) != len(want.Draws) {
					t.Fatalf("native modal frame%d draw count%d want%d", frame, len(step.Draws), len(want.Draws))
				}
				for i, draw := range step.Draws {
					expect := want.Draws[i]
					if draw.Buffer != expect.Buffer || uint16(draw.Requester.Column) != expect.Column || uint16(draw.Requester.Row) != expect.Row || !bytes.Equal(draw.Requester.Text, expect.Text) {
						t.Fatalf("native modal frame%d draw%d differs: %+v want%+v", frame, i, draw, expect)
					}
					if err := r.PaintText(buffers[draw.Buffer], NativePaintingPlan{Requester: &draw.Requester}); err != nil {
						t.Fatal(err)
					}
				}
				for i, address := range []uint32{0xa10000, 0xa20000} {
					if hash := fmt.Sprintf("%x", sha256.Sum256(buffers[address])); hash != want.Hashes[i] {
						t.Fatalf("native modal frame%d buffer%x software pixels differ: got%s want%s", frame, address, hash, want.Hashes[i])
					}
				}
				if !reflect.DeepEqual(append([]uint16{}, step.Audio...), append([]uint16{}, want.Audio...)) {
					t.Fatalf("native modal frame%d audio argument order differs: got%x want%x", frame, step.Audio, want.Audio)
				}
			}
			if modal.Active {
				t.Fatal("native modal sequence never reaches genuine return")
			}
		})
	}
	if count != 1460 {
		t.Fatalf("native modal frame coverage%d want1460", count)
	}
}

func TestNativeNumericModalKeepsWaitAndActualClickOrigin(t *testing.T) {
	r, err := DecodeNativePresentationInputRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	p := r.NewPaintingState()
	for i := range p.Fields {
		p.Fields[i][0], p.Fields[i][1] = '0', 0
	}
	c := NativeHUDRegisters{}
	if _, err := r.compilePainting(&p, &c); err != nil {
		t.Fatal(err)
	}
	input, err := NewNativeInputState(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	input.setWord(0x134, 204)
	input.setWord(0x136, 44)
	end, err := NativePaintingModalOrigin(&p, input.Memory(FollowerCleanupMemory{}))
	if err != nil {
		t.Fatal(err)
	}
	modal, err := BeginNativeNumericModal(&p, 0, end, [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}, 4)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := DecodeNativeInputRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := modal.Advance(&r, &p, &input, keys); err != nil {
		t.Fatal(err)
	}
	if !modal.Active || !modal.Waiting {
		t.Fatal("modal bypassed unsatisfied VBlank wait")
	}
	beforeInput, beforePainting, beforeModal := input, p, modal
	if _, err := modal.Advance(&r, &p, &input, keys); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(input, beforeInput) || !reflect.DeepEqual(p, beforePainting) || !reflect.DeepEqual(modal, beforeModal) {
		t.Fatal("waiting modal consumed input or invented a blank")
	}
}
