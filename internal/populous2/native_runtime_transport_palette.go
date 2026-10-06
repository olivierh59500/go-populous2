package populous2

import "fmt"

// advancePalette binds the handshake failure requester's real102E4 child.
// Source banks and frame registers survive actual VBlank suspension; neither
// an error dialog nor its fade can complete through a host acknowledgment.
func (s *NativeRuntimeTransport) advancePalette(call NativeFileFrameCall, phase *uint32) (NativeSerialFrameChildResult, error) {
	if s == nil || s.Host == nil || call.Frame == nil || phase == nil || call.Arguments&(1<<2|1<<3) != 1<<2|1<<3 {
		return NativeSerialFrameChildResult{}, fmt.Errorf("native transport palette arguments missing")
	}
	h := s.Host
	if *phase == 0 {
		bank := func(a NativeRequesterAddress) (NativeFramePaletteBank, error) {
			p := NativeFramePaletteBank{Address: a.Address}
			if !a.Code || a.Address < h.Memory.CodeBase || a.Address&1 != 0 {
				return p, fmt.Errorf("native transport palette outside aligned CODE")
			}
			for i := range p.Words {
				v, err := h.Memory.Code.Read16(int(a.Address-h.Memory.CodeBase) + i*2)
				if err != nil {
					return p, err
				}
				p.Words[i] = v
			}
			return p, nil
		}
		from, err := bank(call.A[2])
		if err != nil {
			return NativeSerialFrameChildResult{}, err
		}
		to, err := bank(call.A[3])
		if err != nil {
			return NativeSerialFrameChildResult{}, err
		}
		s.palette = NewNativeFramePaletteState(from, to, h.Memory.CodeBase)
		*phase = 1
	}
	if s.palette == nil || s.palette.Source.Address != call.A[2].Address || s.palette.Target.Address != call.A[3].Address {
		return NativeSerialFrameChildResult{}, fmt.Errorf("native transport palette changed during wait")
	}
	complete, err := s.palette.Advance(h.Session.Presentation, call.Frame, h.Memory.BSS)
	if complete {
		s.palette = nil
	}
	return NativeSerialFrameChildResult{Complete: complete, Zero: complete}, err
}
