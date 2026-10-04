package populous2

import (
	"encoding/binary"
	"fmt"
	"go-populous2/internal/amiga"
)

type LightningRules struct {
	MarkerLife                       int16
	DamageShift                      uint16
	Jitter                           [9][2]int8
	HeroHit, HeroDeath, HeroRecovery [6]int
	Frames                           map[int]AnimationFrame
	SequenceLengths                  map[int]int
	LoopOffsets                      map[int]int
}

func DecodeLightningRules(exe *amiga.Executable) (LightningRules, error) {
	var r LightningRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x23d1a+0x2bf0 {
		return r, fmt.Errorf("native lightning tables missing")
	}
	c := exe.Hunks[0].Data
	r.MarkerLife = int16(binary.BigEndian.Uint16(c[0x20d58:]))
	r.DamageShift = binary.BigEndian.Uint16(c[0x20d5a:])
	if r.MarkerLife <= 0 || r.DamageShift > 31 || binary.BigEndian.Uint16(c[0x20f0a:]) != 9 {
		return r, fmt.Errorf("invalid native lightning parameters")
	}
	for i := range r.Jitter {
		r.Jitter[i] = [2]int8{int8(c[0x20f0c+i*2]), int8(c[0x20f0d+i*2])}
	}
	for i := range r.HeroHit {
		r.HeroHit[i] = int(binary.BigEndian.Uint16(c[0x20a84+i*2:]))
		r.HeroDeath[i] = int(binary.BigEndian.Uint16(c[0x20a24+i*2:]))
		r.HeroRecovery[i] = int(binary.BigEndian.Uint16(c[0x20a30+i*2:]))
	}
	r.Frames = make(map[int]AnimationFrame)
	r.SequenceLengths = make(map[int]int)
	r.LoopOffsets = make(map[int]int)
	starts := append([]int{0x6e0, 0x6f8, 0x720, 0x738, 0x744, 0x750}, r.HeroHit[:]...)
	starts = append(starts, r.HeroDeath[:]...)
	starts = append(starts, r.HeroRecovery[:]...)
	for _, start := range starts {
		if start == 0 || r.SequenceLengths[start] != 0 {
			continue
		}
		frames, err := DecodeAnimation(exe, start)
		if err != nil {
			return LightningRules{}, err
		}
		r.SequenceLengths[start] = len(frames)
		for i, frame := range frames {
			r.Frames[start+i*4] = frame
		}
		r.LoopOffsets[start] = int(int16(binary.BigEndian.Uint16(c[0x23d1a+start+len(frames)*4:])))
	}
	return r, nil
}

// Word26 holds a marker's bolt head or a bolt's next reference. Word28 holds
// a bolt's marker. The values are native offsets from $76c0, not Go slots.
type LightningState struct {
	Markers        [2]NativeRecordReference
	Word26, Word28 [NativeEffectCapacity]NativeRecordReference
	RandomWords    [NativeEffectCapacity]uint16
}

type LightningCallbacks struct {
	Insert    func(NativeRecordReference) error
	Move      func(NativeRecordReference, uint16, uint16) error
	Unlink    func(NativeRecordReference) error
	Head      func(int, int) NativeRecordReference
	Record    func(NativeRecordReference) (LightningVictim, bool)
	SetRecord func(NativeRecordReference, LightningVictim)
	Burn      func(int, int)
	Random    func() int
}

func lightningReference(slot int) NativeRecordReference {
	return NativeRecordReference(20800 + slot*32)
}
func lightningSlot(ref NativeRecordReference) (int, bool) {
	loc, ok := LocateNativeRecord(ref)
	return loc.Index, ok && loc.Pool == NativeEffectPool
}

// Place follows command28/$15de2: reuse the owner's marker without resetting
// its lifetime, or initialize the first free effect slot. No RNG is consumed.
func (r *LightningRules) Place(state *LightningState, pool *[NativeEffectCapacity]NativeEffectActor, player, x, y int, cb LightningCallbacks) (bool, error) {
	if r == nil || state == nil || pool == nil || player < 0 || player > 1 || !inside(x, y) {
		return false, fmt.Errorf("invalid lightning marker input")
	}
	if ref := state.Markers[player]; ref != 0 {
		slot, ok := lightningSlot(ref)
		if !ok {
			return false, fmt.Errorf("invalid lightning marker reference")
		}
		if cb.Move != nil {
			if err := cb.Move(ref, uint16(x*256+128), uint16(y*256+128)); err != nil {
				return false, err
			}
		}
		pool[slot].X, pool[slot].Y = int16(x*256+128), int16(y*256+128)
		return true, nil
	}
	for slot := range pool {
		a := &pool[slot]
		if a.Active {
			continue
		}
		a.Active, a.Kind, a.Player = true, 0x28, uint8(player)
		a.X, a.Y, a.State, a.Animation, a.Life = int16(x*256+128), int16(y*256+128), 22, 0x6e0, r.MarkerLife
		state.Markers[player] = lightningReference(slot)
		state.Word26[slot] = 0
		if cb.Insert != nil {
			if err := cb.Insert(state.Markers[player]); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	return false, nil
}

// Activate follows command30/$15e8a. The command's mana debit is unconditional;
// this allocation helper preserves partial volleys and consumes one random
// number per created bolt. It does not charge or refund the caller's balance.
func (r *LightningRules) Activate(state *LightningState, pool *[NativeEffectCapacity]NativeEffectActor, player int, airXP uint8, cb LightningCallbacks) (int, error) {
	if r == nil || state == nil || pool == nil || player < 0 || player > 1 {
		return 0, fmt.Errorf("invalid lightning activation")
	}
	ref := state.Markers[player]
	if ref == 0 {
		return 0, nil
	}
	marker, ok := lightningSlot(ref)
	if !ok {
		return 0, fmt.Errorf("invalid lightning marker")
	}
	if state.Word26[marker] != 0 {
		return 0, nil
	}
	if cb.Random == nil {
		return 0, fmt.Errorf("lightning RNG missing")
	}
	count := 0
	for attempt := 0; attempt < 2+int(airXP>>5); attempt++ {
		slot := -1
		for i := range pool {
			if !pool[i].Active {
				slot = i
				break
			}
		}
		if slot < 0 {
			return count, nil
		}
		delta := r.Jitter[cb.Random()%len(r.Jitter)]
		x, y := int(pool[marker].X)>>8, int(pool[marker].Y)>>8
		xx, yy := x+int(delta[0]), y+int(delta[1])
		if xx < 0 || xx >= 64 {
			xx = x
		}
		if yy < 0 || yy >= 64 {
			yy = y
		}
		a := &pool[slot]
		a.Active, a.Kind, a.Player = true, 0x2a, pool[marker].Player
		a.X, a.Y, a.State, a.Animation = int16(xx*256+128), int16(yy*256+128), 24, 0
		bolt := lightningReference(slot)
		state.Word26[slot] = state.Word26[marker]
		state.Word26[marker] = bolt
		state.Word28[slot] = ref
		if cb.Insert != nil {
			if err := cb.Insert(bolt); err != nil {
				return count, err
			}
		}
		count++
	}
	return count, nil
}

func (r *LightningRules) Dismiss(state *LightningState, pool *[NativeEffectCapacity]NativeEffectActor, player int, cb LightningCallbacks) error {
	if state == nil || pool == nil || player < 0 || player > 1 {
		return fmt.Errorf("invalid lightning dismissal")
	}
	ref := state.Markers[player]
	if ref == 0 {
		return nil
	}
	slot, ok := lightningSlot(ref)
	if !ok {
		return fmt.Errorf("invalid lightning marker")
	}
	state.Markers[player] = 0
	pool[slot].State, pool[slot].Animation = 26, 0x720
	next := state.Word26[slot]
	seen := make(map[NativeRecordReference]bool)
	for next != 0 {
		if seen[next] {
			return fmt.Errorf("cyclic lightning bolt chain")
		}
		seen[next] = true
		slot, ok := lightningSlot(next)
		if !ok {
			return fmt.Errorf("invalid lightning bolt chain reference")
		}
		ref := next
		next = state.Word26[slot]
		pool[slot].Active = false
		if cb.Unlink != nil {
			if err := cb.Unlink(ref); err != nil {
				return err
			}
		}
	}
	return nil
}

// Tick retains original effect-pool order when invoked by the caller. Bolts
// continue until dismissal; their animation word is not a lifetime counter.
func (r *LightningRules) Tick(state *LightningState, pool *[NativeEffectCapacity]NativeEffectActor, slot int, cb LightningCallbacks) error {
	if r == nil || state == nil || pool == nil || slot < 0 || slot >= len(pool) {
		return fmt.Errorf("invalid lightning effect update")
	}
	a := &pool[slot]
	if !a.Active {
		return nil
	}
	switch a.State {
	case 22:
		a.Life--
		if a.Life <= 0 {
			return r.Dismiss(state, pool, int(a.Player), cb)
		}
		next := a.Animation + 4
		if _, ok := r.Frames[next]; !ok {
			next = 0x6f8
		}
		a.Animation = next
	case 24:
		return r.tickBolt(state, pool, slot, cb)
	case 26:
		next := a.Animation + 4
		if _, ok := r.Frames[next]; ok {
			a.Animation = next
			return nil
		}
		a.Active = false
		if cb.Unlink != nil {
			return cb.Unlink(lightningReference(slot))
		}
	}
	return nil
}
