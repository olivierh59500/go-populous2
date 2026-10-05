package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type StormRules struct {
	CountModulus, BaseLife, StrikeModulus, CooldownModulus uint16
	HeroDeath                                              [6]uint16
	Raster                                                 [256]uint8
	Properties                                             [256]uint16
	ImageWords                                             []int16
	Frames                                                 map[int]AnimationFrame
}

func DecodeStormRules(exe *amiga.Executable) (StormRules, error) {
	var rules StormRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33612 {
		return rules, fmt.Errorf("native storm tables missing")
	}
	code := exe.Hunks[0].Data
	for index, destination := range []*uint16{&rules.CountModulus, &rules.BaseLife, &rules.StrikeModulus, &rules.CooldownModulus} {
		*destination = binary.BigEndian.Uint16(code[0x20d8c+index*2:])
	}
	if rules.CountModulus == 0 || rules.StrikeModulus == 0 || rules.CooldownModulus == 0 {
		return StormRules{}, fmt.Errorf("native storm division modulus is zero")
	}
	for index := range rules.HeroDeath {
		rules.HeroDeath[index] = binary.BigEndian.Uint16(code[0x20a24+index*2:])
	}
	copy(rules.Raster[:], code[0x33512:0x33612])
	for index := range rules.Properties {
		rules.Properties[index] = binary.BigEndian.Uint16(code[0x33312+index*2:])
	}
	for at := 0x23d1a; at < 0x26956; at += 2 {
		rules.ImageWords = append(rules.ImageWords, int16(binary.BigEndian.Uint16(code[at:])))
	}
	rules.Frames = make(map[int]AnimationFrame)
	for _, start := range []int{0xce4, 0x49c, 0x5ec} {
		frames, err := DecodeAnimation(exe, start)
		if err != nil {
			return StormRules{}, err
		}
		for index, frame := range frames {
			rules.Frames[start+index*4] = frame
		}
	}
	return rules, nil
}

type StormCallbacks struct {
	Memory       FollowerCleanupMemory
	Frame        *NativeFrameRegisterContext
	Random       func() uint16
	Link, Unlink func(NativeRecordReference) error
	DestroyTown  func(NativeRecordReference) error // Complete $16184.
}

type StormCreation struct {
	References            []NativeRecordReference
	Attempts, RandomDraws int
	Admitted              bool // Original caller's BEQ after $16bfc, independent of count.
	PoolFull              bool
}

// Create is command64/spell20's $16bfc body. It makes13..60 attempts inside
// positive0..7 offsets from the supplied packed origin. Partial creation then
// pool exhaustion rejects the debit; clipped attempts can admit without clouds.
// Caller is original A0's BSS address: its word+$1a is cleared on every spawn.
func (rules StormRules) Create(owner uint16, x, y uint8, caller int, cb StormCallbacks) (StormCreation, error) {
	step := StormCreation{References: []NativeRecordReference{}}
	m := cb.Memory
	if !winMemoryValid(m) || cb.Random == nil || cb.Link == nil {
		return step, fmt.Errorf("native storm creator callbacks missing")
	}
	count := cb.Random()%rules.CountModulus + rules.CountModulus/4
	step.RandomDraws++
	origin := uint16(y)<<8 | uint16(x)
	for attempt := 0; attempt <= int(count); attempt++ {
		address, err := primitiveFreeRecord(m, 0xc800, 0xe740, 32)
		if err != nil {
			return step, err
		}
		if address == 0 {
			step.Admitted, step.PoolFull = false, true
			return step, nil
		}
		bits := cb.Random()
		step.RandomDraws++
		step.Attempts++
		packed := origin + (bits & 0x0707)
		if packed&0xc0c0 != 0 {
			step.Admitted = true
			continue
		}
		if err := m.Write8(address+6, uint8(packed)); err != nil {
			return step, err
		}
		if err := m.Write16(address+8, packed); err != nil {
			return step, err
		}
		if err := m.Write8(address+7, 128); err != nil {
			return step, err
		}
		if err := m.Write8(address+9, 128); err != nil {
			return step, err
		}
		if err := m.Write8(address+12, uint8(owner)); err != nil {
			return step, err
		}
		if err := m.Write16(address+20, 0); err != nil {
			return step, err
		}
		xp := uint8(0)
		if int16(owner) <= 2 {
			xp, err = m.Read8(primitiveDeityAddress(owner) + 0x55)
			if err != nil {
				return step, err
			}
		}
		if err := m.Write16(address+24, rules.BaseLife+uint16(xp)); err != nil {
			return step, err
		}
		if err := m.Write16(caller+26, 0); err != nil {
			return step, err
		}
		if err := m.Write8(address, 0x36); err != nil {
			return step, err
		}
		if err := m.Write16(address+10, 0xce4+(bits&12)); err != nil {
			return step, err
		}
		if err := m.Write8(address+22, 0x2c); err != nil {
			return step, err
		}
		if err := m.Write16(address+26, 0); err != nil {
			return step, err
		}
		ref := NativeRecordReference(uint16(address - 0x76c0))
		if err := cb.Link(ref); err != nil {
			return step, err
		}
		step.References = append(step.References, ref)
		step.Admitted = true
	}
	return step, nil
}

type StormStep struct {
	Expired, Removed, Struck bool
	DamageScans              int
	Hits                     uint16
}

func stormCell(m FollowerCleanupMemory, address int) (int, error) {
	x, err := m.Read8(address + 6)
	if err != nil {
		return 0, err
	}
	y, err := m.Read8(address + 8)
	if err != nil {
		return 0, err
	}
	return 0xf44 + int(int16(uint16(y)<<8|uint16(uint8(x<<2)))), nil
}

func (rules StormRules) imageWord(offset uint16) (int16, error) {
	if offset&1 != 0 || int(offset)/2 >= len(rules.ImageWords) {
		return 0, fmt.Errorf("native storm animation address outside bounded bank")
	}
	return rules.ImageWords[offset/2], nil
}

// Damage is the actual $16542 tile-chain operation. Kind2 followers retain
// owner/links while becoming kind6/state8/population0; protected state3a and
// zero hero-death entries are immune. Tree burns do not contribute to D1 hits.
func (rules StormRules) Damage(ref NativeRecordReference, cb StormCallbacks) (uint16, error) {
	m := cb.Memory
	if !winMemoryValid(m) || cb.DestroyTown == nil {
		return 0, fmt.Errorf("native storm victim callbacks missing")
	}
	grid, err := stormCell(m, cleanupRecordAddress(ref))
	if err != nil {
		return 0, err
	}
	if cb.Frame != nil {
		cb.Frame.Word(0, uint16(grid-0xf44))
		cb.Frame.D[1] = 0
	}
	head, err := m.Read16(grid + 2)
	if err != nil {
		return 0, err
	}
	if cb.Frame != nil {
		cb.Frame.Word(0, head)
	}
	if head == 0 {
		return 0, fmt.Errorf("native storm unlinked cell would enter the external effect-pool tail")
	}
	hits := uint16(0)
	seen := map[uint16]bool{}
	for head != 0 {
		if seen[head] {
			return hits, fmt.Errorf("cyclic native storm victim chain")
		}
		seen[head] = true
		address := cleanupRecordAddress(NativeRecordReference(head))
		kind, err := m.Read8(address)
		if err != nil {
			return hits, err
		}
		if kind == 0x16 {
			if err := m.Write8(address, 0x1e); err != nil {
				return hits, err
			}
			if err := m.Write16(address+10, 0xf10); err != nil {
				return hits, err
			}
		}
		if kind == 2 {
			state, err := m.Read8(address + 22)
			if err != nil {
				return hits, err
			}
			if state != 0x3a {
				animation := uint16(0x178)
				flags, err := m.Read8(address + 13)
				if err != nil {
					return hits, err
				}
				if flags&2 != 0 {
					hero, err := m.Read16(address + 40)
					if err != nil {
						return hits, err
					}
					if hero&1 != 0 || hero > 10 {
						return hits, fmt.Errorf("native storm hero table alias outside decoded window")
					}
					animation = rules.HeroDeath[hero/2]
					if cb.Frame != nil {
						cb.Frame.Word(0, animation)
					}
				}
				if animation != 0 {
					if err := m.Write16(address+10, animation); err != nil {
						return hits, err
					}
					if err := m.Write8(address, 6); err != nil {
						return hits, err
					}
					if err := m.Write8(address+22, 8); err != nil {
						return hits, err
					}
					if err := m.Write32(address+26, 0); err != nil {
						return hits, err
					}
					hits++
					if cb.Frame != nil {
						cb.Frame.Word(1, hits)
					}
				}
			}
		}
		if kind == 4 {
			if err := cb.DestroyTown(NativeRecordReference(head)); err != nil {
				return hits, err
			}
			hits++
			if cb.Frame != nil {
				cb.Frame.Word(1, hits)
			}
		}
		head, err = m.Read16(address + 2)
		if cb.Frame != nil {
			cb.Frame.Word(0, head)
		}
		if err != nil {
			return hits, err
		}
	}
	return hits, nil
}

// Scorch translates $1735a, preserving parcel pressure and occupancy heads.
func (rules StormRules) Scorch(ref NativeRecordReference, m FollowerCleanupMemory) error {
	grid, err := stormCell(m, cleanupRecordAddress(ref))
	if err != nil {
		return err
	}
	tile, err := m.Read8(grid + 1)
	if err != nil {
		return err
	}
	if rules.Raster[tile]&15 == 15 && rules.Properties[tile]&(1<<10) == 0 {
		return m.Write8(grid+1, 95)
	}
	return nil
}

// Tick translates native states2c/2e. A thunder success resets1..2 cooldown,
// scorches and scans victims; no-hit success adds49c/5ec and scans once again.
// Nonnegative cooldown still damages every tick. Failed thunder draws do not.
func (rules StormRules) Tick(ref NativeRecordReference, cb StormCallbacks) (StormStep, error) {
	var step StormStep
	m := cb.Memory
	if !winMemoryValid(m) || cb.Random == nil || cb.Unlink == nil || cb.DestroyTown == nil {
		return step, fmt.Errorf("native storm runtime callbacks missing")
	}
	location, ok := LocateNativeRecord(ref)
	if !ok || location.Pool != NativeEffectPool {
		return step, fmt.Errorf("native storm reference outside shared effect pool")
	}
	address := cleanupRecordAddress(ref)
	state, err := m.Read8(address + 22)
	if err != nil {
		return step, err
	}
	if state != 0x2c && state != 0x2e {
		return step, fmt.Errorf("native storm state%x unsupported", state)
	}
	ending := func() error {
		animation, err := m.Read16(address + 10)
		if err != nil {
			return err
		}
		next := uint16(animation + 4)
		if cb.Frame != nil {
			cb.Frame.Word(0, next)
		}
		word, err := rules.imageWord(next)
		if err != nil {
			return err
		}
		if word < 0 {
			if err := m.Write8(address+12, 0); err != nil {
				return err
			}
			step.Removed = true
			return cb.Unlink(ref)
		}
		return m.Write16(address+10, next)
	}
	if state == 0x2e {
		if err := m.Write8(address+12, 0); err != nil {
			return step, err
		}
		step.Removed = true
		return step, cb.Unlink(ref)
	}
	life, err := m.Read16(address + 24)
	if err != nil {
		return step, err
	}
	if err := m.Write16(address+24, life-1); err != nil {
		return step, err
	}
	if int16(life) <= 1 {
		if err := m.Write8(address+22, 0x2e); err != nil {
			return step, err
		}
		step.Expired = true
		return step, ending()
	}
	animation, err := m.Read16(address + 10)
	if err != nil {
		return step, err
	}
	next := uint16(animation + 4)
	word, err := rules.imageWord(next)
	if err != nil {
		return step, err
	}
	if word < 0 {
		next += uint16(word)
	}
	if cb.Frame != nil {
		cb.Frame.Word(0, next)
	}
	if err := m.Write16(address+10, next); err != nil {
		return step, err
	}
	extra, err := m.Read16(address + 26)
	if err != nil {
		return step, err
	}
	if cb.Frame != nil {
		cb.Frame.Word(0, extra)
	}
	if extra != 0 {
		next := uint16(extra + 4)
		word, err := rules.imageWord(next)
		if err != nil {
			return step, err
		}
		if word < 0 {
			next = 0
		}
		if cb.Frame != nil {
			cb.Frame.Word(0, next)
		}
		if err := m.Write16(address+26, next); err != nil {
			return step, err
		}
	}
	timer, err := m.Read16(address + 20)
	if err != nil {
		return step, err
	}
	if err := m.Write16(address+20, timer-1); err != nil {
		return step, err
	}
	scan := func() (uint16, error) {
		step.DamageScans++
		hits, err := rules.Damage(ref, cb)
		step.Hits += hits
		return hits, err
	}
	if int16(timer) >= 1 {
		_, err := scan()
		return step, err
	}
	if err := m.Write16(address+20, 0); err != nil {
		return step, err
	}
	strike := cb.Random()
	if cb.Frame != nil {
		cb.Frame.D[0] = uint32(strike)
		if e := frameDivide(cb.Frame, 0, rules.StrikeModulus); e != nil {
			return step, e
		}
		cb.Frame.Swap(0)
	}
	if strike%rules.StrikeModulus != 0 {
		return step, nil
	}
	step.Struck = true
	cooldown := cb.Random()
	if cb.Frame != nil {
		cb.Frame.D[0] = uint32(cooldown)
		cb.Frame.Word(1, rules.CooldownModulus)
		if e := frameDivide(cb.Frame, 0, rules.CooldownModulus); e != nil {
			return step, e
		}
		cb.Frame.Swap(0)
		cb.Frame.Word(1, rules.CooldownModulus>>1)
		cb.Frame.Word(0, uint16(cb.Frame.D[0])+uint16(cb.Frame.D[1]))
	}
	if err := m.Write16(address+20, cooldown%rules.CooldownModulus+rules.CooldownModulus/2); err != nil {
		return step, err
	}
	if err := m.Write16(address+26, 0); err != nil {
		return step, err
	}
	if err := rules.Scorch(ref, m); err != nil {
		return step, err
	}
	hits, err := scan()
	if err != nil {
		return step, err
	}
	if hits != 0 {
		return step, nil
	}
	extra = 0x49c
	grid, err := stormCell(m, address)
	if err != nil {
		return step, err
	}
	tile, err := m.Read8(grid + 1)
	if err != nil {
		return step, err
	}
	if cb.Frame != nil {
		cb.Frame.Word(4, uint16(tile)*2)
	}
	if rules.Properties[tile]&8 != 0 {
		extra = 0x5ec
	}
	if err := m.Write16(address+26, extra); err != nil {
		return step, err
	}
	_, err = scan()
	return step, err
}
