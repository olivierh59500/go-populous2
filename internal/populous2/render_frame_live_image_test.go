package populous2

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestNativeLiveImageAfterInitialStartupAgainstOriginalCPUAndDMA(t *testing.T) {
	data, err := os.ReadFile("testdata/render_frame_live_image_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			editorCursorFixture
			BitmapHash string
			Thresholds *[3]uint16
		}
	}
	if err := json.Unmarshal(data, &corpus); err != nil || len(corpus.Cases) != 4201 {
		t.Fatalf("rebased original image corpus incomplete: %v", err)
	}
	bundle := testBundle(t)
	code, _ := nativeSharedCodeTestView(t)
	canonical := code.Physical()
	initial := NativeFrameRegisterContext{}
	var addresses [7]NativeRequesterAddress
	if err := RebaseNativeStartupAnimations(NativeStartupResetFrameCallbacks{Code: canonical, CodeBase: code.CodeBase, Frame: &initial}, &addresses); err != nil {
		t.Fatal(err)
	}
	// The native six-byte walk ends two bytes past the $33194 comparison;
	// the final descriptor WORD still lies within the retained table span.
	if addresses[0].Address != code.CodeBase+0x33196 {
		t.Fatal("genuine startup did not traverse the complete animation table")
	}
	rules, err := DecodeNativeEditorCursorRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	procedureReads := 0
	if err := rules.BindCode(canonical, func(at int) (uint32, error) { procedureReads++; return code.Logical().Read32(at) }); err != nil {
		t.Fatal(err)
	}
	bank, err := DecodeNativeSpriteBitmapBank(bundle, 0)
	if err != nil {
		t.Fatal(err)
	}
	retainedThresholds := false
	mutatedCases := 0
	for _, f := range corpus.Cases {
		if f.Thresholds != nil {
			mutatedCases++
			if !retainedThresholds {
				// The supplied default words equal the source's stack adds.
				// This separately recorded retained input proves nonidentity
				// rebasing without inventing a default configuration change.
				copy(code.Bytes[0x26956:0x33194], bundle.Executable.Hunks[0].Data[0x26956:0x33194])
				for i, v := range f.Thresholds {
					if err := canonical.Write16(0x1a590+i*2, v); err != nil {
						t.Fatal(err)
					}
				}
				if err := RebaseNativeStartupAnimations(NativeStartupResetFrameCallbacks{Code: canonical, CodeBase: code.CodeBase, Frame: &initial}, &addresses); err != nil {
					t.Fatal(err)
				}
				changed := 0
				for at := 0x26956; at < 0x33194; at += 6 {
					value, err := canonical.Read16(at + 2)
					if err != nil {
						t.Fatal(err)
					}
					old := uint16(bundle.Executable.Hunks[0].Data[at+2])<<8 | uint16(bundle.Executable.Hunks[0].Data[at+3])
					if value != old {
						changed++
					}
				}
				if changed == 0 {
					t.Fatal("explicit retained thresholds made no original animation changes")
				}
				retainedThresholds = true
			}
		} else if retainedThresholds {
			t.Fatal("fixture threshold groups lost their source ownership")
		}
		if f.Error != "" {
			t.Fatal("original rebased draw failed", f.Input.Name, f.Error)
		}
		state := rules.NewImageState()
		if f.Input.WrapCounters {
			for i := range state.AudioBank {
				state.AudioBank[i] = 0xff
			}
		}
		want := state
		for _, p := range f.AudioChanges {
			if p.Width != 1 || p.Address < 0 || p.Address >= len(want.AudioBank) {
				t.Fatal("rebased queue delta unavailable")
			}
			want.AudioBank[p.Address] = byte(p.Value)
		}
		registers := f.Input.Registers
		bitmap := make([]byte, 32000)
		for i := range bitmap {
			bitmap[i] = byte(i*7 + 13)
		}
		var sprites []NativePresentationSprite
		if f.Input.Mode == "preview" {
			m := &scenarioScriptMemory{}
			_ = m.write16(0xf0e, f.Input.Edit)
			_ = m.write16(0xf10, f.Input.Tool)
			_ = m.write16(0x138, uint16(f.Input.X))
			_ = m.write16(0x13a, uint16(f.Input.Y))
			sprites, err = rules.Preview(m.callbacks(), &state, &registers)
			for _, s := range sprites {
				if err == nil {
					err = bank.Paint(s, bitmap)
				}
			}
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
				layers, err = rules.DrawImage(&state, &registers)
				if err != nil {
					break
				}
				for _, s := range layers {
					if err = bank.Paint(s, bitmap); err != nil {
						break
					}
				}
				if err != nil {
					break
				}
				sprites = append(sprites, layers...)
			}
		}
		if err != nil {
			t.Fatal(f.Input.Name, err)
		}
		if registers != f.Registers || state.LastY != f.LastY || state.AudioBank != want.AudioBank {
			t.Fatalf("actual rebased image register/CODE output differs for%s", f.Input.Name)
		}
		if fileFrameHash(bitmap) != f.BitmapHash {
			t.Fatalf("actual rebased DMA pixels differ for%s", f.Input.Name)
		}
		if len(sprites) != len(f.Sprites) {
			t.Fatal("rebased sprite count differs", f.Input.Name)
		}
		for i, s := range sprites {
			w := f.Sprites[i]
			if s.Routine != w.PC || s.X != w.X || s.Y != w.Y || uint16(s.Height) != w.Height {
				t.Fatal("rebased source sprite request differs", f.Input.Name)
			}
		}
	}
	if procedureReads == 0 {
		t.Fatal("live image never interpreted an actual physical descriptor procedure")
	}
	if mutatedCases != 96 {
		t.Fatal("retained-threshold source cases incomplete")
	}
}

func TestNativeLiveRenderBackingIsBorrowedAndErrorsPropagate(t *testing.T) {
	bundle := testBundle(t)
	code, _ := nativeSharedCodeTestView(t)
	rules, err := DecodeNativeRenderFrameRules(bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	canonical := code.Physical()
	if err := rules.BindCode(canonical, code.Logical().Read32); err != nil {
		t.Fatal(err)
	}
	// Byte/WORD/mask-LONG data reads stay canonical even at an original
	// relocation operand. Only the descriptor procedure uses subtraction.
	const procedure = 0x21632 + 8
	physical, err := canonical.Read32(procedure)
	if err != nil {
		t.Fatal(err)
	}
	got, err := rules.long(procedure)
	if err != nil || got != physical || physical < code.CodeBase {
		t.Fatal("canonical data LONG was unlinked", got, physical, err)
	}
	logical, err := rules.procedure(procedure)
	if err != nil || logical != physical-code.CodeBase {
		t.Fatal("descriptor procedure used physical numeric label", err)
	}
	if err := canonical.Write16(0x20ac0, 0x7abc); err != nil {
		t.Fatal(err)
	}
	if got, err := rules.Images.word(0x20ac0); err != nil || got != 0x7abc {
		t.Fatal("bound image table detached from render backing", err)
	}
	want := errors.New("actual live CODE byte unavailable")
	failed := canonical
	failed.Read8 = func(int) (uint8, error) { return 0, want }
	if err := rules.BindCode(failed, code.Logical().Read32); err != nil {
		t.Fatal(err)
	}
	if _, err := rules.byte(0x33512); !errors.Is(err, want) {
		t.Fatal("live byte error replaced by immutable fallback", err)
	}
	if _, err := rules.Images.byte(0x26956); !errors.Is(err, want) {
		t.Fatal("live image byte error swallowed", err)
	}
	if err := rules.BindCode(canonical, func(int) (uint32, error) { return 0, want }); err != nil {
		t.Fatal(err)
	}
	cb := NativeRenderFrameCallbacks{Frame: &NativeFrameRegisterContext{}}
	if err := rules.descriptor(0x21632, cb, &NativeRenderFramePlan{}); !errors.Is(err, want) {
		t.Fatal("procedure failure swallowed", err)
	}
	if err := rules.BindCode(canonical, nil); err == nil {
		t.Fatal("physical procedures accepted without logical reader")
	}
}
