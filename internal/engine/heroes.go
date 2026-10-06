package engine

import "fmt"

type HeroKind uint8

const (
	HeroNone HeroKind = iota
	HeroPerseus
	HeroAdonis
	HeroHeracles
	HeroOdysseus
	HeroAchilles
	HeroHelen
)

type HeroPhase uint8

const (
	HeroFindTarget HeroPhase = iota
	HeroPursuing
	HeroWaiting
	HeroCaptive
	HeroDying
)

type HeroState struct {
	Kind                         HeroKind
	Phase                        HeroPhase
	Target, ClaimedBy, CaptiveOf int
	Wait                         int
}
type HeroHazard uint8

const (
	HazardSwamp HeroHazard = iota
	HazardFungus
	HazardBurning
	HazardDrowning
	HazardFatal
	HazardDirectFire
)

func (f Follower) IsHero() bool              { return f.Hero.Kind != HeroNone }
func (f Follower) ImmuneToSwamp() bool       { return f.Hero.Kind == HeroAdonis }
func (f Follower) ImmuneToFungus() bool      { return f.Hero.Kind == HeroAdonis }
func (f Follower) ImmuneToBurning() bool     { return f.Hero.Kind == HeroAchilles }
func (f Follower) ImmuneToDrowning() bool    { return f.Hero.Kind == HeroHelen }
func (f Follower) ImmuneToFatalGround() bool { return f.Hero.Kind == HeroHeracles }

// HazardDeathFrames gives the duration of the original named artwork sequence;
// zero represents elemental immunity, not a missing or immediate animation.
func (f Follower) HazardDeathFrames(hazard HeroHazard) int {
	if !f.IsHero() {
		switch hazard {
		case HazardBurning:
			return 21
		case HazardDirectFire:
			return 9
		default:
			return 2
		}
	}
	index := int(f.Hero.Kind) - 1
	switch hazard {
	case HazardSwamp, HazardFungus:
		return [6]int{5, 0, 9, 9, 8, 3}[index]
	case HazardBurning, HazardDirectFire:
		return [6]int{9, 9, 9, 9, 0, 9}[index]
	case HazardDrowning:
		return [6]int{5, 9, 9, 9, 8, 0}[index]
	case HazardFatal:
		return [6]int{5, 9, 0, 9, 8, 3}[index]
	}
	return 0
}

func HeroKindForPower(id PowerID) (HeroKind, bool) {
	switch id {
	case Perseus:
		return HeroPerseus, true
	case Adonis:
		return HeroAdonis, true
	case Heracles:
		return HeroHeracles, true
	case Odysseus:
		return HeroOdysseus, true
	case Achilles:
		return HeroAchilles, true
	case Helen:
		return HeroHelen, true
	}
	return HeroNone, false
}

// CreateHero selects the current leader. Admission/debit belongs to Cast;
// this method neither spends mana nor advances the shared random generator.
func (w *World) CreateHero(owner int, kind HeroKind) (int, error) {
	if owner < 0 || owner > 1 || kind <= HeroNone || kind > HeroHelen {
		return 0, fmt.Errorf("invalid hero creation")
	}
	id := w.Players[owner].Leader
	if id <= 0 || id >= FollowerCapacity {
		return 0, fmt.Errorf("a leader is required")
	}
	if err := w.ConvertHero(id, kind); err != nil {
		return 0, err
	}
	return id, nil
}

// ConvertHero also serves Adonis offspring. Motion coordinates, velocities,
// leg timer, contacts, and unrelated follower fields survive conversion.
func (w *World) ConvertHero(id int, kind HeroKind) error {
	if id <= 0 || id >= FollowerCapacity || kind <= HeroNone || kind > HeroHelen {
		return fmt.Errorf("invalid hero conversion")
	}
	f := &w.Followers[id]
	if f.State == Inactive {
		return fmt.Errorf("inactive follower")
	}
	owner := int(f.Owner)
	if w.Players[owner].Leader == id {
		w.Players[owner].Leader = 0
	}
	wasTown := f.State == Town
	f.State = Walking
	f.Frame = 0
	f.Hero.Kind = kind
	f.Hero.Phase = HeroFindTarget
	if kind == HeroHeracles {
		f.Population = int(int32(uint32(f.Population) * 2))
	}
	bonus := int(w.Players[owner].Experience[int(kind)-1] >> 3)
	if kind == HeroOdysseus {
		bonus += int(f.MovementSpeed)
	}
	f.MovementSpeed = uint8(min(255, int(f.MovementSpeed)+bonus))
	if wasTown {
		w.repaintFarms()
	}
	return nil
}

// SelectHeroTarget reserves one enemy with a reciprocal claim. Preferred
// distance ties select the later follower slot. If all enemies are captive
// or already claimed, the last eligible fallback is retained.
func (w *World) SelectHeroTarget(id int) int {
	if id <= 0 || id >= FollowerCapacity || !w.Followers[id].IsHero() {
		return 0
	}
	f := &w.Followers[id]
	best := 32767
	preferred, fallback := 0, 0
	for other := 1; other < FollowerCapacity; other++ {
		enemy := w.Followers[other]
		if other == id || enemy.State == Inactive || enemy.Owner == f.Owner || enemy.Population <= 0 {
			continue
		}
		distance := abs(int(enemy.X)-int(f.X)) + abs(int(enemy.Y)-int(f.Y))
		if best < distance {
			continue
		}
		fallback = other
		if enemy.Hero.CaptiveOf != 0 || enemy.Hero.ClaimedBy != 0 {
			continue
		}
		best, preferred = distance, other
	}
	if preferred == 0 {
		preferred = fallback
	}
	if preferred != 0 {
		f.Hero.Target = preferred
		f.Hero.Phase = HeroPursuing
		w.Followers[preferred].Hero.ClaimedBy = id
	}
	return preferred
}
