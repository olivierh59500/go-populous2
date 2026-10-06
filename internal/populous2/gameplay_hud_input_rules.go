package populous2

import (
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeGameplayHUDInputRules struct{ Commands NativeCommandRules }

func DecodeNativeGameplayHUDInputRules(exe *amiga.Executable) (NativeGameplayHUDInputRules, error) {
	r, e := DecodeNativeCommandRules(exe)
	return NativeGameplayHUDInputRules{Commands: r}, e
}

type NativeGameplayHUDInputCallbacks struct {
	NativeStartupResetFrameCallbacks
}
type NativeGameplayHUDInputStep struct {
	Complete, Waiting, FlagsKnown, Zero, Negative bool
	PC                                            int
}

// NativeGameplayHUDCost runs$14768. Its A0/D1-D2 MOVEM leaves the caller's
// complete pointer/register values intact, including incoming upper words.
func NativeGameplayHUDCost(r *NativeGameplayHUDInputRules, cb NativeGameplayHUDInputCallbacks, a *[7]NativeRequesterAddress) error {
	if r == nil || cb.Frame == nil || a == nil {
		return fmt.Errorf("native HUD price context missing")
	}
	context := cb.Frame.CommandContext()
	if e := r.Commands.Cost(&context, cb.Memory); e != nil {
		return e
	}
	cb.Frame.SetCommandContext(context)
	return nil
}

// NativeGameplayHUDAdmission is$147e0 with its actual A1 continuation. The
// final MOVEM.W preserves flags from D4/compare rather than testing restoredD0.
func NativeGameplayHUDAdmission(r *NativeGameplayHUDInputRules, cb NativeGameplayHUDInputCallbacks, a *[7]NativeRequesterAddress) (bool, error) {
	if r == nil || cb.Frame == nil || a == nil {
		return false, fmt.Errorf("native HUD admission context missing")
	}
	free, e := cb.Memory.Read16(0xf0e)
	if e != nil {
		return false, e
	}
	context := cb.Frame.CommandContext()
	allowed, e := r.Commands.Admit(&context, cb.Memory)
	if e != nil {
		return false, e
	}
	cb.Frame.SetCommandContext(context)
	if free == 0 {
		a[1] = NativeRequesterAddress{Address: cb.CodeBase + 0x210b0, Code: true}
		price, e := r.Commands.word(0x210b0 + int(int16(uint16(cb.Frame.D[2]))))
		if e != nil {
			return false, e
		}
		if int16(price) >= 0 {
			a[1] = NativeRequesterAddress{Address: uint32(int64(cb.Frame.AddressBase+0xe76a) + int64(int16(cb.Frame.D[5])))}
		}
	}
	return allowed, nil
}
