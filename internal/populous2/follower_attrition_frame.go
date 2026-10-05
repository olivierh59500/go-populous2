package populous2

import "fmt"

type FollowerAttritionFrameCallbacks struct {
	Memory       FollowerCleanupMemory
	Frame        *NativeFrameRegisterContext
	CleanupFrame func(NativeRecordReference, *NativeFrameRegisterContext) error
}

// ApplyFollowerAttritionWithFrame is the complete $130e4/$130fc body. godAddress
// is the caller's actual A1 BSS-relative address, including explicitly supported
// aliases. It must not be reconstructed from a normalized player index.
//
// MOVE.L stores the wrapped subtraction and resets overflow before BGT. The
// stored signed result, rather than signed SUB overflow, decides survival.
// Cleanup receives MOVEQ1 D0 and its real full register output survives. Death
// retains owner/allocation and reads the hero flag again after cleanup.
func ApplyFollowerAttritionWithFrame(ref NativeRecordReference, godAddress int, cb FollowerAttritionFrameCallbacks) (bool, error) {
	m, c := cb.Memory, cb.Frame
	if c == nil || !winMemoryValid(m) {
		return false, fmt.Errorf("native attrition raw memory/frame missing")
	}
	at := cleanupRecordAddress(ref)
	population, err := m.Read32(at + 26)
	if err != nil {
		return false, err
	}
	c.D[0] = population
	amount, err := m.Read32(godAddress + 20)
	if err != nil {
		return false, err
	}
	c.D[0] -= amount
	if err := m.Write32(at+26, c.D[0]); err != nil {
		return false, err
	}
	if int32(c.D[0]) > 0 {
		c.D[0] = 0
		return false, nil
	}
	c.D[0] = 1
	if cb.CleanupFrame == nil {
		return false, fmt.Errorf("native attrition cleanup child missing")
	}
	if err := cb.CleanupFrame(ref, c); err != nil {
		return false, err
	}
	if err := m.Write16(at+10, 0x7f4); err != nil {
		return false, err
	}
	flags, err := m.Read8(at + 13)
	if err != nil {
		return false, err
	}
	if flags&2 != 0 {
		if err := m.Write16(at+10, 0x9d4); err != nil {
			return false, err
		}
		if err := m.Write8(at+22, 0x40); err != nil {
			return false, err
		}
	} else if err := m.Write8(at+22, 0x2c); err != nil {
		return false, err
	}
	c.D[0] = 1
	return true, nil
}
