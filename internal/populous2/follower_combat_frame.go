package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type FollowerCombatFrameRules struct {
	code              []byte
	PopulationDivisor uint16
}
type FollowerCombatFrameCallbacks struct {
	Memory            FollowerCleanupMemory
	Frame             *NativeFrameRegisterContext
	SetMinimapVariant func(uint16) error // Original mutable CODE $124a0.
	CleanupFrame      func(NativeRecordReference, *NativeFrameRegisterContext) error
	// Return the actual post-$1298c A3 reference, not an inferred loser.
	WinFrame func(winner, loser, originalA0 NativeRecordReference, frame *NativeFrameRegisterContext) (NativeRecordReference, error)
}
type FollowerCombatFrameStep struct {
	Boundary      uint32
	Winner, Loser NativeRecordReference
}

func DecodeFollowerCombatFrameRules(exe *amiga.Executable) (FollowerCombatFrameRules, error) {
	var r FollowerCombatFrameRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x23d1a+0x1e0 {
		return r, fmt.Errorf("native combat frame tables missing")
	}
	r.code = exe.Hunks[0].Data
	r.PopulationDivisor = binary.BigEndian.Uint16(r.code[0x20bb4:])
	if r.PopulationDivisor == 0 {
		return r, fmt.Errorf("native combat DIVU divisor is zero")
	}
	return r, nil
}

func (r *FollowerCombatFrameRules) begin(cb FollowerCombatFrameCallbacks) error {
	if r == nil || cb.Frame == nil || !winMemoryValid(cb.Memory) || cb.SetMinimapVariant == nil {
		return fmt.Errorf("native combat raw frame/backing missing")
	}
	return cb.SetMinimapVariant(8)
}

// Aggressor is $11a86 through the source cleanup/winner/counting boundaries.
// F622 changes only D0 and retained RNG; its D1/D2/D3 MOVEMs preserve their
// exact incoming longs. The later randomized D1 clamp is overwritten by
// MOVEQ0, but the original partial D2 assignments survive into callbacks.
func (r *FollowerCombatFrameRules) Aggressor(ref NativeRecordReference, cb FollowerCombatFrameCallbacks) (FollowerCombatFrameStep, error) {
	var step FollowerCombatFrameStep
	if err := r.begin(cb); err != nil {
		return step, err
	}
	m, c := cb.Memory, cb.Frame
	at := cleanupRecordAddress(ref)
	animation, err := m.Read16(at + 10)
	if err != nil {
		return step, err
	}
	c.Word(0, animation+4)
	address := 0x23d1a + int(int16(c.D[0]))
	if address < 0 || address&1 != 0 || address+2 > len(r.code) {
		return step, fmt.Errorf("native combat animation word outside CODE")
	}
	marker := binary.BigEndian.Uint16(r.code[address:])
	if int16(marker) < 0 {
		c.Word(0, uint16(c.D[0])+marker)
	}
	if err := m.Write16(at+10, uint16(c.D[0])); err != nil {
		return step, err
	}
	contact, err := m.Read16(at + 30)
	if err != nil {
		return step, err
	}
	otherRef := NativeRecordReference(contact)
	other := cleanupRecordAddress(otherRef)
	owner, err := m.Read8(other + 12)
	if err != nil {
		return step, err
	}
	if int8(owner) <= 0 {
		if err := m.Write8(at+22, 2); err != nil {
			return step, err
		}
		if err := m.Write16(at+10, 0); err != nil {
			return step, err
		}
		step.Boundary = 0x123b4
		return step, nil
	}
	seed, err := m.Read32(0xeb28)
	if err != nil {
		return step, err
	}
	if seed == 0 {
		seed = 0x00bc614e
	}
	seed *= 0xbb40e62d
	if err := m.Write32(0xeb28, seed); err != nil {
		return step, err
	}
	c.D[0] = seed >> 8 & 0x7fff
	c.Word(2, uint16(c.D[0]))
	otherPopulation, err := m.Read32(other + 26)
	if err != nil {
		return step, err
	}
	c.D[1] = otherPopulation
	if err := frameDivide(c, 1, r.PopulationDivisor); err != nil {
		return step, err
	}
	c.Word(0, uint16(c.D[0])&3)
	c.Word(0, uint16(c.D[0])+1)
	c.D[1] = uint32(uint16(c.D[1])) * uint32(uint16(c.D[0]))
	c.Word(2, uint16(c.D[2])>>2)
	population, err := m.Read32(at + 26)
	if err != nil {
		return step, err
	}
	c.D[0] = population
	if err := frameDivide(c, 0, r.PopulationDivisor); err != nil {
		return step, err
	}
	c.Word(2, uint16(c.D[2])&3)
	c.Word(2, uint16(c.D[2])+1)
	c.D[1] = uint32(uint16(c.D[1])) * uint32(uint16(c.D[0]))
	if int16(c.D[0]) <= int16(c.D[1]) {
		c.Word(1, uint16(c.D[0]))
	}
	weapon, err := m.Read8(at + 25)
	if err != nil {
		return step, err
	}
	c.D[1] = uint32(weapon)*uint32(uint16(c.D[0])) + 10
	otherPopulation -= c.D[1]
	if err := m.Write32(other+26, otherPopulation); err != nil {
		return step, err
	}
	weapon, err = m.Read8(other + 25)
	if err != nil {
		return step, err
	}
	c.D[1] = uint32(weapon)*uint32(uint16(c.D[0])) + 10
	// A corrupt/self contact can alias both population operands. The source
	// performs each SUB.L against live memory, not a cached record copy.
	population, err = m.Read32(at + 26)
	if err != nil {
		return step, err
	}
	before := population
	population -= c.D[1]
	if err := m.Write32(at+26, population); err != nil {
		return step, err
	}
	// SUB.L/BGT compares signed operands, rather than the wrapped result
	// alone. The subsequent opponent test is an actual TST.L of its result.
	alive := int32(before) > int32(c.D[1])
	otherPopulation, err = m.Read32(other + 26)
	if err != nil {
		return step, err
	}
	if !alive && int32(otherPopulation) <= 0 {
		if cb.CleanupFrame == nil {
			return step, fmt.Errorf("native mutual-death cleanup frame missing")
		}
		c.D[0] = 0
		if err := cb.CleanupFrame(ref, c); err != nil {
			return step, err
		}
		c.D[0] = 0
		if err := cb.CleanupFrame(otherRef, c); err != nil {
			return step, err
		}
		step.Boundary = 0x12462
		return step, nil
	}
	if alive && int32(otherPopulation) > 0 {
		step.Boundary = 0x123b4
		return step, nil
	}
	if !alive {
		step.Winner, step.Loser = otherRef, ref
	} else {
		step.Winner, step.Loser = ref, otherRef
	}
	step.Boundary = 0x1298c
	if cb.WinFrame == nil {
		return step, nil
	} // Explicit original child boundary.
	returnedA3, err := cb.WinFrame(step.Winner, step.Loser, ref, c)
	if err != nil {
		return step, err
	}
	step.Boundary = 0x123b4
	if returnedA3 == ref {
		step.Boundary = 0x12462
	}
	return step, nil
}

// Defender includes the $11b6a minimap prefix and exact $11b72 wait probe.
// Failure jumps directly to $1131c search; it does not rerun common prepass.
func (r *FollowerCombatFrameRules) Defender(ref NativeRecordReference, cb FollowerCombatFrameCallbacks) (FollowerCombatFrameStep, error) {
	var step FollowerCombatFrameStep
	if err := r.begin(cb); err != nil {
		return step, err
	}
	m, c := cb.Memory, cb.Frame
	at := cleanupRecordAddress(ref)
	contact, err := m.Read16(at + 30)
	if err != nil {
		return step, err
	}
	c.Word(0, contact)
	valid := contact != 0
	if valid {
		other := cleanupRecordAddress(NativeRecordReference(contact))
		owner, err := m.Read8(other + 12)
		if err != nil {
			return step, err
		}
		c.Byte(0, owner)
		if int8(c.D[0]) <= 0 {
			valid = false
		} else {
			sourceOwner, err := m.Read8(at + 12)
			if err != nil {
				return step, err
			}
			valid = uint8(c.D[0]) != sourceOwner
			if valid {
				state, err := m.Read8(other + 22)
				if err != nil {
					return step, err
				}
				valid = state == 14
			}
			if valid {
				c.D[0] = uint32(int32(int16(ref)))
				reciprocal, err := m.Read16(other + 30)
				if err != nil {
					return step, err
				}
				valid = uint16(c.D[0]) == reciprocal
			}
		}
	}
	if valid {
		step.Boundary = 0x123b4
		return step, nil
	}
	if err := m.Write16(at+10, 0); err != nil {
		return step, err
	}
	if err := m.Write8(at+22, 2); err != nil {
		return step, err
	}
	step.Boundary = 0x1131c
	return step, nil
}
