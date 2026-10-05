package populous2

import "fmt"

// MenuFrame connects the source entry gate to the concrete menu/child host.
// The caller supplies initialized shared CODE and real audio/transport ports;
// the session supplies its currently borrowed World and physical screens.
func (s *NativeFrameSession) MenuFrame(rules *NativeInGameHostRules, state *NativeInGameHostState, bindings NativeInGameHostCallbacks) func(FollowerCleanupMemory, *NativeFrameRegisterContext, *NativeImageRenderState, *uint32) (bool, error) {
	return func(memory FollowerCleanupMemory, frame *NativeFrameRegisterContext, image *NativeImageRenderState, phase *uint32) (bool, error) {
		if s == nil || s.Presentation == nil || rules == nil || state == nil || frame == nil || image == nil || phase == nil || !winMemoryValid(bindings.Code) {
			return false, fmt.Errorf("native session menu backing missing")
		}
		if *phase == 0 {
			if state.Menu.Started && !state.Menu.Finished {
				return false, fmt.Errorf("native menu invocation is already pending")
			}
			*state = NativeInGameHostState{}
			*phase = 1
		} else if *phase != 1 {
			return false, fmt.Errorf("native session menu already completed")
		}
		cb := bindings
		cb.Memory, cb.Frame, cb.Presentation, cb.Image = memory, frame, s.Presentation, image
		if cb.Bitmap == nil {
			cb.Bitmap = s.bitmapAt
		}
		step, err := state.Advance(rules, cb)
		if err != nil {
			return false, err
		}
		if step.Complete {
			*phase = 2
		}
		return step.Complete, nil
	}
}
