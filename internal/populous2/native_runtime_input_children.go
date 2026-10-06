package populous2

import "fmt"

type NativeRuntimeInputChildren struct {
	Host       *NativeRuntimeHost
	HUD        NativeGameplayHUDHost
	HUDRules   NativeGameplayHUDInputRules
	ChildRules NativeCampaignSelectionChildrenRules
	Render     NativeRenderFrameRules
	Supplied   NativeGameplayHUDHostCallbacks
	Other      func(NativeStartupResetFrameCall, *uint32) (NativeCommandFrameResult, error)
}

func (h *NativeRuntimeHost) NewInputChildren(supplied NativeGameplayHUDHostCallbacks, other func(NativeStartupResetFrameCall, *uint32) (NativeCommandFrameResult, error)) (*NativeRuntimeInputChildren, error) {
	if h == nil || h.Memory == nil {
		return nil, fmt.Errorf("native runtime input child owner missing")
	}
	hud, err := DecodeNativeGameplayHUDInputRules(h.Bundle.Executable)
	if err != nil {
		return nil, err
	}
	children, err := DecodeNativeCampaignSelectionChildrenRules(h.Bundle.Executable)
	if err != nil {
		return nil, err
	}
	render, err := DecodeNativeRenderFrameRules(h.Bundle.Executable)
	if err != nil {
		return nil, err
	}
	logical, err := h.LogicalCode()
	if err != nil {
		return nil, err
	}
	if err := render.BindCode(h.Memory.Code, logical.Read32); err != nil {
		return nil, err
	}
	panel, err := DecodeNativeStartupPanelFrameRules(h.Bundle.Executable)
	if err != nil {
		return nil, err
	}
	supplied.Panel = panel
	return &NativeRuntimeInputChildren{Host: h, HUDRules: hud, ChildRules: children, Render: render, Supplied: supplied, Other: other}, nil
}

func (s *NativeRuntimeInputChildren) Call(call NativeStartupResetFrameCall, phase *uint32) (NativeCommandFrameResult, error) {
	if s == nil || s.Host == nil || call.Frame == nil || call.A == nil || phase == nil {
		return NativeCommandFrameResult{}, fmt.Errorf("native runtime input child context missing")
	}
	h := s.Host
	if call.Frame.AddressBase != h.Memory.BSSBase {
		return NativeCommandFrameResult{}, fmt.Errorf("native runtime input child frame base differs")
	}
	base := NativeStartupResetFrameCallbacks{Frame: call.Frame, Code: h.Memory.Code, Memory: h.Memory.BSS, RAM: h.Memory.RAM, CodeBase: h.Memory.CodeBase}
	switch call.Routine {
	case 0xd2b4, 0xd91a, 0x2914:
		return NativeGameplayTerrainChild(&s.Render, base, call)
	case 0x23ee, 0x2472, 0x147e0:
		return s.HUD.Call(h, &s.HUDRules, &s.ChildRules, s.Supplied, call, phase)
	case 0x184f6:
		if s.Supplied.Audio.DirectCue == nil {
			return NativeCommandFrameResult{}, fmt.Errorf("native input direct audio missing")
		}
		err := s.Supplied.Audio.DirectCue(uint16(call.Frame.D[0]), call.Frame)
		return NativeCommandFrameResult{Complete: err == nil}, err
	case 0x1da0:
		if s.Supplied.Ownership == nil {
			return NativeCommandFrameResult{}, fmt.Errorf("native input panel ownership missing")
		}
		err := h.RestoreStartupPanel(&s.Supplied.Panel, call.Frame, call.A, s.Supplied.Ownership)
		return NativeCommandFrameResult{Complete: err == nil}, err
	case 0xd8cc:
		bitmap, err := h.Bitmap(call.A[0].Address)
		if err != nil {
			return NativeCommandFrameResult{}, err
		}
		err = DrawNativeStartupMinimapFrame(base, call.A, bitmap)
		return NativeCommandFrameResult{Complete: err == nil}, err
	default:
		if s.Other == nil {
			return NativeCommandFrameResult{}, fmt.Errorf("native input body%x has no actual operation", call.Routine)
		}
		return s.Other(call, phase)
	}
}
