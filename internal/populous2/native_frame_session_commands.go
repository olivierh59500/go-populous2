package populous2

import (
	"encoding/binary"
	"fmt"
)

func (s *NativeFrameSession) commandCodeWord(at int) (uint16, error) {
	if at == 0xa2a {
		return s.Presentation.Input.Mouse.Image, nil
	}
	if at < 0 || at&1 != 0 || at+2 > len(s.world.NativeAI.Code) {
		return 0, fmt.Errorf("native session CODE word%x unavailable", at)
	}
	return binary.BigEndian.Uint16(s.world.NativeAI.Code[at:]), nil
}

func (s *NativeFrameSession) writeCommandCodeWord(at int, value uint16) error {
	if at == 0xa2a {
		s.Presentation.Input.Mouse.Image = value
		return nil
	}
	if at < 0 || at&1 != 0 || at+2 > len(s.world.NativeAI.Code) {
		return fmt.Errorf("native session CODE word%x unavailable", at)
	}
	binary.BigEndian.PutUint16(s.world.NativeAI.Code[at:], value)
	return nil
}

// commandExecutor attaches the retained inner17500 state to the outer1744C
// scheduler. Menu/resource waits never restart the dispatcher or repeat its
// preceding seed, terrain, CODE-word or immediate-power mutations.
func (s *NativeFrameSession) commandExecutor(cb NativeFrameSessionCallbacks, bindings NativeCommandWorldBindings) func(int, *NativeCommandRegisterContext, *uint32) (bool, error) {
	return func(at int, c *NativeCommandRegisterContext, _ *uint32) (bool, error) {
		side := (at - 0xeb56) / 10
		if side < 0 || side >= 2 || at != 0xeb56+side*10 {
			return false, fmt.Errorf("native session command caller%x outside two source records", at)
		}
		immediate := s.world.nativeNormalCommandCallbacks(bindings)
		immediate.Memory = s.Presentation.Memory(s.world.nativeCleanupMemory())
		_, done, err := s.commandStates[side].ExecuteFrame(&s.commandRules, at, c, NativeCommandFrameCallbacks{
			Immediate: immediate, ReadCode16: s.commandCodeWord, WriteCode16: s.writeCommandCodeWord,
			Call: func(call NativeCommandFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
				if cb.CommandChild != nil {
					return cb.CommandChild(call, phase)
				}
				return s.AdvanceBuiltInCommandChild(call, phase)
			},
		})
		return done, err
	}
}

// AdvanceBuiltInCommandChild executes proven palette/minimap source children.
// Callers providing file/reset/resource operations can delegate these exact
// bodies here while retaining their own operation phase for other routines.
func (s *NativeFrameSession) AdvanceBuiltInCommandChild(call NativeCommandFrameCall, _ *uint32) (NativeCommandFrameResult, error) {
	if s == nil || s.world == nil || call.Context == nil {
		return NativeCommandFrameResult{}, fmt.Errorf("native command child session backing missing")
	}
	side := (call.Caller - 0xeb56) / 10
	if side < 0 || side >= 2 || call.Caller != 0xeb56+side*10 {
		return NativeCommandFrameResult{}, fmt.Errorf("native command child caller outside source records")
	}
	if call.Routine == 0xd8cc {
		bitmap, err := s.bitmapAt(call.TargetA0)
		if err != nil {
			return NativeCommandFrameResult{}, err
		}
		frame := NativeFrameRegisterContext{D: call.Context.D, AddressBase: s.Frame.AddressBase}
		memory := s.Presentation.Memory(s.world.nativeCleanupMemory())
		if s.world.nativeSharedCode != nil {
			err = DrawNativeSharedCodeMinimap(s.world.nativeSharedCode, memory, &frame, bitmap)
		} else {
			err = DrawNativeMinimapFrame(s.world.NativeAI.Code, memory, &frame, bitmap)
		}
		call.Context.D = frame.D
		return NativeCommandFrameResult{Complete: err == nil}, err
	}

	if call.Routine != 0x102e4 {
		return NativeCommandFrameResult{}, fmt.Errorf("native session command child%x continuation missing", call.Routine)
	}
	if s.commandPalettes[side] == nil {
		bank := func(address uint32) (NativeFramePaletteBank, error) {
			p := NativeFramePaletteBank{Address: address}
			if address&1 != 0 || uint64(address)+32 > uint64(len(s.world.NativeAI.Code)) {
				return p, fmt.Errorf("native command palette%x outside CODE", address)
			}
			for i := range p.Words {
				p.Words[i] = binary.BigEndian.Uint16(s.world.NativeAI.Code[int(address)+i*2:])
			}
			return p, nil
		}
		source, err := bank(call.PaletteA2)
		if err != nil {
			return NativeCommandFrameResult{}, err
		}
		target, err := bank(call.PaletteA3)
		if err != nil {
			return NativeCommandFrameResult{}, err
		}
		s.commandPalettes[side] = NewNativeFramePaletteState(source, target, 0)
	}
	frame := NativeFrameRegisterContext{D: call.Context.D, AddressBase: s.Frame.AddressBase}
	done, err := s.commandPalettes[side].Advance(s.Presentation, &frame, s.Presentation.Memory(s.world.nativeCleanupMemory()))
	call.Context.D = frame.D
	return NativeCommandFrameResult{Complete: done}, err
}
