package populous2

import (
	"fmt"
	"go-populous2/internal/amiga"
)

// NativeInGameHostRules caches the concrete children of the original menu.
type NativeInGameHostRules struct {
	Options      NativeOptionsFrameRules
	Serial       NativeSerialFrameRules
	ProfilePanel NativeProfilePanelFrameRules
	Audio        NativeAudioControlFrameRules
}

func DecodeNativeInGameHostRules(exe *amiga.Executable) (NativeInGameHostRules, error) {
	var r NativeInGameHostRules
	var err error
	if r.Options, err = DecodeNativeOptionsFrameRules(exe); err != nil {
		return r, err
	}
	if r.Serial, err = DecodeNativeSerialFrameRules(exe); err != nil {
		return r, err
	}
	if r.ProfilePanel, err = DecodeNativeProfilePanelFrameRules(exe); err != nil {
		return r, err
	}
	r.Audio, err = DecodeNativeAudioControlFrameRules(exe)
	return r, err
}

type NativeInGameHostCallbacks struct {
	NativeFileFrameCallbacks
	Image  *NativeImageRenderState
	Sprite func(NativePresentationSprite, []byte) error
	// Owned=true is$e4c, false is$e28. Panel release/acquire arguments are
	// adapted to this same host convention before invoking the callback.
	Ownership          func(owned bool, frame *NativeFrameRegisterContext) error
	Audio              NativeAudioControlFrameCallbacks
	SerialTransport    func(NativeSerialFrameCall, *uint32) (NativeSerialFrameChildResult, error)
	CallerStackPointer uint32
}

// NativeInGameHostState retains the actual menu and one nested child. Real
// transport and unsupported source operations remain explicit host callbacks.
type NativeInGameHostState struct {
	Menu         NativeInGameFrameState
	Options      NativeOptionsFrameState
	Serial       NativeSerialFrameState
	Palette      *NativeFramePaletteState
	ChildRoutine int
	failed       error
}

func (s *NativeInGameHostState) Advance(r *NativeInGameHostRules, cb NativeInGameHostCallbacks) (NativeFileFrameStep, error) {
	if s == nil || r == nil {
		return NativeFileFrameStep{}, fmt.Errorf("native in-game host rules/state missing")
	}
	base := cb.NativeFileFrameCallbacks
	base.Call = func(call NativeFileFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
		return s.AdvanceChild(r, cb, call, phase)
	}
	return s.Menu.Advance(base)
}

// AdvanceChild executes original options, profile, panel, palette and audio
// bodies. It never substitutes an acknowledgment for transport or resource I/O.
func (s *NativeInGameHostState) AdvanceChild(r *NativeInGameHostRules, cb NativeInGameHostCallbacks, call NativeFileFrameCall, phase *uint32) (out NativeCommandFrameResult, failure error) {
	if s == nil || r == nil || call.Frame == nil || phase == nil {
		return out, fmt.Errorf("native menu child backing missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if s.ChildRoutine != 0 && s.ChildRoutine != call.Routine {
		return out, fmt.Errorf("native menu child changed during wait")
	}
	if s.ChildRoutine == 0 {
		s.ChildRoutine = call.Routine
		s.Options = NativeOptionsFrameState{}
		s.Serial = NativeSerialFrameState{}
		s.Palette = nil
	}
	defer func() {
		if failure != nil {
			s.failed = failure
		} else if out.Complete {
			s.ChildRoutine = 0
		}
	}()
	base := cb.NativeFileFrameCallbacks
	base.Frame = call.Frame
	switch call.Routine {
	case 0x471c:
		step, err := s.Options.Advance(&r.Options, base)
		return NativeCommandFrameResult{Complete: step.Complete}, err
	case 0x4984:
		step, err := s.Serial.Advance(&r.Serial, NativeSerialFrameCallbacks{NativeFileFrameCallbacks: base, Transport: cb.SerialTransport, CallerStackPointer: cb.CallerStackPointer})
		return NativeCommandFrameResult{Complete: step.Complete}, err
	case 0x111ae:
		_, err := r.ProfilePanel.SwitchProfile(base.Memory, call.Frame)
		return NativeCommandFrameResult{Complete: err == nil}, err
	case 0x1da0:
		if cb.Ownership == nil {
			return out, fmt.Errorf("native menu panel ownership callback missing")
		}
		_, err := r.ProfilePanel.RestorePanel(NativeProfilePanelFrameCallbacks{Memory: base.Memory, Frame: call.Frame, Image: cb.Image, Sprite: cb.Sprite, Bitmap: base.Bitmap, Ownership: func(released bool, frame *NativeFrameRegisterContext) error { return cb.Ownership(!released, frame) }})
		return NativeCommandFrameResult{Complete: err == nil}, err
	case 0x1842e, 0x18474, 0x184f6:
		audio := cb.Audio
		audio.Frame = call.Frame
		audio.Memory = base.Memory
		if audio.CodeBase == 0 {
			audio.CodeBase = base.CodeBase
		}
		_, err := r.Audio.Run(call.Routine, audio)
		return NativeCommandFrameResult{Complete: err == nil}, err
	case 0x102e4:
		if base.Presentation == nil || base.Code.Read16 == nil {
			return out, fmt.Errorf("native menu palette backing missing")
		}
		if s.Palette == nil {
			bank := func(a NativeRequesterAddress) (NativeFramePaletteBank, error) {
				p := NativeFramePaletteBank{Address: a.Address}
				if !a.Code || a.Address < base.CodeBase || a.Address&1 != 0 {
					return p, fmt.Errorf("native menu palette argument outside CODE")
				}
				for i := range p.Words {
					value, err := base.Code.Read16(int(a.Address-base.CodeBase) + i*2)
					if err != nil {
						return p, err
					}
					p.Words[i] = value
				}
				return p, nil
			}
			if call.Arguments&(1<<2|1<<3) != (1<<2 | 1<<3) {
				return out, fmt.Errorf("native menu palette arguments missing")
			}
			source, err := bank(call.A[2])
			if err != nil {
				return out, err
			}
			target, err := bank(call.A[3])
			if err != nil {
				return out, err
			}
			s.Palette = NewNativeFramePaletteState(source, target, 0)
		}
		done, err := s.Palette.Advance(base.Presentation, call.Frame, base.Memory)
		return NativeCommandFrameResult{Complete: done}, err
	default:
		if base.Call == nil {
			return out, fmt.Errorf("native menu host child%x missing", call.Routine)
		}
		return base.Call(call, phase)
	}
}
