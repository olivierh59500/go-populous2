package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type PlagueRules struct {
	Damage     uint16
	ImageWords []int16
	Frames     map[int]AnimationFrame
}

func DecodePlagueRules(exe *amiga.Executable) (PlagueRules, error) {
	var r PlagueRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x26956 {
		return r, fmt.Errorf("native plague tables missing")
	}
	code := exe.Hunks[0].Data
	r.Damage = binary.BigEndian.Uint16(code[0x20d56:])
	for at := 0x23d1a; at < 0x26956; at += 2 {
		r.ImageWords = append(r.ImageWords, int16(binary.BigEndian.Uint16(code[at:])))
	}
	r.Frames = make(map[int]AnimationFrame)
	for _, start := range []int{0xddc, 0x7dc} {
		frames, err := DecodeAnimation(exe, start)
		if err != nil {
			return PlagueRules{}, err
		}
		for i, frame := range frames {
			r.Frames[start+i*4] = frame
		}
	}
	return r, nil
}

type PlagueCallbacks struct {
	Memory FollowerCleanupMemory
	// Cleanup is complete $124a2(mode1), including original statistics,
	// leader relocation, farm removal and raw hero-link cleanup contexts.
	Cleanup func(NativeRecordReference, uint16) error
}

type PlagueCreation struct {
	References []NativeRecordReference
	Admitted   bool
}

// Create is original $1730e. It follows the real cell chain and compares kind
// as a signed byte against4. Only matching owner bytes are excluded; owner0,
// zero population, ruin states and heroes have no additional admission gate.
// Recasts reset the overlay phase to$ddc even for an already infected actor.
func (r *PlagueRules) Create(owner uint16, x, y uint8, cb PlagueCallbacks) (PlagueCreation, error) {
	step := PlagueCreation{References: []NativeRecordReference{}}
	m := cb.Memory
	if r == nil || !winMemoryValid(m) {
		return step, fmt.Errorf("native plague creator memory missing")
	}
	grid := 0xf44 + int(int16(uint16(y)<<8|uint16(uint8(x<<2))))
	head, err := m.Read16(grid + 2)
	if err != nil {
		return step, err
	}
	current := NativeRecordReference(head)
	seen := map[NativeRecordReference]bool{}
	for current != 0 {
		if seen[current] {
			return step, fmt.Errorf("cyclic native plague cell chain")
		}
		seen[current] = true
		at := cleanupRecordAddress(current)
		kind, err := m.Read8(at)
		if err != nil {
			return step, err
		}
		targetOwner, err := m.Read8(at + 12)
		if err != nil {
			return step, err
		}
		if int8(kind) <= 4 && targetOwner != uint8(owner) {
			flags, err := m.Read8(at + 13)
			if err != nil {
				return step, err
			}
			if err := m.Write8(at+13, flags|16); err != nil {
				return step, err
			}
			if err := m.Write16(at+48, 0xddc); err != nil {
				return step, err
			}
			step.References = append(step.References, current)
			step.Admitted = true
		}
		next, err := m.Read16(at + 2)
		if err != nil {
			return step, err
		}
		current = NativeRecordReference(next)
	}
	return step, nil
}

type PlagueStep struct{ OwnerSkipped, Advanced, AlreadyDying, Killed bool }

// Tick is only the plague portion of common prepass$12c3c/$12c46. The caller
// must continue the original terrain checks afterward and then dispatch the
// resulting follower state. Word+$30(decimal48) is the overlay phase, not a
// follower state$48. The shipped native damage constant is0.
func (r *PlagueRules) Tick(ref NativeRecordReference, cb PlagueCallbacks) (PlagueStep, error) {
	var step PlagueStep
	m := cb.Memory
	if r == nil || !winMemoryValid(m) {
		return step, fmt.Errorf("native plague runtime memory missing")
	}
	at := cleanupRecordAddress(ref)
	owner, err := m.Read8(at + 12)
	if err != nil {
		return step, err
	}
	if owner == 3 {
		step.OwnerSkipped = true
		return step, nil
	}
	flags, err := m.Read8(at + 13)
	if err != nil {
		return step, err
	}
	if flags&16 == 0 {
		return step, nil
	}
	phase, err := m.Read16(at + 48)
	if err != nil {
		return step, err
	}
	next := phase + 4
	if next&1 != 0 || int(next)/2 >= len(r.ImageWords) {
		return step, fmt.Errorf("native plague overlay phase outside bounded bank")
	}
	word := r.ImageWords[next/2]
	if word < 0 {
		next += uint16(word)
	}
	if err := m.Write16(at+48, next); err != nil {
		return step, err
	}
	step.Advanced = true
	state, err := m.Read8(at + 22)
	if err != nil {
		return step, err
	}
	if state == 0x3e {
		step.AlreadyDying = true
		return step, nil
	}
	population, err := m.Read32(at + 26)
	if err != nil {
		return step, err
	}
	if err := m.Write32(at+26, population-uint32(r.Damage)); err != nil {
		return step, err
	}
	if int64(int32(population))-int64(r.Damage) > 0 {
		return step, nil
	}
	if err := m.Write8(at+22, 0x3e); err != nil {
		return step, err
	}
	if err := m.Write16(at+10, 0x7dc); err != nil {
		return step, err
	}
	if cb.Cleanup == nil {
		return step, fmt.Errorf("native plague retained cleanup callback missing")
	}
	step.Killed = true
	return step, cb.Cleanup(ref, 1)
}

// InheritMerge is the precise $1291c..$12930 portion after the caller has
// cleared heroic relationships. The complete merge belongs to EntryRules.
// It ORs the original flags and copies the source's overlay phase only when
// that source is infected. Enemy co-location has no equivalent plague scan.
func (r *PlagueRules) InheritMerge(source, target NativeRecordReference, cb PlagueCallbacks) error {
	m := cb.Memory
	if r == nil || !winMemoryValid(m) {
		return fmt.Errorf("native plague merge memory missing")
	}
	from, to := cleanupRecordAddress(source), cleanupRecordAddress(target)
	flags, err := m.Read8(from + 13)
	if err != nil {
		return err
	}
	old, err := m.Read8(to + 13)
	if err != nil {
		return err
	}
	if err := m.Write8(to+13, old|flags); err != nil {
		return err
	}
	if flags&16 == 0 {
		return nil
	}
	phase, err := m.Read16(from + 48)
	if err != nil {
		return err
	}
	return m.Write16(to+48, phase)
}

// InheritBirth is the exact $118da..$118f2 flags/phase portion of town birth.
// The complete town-economy controller owns allocation, leader/hero transfer
// and the later branch. Its child retains flag bits0,1,4 after copying phase.
func (r *PlagueRules) InheritBirth(parent, child NativeRecordReference, cb PlagueCallbacks) error {
	m := cb.Memory
	if r == nil || !winMemoryValid(m) {
		return fmt.Errorf("native plague birth memory missing")
	}
	from, to := cleanupRecordAddress(parent), cleanupRecordAddress(child)
	flags, err := m.Read8(from + 13)
	if err != nil {
		return err
	}
	if err := m.Write8(to+13, flags); err != nil {
		return err
	}
	phase, err := m.Read16(from + 48)
	if err != nil {
		return err
	}
	if err := m.Write16(to+48, phase); err != nil {
		return err
	}
	flags, err = m.Read8(from + 13)
	if err != nil {
		return err
	}
	if err := m.Write8(from+13, flags&0xfc); err != nil {
		return err
	}
	flags, err = m.Read8(to + 13)
	if err != nil {
		return err
	}
	return m.Write8(to+13, flags&0x13)
}
