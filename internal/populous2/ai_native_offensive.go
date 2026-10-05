package populous2

import "fmt"

// NativeAIRegisterContext preserves the low words whose native continuation
// becomes significant in a leader-power choice. Type8 does not initialize
// D4/D5; callers must retain the previous policy's actual register results.
type NativeAIRegisterContext struct{ D4, D5 uint16 }

// TickComplete binds all five original policy bodies while retaining the
// native D4/D5 continuation between sides and policies. Command dispatch is
// still deferred to the caller's original $1744c update stage.
func (r *NativeAIRules) TickComplete(context *NativeAIRegisterContext, cb NativeAICallbacks) (NativeAIStep, error) {
	if context == nil {
		return NativeAIStep{}, fmt.Errorf("native AI dispatcher register context missing")
	}
	cb.Context = context
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

// Offensive translates $13c1c through $13d5a. It uses the generated command
// lists, original base mana prices and per-template Armageddon deadline. No
// fixed Go list of preferred powers or invented nearest enemy is used.
func (r *NativeAIRules) Offensive(god, command int, context *NativeAIRegisterContext, cb NativeAICallbacks) (bool, error) {
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
		choice = (cb.Random() % count) * 4
		if choice == 0 {
			return false, nil
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
	for direction := 0; direction < 4; direction++ {
		offset, err := r.codeWord(0x207ea + direction*2)
		if err != nil {
			return false, err
		}
		for steps := 0; steps < 65536; steps++ {
			probe += offset
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
				continue
			}
			remaining--
			if remaining != 0 {
				continue
			}
			current -= probe
			if int16(current) < 0 {
				current = uint16(-int16(current))
			}
			if int16(best) >= int16(current) {
				best, candidate = current, probe
			}
			break
		}
	}
	context.D4 = current
	context.D5 = probe
	if candidate == 0 {
		return false, nil
	}
	context.D4 = uint16(uint8(candidate))
	context.D5 = candidate >> 8
	return true, nil
}
