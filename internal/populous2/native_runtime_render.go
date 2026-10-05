package populous2

import "fmt"

// MainRenderBindings prepares immutable art once for the current landscape
// and binds mutable CODE/windows to the real runtime. Actual editor, town
// information and ownership callbacks remain caller-supplied operations.
func (h *NativeRuntimeHost) MainRenderBindings(supplied NativeSessionRenderBindings) (NativeSessionRenderBindings, error) {
	if h == nil || h.Bundle == nil || h.World == nil || h.Memory == nil {
		return supplied, fmt.Errorf("native runtime render owners missing")
	}
	if supplied.Rules == nil {
		rules, err := DecodeNativeActorRenderRules(h.Bundle.Executable)
		if err != nil {
			return supplied, err
		}
		supplied.Rules = &rules
	}
	land := int(h.World.Level.Terrain)
	if supplied.Sprites == nil {
		bank, err := DecodeNativeSpriteBitmapBank(h.Bundle, land)
		if err != nil {
			return supplied, err
		}
		supplied.Sprites = bank
	}
	if supplied.Tiles == nil {
		bank, err := DecodeNativeTileBitmapBank(h.Bundle.Raw[fmt.Sprintf("block%d.pak", land)])
		if err != nil {
			return supplied, err
		}
		supplied.Tiles = bank
	}
	supplied.Code = h.Memory.Code
	supplied.Window = h.Host.BitmapWindow
	return supplied, nil
}
