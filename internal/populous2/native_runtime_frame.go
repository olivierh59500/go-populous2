package populous2

import "fmt"

// NativeRuntimeFrame retains the concrete main renderer, input and in-game
// menu children. The caller advances it inside Runtime.Access.Execute and
// supplies real result/reset/file/transport operations when source code asks.
type NativeRuntimeFrame struct {
	Host           *NativeRuntimeHost
	Callbacks      NativeFrameSessionCallbacks
	RenderState    NativeMainRenderState
	InputState     NativeGameplayInputState
	MenuState      NativeInGameHostState
	MenuRules      NativeInGameHostRules
	RenderChildren *NativeRuntimeRenderChildren
	InputChildren  *NativeRuntimeInputChildren
}

type NativeRuntimeFrameBindings struct {
	Audio          NativeRuntimeAudioOperations
	RenderChildren NativeRuntimeRenderChildrenCallbacks
	InputChildren  NativeGameplayHUDHostCallbacks
	InputOther     func(NativeStartupResetFrameCall, *uint32) (NativeCommandFrameResult, error)
	Menu           NativeInGameHostCallbacks
	Session        NativeFrameSessionCallbacks
}

func (h *NativeRuntimeHost) NewFrame(bindings NativeRuntimeFrameBindings) (*NativeRuntimeFrame, error) {
	if h == nil || h.Memory == nil {
		return nil, fmt.Errorf("native runtime frame owner missing")
	}
	s := &NativeRuntimeFrame{Host: h}
	menu, err := DecodeNativeInGameHostRules(h.Bundle.Executable)
	if err != nil {
		return nil, err
	}
	s.MenuRules = menu
	children, err := h.NewRenderChildren(bindings.RenderChildren)
	if err != nil {
		return nil, err
	}
	s.RenderChildren = children
	render, err := h.MainRenderBindings(NativeSessionRenderBindings{})
	if err != nil {
		return nil, err
	}
	render, err = children.Bind(render)
	if err != nil {
		return nil, err
	}
	render.DebugOverlay = func(frame *NativeFrameRegisterContext) error {
		_, err := RenderNativeDebugFrame(NativeDebugFrameCallbacks{Code: h.Memory.Code, CodeBase: h.Memory.CodeBase, Frame: frame, FormatAddress: h.Memory.CodeBase + 0xef6, Bitmap: h.Host.BitmapWindow, ReadAbsolute: func(address uint32) (uint8, error) { return h.Memory.RAM.Read8(int(address)) }})
		return err
	}
	input, err := h.NewInputChildren(bindings.InputChildren, bindings.InputOther)
	if err != nil {
		return nil, err
	}
	s.InputChildren = input
	cb := bindings.Session
	cb.Render = h.Session.RenderMain(render, &s.RenderState)
	cb.Input, err = h.GameplayInput(&s.InputState, NativeStartupResetFrameCallbacks{Call: input.Call})
	if err != nil {
		return nil, err
	}
	cb.Bitmap = h.Bitmap
	cb.Audio = NativeFrameAudioCallbacks{Command: bindings.Audio.Command}
	if bindings.Audio.DirectCue != nil {
		cb.DirectSound = func(offset uint16) error { return bindings.Audio.DirectCue(offset, &h.Session.Frame) }
	}
	menuBindings := bindings.Menu
	menuBindings.Code, menuBindings.Memory, menuBindings.CodeBase = h.Memory.Code, h.Memory.BSS, h.Memory.CodeBase
	menuBindings.Presentation, menuBindings.Bitmap = h.Session.Presentation, h.Bitmap
	if menuBindings.Sprite == nil {
		menuBindings.Sprite = render.Sprites.Paint
	}
	menuBindings.Audio.Command, menuBindings.Audio.MusicCommand = bindings.Audio.Command, bindings.Audio.MusicCommand
	menuBindings.Audio.CodeBase = h.Memory.CodeBase
	cb.Menu = h.Session.MenuFrame(&s.MenuRules, &s.MenuState, menuBindings)
	s.Callbacks = cb
	return s, nil
}

func (s *NativeRuntimeFrame) Advance() (bool, error) {
	if s == nil || s.Host == nil {
		return false, fmt.Errorf("native runtime frame state missing")
	}
	h := s.Host
	if h.Session.Phase == NativeFrameSessionIdle {
		return false, fmt.Errorf("native runtime frame must begin with its actual source register context")
	}
	return h.Session.Advance(s.Callbacks)
}
