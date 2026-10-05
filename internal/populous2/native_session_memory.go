package populous2

import "fmt"

// NativeSessionMemory binds physical BSS reads/writes to the session's actual
// borrowed World and input state. Other physical regions retain their real
// host backing. The returned CODE/BSS views use offsets, not host pointers.
type NativeSessionMemory struct {
	RAM               FollowerCleanupMemory
	Code              FollowerCleanupMemory
	BSS               FollowerCleanupMemory
	Host              *NativeHostMemory
	Session           *NativeFrameSession
	World             *World
	CodeBase, BSSBase uint32
}

func NewNativeSessionMemory(host *NativeHostMemory, session *NativeFrameSession, world *World, codeBase, bssBase uint32) (*NativeSessionMemory, error) {
	if host == nil || session == nil || session.Presentation == nil || world == nil {
		return nil, fmt.Errorf("native session physical memory owner missing")
	}
	if _, err := host.Span(bssBase, 0x11280); err != nil {
		return nil, err
	}
	if _, err := host.Span(codeBase, 0x3fa2c); err != nil {
		return nil, err
	}
	chip, err := host.Span(session.Presentation.ChipBase, NativeFrameChipBytes)
	if err != nil {
		return nil, err
	}
	pointers, err := host.Span(session.Presentation.PointerBase, len(session.Presentation.PointerData))
	if err != nil {
		return nil, err
	}
	// Preserve already initialized screens/pointers while making their actual
	// HUNK allocations the shared owners for renderer and physical loader.
	copy(chip, session.Presentation.Chip)
	copy(pointers, session.Presentation.PointerData)
	session.Presentation.Chip, session.Presentation.PointerData = chip, pointers
	binding := &NativeSessionMemory{Host: host, Session: session, World: world, CodeBase: codeBase, BSSBase: bssBase}
	physical := host.Memory()
	bss := session.Presentation.Memory(world.nativeCleanupMemory())
	read := func(at int) (uint8, error) {
		if at >= int(bssBase) && at < int(bssBase)+0x11280 {
			return bss.Read8(at - int(bssBase))
		}
		if at >= int(codeBase)+0xa2a && at < int(codeBase)+0xa36 {
			offset := at - int(codeBase) - 0xa2a
			mouse := session.Presentation.Input.Mouse
			words := [6]uint16{mouse.Image, mouse.CounterX, mouse.CounterY, mouse.PositionX, mouse.PositionY, mouse.MaximumY}
			return uint8(words[offset/2] >> uint(8-(offset&1)*8)), nil
		}
		return physical.Read8(at)
	}
	write := func(at int, value uint8) error {
		if at >= int(bssBase) && at < int(bssBase)+0x11280 {
			return bss.Write8(at-int(bssBase), value)
		}
		if at >= int(codeBase)+0xa2a && at < int(codeBase)+0xa36 {
			offset := at - int(codeBase) - 0xa2a
			mouse := &session.Presentation.Input.Mouse
			words := [6]*uint16{&mouse.Image, &mouse.CounterX, &mouse.CounterY, &mouse.PositionX, &mouse.PositionY, &mouse.MaximumY}
			word := words[offset/2]
			if offset&1 == 0 {
				*word = *word&255 | uint16(value)<<8
			} else {
				*word = *word&0xff00 | uint16(value)
			}
		}
		return physical.Write8(at, value)
	}
	binding.RAM = nativeByteAddressMemory(read, write)
	binding.Code = nativeOffsetMemory(binding.RAM, codeBase)
	binding.BSS = nativeOffsetMemory(binding.RAM, bssBase)
	return binding, nil
}

// ImportBSS transfers startup's initialized physical bytes to the raw World.
// Invoke before frame Begin; it never hydrates typed actors during a wait.
func (b *NativeSessionMemory) ImportBSS() error {
	if b == nil || b.World == nil || b.Session == nil || b.World.nativeCallDepth != 0 || b.Session.world != nil {
		return fmt.Errorf("native BSS import requires idle ownership")
	}
	span, err := b.Host.Span(b.BSSBase, 0x11280)
	if err != nil {
		return err
	}
	for i, value := range span {
		if err := b.BSS.Write8(i, value); err != nil {
			return err
		}
	}
	return nil
}

// SnapshotBSS is an explicit diagnostic/export copy from the live owner.
// Physical Span must not be mistaken for the callback-backed World view.
func (b *NativeSessionMemory) SnapshotBSS() ([]byte, error) {
	if b == nil {
		return nil, fmt.Errorf("native session memory missing")
	}
	bytes := make([]byte, 0x11280)
	for i := range bytes {
		value, err := b.BSS.Read8(i)
		if err != nil {
			return nil, err
		}
		bytes[i] = value
	}
	return bytes, nil
}

func nativeOffsetMemory(memory FollowerCleanupMemory, base uint32) FollowerCleanupMemory {
	return nativeByteAddressMemory(func(at int) (uint8, error) {
		if at < 0 || uint64(base)+uint64(at) > 0xffffffff {
			return 0, fmt.Errorf("native relative address invalid")
		}
		return memory.Read8(int(base) + at)
	}, func(at int, value uint8) error {
		if at < 0 || uint64(base)+uint64(at) > 0xffffffff {
			return fmt.Errorf("native relative address invalid")
		}
		return memory.Write8(int(base)+at, value)
	})
}

func nativeByteAddressMemory(read func(int) (uint8, error), write func(int, uint8) error) FollowerCleanupMemory {
	m := FollowerCleanupMemory{Read8: read, Write8: write}
	m.Read16 = func(at int) (uint16, error) {
		if at&1 != 0 {
			return 0, fmt.Errorf("native shared word address unaligned")
		}
		high, err := m.Read8(at)
		if err != nil {
			return 0, err
		}
		low, err := m.Read8(at + 1)
		return uint16(high)<<8 | uint16(low), err
	}
	m.Write16 = func(at int, value uint16) error {
		if at&1 != 0 {
			return fmt.Errorf("native shared word address unaligned")
		}
		if err := m.Write8(at, uint8(value>>8)); err != nil {
			return err
		}
		return m.Write8(at+1, uint8(value))
	}
	m.Read32 = func(at int) (uint32, error) {
		high, err := m.Read16(at)
		if err != nil {
			return 0, err
		}
		low, err := m.Read16(at + 2)
		return uint32(high)<<16 | uint32(low), err
	}
	m.Write32 = func(at int, value uint32) error {
		if err := m.Write16(at, uint16(value>>16)); err != nil {
			return err
		}
		return m.Write16(at+2, uint16(value))
	}
	return m
}
