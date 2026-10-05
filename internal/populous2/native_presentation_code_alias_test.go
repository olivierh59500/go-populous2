package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestNativePresentationCodeAliasAgainstOriginalCPU(t *testing.T) {
	for _, catalog := range []struct {
		path          string
		cases, frames int
		rawWord       bool
	}{
		{"testdata/native_frame_presentation_native.json", 189, 414, false},
		{"testdata/native_presentation_code_alias_native.json", 8, 32, true},
	} {
		data, err := os.ReadFile(catalog.path)
		if err != nil {
			t.Fatal(err)
		}
		var corpus struct {
			Cases []nativeFramePresentationFixture
		}
		var raw struct {
			Cases []struct {
				Input  struct{ InterruptWord uint16 }
				Frames []struct{ InterruptWord uint16 }
			}
		}
		if err := json.Unmarshal(data, &corpus); err != nil || len(corpus.Cases) != catalog.cases {
			t.Fatalf("incomplete presentation alias corpus: %v", err)
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatal(err)
		}
		frames := 0
		for index, f := range corpus.Cases {
			t.Run(fmt.Sprintf("%s/%s", catalog.path, f.Input.Name), func(t *testing.T) {
				view, host := nativeSharedCodeTestView(t)
				p, err := NewNativeFramePresentationState(testBundle(t).Executable, 0x500000, 0x400000)
				if err != nil {
					t.Fatal(err)
				}
				p.Chip, err = host.Span(p.ChipBase, NativeFrameChipBytes)
				if err != nil {
					t.Fatal(err)
				}
				p.PointerData, err = host.Span(p.PointerBase, len(p.PointerData))
				if err != nil {
					t.Fatal(err)
				}
				for i := 0x408; i < len(p.Chip); i++ {
					p.Chip[i] = uint8(i*17 + 3 + ((i-0x408)/32000)*91)
				}
				alias, err := NewNativePresentationCodeAlias(p, host.Memory(), view.CodeBase)
				if err != nil {
					t.Fatal(err)
				}
				word := uint16(0)
				if f.Input.Chain {
					word = 1
				}
				if catalog.rawWord {
					word = raw.Cases[index].Input.InterruptWord
				}
				if err := alias.Code.Write16(0x3ea, word); err != nil {
					t.Fatal(err)
				}
				if err := alias.RAM.Write32(int(view.CodeBase)+0x1117c, f.Input.Deadline); err != nil {
					t.Fatal(err)
				}
				p.Input.Mouse.Image = uint16(f.Input.MouseImage)
				b := make([]byte, 0x11280)
				m := p.Memory(commandNativeMemory(b))
				for _, patch := range f.Input.Initial {
					switch patch.Width {
					case 1:
						err = m.Write8(patch.Address, uint8(patch.Value))
					case 2:
						err = m.Write16(patch.Address, uint16(patch.Value))
					case 4:
						err = m.Write32(patch.Address, patch.Value)
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				c := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
				for frameIndex, want := range f.Frames {
					frames++
					sample := NativeMouseSample{CounterX: want.Step.X, CounterY: want.Step.Y, Left: want.Step.Left, Right: want.Step.Right}
					var hardware []NativeFrameHardwareWrite
					pending := ""
					switch want.Step.Kind {
					case "init":
						hardware, err = p.Initialize(testBundle(t).Executable, sample)
					case "swap":
						hardware, err = p.Swap(&c)
					case "vblank":
						var result NativeFrameVBlankResult
						result, err = p.VBlank(sample, m, &c)
						hardware = result.Hardware
						if result.ChainOriginalInterrupt != (word != 0) {
							t.Fatal("raw IRQ WORD lost its source predicate")
						}
					case "clock":
						palette := false
						var complete bool
						complete, err = p.AdvanceClock(&c, NativeFrameClockCallbacks{Memory: m, Palette: func(*NativeFrameRegisterContext) (bool, error) { palette = true; return false, nil }})
						if !complete {
							pending = "idle"
							if palette {
								pending = "palette"
							}
						}
					default:
						t.Fatal("unknown native presentation stage")
					}
					if err != nil {
						t.Fatal(err)
					}
					if c.D != want.D || pending != want.Pending || !reflect.DeepEqual(hardware, want.Hardware) {
						t.Fatalf("native %s registers/wait/hardware differ", want.Step.Kind)
					}
					for offset, value := range map[int]uint32{0x77a: want.CopperSelector, 0x77e: want.SpritePatchPointer, 0x1117c: want.Deadline} {
						code, err := alias.Code.Read32(offset)
						physical, perr := alias.RAM.Read32(int(view.CodeBase) + offset)
						if err != nil || perr != nil || code != value || physical != value {
							t.Fatalf("live CODE alias%x differs: %x/%x/%x", offset, code, physical, value)
						}
					}
					gotWord, err := alias.Code.Read16(0x3ea)
					wantWord := word
					if catalog.rawWord {
						wantWord = raw.Cases[index].Frames[frameIndex].InterruptWord
					}
					if err != nil || gotWord != wantWord || alias.InterruptWord() != wantWord {
						t.Fatal("complete IRQ WORD normalized")
					}
					all := append([]byte(nil), b...)
					copy(all[:0x14c], p.Input.Low[:])
					copy(all[0x14c:0xdc2], p.LowTail[:])
					if fileFrameHash(all) != want.BSSHash || fileFrameHash(p.Chip) != want.ChipHash || fileFrameHash(p.PointerData) != want.PointerHash {
						t.Fatalf("complete native RAM differs after%s", want.Step.Kind)
					}
					img, err := p.Image()
					if err != nil || fileFrameHash(img.Pix) != want.PixelHash {
						t.Fatal("native Copper-selected pixels differ", err)
					}
				}
			})
		}
		if frames != catalog.frames {
			t.Fatalf("native frame count differs: %d/%d", frames, catalog.frames)
		}
	}
}

func TestNativePresentationCodeAliasPartialWritesAndOwnership(t *testing.T) {
	view, host := nativeSharedCodeTestView(t)
	p, err := NewNativeFramePresentationState(testBundle(t).Executable, 0x500000, 0x400000)
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewNativePresentationCodeAlias(p, host.Memory(), view.CodeBase)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Code.Write16(0x3ea, 0x8000); err != nil {
		t.Fatal(err)
	}
	if err := a.RAM.Write8(int(view.CodeBase)+0x3eb, 0x5a); err != nil {
		t.Fatal(err)
	}
	if a.InterruptWord() != 0x805a || !p.InterruptChain {
		t.Fatal("byte update reduced retained interrupt WORD")
	}
	if err := a.Code.Write16(0x3ea, 0); err != nil || p.InterruptChain {
		t.Fatal("zero interrupt predicate lost", err)
	}
	p.CopperSelector = 0x12345678
	if err := a.Code.Write16(0x77c, 0xabcd); err != nil || p.CopperSelector != 0x1234abcd {
		t.Fatal("partial selector update lost untouched bytes", err)
	}
	if err := a.Code.Write32(0x77c, 0x87654321); err != nil || p.CopperSelector != 0x12348765 || p.SpritePatchPointer>>16 != 0x4321 {
		t.Fatal("cross-field LONG did not preserve exact boundaries", err)
	}
	p.Deadline1117C = 0xfedcba98
	if got, err := a.Code.Read32(0x1117c); err != nil || got != 0xfedcba98 {
		t.Fatal("live clock change invisible through CODE", err)
	}
	p.ActiveCopper = 0xdeadbeef
	if err := a.Code.Write32(0x782, 0x76543210); err != nil {
		t.Fatal(err)
	}
	if p.ActiveCopper != 0xdeadbeef {
		t.Fatal("hardware COP1LC was falsely mapped as CODE")
	}
	if got, _ := host.Memory().Read32(int(view.CodeBase) + 0x782); got != 0x76543210 {
		t.Fatal("nonalias write did not reach actual physical RAM")
	}
	if _, err := a.Code.Read16(0x3eb); err == nil {
		t.Fatal("unaligned native WORD normalized")
	}
	if _, err := a.RAM.Read8(0x700000); err == nil {
		t.Fatal("unmapped physical RAM substituted")
	}
}
