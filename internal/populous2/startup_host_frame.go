package populous2

import (
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeStartupHostFrameRules struct {
	Resources NativeResourceFrameRules
	Audio     NativeAudioControlFrameRules
}

func DecodeNativeStartupHostFrameRules(exe *amiga.Executable) (NativeStartupHostFrameRules, error) {
	var r NativeStartupHostFrameRules
	var err error
	if r.Resources, err = DecodeNativeResourceFrameRules(exe); err != nil {
		return r, err
	}
	r.Audio, err = DecodeNativeAudioControlFrameRules(exe)
	return r, err
}

type NativeStartupHostFrameCallbacks struct {
	NativeStartupResetFrameCallbacks
	// Resource is a real loader/error-requester port. Nil retains that source
	// operation at Call rather than acknowledging a missing load.
	Resource *NativeResourceHostFrameCallbacks
	Audio    *NativeAudioControlFrameCallbacks
	Bitmap   func(uint32) ([]byte, error)
}

// NativeStartupHostFrameState retains every nested source body separately.
// One real pending child can hold construction, power loading and the outer
// reset simultaneously; resumption does not repeat any destructive prefix.
type NativeStartupHostFrameState struct {
	Startup         NativeStartupResetFrameState
	Construction    *NativeStartupConstructionFrameState
	Power           *NativeStartupPowerFrameState
	Campaign        *NativeCampaignRecordFrameState
	Resource        *NativeResourceHostFrameState
	resourceRoutine int
	failed          error
}

func (s *NativeStartupHostFrameState) Advance(r *NativeStartupHostFrameRules, cb NativeStartupHostFrameCallbacks) (out NativeStartupResetFrameStep, failure error) {
	if s == nil || r == nil {
		return out, fmt.Errorf("native startup host rules/state missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	defer func() {
		if failure != nil {
			s.failed = failure
		}
	}()
	base := cb.NativeStartupResetFrameCallbacks
	base.Call = func(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
		return s.AdvanceChild(r, cb, call, phase)
	}
	return s.Startup.Advance(base)
}

// AdvanceChild binds only actual translated bodies. The original campaign
// chooser, initial menus, transport and panel-address continuation remain
// genuine host operations at Call until their complete bodies are supplied.
func (s *NativeStartupHostFrameState) AdvanceChild(r *NativeStartupHostFrameRules, cb NativeStartupHostFrameCallbacks, call NativeStartupResetFrameCall, phase *uint32) (out NativeCommandFrameResult, failure error) {
	if s == nil || r == nil || call.Frame == nil || call.A == nil || phase == nil {
		return out, fmt.Errorf("native startup child frame/phase missing")
	}
	base := cb.NativeStartupResetFrameCallbacks
	base.Frame = call.Frame
	base.Call = func(child NativeStartupResetFrameCall, inner *uint32) (NativeCommandFrameResult, error) {
		return s.AdvanceChild(r, cb, child, inner)
	}
	switch call.Routine {
	case 0x10d6a:
		if s.Construction == nil {
			s.Construction = &NativeStartupConstructionFrameState{A: *call.A}
		}
		step, err := s.Construction.Advance(base)
		*call.A = s.Construction.A
		if step.Complete {
			s.Construction = nil
		}
		return NativeCommandFrameResult{Complete: step.Complete, Zero: step.Zero}, err
	case 0x11078:
		if s.Power == nil {
			s.Power = &NativeStartupPowerFrameState{A: *call.A}
		}
		step, err := s.Power.Advance(base)
		*call.A = s.Power.A
		if step.Complete {
			s.Power = nil
		}
		return NativeCommandFrameResult{Complete: step.Complete}, err
	case 0x11044:
		if s.Campaign == nil {
			s.Campaign = &NativeCampaignRecordFrameState{A: *call.A}
		}
		step, err := s.Campaign.Advance(NativeCampaignFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Code: base.Code, Memory: base.Memory, CodeBase: base.CodeBase, Frame: call.Frame}, RAM: base.RAM, Child: base.Call})
		*call.A = s.Campaign.A
		if step.Complete {
			s.Campaign = nil
		}
		return NativeCommandFrameResult{Complete: step.Complete, Zero: step.Zero}, err
	case 0x10df2:
		err := LoadNativeStartupTemplates(base, call.A)
		return NativeCommandFrameResult{Complete: err == nil}, err
	case 0x10e90:
		err := CompileNativeStartupChoices(base, call.A)
		return NativeCommandFrameResult{Complete: err == nil}, err
	case 0xcd22, 0xcd76, 0xcdca, 0xd9d8, 0xda0a, 0xdbd4, 0xdc04, 0xdc6c, 0x10b38, 0x10c7e, 0x10cbe, 0x14048, 0x13fb6, 0x125a0:
		step, err := RunNativeStartupWorldFrame(call.Routine, base, call.A)
		return NativeCommandFrameResult{Complete: step.Complete}, err
	case 0x19cd0, 0x1a32a:
		if cb.Resource != nil {
			if s.Resource == nil {
				s.Resource = &NativeResourceHostFrameState{Landscape: call.Routine == 0x1a32a}
				s.resourceRoutine = call.Routine
			}
			if s.resourceRoutine != call.Routine {
				return out, fmt.Errorf("native startup resource changed during a wait")
			}
			resource := *cb.Resource
			resource.Frame, resource.Memory, resource.Code, resource.RAM, resource.CodeBase = call.Frame, base.Memory, base.Code, base.RAM, base.CodeBase
			resource.ResourceCallerA = *call.A
			step, err := s.Resource.Advance(&r.Resources, resource)
			// Both source entries save and restore A0-A6 in their outer MOVEM.
			// The loader/error requester owns its actual inner addresses.
			if step.Complete {
				s.Resource = nil
				s.resourceRoutine = 0
			}
			return NativeCommandFrameResult{Complete: step.Complete}, err
		}
	case 0xd8cc:
		if cb.Bitmap != nil {
			bitmap, err := cb.Bitmap((*call.A)[0].Address)
			if err != nil {
				return out, err
			}
			err = DrawNativeStartupMinimapFrame(base, call.A, bitmap)
			return NativeCommandFrameResult{Complete: err == nil, Zero: err == nil}, err
		}
	case 0x18474:
		if cb.Audio != nil {
			audio := *cb.Audio
			audio.Frame, audio.Memory, audio.CodeBase = call.Frame, base.Memory, base.CodeBase
			step, err := r.Audio.Run(call.Routine, audio)
			if step.A0Assigned {
				(*call.A)[0] = step.A0
			}
			return NativeCommandFrameResult{Complete: step.Complete}, err
		}
	}
	if cb.Call == nil {
		return out, fmt.Errorf("native startup source child%x has no actual host body", call.Routine)
	}
	return cb.Call(call, phase)
}

// DrawNativeStartupMinimapFrame is the actual $d8cc caller plus its $e196
// (or checked $e17a) pixel child. Besides real planar pixels, it exposes the
// surviving A1 pixel address and A2 parcel cursor rather than resetting them.
func DrawNativeStartupMinimapFrame(cb NativeStartupResetFrameCallbacks, a *[7]NativeRequesterAddress, bitmap []byte) error {
	if cb.Frame == nil || a == nil || len(bitmap) != 32000 || !winMemoryValid(cb.Memory) || !winMemoryValid(cb.Code) {
		return fmt.Errorf("native startup minimap backing missing")
	}
	c := cb.Frame
	m := nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Memory}}
	code := nativeTownFrameMemory{nativeWhirlwindMemory: nativeWhirlwindMemory{m: cb.Code}}
	dx, dy, procedure := code.word(0x33612), code.word(0x33614), code.long(0x33616)
	if procedure != cb.CodeBase+0xe196 && procedure != cb.CodeBase+0xe17a {
		return fmt.Errorf("native startup minimap pixel procedure%x has no actual body", procedure)
	}
	a[2] = NativeRequesterAddress{Address: c.AddressBase + 0xf45}
	c.D[7] = 0
	for y := 0; y < 64; y++ {
		c.D[6] = 0
		for x := 0; x < 64; x++ {
			c.D[2] = uint32(m.byte(int(a[2].Address - c.AddressBase)))
			a[2].Address += 4
			a[1] = NativeRequesterAddress{Address: cb.CodeBase + 0x33744, Code: true}
			c.Byte(2, code.byte(0x33744+int(int16(c.D[2]))))
			c.D[0] = 64
			c.Word(0, uint16(c.D[0])+uint16(c.D[6])-uint16(c.D[7]))
			c.Word(1, (uint16(c.D[6])+uint16(c.D[7]))>>1)
			a[1] = NativeRequesterAddress{Address: cb.CodeBase + 0x33612, Code: true}
			c.Word(0, uint16(c.D[0])+dx)
			c.Word(1, uint16(c.D[1])+dy)
			a[1] = NativeRequesterAddress{Address: procedure, Code: true}
			if procedure != cb.CodeBase+0xe17a || int16(c.D[0]) >= 0 && int16(c.D[0]) < 320 && int16(c.D[1]) >= 0 && int16(c.D[1]) < 200 {
				point, err := PlanNativeMapPoint(c)
				if err != nil {
					return err
				}
				a[1] = NativeRequesterAddress{Address: uint32(int64(a[0].Address) + int64(point.Offset)), Chip: true}
				if err = point.Paint(bitmap); err != nil {
					return err
				}
			}
			c.Word(6, uint16(c.D[6])+1)
		}
		c.Word(7, uint16(c.D[7])+1)
	}
	if m.err != nil {
		return m.err
	}
	return code.err
}
