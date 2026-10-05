package populous2

import "fmt"

// AppendNativeRedrawWord translates the FIFO write at $d87a..$d89c. The
// cursor is a signed word relative to $eb70, so negative offsets retain their
// raw aliases. Only the original upper allocation bound rejects an append.
func (w *World) AppendNativeRedrawWord(word uint16) (bool, error) {
	if w == nil {
		return false, fmt.Errorf("native redraw World missing")
	}
	m := w.nativeCleanupMemory()
	offset, err := m.Read16(0xeb6e)
	if err != nil {
		return false, err
	}
	address := 0xeb70 + int(int16(offset))
	if address >= 0x11280 {
		return false, nil
	}
	if address&1 != 0 {
		return false, fmt.Errorf("native redraw word write at odd address")
	}
	if err := m.Write16(address, word); err != nil {
		return false, err
	}
	// A negative cursor can alias the cursor word itself. ADDI.W reads the
	// just-written memory, not the offset used to compute the destination.
	current, err := m.Read16(0xeb6e)
	if err != nil {
		return true, err
	}
	return true, m.Write16(0xeb6e, current+2)
}
