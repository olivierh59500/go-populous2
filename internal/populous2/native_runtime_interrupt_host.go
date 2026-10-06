package populous2

import "fmt"

// InterruptVectors runs the original install/swap operation against the
// runtime's low RAM and live BSS. Hardware status and writes belong to the
// configured machine boundary, rather than inferred values in ordinary RAM.
func (h *NativeRuntimeHost) InterruptVectors(routine int, frame *NativeFrameRegisterContext, read func(uint32) (uint16, error), write func(NativeFrameHardwareWrite) error) (NativeInterruptVectorStep, error) {
	if h == nil || h.Memory == nil || frame == nil || frame.AddressBase != h.Memory.BSSBase {
		return NativeInterruptVectorStep{}, fmt.Errorf("native interrupt runtime/context missing")
	}
	return RunNativeInterruptVectors(routine, NativeInterruptVectorCallbacks{
		RAM: h.Memory.RAM, Memory: h.Memory.BSS, CodeBase: h.Memory.CodeBase,
		Frame: frame, ReadHardware16: read, Hardware: write,
	})
}
