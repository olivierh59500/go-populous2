package game

import (
	"fmt"
	"go-populous2/internal/populous2"
)

// SetSaveRoot binds native DOS names to an explicitly configured existing
// directory. It does not create/export a game or silently overwrite a file.
func (g *NativeGame) SetSaveRoot(root string) error {
	if g == nil || g.Host == nil {
		return fmt.Errorf("native game file owner missing")
	}
	store, err := populous2.NewNativeRuntimeFileStore(root, []string{"SAVES", "RAM", "DF0", "DF1"}, true)
	if err != nil {
		return err
	}
	g.Files = store
	return nil
}

func (g *NativeGame) fileChild(call populous2.NativeStartupResetFrameCall, phase *uint32) (populous2.NativeCommandFrameResult, error) {
	if g.Files == nil {
		return populous2.NativeCommandFrameResult{}, fmt.Errorf("native file requester requires an explicit -save-root directory")
	}
	if g.FileBrowser == nil {
		rules, err := populous2.DecodeNativeRuntimeFileBrowserRules(g.Host.Bundle.Executable)
		if err != nil {
			return populous2.NativeCommandFrameResult{}, err
		}
		g.FileRules = rules
		g.FileBrowser = &populous2.NativeRuntimeFileBrowserState{}
	}
	cb := populous2.NativeRuntimeFileBrowserCallbacks{Ownership: g.Startup.Ownership, NativeFileFrameCallbacks: populous2.NativeFileFrameCallbacks{Sound: func(cue uint16, c *populous2.NativeFrameRegisterContext) error { return g.Operations.DirectCue(cue, c) }}}
	return g.FileBrowser.AdvanceChild(g.Host, g.Files, &g.FileRules, call, phase, cb)
}

func (g *NativeGame) initialChild(call populous2.NativeStartupResetFrameCall, phase *uint32) (populous2.NativeCommandFrameResult, error) {
	if call.Routine == 0x3f92 {
		return g.fileChild(call, phase)
	}
	if call.Routine == 0xb740 {
		return g.DeityEditor.AdvanceChild(g.Host, call, phase, g.Startup.Campaign)
	}
	return populous2.NativeCommandFrameResult{}, fmt.Errorf("native initial UI body%x is not yet bound", call.Routine)
}

func (g *NativeGame) commandFileChild(call populous2.NativeCommandFrameCall, phase *uint32) (populous2.NativeCommandFrameResult, error) {
	if call.Routine != 0x3f92 {
		return populous2.NativeCommandFrameResult{}, fmt.Errorf("native command UI body%x is not yet bound", call.Routine)
	}
	frame := populous2.NativeFrameRegisterContext{D: call.Context.D, AddressBase: g.Host.Memory.BSSBase}
	var a [7]populous2.NativeRequesterAddress
	result, err := g.fileChild(populous2.NativeStartupResetFrameCall{Routine: call.Routine, Frame: &frame, A: &a}, phase)
	call.Context.D = frame.D
	return result, err
}
