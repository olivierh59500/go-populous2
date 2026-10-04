package populous2

import (
	"encoding/json"
	"fmt"
	"io"

	legacy "go-populous2/internal/legacy"
)

const SaveVersion = 14

type Snapshot struct {
	Version          int
	LevelIndex       int
	ScenarioOptions  [2]uint16
	Custom           bool
	Demo             bool
	Experience       [2][6]uint8
	Deity            Deity
	Core             legacy.WorldSnapshot
	Effects          []Effect
	Marks            [4096]Mark
	Heroes           [legacy.MaxPeeps]Hero
	Random           uint16
	LastSpell        SpellID
	LastPlayer       int
	SpellSerial      int
	HazardSerial     int
	LastHazardCue    int
	Scenery          [SceneryCapacity]SceneryActor
	Walls            WallState
	NativeEffects    [NativeEffectCapacity]NativeEffectActor
	NativeFollowers  [legacy.MaxPeeps]NativeFollower
	Occupancy        NativeWorldOccupancy
	BasaltState      BasaltState
	LightningState   LightningState
	LightningVictims [legacy.MaxPeeps]NativeLightningFollower
	FungusState      FungusState
	FlameDeaths      []FlameDeath
}

func (w *World) Snapshot() Snapshot {
	canonical := *w
	canonical.reconcileActorGraph()
	w = &canonical
	heroes := w.Heroes
	for i := range heroes {
		heroes[i].Captives = append([]int(nil), heroes[i].Captives...)
	}
	return Snapshot{Version: SaveVersion, LevelIndex: w.Level.Number, ScenarioOptions: [2]uint16{w.Rules[0].Raw, w.Rules[1].Raw}, Custom: w.Custom, Demo: w.Demo, Experience: w.Experience, Deity: w.Deity, Core: w.Core.Snapshot(), Effects: append([]Effect(nil), w.Effects...), Marks: w.Marks, Heroes: heroes, Random: w.Random, LastSpell: w.LastSpell, LastPlayer: w.LastPlayer, SpellSerial: w.SpellSerial, HazardSerial: w.HazardSerial, LastHazardCue: w.LastHazardCue, Scenery: w.Scenery, Walls: w.Walls, NativeEffects: w.NativeEffects, NativeFollowers: w.nativeFollowerSnapshot(), Occupancy: w.Occupancy, BasaltState: w.BasaltState, LightningState: w.LightningState, LightningVictims: w.LightningVictims, FungusState: w.FungusState, FlameDeaths: append([]FlameDeath(nil), w.FlameDeaths...)}
}

func Restore(bundle *Bundle, snapshot Snapshot) (*World, error) {
	if snapshot.Version < 1 || snapshot.Version > SaveVersion {
		return nil, fmt.Errorf("unsupported save version %d", snapshot.Version)
	}
	if snapshot.Version >= 13 {
		if err := snapshot.Occupancy.Validate(); err != nil {
			return nil, fmt.Errorf("invalid saved native actor graph: %w", err)
		}
		if err := validateSavedActorGraph(snapshot); err != nil {
			return nil, err
		}
	}
	if snapshot.Version < 3 {
		// Earlier prototypes inverted the two final water powers. Their
		// numeric save IDs are migrated before native catalog validation.
		swap := func(id SpellID) SpellID {
			if id == 33 {
				return 34
			}
			if id == 34 {
				return 33
			}
			return id
		}
		snapshot.LastSpell = swap(snapshot.LastSpell)
		for i := range snapshot.Heroes {
			snapshot.Heroes[i].Spell = swap(snapshot.Heroes[i].Spell)
		}
		for i := range snapshot.Marks {
			snapshot.Marks[i].Spell = swap(snapshot.Marks[i].Spell)
		}
		// The caller may retain its snapshot slice; migration must not edit it.
		snapshot.Effects = append([]Effect(nil), snapshot.Effects...)
		for i := range snapshot.Effects {
			snapshot.Effects[i].Spell = swap(snapshot.Effects[i].Spell)
		}
	}
	if snapshot.Version < 12 {
		// Commands 24 and 74 use water slots 31 and 30, respectively.
		// Earlier Go saves named those slots in the opposite order.
		swap := func(id SpellID) SpellID {
			if id == 30 {
				return 31
			}
			if id == 31 {
				return 30
			}
			return id
		}
		snapshot.LastSpell = swap(snapshot.LastSpell)
		for i := range snapshot.Marks {
			snapshot.Marks[i].Spell = swap(snapshot.Marks[i].Spell)
		}
		snapshot.Effects = append([]Effect(nil), snapshot.Effects...)
		for i := range snapshot.Effects {
			snapshot.Effects[i].Spell = swap(snapshot.Effects[i].Spell)
		}
	}
	for _, part := range snapshot.Deity.FaceParts {
		if part > 7 {
			return nil, fmt.Errorf("invalid saved deity face")
		}
	}
	if len(snapshot.Core.Peeps) > legacy.MaxFollowers || len(snapshot.Effects) > 256 || snapshot.Core.GameTurn < 0 {
		return nil, fmt.Errorf("invalid saved simulation bounds")
	}
	for _, h := range snapshot.Core.Alt {
		if h < 0 || h > 8 {
			return nil, fmt.Errorf("invalid saved altitude %d", h)
		}
	}
	for index, p := range snapshot.Core.Peeps {
		v := snapshot.LightningVictims[index]
		managedSigned := snapshot.Version >= 14 && v.Active && (v.Victim.State == 0x1c || v.Victim.State == 0x1e) && int32(p.Population) == v.Victim.Population
		if p.Player > 1 || p.Population < 0 && !managedSigned || p.AtPos < 0 || p.AtPos >= 4096 {
			return nil, fmt.Errorf("invalid saved follower")
		}
	}
	if snapshot.Version >= 12 {
		for index, record := range snapshot.NativeFollowers {
			if !record.Active {
				continue
			}
			a := record.Actor
			if index >= len(snapshot.Core.Peeps) || record.Generation == 0 || a.Kind != 2 || a.Player > 1 || a.Speed == 0 || a.X < 0 || a.Y < 0 || a.X >= 0x4000 || a.Y >= 0x4000 || a.State != 2 && a.State != 4 && a.State != 18 || a.ReturnState != 2 && a.ReturnState != 18 || a.Animation < 0 || a.Animation >= bundle.FollowerMotion.FrameCount*4 || a.Animation%4 != 0 || a.Timer < -1 || a.Timer > 256/int16(a.Speed) {
				return nil, fmt.Errorf("invalid saved native follower motion")
			}
			p := snapshot.Core.Peeps[index]
			if p.Population <= 0 || p.Flags != legacy.OnMove || p.Status == legacy.KnightStatus || snapshot.Heroes[index].Active || a.Player != p.Player || int(a.X)>>8+(int(a.Y)>>8)*64 != p.AtPos {
				return nil, fmt.Errorf("saved native follower identity mismatch")
			}
			if _, _, ok := bundle.FollowerMotion.Frame(a); !ok {
				return nil, fmt.Errorf("invalid saved native follower image bank")
			}
			velocity := func(value int16) bool { return value == 0 || value == int16(a.Speed) || value == -int16(a.Speed) }
			if !velocity(a.VX) || !velocity(a.VY) || !validNativeFollowerLink(a.Next) || !validNativeFollowerLink(a.Previous) {
				return nil, fmt.Errorf("invalid saved native follower velocity/link")
			}
		}
	}
	for _, index := range snapshot.Core.MapWho {
		if int(index) > len(snapshot.Core.Peeps) {
			return nil, fmt.Errorf("invalid saved occupancy reference")
		}
	}
	for _, mark := range snapshot.Marks {
		if mark.Player < 0 || mark.Player > 1 || mark.Life < 0 || mark.Life > 2000 {
			return nil, fmt.Errorf("invalid saved terrain effect")
		}
	}
	for _, hero := range snapshot.Heroes {
		if hero.Active && (!hero.Spell.IsHero() || hero.Player < 0 || hero.Player > 1) {
			return nil, fmt.Errorf("invalid saved hero")
		}
		for _, captive := range hero.Captives {
			if captive < 0 || captive >= len(snapshot.Core.Peeps) || hero.Spell != Helen || !hero.Active {
				return nil, fmt.Errorf("invalid saved captive")
			}
		}
	}
	for _, effect := range snapshot.Effects {
		if !inside(effect.X, effect.Y) || effect.Player < 0 || effect.Player > 1 || effect.Life < 0 || effect.Life > 2000 {
			return nil, fmt.Errorf("invalid saved active effect")
		}
		if snapshot.Version >= 9 && effect.Spell == Whirlwind {
			return nil, fmt.Errorf("saved whirlwind requires a native effect record")
		}
		if snapshot.Version >= 13 && effect.Spell == Whirlpool {
			return nil, fmt.Errorf("saved whirlpool requires a native effect record")
		}
		if snapshot.Version >= 14 && effect.Spell == Lightning {
			return nil, fmt.Errorf("saved lightning requires a native marker/bolt")
		}
	}
	var sceneryTiles [4096]bool
	for _, actor := range snapshot.Scenery {
		if actor.Active && (!inside(actor.X, actor.Y) || actor.Kind != SceneryTree && actor.Kind != SceneryBoulder) {
			return nil, fmt.Errorf("invalid saved scenery actor")
		}
		if actor.Active {
			pos := actor.X + actor.Y*64
			if sceneryTiles[pos] || actor.Frame < 0 {
				return nil, fmt.Errorf("invalid saved scenery occupancy/frame")
			}
			sceneryTiles[pos] = true
			if _, ok := bundle.Scenery.Frames[actor.Animation]; !ok {
				return nil, fmt.Errorf("unknown saved scenery animation")
			}
		}
	}
	for _, a := range snapshot.NativeEffects {
		if !a.Active {
			continue
		}
		if a.Kind == FungusActorKind {
			if snapshot.Version < 10 {
				return nil, fmt.Errorf("fungus controller requires save version 10")
			}
			continue
		}
		if a.Kind == WhirlpoolActorKind {
			if snapshot.Version < 13 || a.Player > 1 || a.X < 0 || a.Y < 0 || a.X >= 0x4000 || a.Y >= 0x4000 || a.State != 0x0e || a.Speed == 0 || a.Animation < 0x97 || a.Animation > 0xa3 || (a.Animation-0x97)%4 != 0 {
				return nil, fmt.Errorf("invalid saved native whirlpool")
			}
			continue
		}
		if a.Kind == 0x28 || a.Kind == 0x2a {
			if snapshot.Version < 14 || a.Player > 1 || a.X < 0 || a.Y < 0 || a.X >= 0x4000 || a.Y >= 0x4000 || a.Kind == 0x28 && a.State != 22 && a.State != 26 || a.Kind == 0x2a && a.State != 24 {
				return nil, fmt.Errorf("invalid saved lightning actor")
			}
			if a.Kind == 0x28 {
				if _, ok := bundle.LightningRules.Frames[a.Animation]; !ok {
					return nil, fmt.Errorf("invalid saved lightning marker frame")
				}
			}
			continue
		}
		if a.Player > 1 || a.X < 0 || a.Y < 0 || a.X >= 0x4000 || a.Y >= 0x4000 || a.Speed == 0 && a.Kind != BasaltActorKind || !bundle.validNativeEffectPhase(a) {
			return nil, fmt.Errorf("invalid native effect actor")
		}
		if _, ok := bundle.NativeEffectFrame(a); !ok {
			return nil, fmt.Errorf("invalid native effect animation")
		}
	}
	for index, actor := range snapshot.NativeEffects {
		if actor.Active && actor.Kind == BasaltActorKind && (snapshot.Version < 13 || snapshot.BasaltState.Directions[index] > 6 || snapshot.BasaltState.Directions[index]&1 != 0) {
			return nil, fmt.Errorf("invalid saved basalt direction")
		}
	}
	if !validSavedFungus(&snapshot.NativeEffects, &snapshot.FungusState) {
		return nil, fmt.Errorf("invalid saved fungus controller/reference")
	}
	if err := validateLightningSave(bundle, snapshot); err != nil {
		return nil, err
	}
	if len(snapshot.FlameDeaths) > legacy.MaxFollowers {
		return nil, fmt.Errorf("too many saved flame deaths")
	}
	if snapshot.HazardSerial < 0 || snapshot.LastHazardCue < 0 || snapshot.LastHazardCue >= len(bundle.Audio.Patterns) {
		return nil, fmt.Errorf("invalid saved hazard sound event")
	}
	var reservedDeaths [legacy.MaxPeeps]bool
	for _, death := range snapshot.FlameDeaths {
		if !inside(death.X, death.Y) || death.Animation >= death.End || death.Follower < 0 || death.Follower >= len(snapshot.Core.Peeps) {
			return nil, fmt.Errorf("invalid saved flame death")
		}
		if reservedDeaths[death.Follower] || !bundle.validFollowerDeathSpan(death.Animation, death.End) {
			return nil, fmt.Errorf("invalid saved follower death span/reservation")
		}
		reservedDeaths[death.Follower] = true
		if death.Kind != 0 && (death.Kind != bundle.FungusHazards.DeathKind || death.State != bundle.FungusHazards.DeathState) {
			return nil, fmt.Errorf("invalid saved follower hazard state")
		}
		if _, ok := bundle.FollowerDeathFrame(death.Animation); !ok {
			return nil, fmt.Errorf("unknown saved flame death frame")
		}
	}
	w, err := NewWorld(bundle, snapshot.LevelIndex, snapshot.Custom)
	if err != nil {
		return nil, err
	}
	if snapshot.Version >= 8 {
		for player, raw := range snapshot.ScenarioOptions {
			w.Rules[player] = DecodeScenarioRules(raw)
		}
	}
	if err := snapshot.Walls.Validate(&bundle.WallRules); err != nil {
		return nil, fmt.Errorf("invalid saved walls: %w", err)
	}
	townRules := w.Core.OlympianTowns
	w.Core = legacy.WorldFromSnapshot(snapshot.Core, w.Core.Rules)
	w.Core.OlympianTowns = townRules
	w.Core.TileBlocked = func(pos int) bool { return w.sceneryAt(pos) >= 0 }
	w.Core.HabitatBlocked = func(pos int) bool { i := w.sceneryAt(pos); return i >= 0 && w.Scenery[i].Kind == SceneryBoulder }
	w.bindHabitatTerrain()
	w.bindHeroCombat()
	w.bindScenarioRuntime()
	w.Effects = append([]Effect(nil), snapshot.Effects...)
	w.Experience = snapshot.Experience
	w.Deity = snapshot.Deity
	if snapshot.Version < 6 {
		w.Deity = NewDeity("PLAYER")
		w.Deity.Experience = w.Experience[0]
		w.Deity.Bolts = 0
	}
	w.Marks, w.Heroes, w.Random = snapshot.Marks, snapshot.Heroes, snapshot.Random
	for i := range w.Heroes {
		w.Heroes[i].Captives = append([]int(nil), w.Heroes[i].Captives...)
	}
	w.rebuildCaptiveIndex()
	w.Scenery = snapshot.Scenery
	w.Walls = snapshot.Walls
	w.NativeEffects = snapshot.NativeEffects
	w.LightningState, w.LightningVictims = snapshot.LightningState, snapshot.LightningVictims
	w.BasaltState = snapshot.BasaltState
	w.FungusState = snapshot.FungusState
	if snapshot.Version < 7 {
		live := w.Effects[:0]
		for _, effect := range w.Effects {
			if effect.Spell != FireColumn {
				live = append(live, effect)
				continue
			}
			for i := range w.NativeEffects {
				if !w.NativeEffects[i].Active {
					w.NativeEffects[i] = NativeEffectActor{Active: true, Kind: 0x22, Player: uint8(effect.Player), X: int16(effect.X*256 + 128), Y: int16(effect.Y*256 + 128), VX: int16(effect.DX) * 16, VY: int16(effect.DY) * 16, Speed: 16, State: 4, Animation: 0x4b8, Timer: 1, Life: int16(max(1, effect.Life))}
					break
				}
			}
		}
		w.Effects = live
	}
	if snapshot.Version < 9 {
		live := w.Effects[:0]
		for _, effect := range w.Effects {
			if effect.Spell != Whirlwind {
				live = append(live, effect)
				continue
			}
			slot := -1
			for i := range w.NativeEffects {
				if !w.NativeEffects[i].Active {
					slot = i
					break
				}
			}
			if !w.Whirlwinds.Create(&w.NativeEffects, effect.Player, effect.X, effect.Y, w.Experience[effect.Player][Air]) {
				return nil, fmt.Errorf("saved whirlwind exceeds native effect capacity")
			}
			actor := &w.NativeEffects[slot]
			actor.State, actor.Animation = 10, 0x4c8
			actor.Life = int16(max(1, effect.Life))
			actor.VX, actor.VY = int16(effect.DX)*int16(actor.Speed), int16(effect.DY)*int16(actor.Speed)
		}
		w.Effects = live
	}
	if snapshot.Version < 10 {
		for pos, mark := range w.Marks {
			if mark.Spell != Fungus || mark.Life == 0 || mark.NativeTile != 0 {
				continue
			}
			// Old prototypes stored expiring radius damage. Retain each marked
			// cell as an original seed and collect a controller per owner.
			placement := w.castFungus(mark.Player, pos%64, pos/64)
			if placement.Planted && placement.Slot < 0 {
				return nil, fmt.Errorf("saved fungus exceeds native effect capacity")
			}
			if !placement.Planted {
				w.Marks[pos] = Mark{}
			}
		}
	}
	if snapshot.Version < 13 {
		live := w.Effects[:0]
		for _, effect := range w.Effects {
			if effect.Spell != Whirlpool {
				live = append(live, effect)
				continue
			}
			slot := -1
			for index := range w.NativeEffects {
				if !w.NativeEffects[index].Active {
					slot = index
					break
				}
			}
			if slot < 0 {
				return nil, fmt.Errorf("saved whirlpool exceeds native effect capacity")
			}
			// Retain the old effect's remaining life and location even if its
			// former target would now fail exact four-water cast admission.
			w.NativeEffects[slot] = NativeEffectActor{Active: true, Kind: WhirlpoolActorKind, Player: uint8(effect.Player), X: int16(effect.X * 256), Y: int16(effect.Y * 256), Speed: w.Whirlpools.Speed, Timer: int16(w.Whirlpools.Speed), Life: int16(max(1, effect.Life)), State: 0x0e, Animation: 0x97}
		}
		w.Effects = live
		for pos, mark := range w.Marks {
			if mark.Spell == Basalt && mark.Life > 0 && mark.NativeTile == 0 {
				w.Marks[pos] = Mark{Spell: Basalt, Player: mark.Player, Life: 1, Persistent: true, NativeTile: 0xe0}
				w.setBasaltLegacyLand(pos%64, pos/64)
			}
		}
	}
	if snapshot.Version < 14 {
		live := w.Effects[:0]
		for _, effect := range w.Effects {
			if effect.Spell != Lightning {
				live = append(live, effect)
				continue
			}
			// An obsolete instantaneous-area effect has no native bolt chain.
			// Preserve its location as a marker; no invented damage is replayed.
			placed, err := w.LightningRules.Place(&w.LightningState, &w.NativeEffects, effect.Player, effect.X, effect.Y, w.lightningCallbacks())
			if err != nil {
				return nil, fmt.Errorf("saved lightning migration: %w", err)
			}
			if !placed {
				return nil, fmt.Errorf("saved lightning exceeds native marker capacity")
			}
		}
		w.Effects = live
	}
	w.FlameDeaths = append([]FlameDeath(nil), snapshot.FlameDeaths...)
	w.NativeFollowers = snapshot.NativeFollowers
	if snapshot.Version < 12 {
		w.NativeFollowers = [legacy.MaxPeeps]NativeFollower{}
		for index := range w.Core.Peeps {
			w.initializeNativeFollower(index)
		}
	}
	w.bindFollowerMotion()
	w.bindFlameDeaths()
	w.bindFollowerHazards()
	w.bindLightningVictims()
	w.bindWallMovement()
	w.rebuildSceneryIndex()
	if snapshot.Version >= 13 {
		w.Occupancy = snapshot.Occupancy
		if snapshot.Version < 14 {
			w.reconcileActorGraph()
		}
	} else {
		w.initializeActorGraph()
		w.reconcileActorGraph()
	}
	w.bindActorGraphHooks()
	w.Demo, w.LastSpell, w.LastPlayer, w.SpellSerial = snapshot.Demo, snapshot.LastSpell, snapshot.LastPlayer, snapshot.SpellSerial
	w.HazardSerial, w.LastHazardCue = snapshot.HazardSerial, snapshot.LastHazardCue
	return w, nil
}

func ReadSave(bundle *Bundle, reader io.Reader) (*World, error) {
	var snapshot Snapshot
	decoder := json.NewDecoder(io.LimitReader(reader, 8<<20))
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("read save: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("unexpected data after saved world")
	}
	return Restore(bundle, snapshot)
}
