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

// These ordered alternatives retain the hero planner's preference after an
// obstacle. Each row corresponds to the desired horizontal/vertical signs.
var heroAlternatives = [9][8][2]int{
	{{0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}},
	{{-1, -1}, {0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}},
	{{-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}},
	{{1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}},
	{{0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}},
	{{-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}},
	{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}},
	{{1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}},
	{{0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}, {1, 1}},
}

func (w *World) validHeroTarget(id, target int) bool {
	if target <= 0 || target >= FollowerCapacity {
		return false
	}
	a, b := w.Followers[id], w.Followers[target]
	return b.State != Inactive && b.Population > 0 && a.Owner != b.Owner
}

// stepHero separates initial target selection from pursuit. Finding an enemy
// ends that pass; pursuing subtracts attrition before planning another leg.
func (w *World) stepHero(id int) {
	f := &w.Followers[id]
	if f.Hero.Phase == HeroFindTarget {
		w.SelectHeroTarget(id)
		return
	}
	if f.Hero.Phase == HeroWaiting {
		f.Hero.Wait--
		if f.Hero.Wait < 0 {
			f.Hero.Phase = HeroFindTarget
			w.SelectHeroTarget(id)
		}
		return
	}
	if f.moving {
		w.advanceLeg(id)
		return
	}
	f.Population = int(int32(uint32(f.Population) - uint32(w.Level.Players[f.Owner].Attrition)))
	if f.Population <= 0 {
		w.remove(id)
		return
	}
	if !w.validHeroTarget(id, f.Hero.Target) {
		w.SelectHeroTarget(id)
	}
	if !w.validHeroTarget(id, f.Hero.Target) {
		f.Hero.Phase = HeroWaiting
		f.Hero.Wait = 20
		f.Frame = 0
		return
	}
	target := w.Followers[f.Hero.Target]
	dx, dy := sign(int(target.X)-int(f.X)), sign(int(target.Y)-int(f.Y))
	if dx == 0 && dy == 0 {
		if f.Hero.Kind == HeroHelen {
			w.CaptureByHelen(id, f.Hero.Target)
		} else {
			w.beginBattle(id, f.Hero.Target)
		}
		return
	}
	if !w.heroCanEnter(id, int(f.X)+dx, int(f.Y)+dy) {
		selected := false
		for _, d := range heroAlternatives[3*dx+dy+4] {
			if d == [2]int{-dx, -dy} {
				continue
			}
			if w.heroCanEnter(id, int(f.X)+d[0], int(f.Y)+d[1]) {
				dx, dy, selected = d[0], d[1], true
				break
			}
		}
		if !selected && !w.heroCanEnter(id, int(f.X)+dx, int(f.Y)+dy) {
			dx, dy = 0, 0
		}
	}
	f.initialisePosition()
	f.positionX = int(f.X)*256 + 128
	f.positionY = int(f.Y)*256 + 128
	if f.MovementSpeed == 0 {
		return
	}
	f.velocityX, f.velocityY = dx*int(f.MovementSpeed), dy*int(f.MovementSpeed)
	f.legRemaining = 256 / int(f.MovementSpeed)
	f.moving = true
	f.Hero.Phase = HeroPursuing
	f.PreviousX, f.PreviousY = f.X, f.Y
	f.MoveProgress = 0
	for direction, d := range directions {
		if d == [2]int{dx, dy} {
			f.Direction = uint8(direction)
			break
		}
	}
	// Source pursuit redispatches its new motion immediately.
	w.advanceLeg(id)
}

func (w *World) heroCanEnter(id, x, y int) bool {
	if !inside(x, y) || w.Nature.BlocksWalking(x, y) {
		return false
	}
	if w.Cell(x, y).IsWater() && !w.Followers[id].ImmuneToDrowning() {
		return false
	}
	// Perseus requests terrain before entering destructive ground, whereas
	// the other heroes reach the common hazard prepass and use their immunity.
	if w.Followers[id].Hero.Kind == HeroPerseus {
		mark := w.Nature.Ground[x+y*MapSize].Mark
		if mark == GroundSwamp || mark >= GroundFungusFresh && mark <= GroundFungusDying || mark == GroundScorched {
			return false
		}
	}
	return true
}

// clearHeroLinks keeps typed reciprocal references consistent on death or
// conversion. Captives retain their faith and resume ordinary walking.
func (w *World) clearHeroClaim(id int) {
	if id <= 0 || id >= FollowerCapacity {
		return
	}
	source := &w.Followers[id]
	if target := source.Hero.Target; target > 0 && target < FollowerCapacity && w.Followers[target].Hero.ClaimedBy == id {
		w.Followers[target].Hero.ClaimedBy = 0
	}
	if claimant := source.Hero.ClaimedBy; claimant > 0 && claimant < FollowerCapacity && w.Followers[claimant].Hero.Target == id {
		w.Followers[claimant].Hero.Target = 0
	}
	source.Hero.Target = 0
	source.Hero.ClaimedBy = 0
}
func (w *World) clearHeroLinks(id int) {
	if id <= 0 || id >= FollowerCapacity {
		return
	}
	w.clearHeroClaim(id)
	source := &w.Followers[id]

	for other := 1; other < FollowerCapacity; other++ {
		if w.Followers[other].Hero.CaptiveOf == id {
			w.Followers[other].Hero.CaptiveOf = 0
			w.Followers[other].Hero.Phase = HeroFindTarget
			w.Followers[other].State = Walking
		}
	}
	source.Hero.Target = 0
	source.Hero.ClaimedBy = 0
}

// CaptureByHelen changes neither owner nor population. Captives remain linked
// to their captor and are released when that hero dies, rather than converted
// to the captor's faith as the Baptism power would do.
func (w *World) CaptureByHelen(hero, victim int) bool {
	if hero <= 0 || hero >= FollowerCapacity || victim <= 0 || victim >= FollowerCapacity || hero == victim {
		return false
	}
	h, v := &w.Followers[hero], &w.Followers[victim]
	if h.Hero.Kind != HeroHelen || v.State == Inactive || h.Owner == v.Owner {
		return false
	}
	w.clearHeroClaim(hero)
	w.clearHeroLinks(victim)
	wasTown := v.State == Town
	v.State = Walking
	v.Hero.CaptiveOf = hero
	v.Hero.Phase = HeroCaptive
	v.moving = false
	v.Frame = 0
	v.BattleWith = 0
	h.Hero.Phase = HeroFindTarget
	h.moving = false
	h.Frame = 0
	if wasTown {
		w.repaintFarms()
	}
	return true
}

func (w *World) stepCaptive(id int) {
	f := &w.Followers[id]
	captor := f.Hero.CaptiveOf
	if captor <= 0 || captor >= FollowerCapacity || w.Followers[captor].State == Inactive || w.Followers[captor].Hero.Kind != HeroHelen {
		f.Hero.CaptiveOf = 0
		f.Hero.Phase = HeroFindTarget
		f.moving = false
		return
	}
	if f.moving {
		w.advanceLeg(id)
		return
	}
	f.Population -= w.Level.Players[f.Owner].Attrition
	if f.Population <= 0 {
		w.remove(id)
		return
	}
	h := w.Followers[captor]
	if int(f.X) == int(h.X) && int(f.Y) == int(h.Y) {
		return
	}
	x := int(f.X) + sign(int(h.X)-int(f.X))
	y := int(f.Y) + sign(int(h.Y)-int(f.Y))
	if !inside(x, y) || w.Cell(x, y).IsWater() {
		return
	}
	w.beginLeg(id, x, y)
	w.advanceLeg(id)
}

// SplitAdonis halves the surviving population before attempting a single
// clone. A full pool retains that halved parent, matching original admission.
func (w *World) SplitAdonis(id int) int {
	if id <= 0 || id >= FollowerCapacity || w.Followers[id].Hero.Kind != HeroAdonis || w.Followers[id].Population <= 20 {
		return 0
	}
	f := &w.Followers[id]
	f.Population = int(uint32(f.Population) >> 1)
	child := *f
	child.Hero.Target = 0
	child.Hero.ClaimedBy = 0
	child.Hero.Phase = HeroFindTarget
	child.Frame = 0
	child.State = Walking
	child.BattleWith = 0
	child.BattleAggressor = false
	return w.allocate(child)
}
