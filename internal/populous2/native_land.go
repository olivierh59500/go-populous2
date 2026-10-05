package populous2

import "fmt"

// retainNativeLAND installs the same556-byte resource as the original loader
// at CODE$3365a. World owns its copy; another world's palette/minimap/economy
// bytes and the immutable Bundle must not change when this bank is loaded.
func (w *World) retainNativeLAND(data []byte) error {
	const start = 0x3365a
	if w == nil || len(data) != LandDataSize || len(w.NativeAI.Code) < start+LandDataSize {
		return fmt.Errorf("native loaded LAND resource/backing missing")
	}
	code := append([]byte(nil), w.NativeAI.Code...)
	copy(code[start:start+LandDataSize], data)
	w.NativeAI.Code = code
	return nil
}
