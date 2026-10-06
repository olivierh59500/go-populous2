package populous2

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func nativeRuntimeDisabledRenderSound(t *testing.T, h *NativeRuntimeHost) func(uint16, *NativeFrameRegisterContext) error {
	t.Helper()
	r, err := DecodeNativeAudioControlFrameRules(h.Bundle.Executable)
	if err != nil {
		t.Fatal(err)
	}
	return func(offset uint16, c *NativeFrameRegisterContext) error {
		_, err := r.Run(0x184f6, NativeAudioControlFrameCallbacks{Memory: h.Memory.BSS, Frame: c, CodeBase: h.Memory.CodeBase})
		return err
	}
}

func TestNativeRuntimeProtectionLoadsAndBorrowsPhysicalFaces(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	if _, err := h.InitializePresentation(NativeMouseSample{}); err != nil {
		t.Fatal(err)
	}
	if err := h.Memory.BSS.Write16(0x3b0, 1); err != nil {
		t.Fatal(err)
	}
	beamReads, ownership := 0, 0
	children, err := h.NewRenderChildren(NativeRuntimeRenderChildrenCallbacks{
		Beam:      func() (uint16, error) { beamReads++; return 3, nil },
		Ownership: func(bool, *NativeFrameRegisterContext) error { ownership++; return nil },
		Sound:     nativeRuntimeDisabledRenderSound(t, h),
	})
	if err != nil {
		t.Fatal(err)
	}
	frame := NativeFrameRegisterContext{AddressBase: h.Memory.BSSBase, D: [8]uint32{1, 2, 3, 4, 5, 6, 7, 8}}
	done, err := children.AdvanceProtection(0x76f4, &frame)
	if err != nil || done || children.ProtectionStep.PC != 0x31ce {
		t.Fatal("fresh source resource/protection did not reach the real wait", done, err, children.ProtectionStep.PC)
	}
	flags, err := h.Memory.BSS.Read32(0x3ac)
	if err != nil || flags&(1<<8) == 0 {
		t.Fatal("real FACES loader did not set its cache flag", err)
	}
	readBytes, err := h.Memory.Code.Read32(0x19e46)
	if err != nil || readBytes == 0 {
		t.Fatal("actual encoded read count was replaced by acknowledgment", err)
	}
	address, err := h.Memory.Code.Read32(0x212ba)
	if err != nil {
		t.Fatal(err)
	}
	planes, err := h.Host.Span(address, len(children.ProtectionRules.Faces[0].Planes))
	if err != nil || &planes[0] != &children.ProtectionRules.Faces[0].Planes[0] {
		t.Fatal("protection faces detached from genuine prepared physical RAM", err)
	}
	if len(h.Files.handles) != 0 || beamReads != 1 || ownership != 1 {
		t.Fatal("source resource handles or initial child ordering differ")
	}
	if _, err := children.AdvanceProtection(0x7728, &frame); err == nil {
		t.Fatal("suspended actor caller silently changed")
	}
	if _, err := children.AdvanceProtection(0x76f4, &frame); err != nil {
		t.Fatal(err)
	}
	if beamReads != 1 {
		t.Fatal("pending protection repeated its source selection/prefix")
	}
	bindings, err := children.Bind(NativeSessionRenderBindings{})
	if err != nil || bindings.PaintingAdvance == nil || bindings.Children.TownInfoAdvance == nil || bindings.SelectedOwnership == nil {
		t.Fatal("actual retained children were not exposed", err)
	}
}

func TestNativeRuntimeRenderChildrenDoNotInventMissingHostOperations(t *testing.T) {
	h := nativeRuntimeHostTest(t)
	children, err := h.NewRenderChildren(NativeRuntimeRenderChildrenCallbacks{})
	if err != nil {
		t.Fatal(err)
	}
	frame := NativeFrameRegisterContext{AddressBase: h.Memory.BSSBase}
	if _, err := children.AdvanceProtection(0x76f4, &frame); err == nil || children.Protection.Started {
		t.Fatal("missing Beam/ownership/sound was acknowledged")
	}
	if _, err := children.AdvanceEditor(&frame); err == nil || children.Editor.Started {
		t.Fatal("missing editor sound was acknowledged")
	}
	want := errors.New("actual beam backend unavailable")
	children.Callbacks = NativeRuntimeRenderChildrenCallbacks{Beam: func() (uint16, error) { return 0, want }, Ownership: func(bool, *NativeFrameRegisterContext) error { return nil }, Sound: nativeRuntimeDisabledRenderSound(t, h)}
	if _, err := children.AdvanceProtection(0x76f4, &frame); !errors.Is(err, want) {
		t.Fatal("real host failure was replaced", err)
	}
}
func TestNativeRuntimeEditorChildAgainstOriginalCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/render_frame_editor_modal_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []editorModalFixture }
	if e := json.Unmarshal(data, &catalog); e != nil || len(catalog.Cases) != 20 {
		t.Fatalf("native editor modal corpus incomplete: %v", e)
	}
	bundle := testBundle(t)

	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			h := nativeRuntimeHostTest(t)
			p := h.Session.Presentation
			var e error
			for i := 0x408; i < len(p.Chip); i++ {
				p.Chip[i] = byte(i*17 + 3 + ((i-0x408)/32000)*91)
			}
			p.InterruptChain = false
			if _, e := p.Initialize(bundle.Executable, NativeMouseSample{}); e != nil {
				t.Fatal(e)
			}
			m := h.Memory.BSS
			for _, v := range []nativeHeroPatch{{0xeb42, 2, 1}, {0xeb44, 2, 8}, {0xeb6a, 4, 0x200000 + 0xeb56}, {0xdde, 2, 123}, {0xde0, 2, 46}, {0xde2, 1, 20}, {0xde3, 1, 30}, {0x140, 2, 1}, {0x134, 2, uint32(f.Input.X)}, {0x136, 2, uint32(f.Input.Y)}} {
				renderFramePatch(m, v)
			}
			code := h.Code.RawData()
			cm := h.Memory.Code
			_ = cm.Write16(0x3ea, 0)
			frame := NativeFrameRegisterContext{D: f.Input.D, AddressBase: 0x200000}
			children, e := h.NewRenderChildren(NativeRuntimeRenderChildrenCallbacks{Sound: nativeRuntimeDisabledRenderSound(t, h)})
			if e != nil {
				t.Fatal(e)
			}

			check := func(i int, step NativeEditorFrameStep) {
				t.Helper()
				want := f.Frames[i]
				pc := step.PC
				if step.Complete {
					pc = 0
				}
				if step.Waiting && step.ChildRoutine == 0 {
					pc = 0x786
				}
				if pc != want.PC || frame.D != want.D {
					t.Errorf("native completeeditor registers/PC differ at%d: PC%x/%x D%x/%x", i, pc, want.PC, frame.D, want.D)
				}
				if fileFrameHash(fileFrameMemoryBytes(t, m)) != want.BSSHash || fileFrameHash(p.Chip) != want.ChipHash || fileFrameHash(p.PointerData) != want.PointerHash {
					t.Errorf("native completeeditor BSS/chip/pointer differs at%d", i)
				}
				if !bytes.Equal(code[0x37bc:0x381e], want.Fields) || !bytes.Equal(code[0xab4e:0xab4e+2048], want.Scratch) {
					t.Errorf("native completeeditor fields/scratch differ at%d", i)
				}
				if p.CopperSelector != want.Selector || p.SpritePatchPointer != want.Patch || p.ActiveCopper != want.Copper {
					t.Errorf("native completeeditor Copper state differs at%d", i)
				}
			}
			_, e = children.AdvanceEditor(&frame)
			step := children.EditorStep
			if e != nil {
				t.Fatal(e)
			}
			check(0, step)
			_, e = children.AdvanceEditor(&frame)
			step = children.EditorStep
			if e != nil {
				t.Fatal(e)
			}
			check(1, step)
			for i, ev := range f.Input.Events {
				if ev.VBlank {
					if _, e := p.VBlank(NativeMouseSample{}, m, &frame); e != nil {
						t.Fatal(e)
					}
				}
				for _, wire := range ev.Keys {
					if e := p.Input.KeyboardInterrupt(wire); e != nil {
						t.Fatal(e)
					}
				}
				_, e = children.AdvanceEditor(&frame)
				step = children.EditorStep
				if e != nil {
					t.Fatal(e)
				}
				check(i+2, step)
			}
			if !children.Editor.Complete || f.Frames[len(f.Frames)-1].PC != 0 {
				t.Fatal("native numeric editor did notcomplete")
			}
		})
	}
}
