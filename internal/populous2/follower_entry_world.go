package populous2

import (
	"fmt"
	legacy "go-populous2/internal/legacy"
)

// NativeFollowerEntry retains fields outside ordinary motion until the
// original contact/town/hero handlers update them. It does not use inherited
// state-flag values as native byte13 flags.
type NativeFollowerEntry struct {
	Initialized bool
	Actor       FollowerEntryActor
	Managed     bool
}

func (w *World) initializeEntryRecord(index int) {
	if index < 0 || index >= len(w.Core.Peeps) || index >= NativeWorldFollowerCapacity {
		return
	}
	p := w.Core.Peeps[index]
	graph := w.Occupancy.Followers[index].Record
	a := FollowerEntryActor{Owner: p.Player + 1, Weapon: uint8(p.Weapons), Motion: FollowerMotionActor{Kind: 2, Player: p.Player, Next: uint16(graph.Next), Previous: uint16(graph.Previous), X: int16(graph.X), Y: int16(graph.Y), Speed: p.MovementSpeed, State: 2, ReturnState: 2, Population: int32(p.Population), Variant: uint16(index+1) & 14}}
	if p.Flags&legacy.InTown != 0 {
		a.Motion.Kind, a.Motion.State, a.Byte1 = 4, 6, uint8(p.TownStage)
		a.Motion.Timer = int16(uint16(p.TownWork))
	}
	if w.Heroes[index].Active {
		a.Motion.Flags |= 2
		a.Hero40 = uint16(heroIndex(w.Heroes[index].Spell) * 2)
	}
	if p.Player < 2 && w.Core.Magnets[p.Player].Carried == index+1 {
		a.Motion.Flags |= 1
	}
	w.NativeEntries[index] = NativeFollowerEntry{Initialized: true, Actor: a}
	ref := nativeActorReference(NativeFollowerPool, index)
	previous, err := w.RecordImage.ReadFollowerEntry(ref)
	if err != nil {
		panic(err)
	}
	if _, err := w.RecordImage.PatchFollowerEntry(ref, previous, a); err != nil {
		panic(err)
	}
}

func (w *World) readEntryRecord(ref NativeRecordReference) (FollowerEntryActor, error) {
	if w.nativeCallDepth > 0 {
		return w.runtimeMemory().ReadFollowerEntry(ref)
	}
	location, ok := LocateNativeRecord(ref)
	if !ok || location.Pool != NativeFollowerPool {
		return FollowerEntryActor{}, fmt.Errorf("native entry record is not a follower")
	}
	index := location.Index - 1
	if index >= len(w.Core.Peeps) {
		return FollowerEntryActor{}, fmt.Errorf("native entry record exceeds allocated follower pool")
	}
	if !w.NativeEntries[index].Initialized {
		w.initializeEntryRecord(index)
	}
	if w.NativeEntries[index].Managed && !w.LightningVictims[index].Active && !w.flameDeathIndex[index] {
		return w.RecordImage.ReadFollowerEntry(ref)
	}
	a := w.NativeEntries[index].Actor
	p := w.Core.Peeps[index]
	graph := w.Occupancy.Followers[index].Record
	a.Owner = p.Player + 1
	a.Motion.Population = int32(p.Population)
	a.Motion.X, a.Motion.Y, a.Motion.Next, a.Motion.Previous = int16(graph.X), int16(graph.Y), uint16(graph.Next), uint16(graph.Previous)
	if p.Flags&legacy.InTown != 0 {
		if !w.NativeEntries[index].Managed {
			a.Motion.Kind, a.Motion.State = 4, 6
		}
		a.Byte1 = uint8(p.TownStage)
		a.Motion.Timer = int16(uint16(p.TownWork))
	} else if a.Motion.Kind == 4 && !w.NativeEntries[index].Managed {
		// Inherited town demotion precedes the next ordinary-controller
		// initialization. Do not expose the retired settlement to support scans.
		a.Motion.Kind, a.Motion.State, a.Motion.Animation, a.Byte1 = 2, 2, 0, 0
	}
	if native := w.NativeFollowers[index]; native.Active && w.ordinaryWalker(index) {
		flags := a.Motion.Flags
		a.Motion = native.Actor
		// Fractional motion owns its coordinates and leg, while the entry
		// record owns leader, hero, captive and disease flags.
		a.Motion.Flags = flags
		a.Motion.Population = int32(p.Population)
	}
	if w.flameDeathIndex[index] {
		for _, death := range w.FlameDeaths {
			if death.Follower == index {
				a.Motion.Kind, a.Motion.State, a.Motion.Animation = death.Kind, death.State, death.Animation
				if a.Motion.Kind == 0 {
					a.Motion.Kind, a.Motion.State = 6, 8
				}
				break
			}
		}
	}
	if lightning := w.LightningVictims[index]; lightning.Active {
		v := lightning.Victim
		a.Motion.Kind, a.Motion.State, a.Motion.Animation = v.Kind, v.State, v.Animation
		a.Motion.Flags, a.Contact30 = v.Flags, uint16(v.EffectReference)
	}
	if p.Population <= 0 && !w.flameDeathIndex[index] && !w.LightningVictims[index].Active && !(w.NativeEntries[index].Managed && a.Owner != 0) {
		a.Owner = 0
	}
	a.Motion.Player = a.Owner - 1
	return a, nil
}

func (w *World) writeEntryRecord(ref NativeRecordReference, a FollowerEntryActor) error {
	if w.nativeCallDepth > 0 {
		previous, err := w.runtimeMemory().ReadFollowerEntry(ref)
		if err != nil {
			return err
		}
		if _, err := w.RecordImage.PatchFollowerEntry(ref, previous, a); err != nil {
			return err
		}
		w.hydrateNativeRuntimeGraph()
		return nil
	}
	location, ok := LocateNativeRecord(ref)
	if !ok || location.Pool != NativeFollowerPool {
		return fmt.Errorf("native entry write is not a follower")
	}
	index := location.Index - 1
	previous, err := w.readEntryRecord(ref)
	if err != nil {
		return err
	}
	if _, err := w.RecordImage.PatchFollowerEntry(ref, previous, a); err != nil {
		return err
	}
	w.NativeEntries[index] = NativeFollowerEntry{Initialized: true, Actor: a}
	p := &w.Core.Peeps[index]
	w.NativeEntries[index].Managed = nativeRuntimeManagedFollower(a)
	p.Population, p.Weapons, p.MovementSpeed = int(a.Motion.Population), int(a.Weapon), a.Motion.Speed
	if a.Owner > 0 && a.Owner <= 2 {
		p.Player = a.Owner - 1
	}
	if a.Owner == 0 {
		w.unlinkActor(NativeFollowerPool, index)
		return nil
	}
	w.moveActor(NativeFollowerPool, index, uint16(a.Motion.X), uint16(a.Motion.Y))
	p.AtPos = (int(uint16(a.Motion.X)) >> 8) + (int(uint16(a.Motion.Y))>>8)*64
	w.NativeFollowers[index].Actor = a.Motion
	w.NativeFollowers[index].Active = !w.NativeEntries[index].Managed && a.Motion.Kind == 2 && a.Motion.Flags&2 == 0 && (a.Motion.State == 2 || a.Motion.State == 4)
	if a.Motion.State == 4 && a.Motion.ReturnState == 12 {
		w.NativeEntries[index].Managed = true
		w.NativeFollowers[index].Active = true
	}
	switch a.Motion.Kind {
	case 4:
		p.Flags, p.TownStage = legacy.InTown, int(a.Byte1)
		if a.Motion.State == 6 {
			p.TownWork = int(uint16(a.Motion.Timer))
		}
	case 2:
		p.Flags = legacy.OnMove
	}
	return nil
}
