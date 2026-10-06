package populous2

import (
	"fmt"
	"go-populous2/internal/amiga"
)

type NativeRuntimeFileBrowserRules struct {
	File     NativeFileFrameRules
	Children NativeCampaignSelectionChildrenRules
	Panel    NativeStartupPanelFrameRules
}

func DecodeNativeRuntimeFileBrowserRules(exe *amiga.Executable) (NativeRuntimeFileBrowserRules, error) {
	var r NativeRuntimeFileBrowserRules
	var e error
	r.File, e = DecodeNativeFileFrameRules(exe)
	if e == nil {
		r.Children, e = DecodeNativeCampaignSelectionChildrenRules(exe)
	}
	if e == nil {
		r.Panel, e = DecodeNativeStartupPanelFrameRules(exe)
	}
	return r, e
}

type NativeRuntimeFileBrowserCallbacks struct {
	NativeFileFrameCallbacks
	Ownership func(bool, *NativeFrameRegisterContext, *[7]NativeRequesterAddress) error
	Child     func(NativeStartupResetFrameCall, *uint32) (NativeCommandFrameResult, error)
}

// NativeRuntimeFileBrowserState composes the actual $3f92 caller with genuine
// palette, DOS, minimap, panel and redraw bodies. Input and source VBlank gates
// remain retained waits. The caller refreshes loaded caches only after the
// browser returns and the surrounding native frame releases raw ownership.
type NativeRuntimeFileBrowserState struct {
	Browser  *NativeRuntimeFileBrowserFrameState
	Files    NativeRuntimeFilesState
	Children NativeCampaignSelectionChildren
}

func (s *NativeRuntimeFileBrowserState) AdvanceChild(h *NativeRuntimeHost, store *NativeRuntimeFileStore, r *NativeRuntimeFileBrowserRules, call NativeStartupResetFrameCall, phase *uint32, supplied NativeRuntimeFileBrowserCallbacks) (NativeCommandFrameResult, error) {
	if s == nil || h == nil || h.Memory == nil || h.Session == nil || r == nil || call.Frame == nil || call.A == nil || phase == nil || call.Frame.AddressBase != h.Memory.BSSBase || call.Routine != 0x3f92 {
		return NativeCommandFrameResult{}, fmt.Errorf("native file browser host/caller missing")
	}
	if *phase == 0 {
		s.Browser = &NativeRuntimeFileBrowserFrameState{NativeFileFrameState: NativeFileFrameState{A: *call.A}}
		*phase = 1
	} else if s.Browser == nil {
		return NativeCommandFrameResult{}, fmt.Errorf("native file browser continuation missing")
	}
	base := supplied.NativeFileFrameCallbacks
	base.Frame, base.Code, base.Memory, base.CodeBase = call.Frame, h.Memory.Code, h.Memory.BSS, h.Memory.CodeBase
	base.Presentation, base.Bitmap = h.Session.Presentation, h.Bitmap
	base.ReadAbsolute = func(address uint32) (byte, error) { return h.Memory.RAM.Read8(int(address)) }
	campaign := NativeCampaignSelectionChildrenCallbacks{NativeCampaignHelpFrameCallbacks: NativeCampaignHelpFrameCallbacks{NativeCampaignFrameCallbacks: NativeCampaignFrameCallbacks{NativeFileFrameCallbacks: base, RAM: h.Memory.RAM}}}
	cb := NativeRuntimeFileBrowserFrameCallbacks{NativeFileFrameCallbacks: base}
	cb.Child = func(inner NativeStartupResetFrameCall, childPhase *uint32) (NativeCommandFrameResult, error) {
		switch inner.Routine {
		case 0x19936, 0x19afc, 0x19c1c:
			return s.Files.AdvanceChild(h, store, inner, childPhase, NativeRuntimeFilesCallbacks{NativeFileFrameCallbacks: base, Child: supplied.Child})
		case 0x102e4:
			return s.Children.Call(&r.Children, campaign, inner, childPhase)
		case 0xd8cc:
			bitmap, e := h.Bitmap(inner.A[0].Address)
			if e != nil {
				return NativeCommandFrameResult{}, e
			}
			e = DrawNativeStartupMinimapFrame(NativeStartupResetFrameCallbacks{Code: base.Code, Memory: base.Memory, RAM: h.Memory.RAM, CodeBase: base.CodeBase, Frame: inner.Frame}, inner.A, bitmap)
			return NativeCommandFrameResult{Complete: e == nil}, e
		case 0x1da0:
			if supplied.Ownership == nil {
				return NativeCommandFrameResult{}, fmt.Errorf("native file panel ownership body missing")
			}
			e := h.RestoreStartupPanel(&r.Panel, inner.Frame, inner.A, supplied.Ownership)
			return NativeCommandFrameResult{Complete: e == nil}, e
		case 0xd838:
			e := nativeFileBrowserRedraw(NativeGameplayEditorInputCallbacks{NativeStartupResetFrameCallbacks: NativeStartupResetFrameCallbacks{Code: base.Code, Memory: base.Memory, RAM: h.Memory.RAM, CodeBase: base.CodeBase, Frame: inner.Frame}, Bitmap: h.Bitmap}, inner.A)
			return NativeCommandFrameResult{Complete: e == nil}, e
		default:
			if supplied.Child == nil {
				return NativeCommandFrameResult{}, fmt.Errorf("native file browser child $%x missing", inner.Routine)
			}
			return supplied.Child(inner, childPhase)
		}
	}
	step, e := s.Browser.Advance(&r.File, cb)
	*call.A = s.Browser.A
	if step.Complete {
		s.Browser = nil
	}
	// Both terminal source entries use MOVEQ 0/1; waiting is not CCR.Z.
	return NativeCommandFrameResult{Complete: step.Complete, Zero: step.Complete && call.Frame.D[0] == 0}, e
}
