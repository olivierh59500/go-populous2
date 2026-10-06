package game

import (
	"fmt"
	"go-populous2/internal/populous2"
)

func (g *NativeGame) SetScreenExportRoot(root string) error {
	export, err := populous2.NewNativeRuntimeScreenExport(root, false)
	if err != nil {
		return err
	}
	g.ScreenExport = export
	return nil
}

func (g *NativeGame) inputOther(call populous2.NativeStartupResetFrameCall, phase *uint32) (populous2.NativeCommandFrameResult, error) {
	if call.Routine != 0x1a55a {
		return populous2.NativeCommandFrameResult{}, fmt.Errorf("native input child%x unavailable", call.Routine)
	}
	if g.ScreenExport == nil {
		return populous2.NativeCommandFrameResult{}, fmt.Errorf("native screen export requires an explicit -export-root directory")
	}
	return g.ScreenExport.AdvanceChild(g.Host, call, phase)
}
