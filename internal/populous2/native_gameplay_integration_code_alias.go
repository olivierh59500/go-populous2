package populous2

import "fmt"

// NativeFollowerCodeAlias exposes the original follower cache words and
// evaluator scratch through the same physical CODE/RAM view used by other
// native children. Pass owns124A0/13350; Town owns13550 and the50-word scratch.
// Town's duplicate outer/minimap fields are the explicit call/return bridge
// already performed by nativeTownFrameBody, not separate CODE storage.
type NativeFollowerCodeAlias struct {
	State     *NativeFollowerFrameState
	CodeBase  uint32
	RAM, Code FollowerCleanupMemory
	backing   FollowerCleanupMemory
}

// NewNativeFollowerCodeAlias imports the actual current backing once. Source
// writes thereafter update the same owners; no frame-phase copy or reset is
// inferred, and pending callbacks retain the original state object.
func NewNativeFollowerCodeAlias(state *NativeFollowerFrameState, physical FollowerCleanupMemory, codeBase uint32) (*NativeFollowerCodeAlias, error) {
	if state == nil || !winMemoryValid(physical) || uint64(codeBase)+0x1374c > 0x100000000 {
		return nil, fmt.Errorf("native follower CODE alias backing missing")
	}
	a := &NativeFollowerCodeAlias{State: state, CodeBase: codeBase, backing: physical}
	for _, at := range []int{0x124a0, 0x13350, 0x13550} {
		value, err := physical.Read16(int(codeBase) + at)
		if err != nil {
			return nil, err
		}
		*a.word(at) = value
	}
	for i := range state.Town.Scratch136E8 {
		value, err := physical.Read16(int(codeBase) + 0x136e8 + i*2)
		if err != nil {
			return nil, err
		}
		state.Town.Scratch136E8[i] = value
	}
	state.Town.OuterFlag13350 = state.Pass.TownCacheFlag
	state.Town.MinimapVariant = state.Pass.MinimapVariant
	a.RAM = nativeByteAddressMemory(a.readByte, a.writeByte)
	a.Code = nativeOffsetMemory(a.RAM, codeBase)
	return a, nil
}

func (a *NativeFollowerCodeAlias) word(offset int) *uint16 {
	switch offset &^ 1 {
	case 0x124a0:
		return &a.State.Pass.MinimapVariant
	case 0x13350:
		return &a.State.Pass.TownCacheFlag
	case 0x13550:
		return &a.State.Town.Property13550
	}
	if offset >= 0x136e8 && offset < 0x1374c {
		return &a.State.Town.Scratch136E8[(offset-0x136e8)/2]
	}
	return nil
}
func (a *NativeFollowerCodeAlias) readByte(at int) (uint8, error) {
	offset := int(int64(at) - int64(a.CodeBase))
	if value := a.word(offset); value != nil {
		return uint8(*value >> uint(8-(offset&1)*8)), nil
	}
	return a.backing.Read8(at)
}
func (a *NativeFollowerCodeAlias) writeByte(at int, value uint8) error {
	if err := a.backing.Write8(at, value); err != nil {
		return err
	}
	offset := int(int64(at) - int64(a.CodeBase))
	if word := a.word(offset); word != nil {
		shift := uint(8 - (offset&1)*8)
		*word = (*word &^ (uint16(255) << shift)) | uint16(value)<<shift
		if offset&^1 == 0x124a0 {
			a.State.Town.MinimapVariant = *word
		}
		if offset&^1 == 0x13350 {
			a.State.Town.OuterFlag13350 = *word
		}
	}
	return nil
}
