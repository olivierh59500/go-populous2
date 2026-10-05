package populous2

import "fmt"

// LogicalCode preserves the live callback owners of scalar CODE variables
// while interpreting executable relocation operands in linked-offset form.
// Relocation positions come from the source linker table; no patch list or
// per-call CODE array is assembled.
func (h *NativeRuntimeHost) LogicalCode() (FollowerCleanupMemory, error) {
	if h == nil || h.Code == nil || h.Memory == nil {
		return FollowerCleanupMemory{}, fmt.Errorf("native runtime logical CODE owners missing")
	}
	linked := h.Code.Logical()
	return nativeByteAddressMemory(func(at int) (uint8, error) {
		if _, relocated := h.Code.relocation(at); relocated {
			return linked.Read8(at)
		}
		return h.Memory.Code.Read8(at)
	}, func(at int, value uint8) error {
		if _, relocated := h.Code.relocation(at); relocated {
			return linked.Write8(at, value)
		}
		return h.Memory.Code.Write8(at, value)
	}), nil
}
