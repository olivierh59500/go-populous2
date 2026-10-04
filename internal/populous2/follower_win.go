package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type FollowerWinRules struct {
	RewardParameters [3]uint16
	TownRewards      [TownStages]uint16
	AdonisCloneCount uint16
	Frames           map[int]AnimationFrame
}

// DecodeFollowerWinRules snapshots the active LAND reward parameters. Native
// CODE addresses $336f2/$3373e belong to that mutable landscape buffer.
func DecodeFollowerWinRules(exe *amiga.Executable, land Landscape) (FollowerWinRules, error) {
	var rules FollowerWinRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x14768 {
		return rules, fmt.Errorf("native winner tables missing")
	}
	for index := range rules.RewardParameters {
		rules.RewardParameters[index] = uint16(land.Parameters[index])
	}
	for index := range rules.TownRewards {
		rules.TownRewards[index] = uint16(land.Weapons[index])
	}
	rules.AdonisCloneCount = binary.BigEndian.Uint16(exe.Hunks[0].Data[0x14766:])
	rules.Frames = make(map[int]AnimationFrame)
	for _, start := range []int{0x1b2c, 0x1cd0, 0x1e74, 0x9d4} {
		frames, err := DecodeAnimation(exe, start)
		if err != nil {
			return FollowerWinRules{}, err
		}
		for index, frame := range frames {
			rules.Frames[start+index*4] = frame
		}
	}
	return rules, nil
}

type FollowerWinCallbacks struct {
	Memory      FollowerCleanupMemory
	Cleanup     func(NativeRecordReference, uint16) error
	ClearFarms  func(NativeRecordReference, uint8) error
	ReformTown  func(NativeRecordReference, NativeRecordReference) error
	DestroyTown func(NativeRecordReference) error
	// Original $dc2 inhibits Adonis allocation before population is halved.
	PoolBlocked func() bool
	Insert      func(NativeRecordReference) error
}

type FollowerWinStep struct {
	Reward                  uint16
	Hero, Adonis            bool
	WinnerState, LoserState uint8
	AdonisClones            []NativeRecordReference
}

// Reward preserves conditional post-increment reads: a hero-only loser uses
// the first bonus word; leader+hero uses both. All additions wrap as words.
func (rules FollowerWinRules) Reward(flags, kind, stage uint8) (uint16, error) {
	reward, next := rules.RewardParameters[0], 1
	if flags&1 != 0 {
		reward += rules.RewardParameters[next]
		next++
	}
	if flags&2 != 0 {
		reward += rules.RewardParameters[next]
	}
	if kind == 4 {
		if int(stage) >= len(rules.TownRewards) {
			return 0, fmt.Errorf("native defeated town stage outside reward table")
		}
		reward += rules.TownRewards[stage]
	}
	return reward, nil
}

func winMemoryValid(memory FollowerCleanupMemory) bool {
	return memory.Read8 != nil && memory.Read16 != nil && memory.Read32 != nil && memory.Write8 != nil && memory.Write16 != nil && memory.Write32 != nil
}

// SplitAdonis translates $146d8. A failed full-pool scan retains the parent's
// already-halved population. Clones copy all26 words, including stale bytes,
// then clear hero links22/24 and set state24/animation0 before native insertion.
func (rules FollowerWinRules) SplitAdonis(reference NativeRecordReference, cb FollowerWinCallbacks) ([]NativeRecordReference, error) {
	clones := []NativeRecordReference{}
	if !winMemoryValid(cb.Memory) || cb.PoolBlocked == nil || cb.Insert == nil {
		return clones, fmt.Errorf("native Adonis callbacks missing")
	}
	if cb.PoolBlocked() || int16(rules.AdonisCloneCount-1) < 0 {
		return clones, nil
	}
	m, source := cb.Memory, cleanupRecordAddress(reference)
	population, err := m.Read32(source + 0x1a)
	if err != nil {
		return clones, err
	}
	if int32(population) <= 20 {
		return clones, nil
	}
	if err := m.Write32(source+0x1a, population>>1); err != nil {
		return clones, err
	}
	for attempt := 0; attempt < int(rules.AdonisCloneCount); attempt++ {
		slot := -1
		for address := 0x76f4; address < 0xc800; address += 52 {
			owner, err := m.Read8(address + 12)
			if err != nil {
				return clones, err
			}
			if owner == 0 {
				slot = address
				break
			}
		}
		if slot < 0 {
			return clones, nil
		}
		for offset := 0; offset < 52; offset += 2 {
			word, err := m.Read16(source + offset)
			if err != nil {
				return clones, err
			}
			if err := m.Write16(slot+offset, word); err != nil {
				return clones, err
			}
		}
		for _, offset := range []int{0x22, 0x24, 0x0a} {
			if err := m.Write16(slot+offset, 0); err != nil {
				return clones, err
			}
		}
		if err := m.Write8(slot+0x16, 0x24); err != nil {
			return clones, err
		}
		clone := NativeRecordReference(uint16(slot - 0x76c0))
		if err := cb.Insert(clone); err != nil {
			return clones, err
		}
		clones = append(clones, clone)
	}
	return clones, nil
}

// Win translates $1298c. OriginalA0 is the caller's aggressor reference and
// is passed through to town reform; winner/loser alone do not reconstruct it.
func (rules FollowerWinRules) Win(winner, loser, originalA0 NativeRecordReference, cb FollowerWinCallbacks) (FollowerWinStep, error) {
	var step FollowerWinStep
	if !winMemoryValid(cb.Memory) || cb.Cleanup == nil || cb.ClearFarms == nil || cb.ReformTown == nil || cb.DestroyTown == nil {
		return step, fmt.Errorf("native winner callbacks missing")
	}
	m := cb.Memory
	wa, la := cleanupRecordAddress(winner), cleanupRecordAddress(loser)
	flags, err := m.Read8(la + 13)
	if err != nil {
		return step, err
	}
	kind, err := m.Read8(la)
	if err != nil {
		return step, err
	}
	stage, err := m.Read8(la + 1)
	if err != nil {
		return step, err
	}
	step.Reward, err = rules.Reward(flags, kind, stage)
	if err != nil {
		return step, err
	}
	wo, err := m.Read8(wa + 12)
	if err != nil {
		return step, err
	}
	lo, err := m.Read8(la + 12)
	if err != nil {
		return step, err
	}
	wg, lg := 0xe76a+int(wo)*314, 0xe76a+int(lo)*314
	mana, err := m.Read32(wg)
	if err != nil {
		return step, err
	}
	if err := m.Write32(wg, mana+uint32(step.Reward)); err != nil {
		return step, err
	}
	victories, err := m.Read16(wg + 0x48)
	if err != nil {
		return step, err
	}
	if err := m.Write16(wg+0x48, victories+1); err != nil {
		return step, err
	}
	mana, err = m.Read32(lg)
	if err != nil {
		return step, err
	}
	remaining := uint32(0)
	if int32(mana) > int32(step.Reward) {
		remaining = mana - uint32(step.Reward)
	}
	if err := m.Write32(lg, remaining); err != nil {
		return step, err
	}
	winnerFlags, err := m.Read8(wa + 13)
	if err != nil {
		return step, err
	}
	step.Hero = winnerFlags&2 != 0
	death := func() error {
		if err := cb.Cleanup(loser, 1); err != nil {
			return err
		}
		if err := m.Write16(la+0x0a, 0x1e74); err != nil {
			return err
		}
		if err := m.Write8(la, 0x12); err != nil {
			return err
		}
		if err := m.Write8(la+0x16, 0x18); err != nil {
			return err
		}
		flags, err := m.Read8(la + 13)
		if err != nil {
			return err
		}
		if flags&2 != 0 {
			if err := m.Write8(la+0x16, 0x40); err != nil {
				return err
			}
			return m.Write16(la+0x0a, 0x9d4)
		}
		return nil
	}
	if step.Hero {
		if kind == 4 {
			err = cb.DestroyTown(loser)
		} else {
			err = death()
		}
		if err != nil {
			return step, err
		}
		hero, err := m.Read16(wa + 0x28)
		if err != nil {
			return step, err
		}
		if hero == 2 {
			step.Adonis = true
			step.AdonisClones, err = rules.SplitAdonis(winner, cb)
			if err != nil {
				return step, err
			}
		}
		if err := m.Write8(wa+0x16, 0x24); err != nil {
			return step, err
		}
		if err := m.Write16(wa+0x0a, 0); err != nil {
			return step, err
		}
	} else {
		if kind == 4 {
			if err := cb.ClearFarms(loser, 15); err != nil {
				return step, err
			}
			if err := cb.ReformTown(winner, originalA0); err != nil {
				return step, err
			}
		} else {
			winnerKind, err := m.Read8(wa)
			if err != nil {
				return step, err
			}
			if winnerKind == 4 {
				if err := cb.ClearFarms(winner, 15); err != nil {
					return step, err
				}
				if err := cb.ReformTown(winner, originalA0); err != nil {
					return step, err
				}
			} else {
				image := uint16(0x1b2c)
				if wo != 1 {
					image = 0x1cd0
				}
				if err := m.Write16(wa+0x0a, image); err != nil {
					return step, err
				}
				if err := m.Write8(wa+0x16, 0x42); err != nil {
					return step, err
				}
			}
		}
		if err := death(); err != nil {
			return step, err
		}
	}
	step.WinnerState, err = m.Read8(wa + 0x16)
	if err != nil {
		return step, err
	}
	step.LoserState, err = m.Read8(la + 0x16)
	return step, err
}
