package populous2

// NativeEndingPlayback schedules $b142's one initial VBlank and four waits
// per subsequent frame. Keyboard input is cleared after the initial wait and
// checked only after the following text/delta update and screen swap.
type NativeEndingPlayback struct {
	Ending   *NativeEnding
	Intro    bool
	Wait     int
	Keyboard bool
	Finished bool
}

func NewNativeEndingPlayback(raw []byte, presentation *NativePresentation, phase uint16) (*NativeEndingPlayback, error) {
	ending, err := NewNativeEnding(raw, presentation, phase)
	if err != nil {
		return nil, err
	}
	return &NativeEndingPlayback{Ending: ending, Intro: true, Wait: NativeEndingIntroWaitVBlanks}, nil
}

func (playback *NativeEndingPlayback) PressKey() {
	if !playback.Intro && !playback.Finished {
		playback.Keyboard = true
	}
}

// VBlank advances presentation time only; the world simulation stays frozen.
// A true return means the original displayed planes changed on this blank.
func (playback *NativeEndingPlayback) VBlank() (bool, error) {
	if playback.Finished {
		return false, nil
	}
	playback.Wait--
	if playback.Wait > 0 {
		return false, nil
	}
	if playback.Intro {
		playback.Intro = false
		playback.Keyboard = false
		playback.Wait = NativeEndingWaitVBlanks
		return false, nil
	}
	if err := playback.Ending.Advance(); err != nil {
		return false, err
	}
	playback.Wait = NativeEndingWaitVBlanks
	playback.Finished = playback.Keyboard
	return true, nil
}
