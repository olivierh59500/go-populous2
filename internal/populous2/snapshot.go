package populous2

import (
	"encoding/json"
	"fmt"
	"io"

	legacy "go-populous2/internal/legacy"
)

const SaveVersion = 4

type Snapshot struct {
	Version     int
	LevelIndex  int
	Custom      bool
	Demo        bool
	Experience  [2][6]uint8
	Core        legacy.WorldSnapshot
	Effects     []Effect
	Marks       [4096]Mark
	Heroes      [legacy.MaxPeeps]Hero
	Random      uint16
	LastSpell   SpellID
	LastPlayer  int
	SpellSerial int
}

func (w *World) Snapshot() Snapshot {
	heroes := w.Heroes
	for i := range heroes {
		heroes[i].Captives = append([]int(nil), heroes[i].Captives...)
	}
	return Snapshot{Version: SaveVersion, LevelIndex: w.Level.Number, Custom: w.Custom, Demo: w.Demo, Experience: w.Experience, Core: w.Core.Snapshot(), Effects: append([]Effect(nil), w.Effects...), Marks: w.Marks, Heroes: heroes, Random: w.Random, LastSpell: w.LastSpell, LastPlayer: w.LastPlayer, SpellSerial: w.SpellSerial}
}

func Restore(bundle *Bundle, snapshot Snapshot) (*World, error) {
	if snapshot.Version < 1 || snapshot.Version > SaveVersion {
		return nil, fmt.Errorf("unsupported save version %d", snapshot.Version)
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
	if len(snapshot.Core.Peeps) > legacy.MaxFollowers || len(snapshot.Effects) > 256 || snapshot.Core.GameTurn < 0 {
		return nil, fmt.Errorf("invalid saved simulation bounds")
	}
	for _, h := range snapshot.Core.Alt {
		if h < 0 || h > 8 {
			return nil, fmt.Errorf("invalid saved altitude %d", h)
		}
	}
	for _, p := range snapshot.Core.Peeps {
		if p.Player > 1 || p.Population < 0 || p.AtPos < 0 || p.AtPos >= 4096 {
			return nil, fmt.Errorf("invalid saved follower")
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
	}
	w, err := NewWorld(bundle, snapshot.LevelIndex, snapshot.Custom)
	if err != nil {
		return nil, err
	}
	townRules := w.Core.OlympianTowns
	w.Core = legacy.WorldFromSnapshot(snapshot.Core, w.Core.Rules)
	w.Core.OlympianTowns = townRules
	w.bindHeroCombat()
	w.Effects = append([]Effect(nil), snapshot.Effects...)
	w.Experience = snapshot.Experience
	w.Marks, w.Heroes, w.Random = snapshot.Marks, snapshot.Heroes, snapshot.Random
	for i := range w.Heroes {
		w.Heroes[i].Captives = append([]int(nil), w.Heroes[i].Captives...)
	}
	w.rebuildCaptiveIndex()
	w.Demo, w.LastSpell, w.LastPlayer, w.SpellSerial = snapshot.Demo, snapshot.LastSpell, snapshot.LastPlayer, snapshot.SpellSerial
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
