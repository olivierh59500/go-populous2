package populous2

import "fmt"

// CanRetryHostStream permits an explicit serial-requester retry only after
// the source disconnected to local mode and returned all active operations.
// It never completes or discards a packet, error dialog or corrupt RTS.
func (s *NativeRuntimeTransport) CanRetryHostStream() (bool, error) {
	if s == nil || s.Host == nil || s.Conn == nil {
		return false, fmt.Errorf("native retry transport missing")
	}
	if s.Conn.TerminalError() == nil {
		return false, nil
	}
	mode, err := s.Host.Memory.BSS.Read16(0xeb44)
	if err != nil {
		return false, err
	}
	if mode != 4 || s.mismatch != nil || s.palette != nil || s.packets[0] != nil || s.packets[1] != nil ||
		(s.Handshake.Started && !s.Handshake.Finished) || (s.Resume.Started && !s.Resume.Finished) || (s.Transfer.Started && !s.Transfer.Complete) {
		return false, nil
	}
	if s.Host.Session.Phase != NativeFrameSessionMenu && s.Host.Session.Phase != NativeFrameSessionIdle {
		return false, nil
	}
	for _, at := range []int{0xeb5e, 0xeb68} {
		mode, err := s.Host.Memory.BSS.Read8(at)
		if err != nil {
			return false, err
		}
		if mode == 6 || mode == 8 {
			return false, nil
		}
	}
	return true, nil
}
