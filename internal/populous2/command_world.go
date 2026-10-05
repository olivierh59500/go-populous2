package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

// NativeCommandRules retains the original indexed cost, category and command
// tables. Signed indices can address adjacent CODE bytes; they are not mapped
// through the public SpellID enumeration.
type NativeCommandRules struct{ Code []byte }

func DecodeNativeCommandRules(exe *amiga.Executable) (NativeCommandRules, error) {
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x21280 {
		return NativeCommandRules{}, fmt.Errorf("native command tables missing")
	}
	return NativeCommandRules{Code: append([]byte(nil), exe.Hunks[0].Data...)}, nil
}

func (r *NativeCommandRules) byte(at int) (uint8, error) {
	if r == nil || at < 0 || at >= len(r.Code) {
		return 0, fmt.Errorf("native command CODE byte%x unavailable", at)
	}
	return r.Code[at], nil
}

func (r *NativeCommandRules) word(at int) (uint16, error) {
	if at&1 != 0 {
		return 0, fmt.Errorf("unaligned native command CODE word%x", at)
	}
	if r == nil || at < 0 || at+2 > len(r.Code) {
		return 0, fmt.Errorf("native command CODE word%x unavailable", at)
	}
	return binary.BigEndian.Uint16(r.Code[at:]), nil
}

func commandByte(c *NativeCommandRegisterContext, reg int, v uint8) {
	c.D[reg] = c.D[reg]&0xffffff00 | uint32(v)
}

func commandWord(c *NativeCommandRegisterContext, reg int, v uint16) {
	c.D[reg] = c.D[reg]&0xffff0000 | uint32(v)
}

func commandExtend(c *NativeCommandRegisterContext, reg int) {
	commandWord(c, reg, uint16(int16(int8(uint8(c.D[reg])))))
}

func commandGod(word uint16) int {
	return 0xe76a + int(int16(uint16(uint32(word)*314)))
}

// Cost is $14768. D1/D2 are saved as longs; D0's final result is an unsigned
// word even when the raw table contains $ffff. Owner multiplication uses its
// incoming word, while the category index uses a signed byte from CODE.
func (r *NativeCommandRules) Cost(c *NativeCommandRegisterContext, m FollowerCleanupMemory) error {
	if c == nil || !winMemoryValid(m) {
		return fmt.Errorf("native command price memory/context missing")
	}
	category, err := r.byte(0x147bc + int(int16(uint16(c.D[0]))))
	if err != nil {
		return err
	}
	xp, err := m.Read8(commandGod(uint16(c.D[1])) + 0x52 + int(int8(category)))
	if err != nil {
		return err
	}
	divisor, err := r.byte(0x2105e + int(xp>>5))
	if err != nil {
		return err
	}
	price, err := r.word(0x21238 + int(int16(uint16(c.D[0])*2)))
	if err != nil {
		return err
	}
	if price != 0xffff && divisor != 0 {
		price -= price / uint16(divisor)
	}
	c.D[0] = uint32(price)
	return nil
}

// Admit is the upstream $147e0 price check. It does not check the power flags
// and is not repeated by $17500. MOVEM.W restores sign-extended D0/D1; an
// unavailable negative table word retains D4's incoming upper word.
func (r *NativeCommandRules) Admit(c *NativeCommandRegisterContext, m FollowerCleanupMemory) (bool, error) {
	if c == nil || !winMemoryValid(m) {
		return false, fmt.Errorf("native command admission memory/context missing")
	}
	d0, d1 := uint16(c.D[0]), uint16(c.D[1])
	defer func() { c.D[0], c.D[1] = uint32(int32(int16(d0))), uint32(int32(int16(d1))) }()
	free, err := m.Read16(0xf0e)
	if err != nil {
		return false, err
	}
	if free != 0 {
		c.D[4] = 0
		return true, nil
	}
	commandExtend(c, 2)
	priceWord, err := r.word(0x210b0 + int(int16(uint16(c.D[2]))))
	if err != nil {
		return false, err
	}
	commandWord(c, 4, priceWord)
	if int16(priceWord) < 0 {
		return false, nil
	}
	commandWord(c, 0, priceWord>>1)
	commandWord(c, 1, uint16(c.D[3]))
	if err := r.Cost(c, m); err != nil {
		return false, err
	}
	c.D[4] = c.D[0] * 4
	c.D[5] = uint32(uint16(c.D[3])) * 314
	mana, err := m.Read32(0xe76a + int(int16(uint16(c.D[5]))))
	if err != nil {
		return false, err
	}
	if c.D[4] > mana {
		c.D[4] = 1
		return false, nil
	}
	c.D[4] = 0
	return true, nil
}

// debit is $17e38. Its statistics precede the price call. A failed primitive
// never reaches this body unless its original command handler ignores flags.
func (r *NativeCommandRules) debit(caller int, c *NativeCommandRegisterContext, m FollowerCleanupMemory) error {
	free, err := m.Read16(0xf0e)
	if err != nil || free != 0 {
		return err
	}
	owner, err := m.Read8(caller)
	if err != nil {
		return err
	}
	commandByte(c, 1, owner)
	commandExtend(c, 1)
	mode, err := m.Read16(0xeb44)
	if err != nil || mode == 8 || owner == 3 {
		return err
	}
	c.D[2] = uint32(uint16(c.D[1])) * 314
	god := 0xe76a + int(int16(uint16(c.D[2])))
	commandWord(c, 0, uint16(c.D[0])>>1)
	commandWord(c, 2, uint16(c.D[0]))
	dividend := c.D[2]
	if dividend/6 <= 0xffff { // DIVU overflow retains the complete dividend.
		c.D[2] = dividend%6<<16 | dividend/6
	}
	c.D[2] = c.D[2]<<16 | c.D[2]>>16
	commandWord(c, 2, uint16(c.D[2])+1)
	stat, err := m.Read16(god + 0x138)
	if err != nil {
		return err
	}
	if err := m.Write16(god+0x138, stat+uint16(c.D[2])); err != nil {
		return err
	}
	if err := r.Cost(c, m); err != nil {
		return err
	}
	c.D[0] *= 4
	mana, err := m.Read32(god)
	if err != nil {
		return err
	}
	result := mana - c.D[0]
	// BGT follows the mathematical signed subtraction, including V.
	if int64(int32(mana))-int64(int32(c.D[0])) <= 0 {
		result = 0
	}
	return m.Write32(god, result)
}

type NativeCommandCall struct {
	Routine, Caller int
	Context         *NativeCommandRegisterContext
}

type NativeCommandCallbacks struct {
	Memory FollowerCleanupMemory
	// Call performs the actual inner source routine and updates every data
	// register it changes. The returned bool is the original Z flag. UI/I/O
	// boundaries use this same explicit call rather than fabricated success.
	Call func(NativeCommandCall) (bool, error)
}

type NativeCommandStep struct {
	Command         uint8
	Paused, Debited bool
	Calls           []int
}

// Execute translates the normal-player $17500 dispatcher. The ten-byte record
// remains intact here; only the later $1744c scheduler clears byte1/word2.
func (r *NativeCommandRules) Execute(caller int, c *NativeCommandRegisterContext, cb NativeCommandCallbacks) (NativeCommandStep, error) {
	step := NativeCommandStep{Calls: []int{}}
	m := cb.Memory
	if c == nil || !winMemoryValid(m) {
		return step, fmt.Errorf("native command dispatcher memory/context missing")
	}
	owner, err := m.Read8(caller)
	if err != nil {
		return step, err
	}
	command, err := m.Read8(caller + 1)
	if err != nil {
		return step, err
	}
	step.Command = command
	commandByte(c, 0, command)
	if command == 0 {
		return step, nil
	}
	commandExtend(c, 0)
	pause, err := m.Read16(0xf3c)
	if err != nil {
		return step, err
	}
	if pause != 0 && int16(uint16(c.D[0])) <= 102 {
		step.Paused = true
		return step, nil
	}
	// The original table is indexed by a signed word and has no bounds guard.
	// Retain adjacent-table aliases that lead to a translated handler; reject
	// an actual unmapped target rather than inventing a command.
	offset, err := r.word(0x17524 + int(int16(uint16(c.D[0]))))
	if err != nil {
		return step, err
	}
	commandWord(c, 0, offset)
	handler := 0x17524 + int(int16(offset))
	x, err := m.Read8(caller + 2)
	if err != nil {
		return step, err
	}
	y, err := m.Read8(caller + 3)
	if err != nil {
		return step, err
	}
	call := func(routine int) (bool, error) {
		if cb.Call == nil {
			return false, fmt.Errorf("native command inner routine%x missing", routine)
		}
		step.Calls = append(step.Calls, routine)
		return cb.Call(NativeCommandCall{Routine: routine, Caller: caller, Context: c})
	}
	coords := func(clearOwner bool) {
		c.D[0], c.D[1] = uint32(x), uint32(y)
		if clearOwner {
			c.D[2] = uint32(owner)
		} else {
			commandByte(c, 2, owner)
		}
	}
	sound := func(cue uint16) error { commandWord(c, 0, cue); _, e := call(0x184f6); return e }
	debit := func(power uint16) error {
		commandWord(c, 0, power)
		step.Debited = true
		return r.debit(caller, c, m)
	}
	metric := func(value uint16) error {
		c.D[2] = uint32(owner) * 314
		god := 0xe76a + int(int16(uint16(c.D[2])))
		old, e := m.Read16(god + 0x44)
		if e != nil {
			return e
		}
		return m.Write16(god+0x44, old+value)
	}
	// Most effect handlers have the same MOVEQ coordinate setup, optional
	// BEQ admission, cue and raw power word. Their helper context is mutable.
	type effect struct {
		routine    int
		gate       bool
		cue, power uint16
		charged    bool
	}
	effects := map[int]effect{
		0x17638: {0x15b7c, true, 0x47e, 0x30, true}, 0x1772c: {0x15c3e, true, 0x438, 0x26, true},
		0x1775a: {0x15cc8, true, 0xbe, 0x3e, true}, 0x17788: {0x15fda, false, 0, 0x12, true},
		0x177a8: {0x15de2, false, 0, 0, false}, 0x177c4: {0x15e8a, false, 0, 0x24, true},
		0x177e4: {0x15f80, false, 0, 0, false}, 0x178d8: {0x1648c, true, 0x488, 0x32, true},
		0x179a2: {0x16744, false, 0, 0, false}, 0x179f8: {0xdf68, true, 0, 0x48, true},
		0x17a1c: {0x16938, false, 0, 0x40, true}, 0x17a3c: {0x169cc, false, 0, 0x10, true},
		0x17a5c: {0x16a62, false, 0, 0x0e, true}, 0x17a7c: {0x16af4, false, 0xaa, 0x44, true},
		0x17aa6: {0x16cc8, true, 0x4ce, 0x34, true}, 0x17ad4: {0x16bfc, true, 0x136, 0x28, true},
		0x17bba: {0x1730e, true, 0x1e, 0x06, true},
	}
	if effect, ok := effects[handler]; ok {
		coords(true)
		zero, e := call(effect.routine)
		if e != nil || effect.gate && zero {
			return step, e
		}
		if effect.cue != 0 {
			if e := sound(effect.cue); e != nil {
				return step, e
			}
		}
		if effect.charged {
			return step, debit(effect.power)
		}
		return step, nil
	}
	switch handler {
	case 0x17e96, 0x17cf0, 0x17cf4:
		return step, nil
	case 0x176bc, 0x176d8, 0x176f4, 0x17710:
		commandByte(c, 2, owner)
		commandExtend(c, 2)
		c.D[2] = uint32(uint16(c.D[2])) * 314
		mode := map[int]uint16{0x176bc: 14, 0x176d8: 18, 0x176f4: 20, 0x17710: 16}[handler]
		return step, m.Write16(0xe76a+int(int16(uint16(c.D[2])))+12, mode)
	case 0x175a2, 0x1761a:
		coords(false)
		if handler == 0x175a2 {
			scenarioAt, mask := 0xeb2c, uint8(0x3f)
			if owner != 1 {
				scenarioAt, mask = 0xeb2e, 0x2f
			}
			scenario, e := m.Read16(scenarioAt)
			if e != nil {
				return step, e
			}
			commandWord(c, 2, scenario)
			commandByte(c, 3, mask)
			if scenario&4 == 0 {
				commandWord(c, 3, 0)
			}
			if scenario&8 != 0 {
				return step, nil
			}
			if _, e := call(0xd80c); e != nil {
				return step, e
			}
		} else {
			zero, e := call(0x12f8a)
			if e != nil || zero {
				return step, e
			}
		}
		commandByte(c, 2, owner)
		commandExtend(c, 2)
		commandWord(c, 1, uint16(c.D[2]))
		c.D[2] = uint32(uint16(c.D[2])) * 314
		god := 0xe76a + int(int16(uint16(c.D[2])))
		c.D[0] = 0
		if e := r.Cost(c, m); e != nil {
			return step, e
		}
		changes, e := m.Read16(0xdd2)
		if e != nil {
			return step, e
		}
		c.D[0] = uint32(uint16(c.D[0])) * uint32(changes) * 4
		mana, e := m.Read32(god)
		if e != nil {
			return step, e
		}
		result := mana - c.D[0]
		if int64(int32(mana))-int64(int32(c.D[0])) < 0 {
			result = 0
		}
		step.Debited = true
		return step, m.Write32(god, result)
	case 0x17666:
		coords(false)
		commandExtend(c, 2)
		c.D[2] = uint32(uint16(c.D[2])) * 314
		leader, e := m.Read16(0xe76a + int(int16(uint16(c.D[2]))) + 8)
		if e != nil || leader == 0 {
			return step, e
		}
		commandByte(c, 2, owner)
		if _, e := call(0x13fe4); e != nil {
			return step, e
		}
		cue := uint16(0x4ec)
		if owner == 1 {
			cue = 0x4f6
		}
		if e := sound(cue); e != nil {
			return step, e
		}
		return step, debit(2)
	case 0x17800, 0x17966:
		coords(true)
		routine, power := 0x1626c, uint16(0x1a)
		if handler == 0x17966 {
			routine, power = 0x1677a, 0x18
		}
		zero, e := call(routine)
		if e != nil || zero {
			return step, e
		}
		if e := metric(1); e != nil {
			return step, e
		}
		return step, debit(power)
	case 0x1783c, 0x17856, 0x17870, 0x1788a, 0x178a4, 0x178be:
		selector := map[int]uint32{0x1783c: 0, 0x17856: 2, 0x17870: 4, 0x1788a: 6, 0x178a4: 8, 0x178be: 10}[handler]
		power := map[int]uint16{0x1783c: 4, 0x17856: 0x14, 0x17870: 0x20, 0x1788a: 0x2a, 0x178a4: 0x36, 0x178be: 0x42}[handler]
		c.D[2], c.D[0] = uint32(owner), selector
		zero, e := call(0x142d4)
		if e != nil || zero {
			return step, e
		}
		return step, debit(power)
	case 0x17906, 0x17b24, 0x17b7a:
		coords(true)
		routine, power, cue := 0x165da, uint16(0x1c), uint16(0x424)
		if handler == 0x17b7a {
			c.D[3] = uint32(r.Code[0x17bb6+int(x>>6)])
			routine, power, cue = 0x172a4, 0x2c, 0x528
		} else {
			commandByte(c, 3, x)
			if handler == 0x17906 {
				commandWord(c, 3, (uint16(c.D[3])>>6)&3)
				commandByte(c, 3, r.Code[0x17962+int(uint16(c.D[3]))])
			} else {
				commandWord(c, 3, (uint16(c.D[3])>>5)&6)
			}
			c.D[4] = 0
			readXP := int8(owner) <= 2
			xpOffset, baseAt := 0x54, 0x20d66
			if handler == 0x17b24 {
				ownerCommand, e := m.Read16(caller)
				if e != nil {
					return step, e
				}
				readXP = int16(ownerCommand) <= 2
				xpOffset, baseAt = 0x57, 0x172a2
				routine, power, cue = 0x171ea, 0x3c, 0
			}
			if readXP {
				xp, e := m.Read8(commandGod(uint16(owner)) + xpOffset)
				if e != nil {
					return step, e
				}
				c.D[4] = uint32(xp)
			}
			base, e := r.word(baseAt)
			if e != nil {
				return step, e
			}
			commandWord(c, 4, uint16(c.D[4])+base)
		}
		commandWord(c, 0, uint16(c.D[0])&63)
		zero, e := call(routine)
		if e != nil || handler != 0x17906 && zero {
			return step, e
		}
		if cue != 0 {
			if e := sound(cue); e != nil {
				return step, e
			}
		}
		return step, debit(power)
	case 0x179be:
		c.D[2] = uint32(owner)
		commandWord(c, 3, uint16(y)<<8|uint16(x))
		if _, e := call(0xda0a); e != nil {
			return step, e
		}
		if uint16(c.D[0]) == 0 {
			return step, nil
		}
		if e := metric(uint16(c.D[0])); e != nil {
			return step, e
		}
		return step, debit(0x0c)
	case 0x17b02:
		if e := sound(0x51e); e != nil {
			return step, e
		}
		c.D[2] = uint32(owner)
		zero, e := call(0x13022)
		if e != nil || zero {
			return step, e
		}
		return step, debit(8)
	case 0x17be8, 0x17c04:
		commandWord(c, 0, uint16(y)<<8|uint16(uint8(x*4)))
		v := uint16(1)
		if handler == 0x17c04 {
			v = 2
		}
		commandWord(c, 2, v)
		_, e := call(0x10cbe)
		return step, e
	case 0x17c20, 0x17c3a:
		coords(false)
		routine := 0xdb26
		if handler == 0x17c3a {
			routine = 0xdd1c
		}
		_, e := call(routine)
		return step, e
	case 0x17c54, 0x17c6e, 0x17c88, 0x17ca2, 0x17cbc, 0x17cd6:
		c.D[0], c.D[1] = uint32(x), uint32(y)
		commandWord(c, 2, uint16(2+(handler-0x17c54)/0x1a*2))
		_, e := call(0x131cc)
		return step, e
	case 0x17cf8:
		c.D[0], c.D[1] = uint32(x), uint32(y)
		_, e := call(0xdff4)
		return step, e
	case 0x17d0e:
		return step, m.Write16(0xf3c, ^pause)
	case 0x17d18, 0x17d22:
		if handler == 0x17d18 {
			seed, e := m.Read32(0xeb24)
			if e != nil {
				return step, e
			}
			if e := m.Write32(0xeb28, seed); e != nil {
				return step, e
			}
		}
		if _, e := call(0x102e4); e != nil {
			return step, e
		}
		_, e := call(0x10ad8)
		return step, e
	case 0x17d3e, 0x17d58:
		// These two words are mutable CODE UI state, owned by the explicit UI
		// callback together with the original $3f92 routine.
		_, e := call(handler)
		return step, e
	case 0x17d6a:
		_, e := call(handler)
		return step, e
	case 0x17d9e:
		commandByte(c, 1, x)
		commandByte(c, 2, owner)
		_, e := call(0x4f8e)
		return step, e
	case 0x17db0:
		return step, m.Write16(0xdce, 1)
	case 0x17dbc:
		c.D[0] = uint32(owner) * 314
		god := 0xe76a + int(int16(uint16(c.D[0])))
		commandWord(c, 0, uint16(x)<<8|uint16(y))
		mana, e := m.Read32(god)
		if e != nil {
			return step, e
		}
		return step, m.Write32(god, mana+c.D[0])
	case 0x17dda:
		terrain, e := m.Read16(0xeb22)
		if e != nil {
			return step, e
		}
		terrain++
		if terrain == 4 {
			terrain = 0
		}
		if e := m.Write16(0xeb22, terrain); e != nil {
			return step, e
		}
		_, e = call(handler)
		return step, e
	case 0x17e1c:
		selected, e := m.Read16(0xeb42)
		if e != nil {
			return step, e
		}
		v := uint16(2)
		if selected != 1 {
			v = 1
		}
		commandWord(c, 0, v)
		_, e = call(0x111ae)
		return step, e
	default:
		return step, fmt.Errorf("native command byte%d dispatch target%x unavailable", command, handler)
	}
}

func (w *World) admitNativeCommand(rules *NativeCommandRules, context *NativeCommandRegisterContext) (bool, error) {
	if w == nil || w.Core == nil {
		return false, fmt.Errorf("native command World missing")
	}
	return rules.Admit(context, w.nativeCleanupMemory())
}

// NativeCommandWorldBindings supplies retained controller scratch and the
// remaining explicit source boundaries. UI actions are delivered as routines
// to the presentation owner; a missing boundary remains an error.
type NativeCommandWorldBindings struct {
	WallRules     *NativeWallRules
	WallPlacement *NativeWallPlacementState
	Other         func(NativeCommandCall) (bool, error)
	Trace         func(NativeCommandCall)
}

func (w *World) nativeNormalCommandCallbacks(bindings NativeCommandWorldBindings) NativeCommandCallbacks {
	return NativeCommandCallbacks{Memory: w.nativeCleanupMemory(), Call: func(call NativeCommandCall) (bool, error) {
		if bindings.Trace != nil {
			bindings.Trace(call)
		}
		c, m := call.Context, w.nativeCleanupMemory()
		owner, x, y := uint16(c.D[2]), uint8(c.D[0]), uint8(c.D[1])
		switch call.Routine {
		case 0x171ea:
			return w.commandBasaltCreation(call)
		case 0x15de2, 0x15e8a, 0x15f80:
			return w.commandLightning(call)
		case 0x142d4:
			return w.commandHeroCreation(call, bindings.Trace)
		case 0x1677a, 0x16744:
			return w.commandRoad(call)
		case 0x1648c, 0x16bfc:
			return w.commandWeatherCreation(call)
		case 0x16938, 0x169cc, 0x16a62:
			return w.commandGroundCreation(call)
		case 0x184f6:
			return false, w.nativeEntryCallbacks().Sound(uint16(c.D[0]))
		case 0x15b7c, 0x15c3e:
			cb := w.nativePrimitiveCallbacks()
			originalRandom := cb.Random
			firstDraw := uint16(0)
			draws := 0
			cb.Random = func() uint16 {
				v := originalRandom()
				if draws == 0 {
					firstDraw = v
				}
				draws++
				return v
			}
			originalLink := cb.Link
			cb.Link = func(ref NativeRecordReference) error {
				at := cleanupRecordAddress(ref)
				xx, e := m.Read8(at + 6)
				if e != nil {
					return e
				}
				yy, e := m.Read8(at + 8)
				if e != nil {
					return e
				}
				head, e := m.Read16(tsunamiGrid(uint16(yy)<<8|uint16(xx)) + 2)
				if e != nil {
					return e
				}
				commandWord(c, 1, head)
				return originalLink(ref)
			}
			var step NativePrimitiveCreation
			var err error
			if call.Routine == 0x15c3e {
				step, err = w.PrimitiveCreators.CreateWhirlwind(owner, x, y, cb)
				if step.Created && int8(uint8(owner)) <= 2 {
					c.D[2] = uint32(owner) * 314
				}
			} else {
				step, err = w.PrimitiveCreators.CreateFireColumn(owner, x, y, cb)
				packed := uint16(y)<<8 | uint16(x)
				packed += uint16(w.PrimitiveCreators.FireColumnJitter[(firstDraw%18)/2])
				if !step.Created {
					commandWord(c, 1, packed)
				}
			}
			if err != nil {
				return false, err
			}
			c.D[0] = 0
			if step.Created {
				c.D[0] = 1
			}
			return !step.Created, nil
		case 0x15fda:
			_, err := w.NativeFungus.Create(owner, x, y, w.nativeFungusCallbacks())
			// $15fda saves D0-D3 and changes no other data registers. Its
			// caller unconditionally reaches debit regardless of returned Z.
			return false, err
		case 0xda0a:
			step, err := w.ForestNative.Plant(owner, NativePackedTile(uint16(c.D[3])), w.nativeForestCallbacks())
			if err != nil {
				return false, err
			}
			c.D[0] = uint32(step.Count)
			return step.Count == 0, nil
		case 0x1626c:
			if bindings.WallRules == nil || bindings.WallPlacement == nil {
				return false, fmt.Errorf("native command wall rules/scratch missing")
			}
			step, err := bindings.WallRules.Create(owner, x, y, bindings.WallPlacement, w.nativeWallCallbacks(uint16(c.D[7])))
			return !step.Created, err // $1626c saves all eight data registers.
		case 0x13fe4:
			god := heroGodAddress(uint8(owner))
			ref, err := m.Read16(god + 10)
			if err != nil {
				return false, err
			}
			if err := w.nativeRuntimeUnlink(NativeRecordReference(ref)); err != nil {
				return false, err
			}
			clamp := func(v uint8) uint8 {
				if int8(v) < 0 {
					return 0
				}
				return min(v, 63)
			}
			at := cleanupRecordAddress(NativeRecordReference(ref))
			if err := m.Write16(at+6, uint16(clamp(x))*256+128); err != nil {
				return false, err
			}
			if err := m.Write16(at+8, uint16(clamp(y))*256+128); err != nil {
				return false, err
			}
			return false, w.nativeRuntimeInsert(NativeRecordReference(ref))
		case 0x15cc8:
			// The admission pass leaves D3-D5 live; D0-D2 are restored by
			// MOVEM. The initial terrain stamp leaves D4 at its sentinel.
			origin := uint16(y)<<8 | uint16(x)
			commandWord(c, 3, origin)
			for _, offset := range w.NativeWhirlwind.Whirlpool.Footprint {
				parcel := origin + offset
				commandWord(c, 4, parcel)
				commandWord(c, 5, parcel&0xc0c0)
				if parcel&0xc0c0 != 0 {
					return true, nil
				}
				commandWord(c, 4, parcel&0xff00|uint16(uint8(parcel)*4))
				tile, err := m.Read8(tsunamiGrid(parcel) + 1)
				if err != nil {
					return false, err
				}
				if tile != 0 {
					return true, nil
				}
			}
			commandWord(c, 4, 0xff9d)
			step, err := w.NativeWhirlwind.CreateWaterChild(owner, x, y, m)
			return !step.Created, err
		case 0x16cc8:
			// This allocation's XP multiplication changes only D2; D1 and
			// the caller's remaining registers survive unchanged.
			ref, created, err := w.VolcanoRules.Create(uint8(owner), x, y, w.volcanoCallbacks())
			if err != nil {
				return false, err
			}
			c.D[0] = 0
			if created {
				c.D[0] = 1
				if int16(owner) <= 2 {
					c.D[2] = uint32(owner) * 314
				}
				if loc, ok := LocateNativeRecord(ref); ok {
					w.NativeEnvironment[loc.Index] = NativeEnvironmentVolcano
				}
			}
			return !created, nil
		case 0x172a4:
			direction := uint16(c.D[3])
			step, err := w.HurricaneRules.Create(uint8(owner), x, y, direction, w.hurricaneCallbacks())
			if err != nil {
				return false, err
			}
			if step.Admitted {
				commandByte(c, 5, w.HurricaneRules.Axes[direction/2][1])
				c.D[1] = 1
				if loc, ok := LocateNativeRecord(step.Reference); ok {
					w.NativeEnvironment[loc.Index] = NativeEnvironmentHurricane
				}
			} else {
				c.D[0] = 0
			}
			return !step.Admitted, nil
		case 0x1730e:
			step, err := w.PlagueRules.Create(owner, x, y, PlagueCallbacks{Memory: m})
			if err != nil {
				return false, err
			}
			c.D[0] = 0
			if step.Admitted {
				c.D[0] = 1
			}
			commandWord(c, 1, 0)
			return !step.Admitted, nil
		default:
			if bindings.Other != nil {
				return bindings.Other(call)
			}
			return false, fmt.Errorf("native World command routine%x context/body unavailable", call.Routine)
		}
	}}
}

func (w *World) executeNativeNormalCommand(rules *NativeCommandRules, caller int, context *NativeCommandRegisterContext, bindings NativeCommandWorldBindings) (NativeCommandStep, error) {
	if w == nil || w.Core == nil || rules == nil {
		return NativeCommandStep{}, fmt.Errorf("native command World/rules missing")
	}
	// This borrows authoritative bytes already present at $1744c. An outer
	// bridge/hydration, when needed, belongs around both side commands.
	return rules.Execute(caller, context, w.nativeNormalCommandCallbacks(bindings))
}
