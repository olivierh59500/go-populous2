package populous2

import "fmt"

// retainNativeLAND installs the same556-byte resource as the original loader
// at CODE$3365a. A standalone World owns its copy; the native host retains
// its actual shared physical allocation. Neither changes immutable Bundle
// bytes or another world's palette/minimap/economy data.
func (w *World) retainNativeLAND(data []byte) error {
	const start = 0x3365a
	if w == nil || len(data) != LandDataSize || len(w.NativeAI.Code) < start+LandDataSize {
		return fmt.Errorf("native loaded LAND resource/backing missing")
	}
	if w.nativeSharedCode != nil {
		physical := w.nativeSharedCode.RawData()
		if len(physical) != len(w.NativeAI.Code) || &physical[0] != &w.NativeAI.Code[0] {
			return fmt.Errorf("native loaded LAND lost its shared CODE owner")
		}
		copy(physical[start:start+LandDataSize], data)
		return nil
	}
	code := append([]byte(nil), w.NativeAI.Code...)
	copy(code[start:start+LandDataSize], data)
	w.NativeAI.Code = code
	return nil
}
