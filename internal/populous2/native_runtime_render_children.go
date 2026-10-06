package populous2

import "fmt"

// NativeRuntimeRenderChildrenCallbacks supplies real host boundaries. Sound
// and ownership are synchronous children of an already-owned runtime Execute;
// they must not reacquire its outer lock. Beam is the explicit VPOSR sample,
// not an invented random value. IRQs and keys enter Presentation separately.
type NativeRuntimeRenderChildrenCallbacks struct {
	// SkipCopyProtection is a recreation host policy. The original manual
	// challenge remains available for source comparisons and diagnostic runs.
	SkipCopyProtection bool
	Beam               func() (uint16, error)
	Ownership          func(bool, *NativeFrameRegisterContext) error
	Sound              func(uint16, *NativeFrameRegisterContext) error
	Errors             NativeErrorFrameCallbacks
	// CallerA is optional explicit address-register backing. Main's D-only
	// callback cannot claim numeric A preservation when this is absent.
	CallerA *[7]NativeRequesterAddress
}

// NativeRuntimeRenderChildren retains the original314A protection and346A
// editor parents, including real resource/error/UI waits.314A is the Zeus
// protection requester invoked by the town draw, not town information.
type NativeRuntimeRenderChildren struct {
	Host            *NativeRuntimeHost
	Callbacks       NativeRuntimeRenderChildrenCallbacks
	ProtectionRules NativeProtectionFrameRules
	EditorRules     NativeEditorFrameRules
	Sprites         *NativeSpriteBitmapBank
	Protection      NativeProtectionFrameState
	Editor          NativeEditorFrameState
	ProtectionStep  NativeProtectionFrameStep
	EditorStep      NativeEditorFrameStep
	resource        *NativeResourceHostFrameState
	resourceRoutine int
	actor           int
	actorActive     bool
}

func (h *NativeRuntimeHost) NewRenderChildren(supplied NativeRuntimeRenderChildrenCallbacks) (*NativeRuntimeRenderChildren, error) {
	if h == nil || h.Bundle == nil || h.World == nil || h.Host == nil || h.Memory == nil || h.Session == nil || h.Session.Presentation == nil {
		return nil, fmt.Errorf("native runtime render child owners missing")
	}
	protection, err := DecodeNativeProtectionFrameRules(h.Bundle.Executable, h.Bundle.Raw["faces.pak"])
	if err != nil {
		return nil, err
	}
	editor, err := DecodeNativeEditorFrameRules(h.Bundle.Executable)
	if err != nil {
		return nil, err
	}
	logical, err := h.LogicalCode()
	if err != nil {
		return nil, err
	}
	if err := protection.Render.BindCode(h.Memory.Code, logical.Read32); err != nil {
		return nil, err
	}
	if err := editor.Frames.BindCode(h.Memory.Code, logical.Read32); err != nil {
		return nil, err
	}
	sprites, err := DecodeNativeSpriteBitmapBank(h.Bundle, int(h.World.Level.Terrain))
	if err != nil {
		return nil, err
	}
	return &NativeRuntimeRenderChildren{Host: h, Callbacks: supplied, ProtectionRules: protection, EditorRules: editor, Sprites: sprites}, nil
}

func (s *NativeRuntimeRenderChildren) valid(frame *NativeFrameRegisterContext) error {
	if s == nil || s.Host == nil || s.Host.Memory == nil || frame == nil || frame.AddressBase != s.Host.Memory.BSSBase {
		return fmt.Errorf("native runtime render child context/base missing")
	}
	return nil
}

// Bind supplies only these actual retained children. Parent traversal keeps
// its own caller/index/screen continuation and refreshes targets after swaps.
func (s *NativeRuntimeRenderChildren) Bind(bindings NativeSessionRenderBindings) (NativeSessionRenderBindings, error) {
	if s == nil || s.Host == nil {
		return bindings, fmt.Errorf("native runtime render children missing")
	}
	bindings.PaintingAdvance = s.AdvanceEditor
	bindings.Children.TownInfoAdvance = s.AdvanceProtection
	bindings.SelectedOwnership = s.Callbacks.Ownership
	return bindings, nil
}

func (s *NativeRuntimeRenderChildren) AdvanceProtection(actor int, frame *NativeFrameRegisterContext) (bool, error) {
	if err := s.valid(frame); err != nil {
		return false, err
	}
	if s.Callbacks.SkipCopyProtection {
		// The original successful challenge records this session flag.
		// Gameplay state, clocks and resources are otherwise untouched.
		err := s.Host.Memory.BSS.Write16(0x3b8, 1)
		return err == nil, err
	}
	if s.Callbacks.Beam == nil || s.Callbacks.Ownership == nil || s.Callbacks.Sound == nil {
		return false, fmt.Errorf("native protection Beam/ownership/sound child missing")
	}
	if s.Protection.Finished {
		s.Protection = NativeProtectionFrameState{}
		s.actorActive = false
	}
	if !s.actorActive {
		s.actor, s.actorActive = actor, true
		if s.Callbacks.CallerA != nil {
			s.Protection.A = *s.Callbacks.CallerA
		}
	} else if s.actor != actor {
		return false, fmt.Errorf("native protection actor changed while suspended")
	}
	h := s.Host
	cb := NativeProtectionFrameCallbacks{Code: h.Memory.Code, Memory: h.Memory.BSS, CodeBase: h.Memory.CodeBase, Frame: frame, Presentation: h.Session.Presentation, Bitmap: h.Bitmap, Sound: s.Callbacks.Sound, Beam: s.Callbacks.Beam, Ownership: s.Callbacks.Ownership, Call: s.resourceCall}
	step, err := s.Protection.Advance(&s.ProtectionRules, cb)
	s.ProtectionStep = step
	if step.Complete && s.Callbacks.CallerA != nil {
		*s.Callbacks.CallerA = s.Protection.A
	}
	return step.Complete, err
}

func (s *NativeRuntimeRenderChildren) AdvanceEditor(frame *NativeFrameRegisterContext) (bool, error) {
	if err := s.valid(frame); err != nil {
		return false, err
	}
	if s.Callbacks.Sound == nil {
		return false, fmt.Errorf("native editor sound child missing")
	}
	if s.Editor.Complete {
		s.Editor = NativeEditorFrameState{}
	}
	h := s.Host
	cb := NativeEditorFrameCallbacks{NativeFileFrameCallbacks: NativeFileFrameCallbacks{Code: h.Memory.Code, Memory: h.Memory.BSS, CodeBase: h.Memory.CodeBase, Frame: frame, Presentation: h.Session.Presentation, Bitmap: h.Bitmap, Sound: s.Callbacks.Sound, ReadAbsolute: func(at uint32) (byte, error) { return h.Memory.RAM.Read8(int(at)) }}, Image: &h.Session.Image, Sprite: s.Sprites.Paint}
	step, err := s.Editor.Advance(&s.EditorRules, cb)
	s.EditorStep = step
	return step.Complete, err
}

func (s *NativeRuntimeRenderChildren) resourceCall(call NativeFileFrameCall, _ *uint32) (NativeCommandFrameResult, error) {
	if call.Routine != 0x19cd0 && call.Routine != 0x1a32a {
		return NativeCommandFrameResult{}, fmt.Errorf("native render resource child%x unsupported", call.Routine)
	}
	if s.resource == nil {
		s.resource = &NativeResourceHostFrameState{Landscape: call.Routine == 0x1a32a}
		s.resourceRoutine = call.Routine
	} else if s.resourceRoutine != call.Routine {
		return NativeCommandFrameResult{}, fmt.Errorf("native render resource changed while suspended")
	}
	errors := s.Callbacks.Errors
	errors.Sound = s.Callbacks.Sound
	cb, err := s.Host.ResourceCallbacks(call.Frame, errors)
	if err != nil {
		return NativeCommandFrameResult{}, err
	}
	cb.ResourceCallerA = call.A
	step, err := s.resource.Advance(&s.Host.ResourceRules, cb)
	if err != nil || !step.Complete {
		return NativeCommandFrameResult{Complete: step.Complete}, err
	}
	if call.Routine == 0x19cd0 {
		if err := s.bindPhysicalFaces(); err != nil {
			return NativeCommandFrameResult{}, err
		}
	}
	s.resource = nil
	return NativeCommandFrameResult{Complete: true}, nil
}

// The real loader already prepared these face planes. Borrow their physical
// destination; do not transpose a detached copy or assume that loading worked.
func (s *NativeRuntimeRenderChildren) bindPhysicalFaces() error {
	for i := range s.ProtectionRules.Faces {
		at := 0x212ba + i*12
		address, err := s.Host.Memory.Code.Read32(at)
		if err != nil {
			return err
		}
		half, err := s.Host.Memory.Code.Read16(at + 4)
		if err != nil {
			return err
		}
		height, err := s.Host.Memory.Code.Read16(at + 6)
		if err != nil {
			return err
		}
		if half != 16 || height == 0 {
			return fmt.Errorf("native runtime face%d descriptor dimensions unsupported", i)
		}
		planes, err := s.Host.Host.Span(address, int(height)*20)
		if err != nil {
			return err
		}
		s.ProtectionRules.Faces[i] = NativePreparedSprite{Width: 32, Height: int(height), Planes: planes}
	}
	return nil
}
