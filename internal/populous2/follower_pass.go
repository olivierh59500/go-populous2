package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

// NativeFollowerPassState retains the two mutable CODE words reset at each
// active dispatch. It is shared with the town evaluator and map-marker paths.
type NativeFollowerPassState struct {
	TownCacheFlag, MinimapVariant uint16 // CODE $13350/$124a0.
}

type NativeFollowerPassFlow uint8

const (
	NativeFollowerNext NativeFollowerPassFlow = iota
	NativeFollowerCount
	NativeFollowerRedispatch
)

type NativeFollowerPassCallbacks struct {
	Memory  FollowerCleanupMemory
	Frame   *NativeFrameRegisterContext
	State   *NativeFollowerPassState
	Prepass func(NativeRecordReference, *NativeFrameRegisterContext, *NativeFollowerPassState) error
	// Body starts at the actual $112d4 table target and returns at an
	// explicit $12462/$123b4/$112b8 boundary. It must supply real register
	// outputs; a missing active-state body is never a successful no-op.
	Body     func(NativeRecordReference, uint16, *NativeFrameRegisterContext, *NativeFollowerPassState) (NativeFollowerPassFlow, error)
	MapPoint func(uint16, *NativeFrameRegisterContext) error
	Result   func(uint16, *NativeFrameRegisterContext) error
}

type NativeFollowerPassRules struct {
	Targets    [36]uint16
	MapMarkers [38]uint16
}

func DecodeNativeFollowerPassRules(exe *amiga.Executable) (NativeFollowerPassRules, error) {
	var rules NativeFollowerPassRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x20be6 {
		return rules, fmt.Errorf("native follower pass tables missing")
	}
	code := exe.Hunks[0].Data
	for i := range rules.Targets {
		rules.Targets[i] = binary.BigEndian.Uint16(code[0x112d4+i*2:])
	}
	for i := range rules.MapMarkers {
		// $12414's brief index extension has displacement $f0 (-16),
		// not an unsigned +240 displacement from the $20a9c base.
		rules.MapMarkers[i] = binary.BigEndian.Uint16(code[0x20a8c+i*2:])
	}
	return rules, nil
}

func (rules *NativeFollowerPassRules) begin(cb NativeFollowerPassCallbacks) error {
	if err := cb.Memory.Write16(0xdc2, 0); err != nil {
		return err
	}
	for god := 0xe8a4; god < 0xeb18; god += 314 {
		for _, pair := range [][2]int{{0, 0x40}, {4, 0x3c}} {
			value, err := cb.Memory.Read32(god + pair[0])
			if err != nil {
				return err
			}
			cb.Frame.D[0] = value
			maximum, err := cb.Memory.Read32(god + pair[1])
			if err != nil {
				return err
			}
			if int32(value) > int32(maximum) {
				if err := cb.Memory.Write32(god+pair[1], value); err != nil {
					return err
				}
			}
		}
		if err := cb.Memory.Write32(god+4, 0); err != nil {
			return err
		}
		for _, offset := range []int{0x1c, 0x24, 0x36, 0x2e, 0x32} {
			if err := cb.Memory.Write16(god+offset, 0); err != nil {
				return err
			}
		}
		if err := cb.Memory.Write16(god+0x20, 0xffff); err != nil {
			return err
		}
	}
	return nil
}

func (rules *NativeFollowerPassRules) count(ref NativeRecordReference, cb NativeFollowerPassCallbacks) error {
	m, c := cb.Memory, cb.Frame
	at := cleanupRecordAddress(ref)
	owner, err := m.Read8(at + 12)
	if err != nil {
		return err
	}
	c.Byte(2, owner)
	c.ExtendWord(2)
	profile, err := m.Read16(0xeb42)
	if err != nil {
		return err
	}
	ruleAddress := 0xeb2c
	if profile != 1 {
		ruleAddress = 0xeb2e
	}
	rule, err := m.Read16(ruleAddress)
	if err != nil {
		return err
	}
	c.Word(0, rule)
	view, err := m.Read16(0xf0c)
	if err != nil {
		return err
	}
	if view == 8 && (rule&0x40 == 0 || uint16(c.D[2]) == profile) {
		clock, err := m.Read16(0xf42)
		if err != nil {
			return err
		}
		c.Word(0, clock&1)
		c.Word(2, uint16(c.D[2])<<4)
		c.Word(2, uint16(c.D[2])+uint16(c.D[0])*2)
		flags, err := m.Read8(at + 13)
		if err != nil {
			return err
		}
		if flags&16 != 0 {
			cb.State.MinimapVariant = 12
		}
		c.Word(2, uint16(c.D[2])+cb.State.MinimapVariant)
		index := int(int16(uint16(c.D[2])))
		if index < 0 || index&1 != 0 || index/2 >= len(rules.MapMarkers) {
			return fmt.Errorf("native follower marker outside bounded table")
		}
		marker := rules.MapMarkers[index/2]
		c.Word(2, marker)
		if int16(marker) >= 0 {
			y, err := m.Read8(at + 8)
			if err != nil {
				return err
			}
			x, err := m.Read8(at + 6)
			if err != nil {
				return err
			}
			c.D[1] = uint32(y)
			c.D[0] = 64
			c.Word(0, uint16(c.D[0])-uint16(y))
			c.D[3] = uint32(x)
			c.Word(0, uint16(c.D[0])+uint16(x))
			c.Word(1, uint16(c.D[1])+uint16(x))
			c.Word(1, uint16(c.D[1])>>1)
			c.Word(0, uint16(c.D[0])+4)
			c.Word(1, uint16(c.D[1])+4)
			if cb.MapPoint == nil {
				return fmt.Errorf("native follower map-point continuation missing")
			}
			if err := cb.MapPoint(marker, c); err != nil {
				return err
			}
		}
	}
	// $12448 reloads the raw owner after the actual marker callback.
	owner, err = m.Read8(at + 12)
	if err != nil {
		return err
	}
	c.D[0] = uint32(owner) * 314
	population, err := m.Read32(at + 26)
	if err != nil {
		return err
	}
	c.D[0] = population
	god := 0xe76a + int(int16(uint16(uint32(owner)*314)))
	total, err := m.Read32(god + 4)
	if err != nil {
		return err
	}
	return m.Write32(god+4, total+population)
}

// Tick follows $11252's physical record order. Initialization and inactive
// slots preserve caller D1-D7, including an empty pool. Register-bearing
// actor/image/result bodies remain explicit until their actual adapters bind.
func (rules *NativeFollowerPassRules) Tick(cb NativeFollowerPassCallbacks) error {
	if rules == nil || cb.Frame == nil || cb.State == nil || !winMemoryValid(cb.Memory) {
		return fmt.Errorf("native follower pass backing missing")
	}
	if err := rules.begin(cb); err != nil {
		return err
	}
	for slot := 0; slot < 400; slot++ {
		ref := NativeRecordReference(uint16(slot * 52))
		owner, err := cb.Memory.Read8(cleanupRecordAddress(ref) + 12)
		if err != nil {
			return err
		}
		if owner == 0 {
			continue
		}
		for dispatch := 0; dispatch < 64; dispatch++ {
			cb.State.TownCacheFlag, cb.State.MinimapVariant = 0, 0
			if cb.Prepass == nil {
				return fmt.Errorf("native follower prepass continuation missing")
			}
			if err := cb.Prepass(ref, cb.Frame, cb.State); err != nil {
				return err
			}
			state, err := cb.Memory.Read8(cleanupRecordAddress(ref) + 22)
			if err != nil {
				return err
			}
			cb.Frame.Byte(0, state)
			cb.Frame.ExtendWord(0)
			if state&1 != 0 || int(state/2) >= len(rules.Targets) {
				return fmt.Errorf("native follower state outside bounded dispatch table")
			}
			target := rules.Targets[state/2]
			cb.Frame.Word(0, target)
			if cb.Body == nil {
				return fmt.Errorf("native follower body continuation missing")
			}
			flow, err := cb.Body(ref, target, cb.Frame, cb.State)
			if err != nil {
				return err
			}
			if flow == NativeFollowerRedispatch {
				if dispatch == 63 {
					return fmt.Errorf("native follower redispatch exceeds bounded window")
				}
				continue
			}
			if flow == NativeFollowerCount {
				if err := rules.count(ref, cb); err != nil {
					return err
				}
			} else if flow != NativeFollowerNext {
				return fmt.Errorf("native follower body returned an unknown boundary")
			}
			break
		}
	}
	mode, err := cb.Memory.Read16(0xeb44)
	if err != nil || mode == 8 {
		return err
	}
	for god := 0xe8a4; god < 0xeb18; god += 314 {
		population, err := cb.Memory.Read32(god + 4)
		if err != nil {
			return err
		}
		if population != 0 {
			continue
		}
		identity, err := cb.Memory.Read16(god + 0x18)
		if err != nil {
			return err
		}
		cb.Frame.Word(0, identity)
		if cb.Result == nil {
			return fmt.Errorf("native follower result continuation missing")
		}
		return cb.Result(identity, cb.Frame)
	}
	return nil
}
