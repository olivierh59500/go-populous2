package populous2

import "fmt"

// NativeGameplayTerrainChild supplies the source's small physical terrain
// readers. The D2B4 MOVEM restores A0/A1; D91A deliberately leaves its deity
// A0 pointer. Other input children are not acknowledged here.
func NativeGameplayTerrainChild(r *NativeRenderFrameRules, cb NativeStartupResetFrameCallbacks, call NativeStartupResetFrameCall) (NativeCommandFrameResult, error) {
	if r == nil || call.Frame == nil || call.A == nil || !winMemoryValid(cb.Memory) {
		return NativeCommandFrameResult{}, fmt.Errorf("native gameplay terrain child context missing")
	}
	var err error
	switch call.Routine {
	case 0xd2b4:
		err = r.TerrainHeight(cb.Memory, call.Frame)
	case 0xd91a:
		free, e := cb.Memory.Read16(0xf0e)
		if e != nil {
			return NativeCommandFrameResult{}, e
		}
		if free == 0 {
			owner := uint16(call.Frame.D[3])
			product := uint32(owner) * 314
			call.A[0] = NativeRequesterAddress{Address: uint32(int64(call.Frame.AddressBase+0xe76a) + int64(int16(product)))}
		}
		err = r.TerrainAdmission(cb.Memory, call.Frame)
	case 0x2914:
		selected, e := cb.Memory.Read32(0xf36)
		if e != nil {
			return NativeCommandFrameResult{}, e
		}
		if selected != 0 {
			call.A[1] = NativeRequesterAddress{Address: selected}
		}
		err = r.SelectedHit(NativeRenderFrameCallbacks{Memory: cb.Memory, Frame: call.Frame})
	default:
		return NativeCommandFrameResult{}, fmt.Errorf("native gameplay terrain routine%x unavailable", call.Routine)
	}
	return NativeCommandFrameResult{Complete: err == nil}, err
}
