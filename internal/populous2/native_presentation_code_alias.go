package populous2

import "fmt"

// NativePresentationCodeAlias exposes live presentation-owned CODE data through
// physical and HUNK-relative memory callbacks. Host Span/RawData is not a live
// view of these fields: the callbacks are the authoritative read/write boundary.
// Mouse CODE remains owned by the caller's lower memory layer. COP1LC is a
// hardware register, not an additional CODE variable.
type NativePresentationCodeAlias struct {
	Presentation  *NativeFramePresentationState
	CodeBase      uint32
	RAM, Code     FollowerCleanupMemory
	backing       FollowerCleanupMemory
	interruptWord uint16
}

// NewNativePresentationCodeAlias retains the actual WORD at CODE $3ea, including
// noncanonical nonzero values. The existing InterruptChain bool is only the
// source TST.W nonzero predicate. Other values already belong to presentation;
// construction does not reset initialized Copper or clock state from old RAM.
func NewNativePresentationCodeAlias(p *NativeFramePresentationState, physical FollowerCleanupMemory, codeBase uint32) (*NativePresentationCodeAlias, error) {
	if p == nil || !winMemoryValid(physical) || uint64(codeBase)+0x11180 > 0x100000000 {
		return nil, fmt.Errorf("native presentation CODE alias backing missing")
	}
	word, err := physical.Read16(int(codeBase) + 0x3ea)
	if err != nil {
		return nil, err
	}
	a := &NativePresentationCodeAlias{Presentation: p, CodeBase: codeBase, backing: physical, interruptWord: word}
	p.InterruptChain = word != 0
	a.RAM = nativeByteAddressMemory(a.readByte, a.writeByte)
	a.Code = nativeOffsetMemory(a.RAM, codeBase)
	return a, nil
}

// InterruptWord returns the complete retained source WORD, rather than encoding
// its predicate as 0/1. Change it through the Code/RAM writes, not the bool.
func (a *NativePresentationCodeAlias) InterruptWord() uint16 { return a.interruptWord }

func (a *NativePresentationCodeAlias) field(at int) (start, width int, value uint32, ok bool) {
	offset := int64(at) - int64(a.CodeBase)
	for _, f := range []struct {
		start, width int
		value        uint32
	}{
		{0x3ea, 2, uint32(a.interruptWord)},
		{0x77a, 4, a.Presentation.CopperSelector},
		{0x77e, 4, a.Presentation.SpritePatchPointer},
		{0x1117c, 4, a.Presentation.Deadline1117C},
	} {
		if offset >= int64(f.start) && offset < int64(f.start+f.width) {
			return f.start, f.width, f.value, true
		}
	}
	return 0, 0, 0, false
}

func (a *NativePresentationCodeAlias) readByte(at int) (uint8, error) {
	if start, width, value, ok := a.field(at); ok {
		shift := uint((width - 1 - (at - int(a.CodeBase) - start)) * 8)
		return uint8(value >> shift), nil
	}
	return a.backing.Read8(at)
}

func (a *NativePresentationCodeAlias) writeByte(at int, v uint8) error {
	// Preserve the original write failure before changing the typed owner.
	if err := a.backing.Write8(at, v); err != nil {
		return err
	}
	if start, width, value, ok := a.field(at); ok {
		shift := uint((width - 1 - (at - int(a.CodeBase) - start)) * 8)
		value = value & ^(uint32(255)<<shift) | uint32(v)<<shift
		switch start {
		case 0x3ea:
			a.interruptWord = uint16(value)
			a.Presentation.InterruptChain = a.interruptWord != 0
		case 0x77a:
			a.Presentation.CopperSelector = value
		case 0x77e:
			a.Presentation.SpritePatchPointer = value
		case 0x1117c:
			a.Presentation.Deadline1117C = value
		}
	}
	return nil
}
