package populous2

import "fmt"

// NativeMapPointPlan is the exact four-plane pixel addressed by $e196.
// Offset is relative to the supplied bitmap; each plane is 8000 bytes apart.
type NativeMapPointPlan struct {
	Offset     int
	Bit, Color uint8
}

func PlanNativeMapPoint(frame *NativeFrameRegisterContext) (NativeMapPointPlan, error) {
	if frame == nil {
		return NativeMapPointPlan{}, fmt.Errorf("native map point frame missing")
	}
	y := uint16(frame.D[1]) << 3
	offset := int(int16(y)) + int(int16(y<<2))
	frame.Word(1, uint16(frame.D[0]))
	frame.Word(0, uint16(frame.D[0])&7)
	frame.Byte(0, uint8(frame.D[0])^7)
	frame.Word(1, uint16(frame.D[1])>>3)
	offset += int(int16(uint16(frame.D[1])))
	frame.Word(2, uint16(frame.D[2])&15)
	color := uint8(frame.D[2])
	frame.Word(2, uint16(frame.D[2])<<4)
	return NativeMapPointPlan{Offset: offset, Bit: uint8(frame.D[0]) & 7, Color: color}, nil
}

// Paint writes only the addressed bit in the original four contiguous
// bitmap planes, preserving all neighboring pixels. Outlying offsets remain
// explicit errors rather than a fabricated pixel clamp.
func (plan NativeMapPointPlan) Paint(bitmap []byte) error {
	for plane := range 4 {
		at := plan.Offset + plane*8000
		if at < 0 || at >= len(bitmap) {
			return fmt.Errorf("native map point outside retained bitmap")
		}
		mask := byte(1 << plan.Bit)
		if plan.Color&(1<<uint(plane)) != 0 {
			bitmap[at] |= mask
		} else {
			bitmap[at] &= ^mask
		}
	}
	return nil
}
