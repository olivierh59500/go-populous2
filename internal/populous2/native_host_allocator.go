package populous2

import "fmt"

// NativeHostAllocator assigns addresses inside an explicit caller-supplied
// host range. These are configured Go addresses, not claimed AmigaOS returns.
// Every successful allocation becomes real mapped RAM of the requested size.
type NativeHostAllocator struct {
	Memory      *NativeHostMemory
	Next, Limit uint32
	owned       map[uint32]int
}

func NewNativeHostAllocator(memory *NativeHostMemory, start, limit uint32) (*NativeHostAllocator, error) {
	if memory == nil || start == 0 || start&1 != 0 || start >= limit || limit > 0x80000000 {
		return nil, fmt.Errorf("native host allocation range invalid")
	}
	for _, region := range memory.Regions {
		if uint64(start) < uint64(region.Base)+uint64(len(region.Bytes)) && region.Base < limit {
			return nil, fmt.Errorf("native host allocation range overlaps existing RAM")
		}
	}
	return &NativeHostAllocator{Memory: memory, Next: start, Limit: limit, owned: make(map[uint32]int)}, nil
}

func (a *NativeHostAllocator) Allocate(call NativeStartupAllocationCall, _ *uint32) (NativeStartupAllocationResult, error) {
	out := NativeStartupAllocationResult{Complete: true}
	if a == nil || a.Memory == nil || a.owned == nil {
		return out, fmt.Errorf("native host allocator missing")
	}
	if call.Free {
		length, exists := a.owned[call.Address]
		if !exists || uint32(length) != call.Size {
			return out, fmt.Errorf("native host free does not match an owned allocation")
		}
		if err := a.Memory.ReleaseRegion(call.Address, length); err != nil {
			return out, err
		}
		delete(a.owned, call.Address)
		return out, nil
	}
	if call.Flags != 2 || call.Size == 0 || call.Size > MaxDecodedBytes {
		return out, fmt.Errorf("native host allocation request unsupported")
	}
	end := uint64(a.Next) + uint64(call.Size)
	if end > uint64(a.Limit) {
		return out, nil
	}
	address := a.Next
	if err := a.Memory.MapRegion(NativeHostRegion{Name: "startup chip allocation", Base: address, Bytes: make([]byte, int(call.Size))}); err != nil {
		return out, err
	}
	a.owned[address] = int(call.Size)
	a.Next = uint32((end + 1) &^ 1)
	out.Value = address
	return out, nil
}
