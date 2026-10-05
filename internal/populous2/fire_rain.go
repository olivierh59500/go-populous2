package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

const FireRainActorKind uint8 = 0x2c

type FireRainRules struct {
	BaseCount  uint16
	Raster     [256]uint8
	Properties [256]uint16
	ImageWords []int16
	Frames     map[int]AnimationFrame
}

func DecodeFireRainRules(exe *amiga.Executable) (FireRainRules, error) {
	var r FireRainRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33612 {
		return r, fmt.Errorf("native fire rain tables missing")
	}
	code := exe.Hunks[0].Data
	r.BaseCount = binary.BigEndian.Uint16(code[0x20d64:])
	if r.BaseCount == 0 {
		return r, fmt.Errorf("native fire rain count divisor zero")
	}
	copy(r.Raster[:], code[0x33512:0x33612])
	for i := range r.Properties {
		r.Properties[i] = binary.BigEndian.Uint16(code[0x33312+i*2:])
	}
	for at := 0x23d1a; at < 0x26956; at += 2 {
		r.ImageWords = append(r.ImageWords, int16(binary.BigEndian.Uint16(code[at:])))
	}
	r.Frames = make(map[int]AnimationFrame)
	for _, start := range []int{0x81c, 0x49c, 0x5ec} {
		frames, err := DecodeAnimation(exe, start)
		if err != nil {
			return FireRainRules{}, err
		}
		for i, frame := range frames {
			r.Frames[start+i*4] = frame
		}
	}
	return r, nil
}

type FireRainCallbacks struct {
	Memory       FollowerCleanupMemory
	Random       func() uint16
	Link, Unlink func(NativeRecordReference) error
	// Scorch is $1735a, and Damage is the complete $16542 raw linked-record
	// scan, including hero immunity, retained follower deaths and town16184.
	Scorch func(NativeRecordReference) error
	Damage func(NativeRecordReference) (uint16, error)
}

type FireRainCreation struct {
	References            []NativeRecordReference
	Attempts, RandomDraws int
	Admitted, PoolFull    bool
}

// Create translates command38/spell25's $1648c. Each meteor is initially
// unlinked; its delay expires before $125a0. The original caller's BEQ tests
// the final Z flag, so pool exhaustion rejects a partially created cast while
// clipped final attempts admit even when no actor was created. No terrain
// admission or FireColumn creator is substituted.
func (r *FireRainRules) Create(owner uint16, x, y uint8, cb FireRainCallbacks) (FireRainCreation, error) {
	step := FireRainCreation{References: []NativeRecordReference{}}
	m := cb.Memory
	if r == nil || !winMemoryValid(m) || cb.Random == nil {
		return step, fmt.Errorf("native fire rain allocation callbacks missing")
	}
	base := r.BaseCount
	if int16(owner) <= 2 {
		xp, err := m.Read8(primitiveDeityAddress(owner) + 0x56)
		if err != nil {
			return step, err
		}
		base += uint16(xp >> 5)
	}
	if base == 0 {
		return step, fmt.Errorf("native fire rain effective count divisor zero")
	}
	count := cb.Random()%base + base/4
	step.RandomDraws++
	origin := uint16(y)<<8 | uint16(x)
	for attempt := 0; attempt <= int(count); attempt++ {
		at, err := primitiveFreeRecord(m, 0xc800, 0xe740, 32)
		if err != nil {
			return step, err
		}
		if at == 0 {
			step.PoolFull = true
			step.Admitted = false
			return step, nil
		}
		packed := origin + (cb.Random() & 0x0707)
		step.Attempts++
		step.RandomDraws++
		if packed&0xc0c0 != 0 {
			step.Admitted = true
			continue
		}
		for _, w := range []struct {
			Offset int
			Value  uint8
		}{{6, uint8(packed)}, {7, 128}, {9, 128}, {12, uint8(owner)}} {
			if w.Offset == 7 {
				if err := m.Write16(at+8, packed); err != nil {
					return step, err
				}
			}
			if err := m.Write8(at+w.Offset, w.Value); err != nil {
				return step, err
			}
		}
		delay := cb.Random() % 9
		step.RandomDraws++
		if err := m.Write16(at+20, delay); err != nil {
			return step, err
		}
		if err := m.Write16(at+24, 24); err != nil {
			return step, err
		}
		if err := m.Write8(at, FireRainActorKind); err != nil {
			return step, err
		}
		if err := m.Write16(at+10, 0x81c); err != nil {
			return step, err
		}
		if err := m.Write8(at+22, 0x1c); err != nil {
			return step, err
		}
		step.References = append(step.References, NativeRecordReference(at-0x76c0))
		step.Admitted = true
	}
	return step, nil
}

type FireRainStep struct {
	Linked, Impacted, Removed bool
	Hits                      uint16
	// NextActor is $15b6a. Otherwise the original reaches $15ad4 for its
	// presentation bookkeeping before advancing the shared pool cursor.
	NextActor bool
}

func (r *FireRainRules) imageWord(offset uint16) (int16, error) {
	if offset&1 != 0 || int(offset)/2 >= len(r.ImageWords) {
		return 0, fmt.Errorf("native fire rain animation outside bounded bank")
	}
	return r.ImageWords[offset/2], nil
}

// Tick translates states$1c/$1e/$20 at $151e8/$15200/$152c0. Delay expiry
// links and advances the falling animation in the same update. Impact on an
// empty cell advances its land/water after-animation immediately. A hit clears
// only the meteor's owner and unlinks; victim lifecycles belong to $16542.
func (r *FireRainRules) Tick(ref NativeRecordReference, cb FireRainCallbacks) (FireRainStep, error) {
	var step FireRainStep
	m := cb.Memory
	if r == nil || !winMemoryValid(m) || cb.Link == nil || cb.Unlink == nil {
		return step, fmt.Errorf("native fire rain controller callbacks missing")
	}
	at, err := volcanoAddress(ref)
	if err != nil {
		return step, err
	}
	phase, err := m.Read8(at + 22)
	if err != nil {
		return step, err
	}
	if phase == 0x1c {
		timer, err := m.Read16(at + 20)
		if err != nil {
			return step, err
		}
		if err := m.Write16(at+20, timer-1); err != nil {
			return step, err
		}
		if int16(timer) > 1 {
			return step, nil
		}
		if err := m.Write8(at+22, 0x1e); err != nil {
			return step, err
		}
		if err := cb.Link(ref); err != nil {
			return step, err
		}
		step.Linked = true
		phase = 0x1e
	}
	if phase != 0x1e && phase != 0x20 {
		return step, fmt.Errorf("native fire rain phase outside controller")
	}
	animation, err := m.Read16(at + 10)
	if err != nil {
		return step, err
	}
	next := animation + 4
	word, err := r.imageWord(next)
	if err != nil {
		return step, err
	}
	remove := func() (FireRainStep, error) {
		step.Removed, step.NextActor = true, true
		if err := m.Write8(at+12, 0); err != nil {
			return step, err
		}
		return step, cb.Unlink(ref)
	}
	if phase == 0x20 {
		if word < 0 {
			return remove()
		}
		return step, m.Write16(at+10, next)
	}
	impact := word < 0
	if !impact {
		if err := m.Write16(at+10, next); err != nil {
			return step, err
		}
		life, err := m.Read16(at + 24)
		if err != nil {
			return step, err
		}
		life--
		if err := m.Write16(at+24, life); err != nil {
			return step, err
		}
		grid, err := fireRainCell(m, at)
		if err != nil {
			return step, err
		}
		header, err := m.Read8(grid)
		if err != nil {
			return step, err
		}
		tile, err := m.Read8(grid + 1)
		if err != nil {
			return step, err
		}
		height := uint16(header&7) + uint16(r.Raster[tile]&1)
		impact = int16(height*3) >= int16(life)
	}
	if !impact {
		return step, nil
	}
	if cb.Scorch == nil || cb.Damage == nil {
		return step, fmt.Errorf("native fire rain impact callbacks missing")
	}
	step.Impacted = true
	if err := cb.Scorch(ref); err != nil {
		return step, err
	}
	step.Hits, err = cb.Damage(ref)
	if err != nil {
		return step, err
	}
	if step.Hits != 0 {
		return remove()
	}
	if err := m.Write8(at+22, 0x20); err != nil {
		return step, err
	}
	if err := m.Write16(at+10, 0x49c); err != nil {
		return step, err
	}
	grid, err := fireRainCell(m, at)
	if err != nil {
		return step, err
	}
	tile, err := m.Read8(grid + 1)
	if err != nil {
		return step, err
	}
	animation = 0x49c
	if r.Properties[tile]&8 != 0 {
		animation = 0x5ec
		if err := m.Write16(at+10, animation); err != nil {
			return step, err
		}
	}
	next = animation + 4
	word, err = r.imageWord(next)
	if err != nil {
		return step, err
	}
	if word < 0 {
		return remove()
	}
	return step, m.Write16(at+10, next)
}

func fireRainCell(m FollowerCleanupMemory, at int) (int, error) {
	y, err := m.Read16(at + 8)
	if err != nil {
		return 0, err
	}
	x, err := m.Read8(at + 6)
	if err != nil {
		return 0, err
	}
	packed := y&0xff00 | uint16(uint8(y)+x)
	return 0xf44 + int(int16(packed&0xff00|uint16(uint8(packed)<<2))), nil
}
