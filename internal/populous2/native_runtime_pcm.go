package populous2

import "fmt"

// CreatePCM creates a stream controlled by this runtime's same outer owner.
// Invoke inside Access.Execute. The returned Read and external controls lock
// that owner; synchronous native children use WithinExecuteCallbacks instead.
func (h *NativeRuntimeHost) CreatePCM(device *NativeAudioDevice, timing NativeAudioTiming, dma NativePaulaDMAConfig) (*NativeRuntimePCMAccess, error) {
	if h == nil || h.Code == nil || device == nil || len(device.Code) == 0 || len(h.Code.RawData()) == 0 || &device.Code[0] != &h.Code.RawData()[0] {
		return nil, fmt.Errorf("native runtime PCM requires its actual shared device")
	}
	pcm, err := NewNativeAudioPCMWithDMA(device, timing, dma)
	if err != nil {
		return nil, err
	}
	return h.Access.BindPCM(pcm)
}
