package populous2

import "fmt"

// advanceFrameEntry is the originalE76..E94 prefix. It clears the pending
// menu word once, retains the actual446A child and tests exit only after that
// child returns. The following786 VBlank wait cannot replay this prefix.
func (s *NativeFrameSession) advanceFrameEntry(memory FollowerCleanupMemory, cb NativeFrameSessionCallbacks) (ready, exit bool, failure error) {
	if s == nil || !winMemoryValid(memory) {
		return false, false, fmt.Errorf("native frame entry backing missing")
	}
	if !s.entryChecked {
		menu, err := memory.Read16(0xdce)
		if err != nil {
			return false, false, err
		}
		if menu != 0 {
			if err := memory.Write16(0xdce, 0); err != nil {
				return false, false, err
			}
			s.Phase = NativeFrameSessionMenu
		}
		s.entryChecked = true
	}
	if s.Phase == NativeFrameSessionMenu {
		if cb.Menu == nil {
			return false, false, fmt.Errorf("native main entry menu446A continuation missing")
		}
		done, err := cb.Menu(memory, &s.Frame, &s.Image, &s.MenuPhase)
		if err != nil {
			return false, false, err
		}
		if !done {
			return false, false, nil
		}
		s.Phase = NativeFrameSessionVBlank
	}
	if s.Phase == NativeFrameSessionVBlank && !s.exitChecked {
		value, err := memory.Read16(0x3aa)
		if err != nil {
			return false, false, err
		}
		s.exitChecked = true
		if value != 0 {
			return false, true, nil
		}
	}
	return true, false, nil
}
