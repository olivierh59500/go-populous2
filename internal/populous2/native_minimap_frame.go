package populous2

import (
	"encoding/binary"
	"fmt"
)

// DrawNativeMinimapFrame is $d8cc, including its mutableCODE coordinate and
// pixel-procedure selector. The supplied CODE must contain the active LAND
// resource. The caller supplies the exact numeric A0 target's real bitmap.
func DrawNativeMinimapFrame(code []byte, m FollowerCleanupMemory, c *NativeFrameRegisterContext, bitmap []byte) error {
	if len(code) < 0x33844 || !winMemoryValid(m) || c == nil || len(bitmap) != 32000 {
		return fmt.Errorf("native minimap frame backing missing")
	}
	dx, dy := binary.BigEndian.Uint16(code[0x33612:]), binary.BigEndian.Uint16(code[0x33614:])
	procedure := binary.BigEndian.Uint32(code[0x33616:])
	if procedure != 0xe196 && procedure != 0xe17a {
		return fmt.Errorf("native minimap pixel procedure%x unavailable", procedure)
	}
	c.D[7] = 0
	for y := 0; y < 64; y++ {
		c.D[6] = 0
		for x := 0; x < 64; x++ {
			c.D[2] = 0
			tile, err := m.Read8(0xf45 + (x+y*64)*4)
			if err != nil {
				return err
			}
			c.Byte(2, tile)
			c.Byte(2, code[0x33744+int(uint16(c.D[2]))])
			c.D[0] = 64
			c.Word(0, uint16(c.D[0])+uint16(c.D[6])-uint16(c.D[7]))
			c.Word(1, (uint16(c.D[6])+uint16(c.D[7]))>>1)
			c.Word(0, uint16(c.D[0])+dx)
			c.Word(1, uint16(c.D[1])+dy)
			if procedure != 0xe17a || (int16(c.D[0]) >= 0 && int16(c.D[0]) < 320 && int16(c.D[1]) >= 0 && int16(c.D[1]) < 200) {
				point, err := PlanNativeMapPoint(c)
				if err != nil {
					return err
				}
				if err := point.Paint(bitmap); err != nil {
					return err
				}
			}
			c.Word(6, uint16(c.D[6])+1)
		}
		c.Word(7, uint16(c.D[7])+1)
	}
	return nil
}
