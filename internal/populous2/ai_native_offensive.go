package populous2

import "fmt"

// NativeAIRegisterContext preserves the low words whose native continuation
// becomes significant in a leader-power choice. Type8 does not initialize
// D4/D5; callers must retain the previous policy's actual register results.
type NativeAIRegisterContext struct {
	D4, D5 uint16
	Frame  *NativeFrameRegisterContext
}

// TickComplete binds all five original policy bodies while retaining the
// native D4/D5 continuation between sides and policies. Command dispatch is
// still deferred to the caller's original $1744c update stage.
func (r *NativeAIRules) TickComplete(context *NativeAIRegisterContext, cb NativeAICallbacks) (NativeAIStep, error) {
	if context == nil {
		return NativeAIStep{}, fmt.Errorf("native AI dispatcher register context missing")
	}
	cb.Context = context
	context.Frame = cb.Frame
	if cb.Frame != nil && cb.Random != nil {
		random := cb.Random
		cb.Random = func() uint16 { v := random(); cb.Frame.D[0] = uint32(v); return v }
	}
	cb.Policy = func(kind NativeAIPolicy, god, command int) (bool, error) {
		switch kind {
		case NativeAIOffensivePower:
			return r.Offensive(god, command, context, cb)
		case NativeAIMagnetMode:
			return r.MagnetMode(god, command, cb)
		default:
			return false, fmt.Errorf("native AI complete policy outside dispatcher")
		}
	}
	return r.Tick(cb)
}

func (r *NativeAIRules) TickFrame(context *NativeFrameRegisterContext, cb NativeAICallbacks) (NativeAIStep, error) {
	if context == nil {
		return NativeAIStep{}, fmt.Errorf("native full AI frame context missing")
	}
	low := NativeAIRegisterContext{D4: uint16(context.D[4]), D5: uint16(context.D[5]), Frame: context}
	cb.Frame = context
	return r.TickComplete(&low, cb)
}

// Offensive translates $13c1c through $13d5a. It uses the generated command
// lists, original base mana prices and per-template Armageddon deadline. No
// fixed Go list of preferred powers or invented nearest enemy is used.
func (r *NativeAIRules) Offensive(god, command int, context *NativeAIRegisterContext, cb NativeAICallbacks) (chosen bool, failure error) {
	defer func() {
		if cb.Frame != nil && failure == nil {
			cb.Frame.D[0] = 0
			if chosen {
				cb.Frame.D[0] = 1
			}
		}
	}()
	m := cb.Memory
	if r == nil || !winMemoryValid(m) || context == nil {
		return false, fmt.Errorf("native AI offensive context/memory missing")
	}
	clock, err := m.Read32(0xf40)
	if err != nil {
		return false, err
	}
	if int32(clock) < 250 {
		return false, nil
	}
	identity, err := m.Read16(god + 0x18)
	if err != nil {
		return false, err
	}
	enemy := 0xe8a4
	if identity == 1 {
		enemy = 0xe9de
	}
	count, err := m.Read16(enemy + 0x1c)
	if err != nil {
		return false, err
	}
	if count == 0 {
		return false, nil
	}
	choice, err := m.Read16(god + 0x26)
	if err != nil {
		return false, err
	}
	if cb.Frame != nil {
		cb.Frame.Word(2, choice)
	}
	if choice == 0 {
		if cb.Random == nil {
			return false, fmt.Errorf("native AI choice RNG missing")
		}
		count, err := m.Read16(god + 0x94)
		if err != nil {
			return false, err
		}
		if count == 0 {
			return false, fmt.Errorf("native AI choice DIVU by zero")
		}
		bits := cb.Random()
		choice = (bits % count) * 4
		if cb.Frame != nil {
			cb.Frame.D[2] = uint32(bits)
			_ = frameDivide(cb.Frame, 2, count)
			cb.Frame.Swap(2)
			cb.Frame.Word(2, uint16(cb.Frame.D[2])*2)
		}
		if choice == 0 {
			return false, nil
		}
		if cb.Frame != nil {
			cb.Frame.Word(2, uint16(cb.Frame.D[2])*2)
		}
	}
	selected := god + int(int16(choice))
	power, err := m.Read16(selected + 0x98)
	if err != nil {
		return false, err
	}
	kind, err := m.Read16(selected + 0x9a)
	if err != nil {
		return false, err
	}
	if cb.Frame != nil {
		cb.Frame.Word(1, power)
		cb.Frame.Word(3, kind)
	}
	if kind&1 != 0 || kind > 10 {
		return false, fmt.Errorf("native AI power target type outside table")
	}
	if kind == 10 {
		population, err := m.Read32(god + 4)
		if err != nil {
			return false, err
		}
		opponent, err := m.Read32(enemy + 4)
		if err != nil {
			return false, err
		}
		deadline, err := m.Read16(god + 0x6a)
		if err != nil {
			return false, err
		}
		if cb.Frame != nil {
			cb.Frame.D[7] = uint32(deadline) << 6
		}
		if int32(uint32(deadline)<<6) < int32(clock) || int32(population) <= int32(opponent) {
			return false, m.Write16(god+0x26, 0)
		}
	}
	price, err := r.basePriceGeneral(power)
	if err != nil {
		return false, err
	}
	mana, err := m.Read32(god)
	if err != nil {
		return false, err
	}
	if int32(price) >= int32(mana) {
		return false, m.Write16(god+0x26, choice)
	}
	if err := m.Write16(god+0x26, 0); err != nil {
		return false, err
	}
	target, err := m.Read16(enemy + 0x1e)
	if err != nil {
		return false, err
	}
	actor := aiReferenceAddress(target)
	switch kind {
	case 0, 10:
		x, err := m.Read8(actor + 6)
		if err != nil {
			return false, err
		}
		y, err := m.Read8(actor + 8)
		if err != nil {
			return false, err
		}
		context.D4 = context.D4&0xff00 | uint16(x)
		context.D5 = context.D5&0xff00 | uint16(y)
		if cb.Frame != nil {
			cb.Frame.Byte(4, x)
			cb.Frame.Byte(5, y)
		}
	case 2:
		if err := m.Write16(god+0x38, power); err != nil {
			return false, err
		}
		x, err := m.Read8(actor + 6)
		if err != nil {
			return false, err
		}
		y, err := m.Read8(actor + 8)
		if err != nil {
			return false, err
		}
		if err := m.Write8(god+0x3a, x); err != nil {
			return false, err
		}
		return false, m.Write8(god+0x3b, y)
	case 4, 6:
		water := uint16(2)
		if kind == 6 {
			water = 6
		}
		if cb.Frame != nil {
			cb.Frame.Word(7, water)
		}
		found, err := r.WaterTarget(actor, water, context, m)
		if err != nil || !found {
			return false, err
		}
	case 8:
		leader, err := m.Read16(god + 8)
		if err != nil {
			return false, err
		}
		if leader == 0 {
			return false, nil
		}
		population, err := m.Read32(aiReferenceAddress(leader) + 26)
		if err != nil {
			return false, err
		}
		if int32(population) < 4096 {
			return false, nil
		}
	}
	if err := m.Write8(command+1, uint8(power)); err != nil {
		return false, err
	}
	if err := m.Write8(command+2, uint8(context.D4)); err != nil {
		return false, err
	}
	return true, m.Write8(command+3, uint8(context.D5))
}

func (r *NativeAIRules) basePriceGeneral(command uint16) (uint32, error) {
	offset, err := r.codeWord(0x210b0 + int(int16(command)))
	if err != nil {
		return 0, err
	}
	price, err := r.codeWord(0x21238 + int(int16(offset)))
	return uint32(price) * 4, err
}

// WaterTarget is original $13d5c. D4 changes to an absolute packed-coordinate
// difference when a strip reaches its required run; the next direction then
// starts from that mutated word. Its partial consecutive-water count is also
// retained across directions. These are original arithmetic continuations,
// not a geometric nearest-water search.
func (r *NativeAIRules) WaterTarget(actor int, required uint16, context *NativeAIRegisterContext, m FollowerCleanupMemory) (bool, error) {
	if r == nil || context == nil || !winMemoryValid(m) {
		return false, fmt.Errorf("native AI water-strip context/memory missing")
	}
	y, err := m.Read16(actor + 8)
	if err != nil {
		return false, err
	}
	x, err := m.Read8(actor + 6)
	if err != nil {
		return false, err
	}
	current := y&0xff00 | uint16(x)
	best, candidate, remaining := uint16(0x7fff), uint16(0), required
	probe := current
	frame := context.Frame
	if frame != nil {
		frame.Word(4, current)
		frame.Word(2, best)
		frame.D[3] = 0
		frame.Word(6, required)
		frame.Word(5, current)
	}
	for direction := 0; direction < 4; direction++ {
		offset, err := r.codeWord(0x207ea + direction*2)
		if err != nil {
			return false, err
		}
		for steps := 0; steps < 65536; steps++ {
			probe += offset
			if frame != nil {
				frame.Word(5, probe)
			}
			if probe&0xc0c0 != 0 {
				break
			}
			grid := 0xf44 + int(int16(probe&0xff00|uint16(uint8(probe)<<2)))
			tile, err := m.Read8(grid + 1)
			if err != nil {
				return false, err
			}
			if r.Properties[tile]&8 == 0 {
				remaining = required
				if frame != nil {
					frame.Word(6, required)
				}
				continue
			}
			remaining--
			if frame != nil {
				frame.Word(6, remaining)
			}
			if remaining != 0 {
				continue
			}
			current -= probe
			if int16(current) < 0 {
				current = uint16(-int16(current))
			}
			if frame != nil {
				frame.Word(4, current)
			}
			if int16(best) >= int16(current) {
				best, candidate = current, probe
				if frame != nil {
					frame.Word(2, best)
					frame.Word(3, candidate)
				}
			}
			break
		}
	}
	context.D4 = current
	context.D5 = probe
	if candidate == 0 {
		if frame != nil {
			frame.D[0] = 0
		}
		return false, nil
	}
	context.D4 = uint16(uint8(candidate))
	context.D5 = candidate >> 8
	if frame != nil {
		frame.D[4] = uint32(uint8(candidate))
		frame.Word(5, candidate>>8)
		frame.D[0] = 1
	}
	return true, nil
}
