package populous2

import "fmt"

// NativeInterruptVectorCallbacks supplies the actual low vector RAM and the
// saved-handler fields in BSS. INTENAR and hardware writes are explicit host
// boundaries; ordinary RAM access never invents memory-mapped hardware.
type NativeInterruptVectorCallbacks struct {
	RAM, Memory    FollowerCleanupMemory
	CodeBase       uint32
	Frame          *NativeFrameRegisterContext
	ReadHardware16 func(uint32) (uint16, error)
	Hardware       func(NativeFrameHardwareWrite) error
}

type NativeInterruptVectorStep struct {
	Complete bool
	Hardware []NativeFrameHardwareWrite
}

// RunNativeInterruptVectors executes original 39E (install) or 370 (swap back).
// The real game installs vector5 to 43E/RTE, allowing DIVU-zero to resume with
// its dividend unchanged. 370 swaps the saved and active vectors; it does not
// clear them. A0 and all other address registers survive the source save/restore.
func RunNativeInterruptVectors(routine int, cb NativeInterruptVectorCallbacks) (out NativeInterruptVectorStep, failure error) {
	if cb.Frame == nil || cb.CodeBase == 0 || !winMemoryValid(cb.RAM) || !winMemoryValid(cb.Memory) {
		return out, fmt.Errorf("native interrupt vector backing missing")
	}
	c, ram, m := cb.Frame, cb.RAM, cb.Memory
	writeHardware := func(pc int, value uint16) error {
		p := NativeFrameHardwareWrite{PC: uint32(pc), Address: 0xdff09a, Value: uint32(value), Width: 2}
		out.Hardware = append(out.Hardware, p)
		if cb.Hardware != nil {
			return cb.Hardware(p)
		}
		return nil
	}
	switch routine {
	case 0x39e:
		if cb.ReadHardware16 == nil {
			return out, fmt.Errorf("native interrupt INTENAR reader missing")
		}
		status, err := cb.ReadHardware16(0xdff01c)
		if err != nil {
			return out, err
		}
		c.Word(0, status)
		c.Word(0, uint16(c.D[0])&0x70)
		if err = writeHardware(0x3aa, uint16(c.D[0])); err != nil {
			return out, err
		}
		irq, err := ram.Read32(0x6c)
		if err != nil {
			return out, err
		}
		if err = m.Write32(6, irq); err != nil {
			return out, err
		}
		if err = ram.Write32(0x6c, cb.CodeBase+0x3ec); err != nil {
			return out, err
		}
		divide, err := ram.Read32(0x14)
		if err != nil {
			return out, err
		}
		if err = m.Write32(2, divide); err != nil {
			return out, err
		}
		if err = ram.Write32(0x14, cb.CodeBase+0x43e); err != nil {
			return out, err
		}
		c.Word(0, uint16(c.D[0])|0x8020)
		if err = writeHardware(0x3e0, uint16(c.D[0])); err != nil {
			return out, err
		}
	case 0x370:
		for _, p := range []struct{ physical, saved int }{{0x14, 2}, {0x6c, 6}} {
			value, err := ram.Read32(p.physical)
			if err != nil {
				return out, err
			}
			c.D[0] = value
			old, err := m.Read32(p.saved)
			if err != nil {
				return out, err
			}
			if err = ram.Write32(p.physical, old); err != nil {
				return out, err
			}
			if err = m.Write32(p.saved, c.D[0]); err != nil {
				return out, err
			}
		}
	default:
		return out, fmt.Errorf("native interrupt vector routine%x unsupported", routine)
	}
	out.Complete = true
	return out, nil
}

// nativeResultScoreDivide models only 39A8's actual installed exception/RTE
// continuation. Other DIVU sites keep their existing explicit contracts.
func nativeResultScoreDivide(h *NativeRuntimeHost, c *NativeFrameRegisterContext) error {
	divisor := uint16(c.D[1])
	if divisor != 0 {
		return frameDivide(c, 0, divisor)
	}
	handler, err := h.Memory.RAM.Read32(0x14)
	if err != nil {
		return err
	}
	opcode, err := h.Memory.Code.Read16(0x43e)
	if err != nil {
		return err
	}
	if handler != h.Memory.CodeBase+0x43e || opcode != 0x4e73 {
		return fmt.Errorf("native result DIVU-zero requires the installed 43E RTE handler")
	}
	// The 68000 stacks the post-instruction PC and SR. The original RTE restores
	// those and changes no D register; the following MOVE.W sets the next CCR.
	return nil
}
