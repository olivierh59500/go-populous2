package populous2

import "fmt"

// NativeFrameRegisterContext is the explicit data-register continuation of
// the ordered original update. Empty pools and saved-register wrappers retain
// caller values; byte and word assignments do not erase unrelated bits.
type NativeFrameRegisterContext struct {
	D [8]uint32
	// AddressBase is the original BSS relocation base. World uses relative
	// addresses (zero); CPU fixtures can retain their actual relocation.
	AddressBase uint32
}

func frameDivide(c *NativeFrameRegisterContext, reg int, divisor uint16) error {
	if divisor == 0 {
		return fmt.Errorf("native frame DIVU zero divisor")
	}
	v := c.D[reg]
	if v/uint32(divisor) <= 0xffff {
		c.D[reg] = v%uint32(divisor)<<16 | v/uint32(divisor)
	}
	return nil
}

func (c *NativeFrameRegisterContext) Byte(reg int, value uint8) {
	c.D[reg] = c.D[reg]&0xffffff00 | uint32(value)
}
func (c *NativeFrameRegisterContext) Word(reg int, value uint16) {
	c.D[reg] = c.D[reg]&0xffff0000 | uint32(value)
}
func (c *NativeFrameRegisterContext) ExtendWord(reg int) {
	c.Word(reg, uint16(int16(int8(uint8(c.D[reg])))))
}
func (c *NativeFrameRegisterContext) RestoreWord(reg int, value uint16) {
	c.D[reg] = uint32(int32(int16(value)))
}
func (c *NativeFrameRegisterContext) Swap(reg int) { c.D[reg] = c.D[reg]<<16 | c.D[reg]>>16 }
func (c *NativeFrameRegisterContext) CommandContext() NativeCommandRegisterContext {
	return NativeCommandRegisterContext{D: c.D}
}
func (c *NativeFrameRegisterContext) SetCommandContext(value NativeCommandRegisterContext) {
	c.D = value.D
}

// FollowerBegin is the data-register portion of $11252 initialization. The
// final MOVE.L is the second deity's old population total before CLR.L. It
// changes D0 only; it is not an all-register reset for an empty follower pass.
func (c *NativeFrameRegisterContext) FollowerBegin(memory FollowerCleanupMemory) error {
	value, e := memory.Read32(0xe9de + 4)
	if e != nil {
		return e
	}
	c.D[0] = value
	return nil
}

// ScenarioProbe is the unconditionally executed MOVE.W at $17ea6, including
// zero/pending events. The actual command body owns subsequent register writes.
func (c *NativeFrameRegisterContext) ScenarioProbe(memory FollowerCleanupMemory) error {
	offset, e := memory.Read16(0xf0a)
	if e != nil {
		return e
	}
	value, e := memory.Read16(0xdde + int(int16(offset)))
	if e != nil {
		return e
	}
	c.Word(0, value)
	return nil
}

// ObserveMove is the register portion of$12518, before its actual graph write.
// D6 becomes the signed relative reference only on a changed-cell move.
func (c *NativeFrameRegisterContext) ObserveMove(ref NativeRecordReference, x, y uint16, m FollowerCleanupMemory) error {
	at := cleanupRecordAddress(ref)
	oldY, e := m.Read16(at + 8)
	if e != nil {
		return e
	}
	oldX, e := m.Read8(at + 6)
	if e != nil {
		return e
	}
	old := oldY&0xff00 | uint16(oldX)
	next := y&0xff00 | uint16(uint8(x>>8))
	c.Word(0, old)
	c.Word(7, next)
	if old == next {
		c.D[0] = 0
		return nil
	}
	c.Word(7, next&0xff00|uint16(uint8(next)*4))
	c.D[6] = uint32(int32(int16(ref)))
	c.D[0] = 1
	return nil
}
