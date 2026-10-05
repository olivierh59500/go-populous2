package populous2

// nativeBirthBlockWord preserves arbitrary raw writes while accepting legacy
// callers that still change the public bool. A bool-only change has no raw
// representation, so it uses the original allocation-failure value one.
func (w *World) nativeBirthBlockWord() uint16 {
	if w.NativeBirthBlocked != (w.NativeBirthBlockWord != 0) {
		if w.NativeBirthBlocked {
			w.NativeBirthBlockWord = 1
		} else {
			w.NativeBirthBlockWord = 0
		}
	}
	return w.NativeBirthBlockWord
}

func (w *World) setNativeBirthBlockWord(value uint16) {
	w.NativeBirthBlockWord = value
	w.NativeBirthBlocked = value != 0
}
