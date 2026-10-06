package populous2

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"

	embedded "go-populous2/assets"
)

type resourceFrameFixture struct {
	Input struct {
		Name, Mode  string
		Index, Land uint16
		Flags       uint32
		D           [8]uint32
		Failure     string
	}
	D       [8]uint32
	Child   uint32
	Dialogs []uint32
	Calls   []struct {
		Operation, Name           string
		D                         [8]uint32
		Value, Destination, Limit uint32
	}
	Spans []struct {
		Start, Length uint32
		Hash          string
	}
	Error     string
	ReadCount uint32
	Cursor    uint16
	Flags     uint32
}

func resourceFrameInitialRAM(t *testing.T) []byte {
	t.Helper()
	exe := testBundle(t).Executable
	bases := [6]uint32{0x100000, 0x200000, 0x300000, 0x400000, 0x500000, 0x600000}
	ram := make([]byte, 1<<24)
	for i, h := range exe.Hunks {
		copy(ram[bases[i]:], h.Data)
		for _, r := range h.Relocations {
			for _, at := range r.Offsets {
				where := int(bases[i]) + int(at)
				binary.BigEndian.PutUint32(ram[where:], binary.BigEndian.Uint32(ram[where:])+bases[r.Target])
			}
		}
	}
	return ram
}

func TestNativeResourceFramesAgainstOriginalEncodedDiskAndCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/resource_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []resourceFrameFixture }
	if err := json.Unmarshal(data, &catalog); err != nil || len(catalog.Cases) != 76 {
		t.Fatalf("native resource corpus incomplete: %v", err)
	}
	rules, err := DecodeNativeResourceFrameRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	base := resourceFrameInitialRAM(t)
	files, err := embedded.DataFS()
	if err != nil {
		t.Fatal(err)
	}
	loaded, cached, landscapes, failures := 0, 0, 0, 0
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			if f.Error != "" {
				t.Fatal("original source resource stopped", f.Error)
			}
			ram := append([]byte(nil), base...)
			m := commandFrameBacking(ram)
			clear(ram[0x200000:0x211280])
			_ = m.Write32(0x20014c, 0xd00000)
			_ = m.Write32(0x2003ac, f.Input.Flags)
			_ = m.Write16(0x200000+0xeb22, f.Input.Land)
			_ = m.Write32(0x100000+0x19e4a+13*48, 0x900000)
			_ = m.Write32(0x100000+0x19e4a+14*48, 0x920000)
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			frame.Word(0, f.Input.Index)
			input, err := NewNativeInputState(testBundle(t).Executable)
			if err != nil {
				t.Fatal(err)
			}
			calls, dialogs, openAttempts, readAttempts := 0, 0, 0, 0
			var opened []byte
			cb := NativeResourceFrameCallbacks{RAM: m, CodeBase: 0x100000, Frame: &frame, Input: &input, IO: func(call NativeResourceFrameIOCall, phase *uint32) (NativeResourceFrameIOResult, error) {
				if calls >= len(f.Calls) {
					return NativeResourceFrameIOResult{}, fmt.Errorf("unexpected resource %s", call.Operation)
				}
				want := f.Calls[calls]
				if call.Operation != want.Operation || call.Frame.D != want.D || call.Destination != want.Destination || call.Limit != want.Limit {
					return NativeResourceFrameIOResult{}, fmt.Errorf("native I/O%d input differs: %s D%x want%s D%x", calls, call.Operation, call.Frame.D, want.Operation, want.D)
				}
				if *phase == 0 {
					*phase = 1
					return NativeResourceFrameIOResult{}, nil
				}
				calls++
				result := NativeResourceFrameIOResult{Complete: true}
				switch call.Operation {
				case "open":
					openAttempts++
					if call.Name != want.Name {
						return result, fmt.Errorf("source filename changed")
					}
					if f.Input.Failure != "open" && !(f.Input.Failure == "open-retry" && openAttempts == 1) {
						var err error
						opened, err = fs.ReadFile(files, strings.ToLower(call.Name))
						if err != nil {
							return result, err
						}
						result.Value = 37
					}
				case "read":
					readAttempts++
					if f.Input.Failure != "read" && !(f.Input.Failure == "read-retry" && readAttempts == 1) {
						result.Data = opened[:min(len(opened), int(call.Limit))]
						result.Value = int32(len(result.Data))
					}
				case "close":
				default:
					return result, fmt.Errorf("unknown resource operation")
				}
				if uint32(result.Value) != want.Value {
					return result, fmt.Errorf("real resource count/handle differs")
				}
				return result, nil
			}, Call: func(call NativeFileFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
				if dialogs >= len(f.Dialogs) || uint32(call.Routine) != f.Dialogs[dialogs] {
					return NativeCommandFrameResult{}, fmt.Errorf("unexpected resource dialog")
				}
				if !strings.HasSuffix(f.Input.Failure, "-retry") {
					return NativeCommandFrameResult{}, nil
				}
				// The declared retry oracle boundary acknowledges the UI child.
				// Source33B2's outer MOVEM restores all data registers, including D0.
				if *phase == 0 {
					*phase = 1
					return NativeCommandFrameResult{}, nil
				}
				dialogs++
				return NativeCommandFrameResult{Complete: true}, nil
			}}
			var step NativeResourceFrameStep
			resource := NativeResourceFrameState{}
			landscape := NativeLandscapeResourceFrameState{}
			if f.Input.Mode == "landscape" {
				step, err = landscape.Advance(&rules, cb)
				for pending := 0; err == nil && step.Waiting && (step.IO != "" || strings.HasSuffix(f.Input.Failure, "-retry")); pending++ {
					if pending > 64 {
						t.Fatal("resource I/O did not progress")
					}
					step, err = landscape.Advance(&rules, cb)
				}
				landscapes++
			} else {
				step, err = resource.Advance(&rules, cb)
				for pending := 0; err == nil && step.Waiting && (step.IO != "" || strings.HasSuffix(f.Input.Failure, "-retry")); pending++ {
					if pending > 64 {
						t.Fatal("resource I/O did not progress")
					}
					step, err = resource.Advance(&rules, cb)
				}
				if f.Child != 0 {
					failures++
				} else if frame.D[0] == 2 {
					cached++
				} else {
					loaded++
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if step.Complete != (f.Child == 0) || step.Waiting != (f.Child != 0) || frame.D != f.D {
				t.Errorf("native completion/D differs: got%+v D%x wantchild%x D%x", step, frame.D, f.Child, f.D)
			}
			if calls != len(f.Calls) {
				t.Errorf("native actual load sequence count differs: got%d want%d", calls, len(f.Calls))
			}
			for _, span := range f.Spans {
				if span.Length == 0 || uint64(span.Start)+uint64(span.Length) > uint64(len(ram)) {
					t.Fatal("malformed native physical span")
				}
				if got := fileFrameHash(ram[span.Start : span.Start+span.Length]); got != span.Hash {
					t.Errorf("native physical span%x/%x differs: got%s want%s", span.Start, span.Length, got, span.Hash)
				}
			}
			count, _ := m.Read32(0x100000 + 0x19e46)
			cursor, _ := m.Read16(0x100a2a)
			flags, _ := m.Read32(0x2003ac)
			if count != f.ReadCount || cursor != f.Cursor || input.Mouse.Image != f.Cursor || flags != f.Flags {
				t.Errorf("native read byte-count/cursor/cache metadata differs: count%d/%d cursor%x/%x flags%x/%x", count, f.ReadCount, cursor, f.Cursor, flags, f.Flags)
			}
		})
	}
	if loaded != 30 || cached != 26 || landscapes != 16 || failures != 4 {
		t.Fatalf("resource source coverage changed: %d/%d/%d/%d", loaded, cached, landscapes, failures)
	}
}
