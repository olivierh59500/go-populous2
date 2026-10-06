package engine

// AirHabitat supplies game interactions independently of each effect's motion
// and animation. Lightning scans and whirlwind carrying remain distinct
// operations; neither is replaced with generic radius damage.
type AirHabitat interface {
	Reserve(EffectKind, uint8) int
	Release(int)
	Random() uint16
	AirExperience(uint8) uint8
	Parcel(x, y int) FireParcel
	StrikeLightning(bolt, x, y int) bool // False means a wall stopped the scan.
	Scorch(x, y int)
	DamageStorm(x, y int) int
	CreateWhirlpool(owner uint8, x, y int)
	LiftFollowers(effect, x, y int)
	ReleaseFollowers(effect, x, y int)
}

type LightningPhase uint8

const (
	LightningAppearing LightningPhase = iota
	LightningSteady
	LightningDisappearing
)

type LightningMarker struct {
	Active    bool
	Owner     uint8
	X, Y      int
	Life      int
	Phase     LightningPhase
	Frame     int
	FirstBolt int // Shared slot plus one; zero is the empty list.
}

type LightningBolt struct {
	Active       bool
	Owner        uint8
	X, Y         int
	Marker, Next int // Shared slots plus one.
	Random       uint16
}

type WhirlwindPhase uint8

const (
	WhirlwindAppearing WhirlwindPhase = iota
	WhirlwindMoving
	WhirlwindDisappearing
)

type WhirlwindEffect struct {
	Active       bool
	Owner        uint8
	X, Y, VX, VY int
	Life, Timer  int
	Phase        WhirlwindPhase
	Frame        int
}

type AirEffects struct {
	MarkerSlots [2]int
	Markers     [EffectCapacity]LightningMarker
	Bolts       [EffectCapacity]LightningBolt
	Whirlwinds  [EffectCapacity]WhirlwindEffect
	Carry       [FollowerCapacity]AirCarryState
	Storms      [EffectCapacity]StormEffect
}
type StormEffect struct {
	Active, Ending            bool
	Owner                     uint8
	X, Y                      int
	Life, Timer, Frame        int
	ImpactActive, WaterImpact bool
	ImpactFrame               int
}

func (s *AirEffects) CreateStorm(owner uint8, x, y int, h AirHabitat) bool {
	count := int(h.Random()%48) + 12
	admitted := false
	for attempt := 0; attempt <= count; attempt++ {
		id := h.Reserve(EffectStorm, owner)
		if id < 0 {
			return false
		}
		bits := h.Random()
		nx, ny, valid := offsetFireParcel(x, y, int(bits&7), int(bits>>8&7))
		if !valid {
			h.Release(id)
			admitted = true
			continue
		}
		life := 200
		if owner < 2 {
			life += int(h.AirExperience(owner))
		}
		s.Storms[id] = StormEffect{Active: true, Owner: owner, X: nx*256 + 128, Y: ny*256 + 128, Life: life, Frame: int(bits&12) / 4}
		admitted = true
	}
	return admitted
}

func (s *AirEffects) TickStorm(id int, h AirHabitat) {
	e := &s.Storms[id]
	if !e.Active {
		return
	}
	remove := func() { e.Active = false; h.Release(id) }
	if e.Ending {
		remove()
		return
	}
	previousLife := e.Life
	e.Life = int(int16(uint16(e.Life) - 1))
	if previousLife <= 1 {
		e.Ending = true
		if e.Frame+1 >= 4 {
			remove()
		} else {
			e.Frame++
		}
		return
	}
	e.Frame = (e.Frame + 1) % 4
	if e.ImpactActive {
		if e.ImpactFrame+1 >= 4 {
			e.ImpactActive = false
		} else {
			e.ImpactFrame++
		}
	}
	previousTimer := e.Timer
	e.Timer = int(int16(uint16(e.Timer) - 1))
	x, y := e.X>>8, e.Y>>8
	if previousTimer >= 1 {
		h.DamageStorm(x, y)
		return
	}
	e.Timer = 0
	if h.Random()%97 != 0 {
		return
	}
	e.Timer = int(h.Random()%2) + 1
	e.ImpactActive = false
	h.Scorch(x, y)
	if h.DamageStorm(x, y) != 0 {
		return
	}
	e.ImpactActive, e.ImpactFrame, e.WaterImpact = true, 0, h.Parcel(x, y).Water
	h.DamageStorm(x, y)
}

type AirCarryPhase uint8

const (
	AirCarryNone AirCarryPhase = iota
	AirCarryFlying
	AirCarryLanding
)

type AirCarryState struct {
	Phase  AirCarryPhase
	Effect int // Shared slot plus one.
	Frame  int
	Frames int
}

type LightningVictimPhase uint8

const (
	LightningVictimNone LightningVictimPhase = iota
	LightningVictimWalkingHit
	LightningVictimTownHit
	LightningVictimDeath
	LightningVictimRecovery
)

type LightningVictimState struct {
	Phase     LightningVictimPhase
	Bolt      int // Shared slot plus one.
	Frame     int
	Frames    int
	Hero      HeroKind
	BoundOnly bool
	Sequence  LightningVictimSequence
}

type LightningVictimSequence uint8

const (
	LightningHitSequence LightningVictimSequence = iota
	LightningDeathSequence
	LightningRecoverySequence
)

func (v *LightningVictimState) Bind(bolt int, town, protected bool, hero HeroKind) {
	v.Bolt = bolt + 1
	if v.Phase == LightningVictimWalkingHit || v.Phase == LightningVictimTownHit || protected {
		return
	}
	if !town && hero == HeroOdysseus {
		v.BoundOnly, v.Hero = true, hero
		return
	}
	v.Hero, v.Frame, v.Frames, v.Sequence, v.BoundOnly = hero, 0, 2, LightningHitSequence, false
	if town {
		v.Phase = LightningVictimTownHit
	} else {
		v.Phase = LightningVictimWalkingHit
	}
}

type LightningVictimTransition uint8

const (
	LightningVictimWaiting LightningVictimTransition = iota
	LightningVictimRemove
	LightningVictimReformTown
	LightningVictimRetainDeath
	LightningVictimResume
)

// TickLightningVictim applies the original signed 32-bit population damage.
// Ending a bolt releases towns immediately or starts a walker's distinct
// death/recovery animation; callbacks perform the actual world cleanup.
func TickLightningVictim(v *LightningVictimState, population *int, boltAlive bool) LightningVictimTransition {
	if v.Phase == LightningVictimNone {
		return LightningVictimWaiting
	}
	if v.Phase == LightningVictimDeath || v.Phase == LightningVictimRecovery {
		if v.Frame+1 < v.Frames {
			v.Frame++
			return LightningVictimWaiting
		}
		if v.Phase == LightningVictimDeath {
			return LightningVictimRemove
		}
		v.Phase = LightningVictimNone
		return LightningVictimResume
	}
	v.Frame = (v.Frame + 1) % 2
	if *population > 0 {
		shifted := int32(uint32(*population) << 3)
		loss := (shifted >> 7) + 4
		*population = int(int32(uint32(*population) - uint32(loss)))
	}
	if boltAlive {
		return LightningVictimWaiting
	}
	if v.Phase == LightningVictimTownHit {
		if *population <= 0 {
			return LightningVictimRemove
		}
		return LightningVictimReformTown
	}
	if *population <= 0 {
		v.Phase = LightningVictimDeath
		if v.Hero != HeroAchilles {
			v.Frame, v.Frames, v.Sequence = 0, 2, LightningDeathSequence
			if v.Hero != HeroNone {
				v.Frames = 9
			}
			*population = 0
			return LightningVictimRetainDeath
		}
	} else {
		v.Phase = LightningVictimRecovery
		if v.Hero != HeroAchilles {
			v.Frame, v.Frames, v.Sequence = 0, 10, LightningRecoverySequence
			if v.Hero == HeroPerseus || v.Hero == HeroAdonis {
				v.Frames = 9
			}
		}
	}
	return LightningVictimWaiting
}

var lightningJitter = [9][2]int{{-1, -1}, {0, -1}, {1, -1}, {-1, 0}, {0, 0}, {1, 0}, {1, -1}, {1, 0}, {1, 1}}

// PlaceLightning moves the existing marker without resetting its lifetime or
// volley. A new marker reserves one shared slot and consumes no randomness.
func (s *AirEffects) PlaceLightning(owner uint8, x, y int, h AirHabitat) bool {
	if owner > 1 || !inside(x, y) {
		return false
	}
	if reference := s.MarkerSlots[owner]; reference != 0 {
		marker := &s.Markers[reference-1]
		marker.X, marker.Y = x*256+128, y*256+128
		return true
	}
	id := h.Reserve(EffectLightning, owner)
	if id < 0 {
		return false
	}
	s.Markers[id] = LightningMarker{Active: true, Owner: owner, X: x*256 + 128, Y: y*256 + 128, Life: 200}
	s.MarkerSlots[owner] = id + 1
	return true
}

// ActivateLightning retains partial volleys. The original activation command
// charges unconditionally even if a marker is missing, already firing, or the
// pool is full; that debit is the caller's separate admission policy.
func (s *AirEffects) ActivateLightning(owner uint8, h AirHabitat) int {
	if owner > 1 || s.MarkerSlots[owner] == 0 {
		return 0
	}
	markerID := s.MarkerSlots[owner] - 1
	marker := &s.Markers[markerID]
	if marker.FirstBolt != 0 {
		return 0
	}
	created := 0
	for attempt := 0; attempt < 2+int(h.AirExperience(owner)>>5); attempt++ {
		id := h.Reserve(EffectLightning, owner)
		if id < 0 {
			return created
		}
		d := lightningJitter[h.Random()%9]
		x, y := marker.X>>8, marker.Y>>8
		nx, ny := x+d[0], y+d[1]
		if nx < 0 || nx >= MapSize {
			nx = x
		}
		if ny < 0 || ny >= MapSize {
			ny = y
		}
		s.Bolts[id] = LightningBolt{Active: true, Owner: owner, X: nx*256 + 128, Y: ny*256 + 128, Marker: markerID + 1, Next: marker.FirstBolt}
		marker.FirstBolt = id + 1
		created++
	}
	return created
}

func (s *AirEffects) DismissLightning(owner uint8, h AirHabitat) {
	if owner > 1 || s.MarkerSlots[owner] == 0 {
		return
	}
	id := s.MarkerSlots[owner] - 1
	s.MarkerSlots[owner] = 0
	marker := &s.Markers[id]
	marker.Phase, marker.Frame = LightningDisappearing, 0
	next := marker.FirstBolt
	for visits := 0; next != 0 && visits < EffectCapacity; visits++ {
		boltID := next - 1
		bolt := &s.Bolts[boltID]
		next = bolt.Next
		bolt.Active = false
		h.Release(boltID)
	}
}

func (s *AirEffects) TickLightning(id int, h AirHabitat) {
	if bolt := &s.Bolts[id]; bolt.Active {
		bolt.Random = h.Random()
		bolt.X = bolt.X&^255 | int(uint8(bolt.Random>>2))
		x, y := bolt.X>>8, bolt.Y>>8
		if h.StrikeLightning(id, x, y) {
			h.Scorch(x, y)
		}
		return
	}
	marker := &s.Markers[id]
	if !marker.Active {
		return
	}
	if marker.Phase == LightningDisappearing {
		if marker.Frame+1 < 5 {
			marker.Frame++
		} else {
			marker.Active = false
			h.Release(id)
		}
		return
	}
	marker.Life = int(int16(uint16(marker.Life) - 1))
	if marker.Life <= 0 {
		s.DismissLightning(marker.Owner, h)
		return
	}
	if marker.Phase == LightningAppearing {
		if marker.Frame+1 < 5 {
			marker.Frame++
		} else {
			marker.Phase, marker.Frame = LightningSteady, 0
		}
	} else {
		marker.Frame = (marker.Frame + 1) % 9
	}
}

func (s *AirEffects) CreateWhirlwind(owner uint8, x, y int, h AirHabitat) bool {
	if owner > 2 || !inside(x, y) {
		return false
	}
	id := h.Reserve(EffectWhirlwind, owner)
	if id < 0 {
		return false
	}
	previous := s.Whirlwinds[id]
	life := 200
	if owner < 2 {
		life += int(h.AirExperience(owner))
	}
	s.Whirlwinds[id] = WhirlwindEffect{Active: true, Owner: owner, X: x*256 + 128, Y: y*256 + 128, VX: previous.VX, VY: previous.VY, Life: life, Timer: 1}
	return true
}

func (s *AirEffects) TickWhirlwind(id int, h AirHabitat) {
	e := &s.Whirlwinds[id]
	if !e.Active {
		return
	}
	finish := func() {
		e.Active = false
		h.Release(id)
		h.ReleaseFollowers(id, e.X>>8, e.Y>>8)
	}
	if e.Phase == WhirlwindAppearing {
		if e.Frame+1 < 2 {
			e.Frame++
			return
		}
		e.Phase, e.Frame = WhirlwindMoving, 0
	}
	if e.Phase == WhirlwindDisappearing {
		if e.Frame+1 < 4 {
			e.Frame++
		} else {
			finish()
		}
		return
	}
	previousLife := e.Life
	e.Life = int(int16(uint16(e.Life) - 1))
	if previousLife <= 1 {
		e.Phase, e.Frame = WhirlwindDisappearing, 1
		return
	}
	e.Frame = (e.Frame + 1) % 2
	previousTimer := e.Timer
	e.Timer = int(int16(uint16(e.Timer) - 1))
	if previousTimer <= 1 {
		s.routeWhirlwind(e, h)
	}
	x, y := int(int16(uint16(e.X+e.VX))), int(int16(uint16(e.Y+e.VY)))
	if x < 0 || y < 0 || x >= MapSize*256 || y >= MapSize*256 {
		finish()
		return
	}
	e.X, e.Y = x, y
	if h.Random()%5 == 0 {
		h.CreateWhirlpool(e.Owner, x>>8, y>>8)
	}
	h.LiftFollowers(id, x>>8, y>>8)
}

func (s *AirEffects) routeWhirlwind(e *WhirlwindEffect, h AirHabitat) {
	x, y := e.X>>8, e.Y>>8
	height := int(h.Parcel(x, y).Altitude)
	bits := h.Random()
	start, selected := int(bits&14)/2, 0
	for i := 0; i < 8; i++ {
		d := fireNeighbors[start+i]
		nx, ny, valid := offsetFireParcel(x, y, d[0], d[1])
		if !valid {
			continue
		}
		parcel := h.Parcel(nx, ny)
		candidate := int(parcel.Altitude)
		if parcel.ExtraRise {
			candidate++
		}
		if candidate > height {
			continue
		}
		if candidate == height {
			accept := bits&1 != 0
			bits >>= 1
			if !accept {
				continue
			}
		}
		height, selected = candidate, start+i
	}
	if selected == 0 {
		selected = int(bits&0x3c) / 4
	}
	d := fireNeighbors[selected]
	e.VX, e.VY = d[0]*24, d[1]*24
	e.Timer = int(h.Random() & 0x78)
}
