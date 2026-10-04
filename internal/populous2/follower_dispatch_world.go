package populous2

import "fmt"

func (w *World) enterNativeFollowerCell(index int) FollowerEntryStep {
	var step FollowerEntryStep
	err := w.runNativeFollowerCall(func() error {
		var err error
		step, err = w.FollowerEntry.Enter(nativeActorReference(NativeFollowerPool, index), w.nativeEntryCallbacks())
		return err
	})
	if err != nil {
		panic(err)
	}
	return step
}

func (w *World) ManagedFollowerFrame(index int) (AnimationFrame, bool) {
	if index < 0 || index >= NativeWorldFollowerCapacity || !w.NativeEntries[index].Managed {
		return AnimationFrame{}, false
	}
	a, err := w.RecordImage.ReadFollowerEntry(nativeActorReference(NativeFollowerPool, index))
	if err != nil || a.Owner == 0 || a.Motion.Flags&8 != 0 {
		return AnimationFrame{}, false
	}
	if a.Motion.State == 4 {
		if a.Motion.Flags&2 == 0 {
			frame, _, ok := w.FollowerMotion.Frame(a.Motion)
			return frame, ok
		}
		if a.Hero40&1 != 0 || a.Hero40 > 10 {
			return AnimationFrame{}, false
		}
		return AnimationFrame{Layers: w.HeroArt.Layers(heroIDs[a.Hero40/2], int(w.FollowerMotion.Angle(a.Motion.VX, a.Motion.VY)>>5), a.Motion.Animation/4)}, true
	}
	for _, frames := range []map[int]AnimationFrame{w.FollowerEntry.WaitingFrames, w.FollowerCombat.Frames, w.FollowerAftermath.Frames, w.FollowerWin.Frames} {
		if frame, ok := frames[a.Motion.Animation]; ok {
			return frame, true
		}
	}
	return AnimationFrame{}, false
}

func (w *World) nativeSearchFollower(ref NativeRecordReference) (bool, error) {
	a, err := w.readEntryRecord(ref)
	if err != nil {
		return false, err
	}
	deity, ok := NativeDeityAddress(a.Owner)
	if !ok {
		return false, fmt.Errorf("native search owner outside deity records")
	}
	attrition, err := w.runtimeMemory().Read32(deity + 20)
	if err != nil {
		return false, err
	}
	step, err := ApplyFollowerAttrition(ref, attrition, FollowerAttritionCallbacks{Read: w.readEntryRecord, Write: w.writeEntryRecord, Cleanup: w.cleanupNativeFollower})
	if err != nil {
		return false, err
	}
	if step.Died {
		if _, err := w.CommonPrepass.Tick(ref, w.nativeCommonPrepassCallbacks()); err != nil {
			return false, err
		}
		return true, nil
	}
	a, err = w.readEntryRecord(ref)
	if err != nil {
		return false, err
	}
	if a.Motion.Flags&2 != 0 {
		_, err := w.FollowerHero.SelectTarget(ref, w.nativeCleanupMemory())
		return false, err
	}
	mode, err := w.runtimeMemory().Read16(deity + 12)
	if err != nil {
		return false, err
	}
	if mode == uint16(NativeFollowerMagnet) {
		return false, fmt.Errorf("native magnet redispatch requires its decision handler")
	}
	search, err := w.runtimeMemory().Read8(cleanupRecordAddress(ref) + 24)
	if err != nil {
		return false, err
	}
	decision, err := w.FollowerDecision.Select(&a.Motion, search, NativeFollowerMode(mode), &w.Occupancy.Grid, FollowerDecisionCallbacks{Record: w.followerDecisionRecord, Random: w.random})
	if err != nil {
		return false, err
	}
	return decision.Fallthrough, w.writeEntryRecord(ref, a)
}

func (w *World) updateNativeManagedFollower(index int) bool {
	if index < 0 || index >= len(w.Core.Peeps) || !w.NativeEntries[index].Initialized || !w.NativeEntries[index].Managed {
		return false
	}
	ref := nativeActorReference(NativeFollowerPool, index)
	count := false
	err := w.runNativeFollowerCall(func() error {
		if _, err := w.CommonPrepass.Tick(ref, w.nativeCommonPrepassCallbacks()); err != nil {
			return err
		}
		var entry FollowerEntryStep
		for redispatch := 0; redispatch < 32; redispatch++ {
			a, err := w.readEntryRecord(ref)
			if err != nil {
				return err
			}
			switch a.Motion.State {
			case 2:
				fallthroughMotion, err := w.nativeSearchFollower(ref)
				if err != nil {
					return err
				}
				if fallthroughMotion {
					continue
				}
				count = true
				return nil
			case 4:
				before := a
				crossed := false
				step := w.FollowerMotion.Tick(&a.Motion, FollowerMotionCallbacks{
					Admit: func(actor *FollowerMotionActor, x, y int16) bool {
						result, err := w.FollowerHero.Probe(ref, entryTile(a), int16(followerMotionSign(int(x>>8)-int(actor.X>>8))), int16(followerMotionSign(int(y>>8)-int(actor.Y>>8))), w.nativeCleanupMemory())
						if err != nil {
							panic(err)
						}
						return result == 0
					},
					Move: func(actor *FollowerMotionActor, _, _ int16) {
						crossed = true
						if _, err := w.Occupancy.Grid.Move(ref, uint16(actor.X), uint16(actor.Y), w.runtimeMemory().RecordAccess()); err != nil {
							panic(err)
						}
						record, _ := w.runtimeMemory().RecordAccess().Record(ref)
						actor.Next, actor.Previous = uint16(record.Next), uint16(record.Previous)
						a.Motion = *actor
						if err := w.writeEntryRecord(ref, a); err != nil {
							panic(err)
						}
						w.hydrateNativeRuntimeGraph()
						if _, err := w.FollowerEntry.Enter(ref, w.nativeEntryCallbacks()); err != nil {
							panic(err)
						}
					},
				})
				if !crossed {
					if _, err := w.RecordImage.PatchFollowerEntry(ref, before, a); err != nil {
						return err
					}
					w.hydrateNativeRuntimeGraph()
				}
				if step.NeedDecision {
					continue
				}
				count = true
				return nil
			case 10:
				entry, err = w.FollowerEntry.TickWaiting(ref, w.nativeEntryCallbacks())
			case 12:
				entry, err = w.FollowerEntry.CompleteContact(ref, w.nativeEntryCallbacks())
			case 0x24, 0x26:
				step, err := w.FollowerHero.Decide(ref, w.nativeHeroCallbacks())
				if err != nil {
					return err
				}
				if step.Redispatch {
					if _, err := w.CommonPrepass.Tick(ref, w.nativeCommonPrepassCallbacks()); err != nil {
						return err
					}
					continue
				}
				count = step.CountPopulation
				return nil
			case 14:
				step, err := w.FollowerCombat.TickAggressor(ref, w.nativeCombatCallbacks(ref))
				count = step.CurrentTotal
				return err
			case 16:
				step, err := w.FollowerCombat.TickDefender(ref, w.nativeCombatCallbacks(ref))
				if err != nil {
					return err
				}
				entry.RedispatchSearch, count = step.RedispatchSearch, step.CurrentTotal
			case 8, 0x18, 0x1a, 0x20, 0x28, 0x2a, 0x2c, 0x2e, 0x30, 0x32, 0x38, 0x3e, 0x40, 0x42:
				step, err := w.FollowerAftermath.Tick(ref, w.nativeAftermathCallbacks())
				count = step.CurrentTotal
				return err
			default:
				return fmt.Errorf("native managed follower state %02x requires its own dispatcher: index %d raw %+v lightning %+v flame %v", a.Motion.State, index, a, w.LightningVictims[index], w.flameDeathIndex[index])
			}
			if err != nil {
				return err
			}
			if entry.RedispatchHero {
				if _, err := w.FollowerHero.SelectTarget(ref, w.nativeCleanupMemory()); err != nil {
					return err
				}
				count = true
				return nil
			}
			if entry.RedispatchSearch {
				fallthroughMotion, err := w.nativeSearchFollower(ref)
				if err != nil {
					return err
				}
				if fallthroughMotion {
					continue
				}
				count = true
				return nil
			}
			count = !entry.NextFollower
			return nil
		}
		return fmt.Errorf("native follower redispatch exceeds bounded iteration window")
	})
	if err != nil {
		panic(err)
	}
	if count {
		w.Core.Magnets[w.Core.Peeps[index].Player].Population += max(0, w.Core.Peeps[index].Population)
	}
	return true
}
