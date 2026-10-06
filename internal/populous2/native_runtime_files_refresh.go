package populous2

import (
	"bytes"
	"fmt"

	legacy "go-populous2/internal/legacy"
)

// refreshNativeRuntimeLoadedViews reads the live owner into existing caches.
// No World/actor creation or raw-memory write is permitted in this operation.
func refreshNativeRuntimeLoadedViews(h *NativeRuntimeHost) error {
	w, m := h.World, h.Memory.BSS
	world, e := m.Read16(0xeb46)
	if e != nil {
		return e
	}
	land, e := m.Read16(0xeb22)
	if e != nil {
		return e
	}
	profile, e := m.Read16(0xeb42)
	if e != nil {
		return e
	}
	mode, e := m.Read16(0xeb44)
	if e != nil {
		return e
	}
	if int(world) >= len(h.Bundle.Levels) || int(land) >= len(h.Bundle.Landscapes) || profile < 1 || profile > 2 {
		return fmt.Errorf("native loaded session cannot hydrate its raw world/LAND/profile")
	}
	// The existing host cache loader verifies the actual source-loaded LAND.
	if e := h.RefreshWorldCaches(); e != nil {
		return e
	}
	w.NativeGameMode, w.NativeProfileSide = mode, uint8(profile)
	w.Custom = mode != 2
	// Loading does not touch the unsaved mutable town CODE cache/scratch.
	clock, e := m.Read32(0xf40)
	if e != nil {
		return e
	}
	seed, e := m.Read32(0xeb24)
	if e != nil {
		return e
	}
	random, e := m.Read32(0xeb28)
	if e != nil {
		return e
	}
	w.NativeClock, w.Core.GameTurn, w.Level.RandomSeed = clock, int(clock), seed
	w.Core.SetRandomState(random)
	w.Random = uint16(random)
	selected, e := m.Read32(0xf36)
	if e != nil {
		return e
	}
	w.NativeSelected = 0
	if selected != 0 {
		w.NativeSelected = NativeRecordReference(uint16(selected - (h.Memory.BSSBase + 0x76c0)))
	}
	clear(w.Core.MapWho[:])
	clear(w.Core.MapBk2[:])
	clear(w.Core.MapBlk[:])
	clear(w.Marks[:])
	clear(w.Core.Peeps)
	w.Core.Peeps = nil
	w.NativeFollowers = [legacy.MaxPeeps]NativeFollower{}
	w.NativeEntries = [legacy.MaxPeeps]NativeFollowerEntry{}
	w.Heroes = [legacy.MaxPeeps]Hero{}
	w.LightningVictims = [legacy.MaxPeeps]NativeLightningFollower{}
	w.FlameDeaths = nil
	clear(w.flameDeathIndex[:])
	clear(w.captiveIndex[:])
	for _, pool := range [][]NativeWorldOccupancyRecord{w.Occupancy.Walls[:], w.Occupancy.Scenery[:], w.Occupancy.Followers[:], w.Occupancy.Effects[:], w.Occupancy.Magnets[:]} {
		for i := range pool {
			pool[i].Linked = false
		}
	}
	w.hydrateNativeRuntimeGraph()
	seen := map[NativeRecordReference]bool{}
	for pos, cell := range w.Occupancy.Grid.Cells {
		previous := NativeRecordReference(0)
		for ref := cell.Head; ref != 0; {
			if seen[ref] {
				return fmt.Errorf("native loaded map chain cycles at $%x", uint16(ref))
			}
			seen[ref] = true
			entry, ok := w.Occupancy.entry(ref)
			if !ok || entry.Record.Previous != previous {
				return fmt.Errorf("native loaded map chain alias $%x unavailable", uint16(ref))
			}
			parcel, e := nativeOccupancyIndex(entry.Record.X, entry.Record.Y)
			if e != nil || parcel != pos {
				return fmt.Errorf("native loaded map chain parcel differs at $%x", uint16(ref))
			}
			entry.Linked = true
			previous, ref = ref, entry.Record.Next
		}
		switch cell.Tile {
		case 47, 63:
			owner := uint8(0)
			if cell.Tile == 63 {
				owner = 1
			}
			w.Core.MapBlk[pos] = uint8(legacy.FarmBlock) + owner
		case 15:
			w.Core.MapBlk[pos] = legacy.FlatBlock
		default:
			w.Marks[pos] = Mark{Life: 1, Persistent: true, NativeTile: cell.Tile, NativeCodeValid: true}
		}
	}
	w.NativeEnvironment = [NativeEffectCapacity]NativeEnvironmentController{}
	w.hydrateNativeRuntimeRecords()
	w.hydrateCommandTerrainHeights()
	for index, actor := range w.NativeEffects {
		if !actor.Active {
			continue
		}
		switch actor.State {
		case 0x22, 0x24, 0x26:
			w.NativeEnvironment[index] = NativeEnvironmentQuake
		case 0x28, 0x2a:
			w.NativeEnvironment[index] = NativeEnvironmentTsunami
		case 0x2c, 0x2e:
			w.NativeEnvironment[index] = NativeEnvironmentStorm
		case 0x30, 0x32:
			w.NativeEnvironment[index] = NativeEnvironmentVolcano
		case 0x34, 0x36:
			w.NativeEnvironment[index] = NativeEnvironmentLava
		case 0x3c:
			w.NativeEnvironment[index] = NativeEnvironmentHurricane
		case 0x1c, 0x1e, 0x20:
			w.NativeEnvironment[index] = NativeEnvironmentFireRain
		}
	}
	for index, entry := range w.NativeEntries {
		a := entry.Actor
		if a.Owner != 0 && (a.Motion.State == 0x1c || a.Motion.State == 0x1e || a.Motion.State == 0x20 || a.Motion.State == 0x22) {
			w.LightningVictims[index] = NativeLightningFollower{Active: true, Victim: LightningVictim{Kind: a.Motion.Kind, Owner: a.Owner, Flags: a.Motion.Flags, State: a.Motion.State, Next: NativeRecordReference(a.Motion.Next), HeroType: a.Hero40, Animation: a.Motion.Animation, Population: a.Motion.Population, EffectReference: NativeRecordReference(a.Contact30)}}
		}
	}
	x, e := m.Read16(0x5f44)
	if e != nil {
		return e
	}
	y, e := m.Read16(0x5f46)
	if e != nil {
		return e
	}
	w.effectViewX, w.effectViewY = int(int16(x)), int(int16(y))
	for player := 0; player < 2; player++ {
		god := 0xe8a4 + player*314
		rules, e := m.Read16(0xeb2c + player*2)
		if e != nil {
			return e
		}
		w.Rules[player] = DecodeScenarioRules(rules)
		for n := range w.Level.Players[player].Parameters {
			v, e := m.Read16(god + 0x5a + n*2)
			if e != nil {
				return e
			}
			w.Level.Players[player].Parameters[n] = v
		}
		for n := range w.Level.Players[player].Powers {
			v, e := m.Read8(god + 0x70 + n)
			if e != nil {
				return e
			}
			w.Level.Players[player].Powers[n] = int8(v) > 0
		}
		for n := range w.Experience[player] {
			v, e := m.Read8(god + 0x52 + n)
			if e != nil {
				return e
			}
			w.Experience[player][n] = v
		}
		control, e := m.Read16(god + 12)
		if e != nil {
			return e
		}
		switch NativeFollowerMode(control) {
		case NativeFollowerMagnet:
			w.Core.Magnets[player].Flags = legacy.MagnetMode
		case NativeFollowerJoin:
			w.Core.Magnets[player].Flags = legacy.JoinMode
		case NativeFollowerFight:
			w.Core.Magnets[player].Flags = legacy.FightMode
		default:
			w.Core.Magnets[player].Flags = legacy.SettleMode
		}
	}
	name := make([]byte, 16)
	for i := range name {
		v, e := m.Read8(0xeb30 + i)
		if e != nil {
			return e
		}
		name[i] = v
	}
	if end := bytes.IndexByte(name, 0); end >= 0 {
		name = name[:end]
	}
	w.Deity.Name = string(name)
	w.Deity.Experience = w.Experience[profile-1]
	god := 0xe8a4 + int(profile-1)*314
	w.Deity.Bolts, e = m.Read16(god + 0x58)
	if e != nil {
		return e
	}
	for i := range w.Deity.FaceParts {
		v, e := m.Read8(god + 0x4e + i)
		if e != nil {
			return e
		}
		w.Deity.FaceParts[i] = v
	}
	w.bindScenarioRuntime()
	w.Core.FollowerAttrition = func(player int, _ bool) int {
		if player < 0 || player > 1 {
			return 0
		}
		god, _ := NativeDeityAddress(uint8(player + 1))
		value, _ := w.runtimeMemory().Read32(god + 20)
		return int(value)
	}
	return nil
}
