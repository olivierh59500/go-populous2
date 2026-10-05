package populous2

import "fmt"

// InitializeAudio executes actual18ADA only after the source allocation/load.
// The returned device borrows canonical CODE/FX; a host streaming PCM must
// serialize its access with frame, menu and resource mutations.
func (h *NativeRuntimeHost) InitializeAudio(frame *NativeFrameRegisterContext, timerLow uint8) (*NativeAudioDevice, []NativeFrameHardwareWrite, error) {
	if h == nil || h.Memory == nil || frame == nil || frame.AddressBase != h.Memory.BSSBase {
		return nil, nil, fmt.Errorf("native runtime audio context/base missing")
	}
	resource, err := h.Memory.BSS.Read32(0x3b4)
	if err != nil {
		return nil, nil, err
	}
	if int32(resource) <= 0 {
		return nil, nil, fmt.Errorf("native runtime audio allocation has not completed")
	}
	fx, err := h.Host.Span(resource, NativeStartupAudioBytes)
	if err != nil {
		return nil, nil, err
	}
	device, err := NewNativeSharedAudioDevice(h.Bundle.Executable, fx, h.Code, resource, timerLow)
	if err != nil {
		return nil, nil, err
	}
	device.ReadAbsolute8 = func(address uint32) (uint8, error) { return h.Memory.RAM.Read8(int(address)) }
	writes, err := device.InitializeWithFrame(frame)
	if err != nil {
		return nil, writes, err
	}
	return device, writes, nil
}
