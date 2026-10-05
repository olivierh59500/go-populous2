package populous2

import "fmt"

// RebaseNativeStartupAnimations is the source$10a36..$10a80 loop. It runs
// once during initial startup after the actual ICONS plane preparation.
// The three thresholds are mutable CODE words and the additions are the
// three stack words12/12/1fec; substituting current sprite identifiers would
// lose the original signed comparison and word-register results.
func RebaseNativeStartupAnimations(cb NativeStartupResetFrameCallbacks, a *[7]NativeRequesterAddress) error {
	if cb.Frame == nil || a == nil || !winMemoryValid(cb.Code) {
		return fmt.Errorf("native initial animation relocation backing missing")
	}
	c := cb.Frame
	a[0] = NativeRequesterAddress{Address: cb.CodeBase + 0x26956, Code: true}
	a[1] = NativeRequesterAddress{Address: cb.CodeBase + 0x1a590, Code: true}
	c.D[0], c.D[2] = 0, 0
	add := [3]uint16{12, 12, 0x1fec}
	for at := 0x26956; at < 0x33194; at += 6 {
		value, e := cb.Code.Read16(at + 2)
		if e != nil {
			return e
		}
		c.Word(0, value)
		if uint16(c.D[0]) != 0 {
			c.D[1] = 4
			for {
				threshold, e := cb.Code.Read16(0x1a590 + int(int16(c.D[1])))
				if e != nil {
					return e
				}
				c.Word(2, threshold)
				if int32(c.D[0]) >= int32(c.D[2]) {
					c.D[0] -= c.D[2]
					c.Word(0, uint16(c.D[0])+add[int(uint16(c.D[1]))/2])
					if e = cb.Code.Write16(at+2, uint16(c.D[0])); e != nil {
						return e
					}
					break
				}
				c.Word(1, uint16(c.D[1])-2)
				if int16(c.D[1]) < 0 {
					break
				}
			}
		}
		a[0].Address = cb.CodeBase + uint32(at+6)
	}
	return nil
}
