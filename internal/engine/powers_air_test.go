package engine

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

type airTestHabitat struct {
	*fireTestHabitat
	walls                    bool
	strikes, lifts, released int
}

func newAirTestHabitat() *airTestHabitat {
	return &airTestHabitat{fireTestHabitat: newFireTestHabitat()}
}
func (h *airTestHabitat) DamageStorm(x, y int) int           { return h.Damage(x, y) }
func (h *airTestHabitat) AirExperience(uint8) uint8          { return h.experience }
func (h *airTestHabitat) StrikeLightning(int, int, int) bool { h.strikes++; return !h.walls }
func (h *airTestHabitat) CreateWhirlpool(uint8, int, int)    {}
func (h *airTestHabitat) LiftFollowers(int, int, int)        { h.lifts++ }
func (h *airTestHabitat) ReleaseFollowers(int, int, int)     { h.released++ }

func TestLightningMarkerAndBoltsMatchOriginalNumericTraces(t *testing.T) {
	type snapshot struct {
		MarkerReference uint16
		Actors          []struct {
			Slot int
			Raw  string
		}
		RNG  uint32
		Tick int
	}
	var catalog struct {
		Fixtures []struct {
			Name    string
			XP      uint8
			X, Y    int
			Initial snapshot
			Trace   []snapshot
		}
	}
	data, err := os.ReadFile("../populous2/testdata/lightning_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Fixtures) != 5 {
		t.Fatal("original lightning lifecycle traces incomplete")
	}
	for _, f := range catalog.Fixtures {
		t.Run(f.Name, func(t *testing.T) {
			h, s := newAirTestHabitat(), &AirEffects{}
			h.rng, h.useRNG, h.experience = 4311, true, f.XP
			if f.Name == "full-pool" {
				for id := 1; id < EffectCapacity; id++ {
					h.pool.Slots[id] = EffectReservation{Kind: EffectStorm, Owner: 2}
				}
			}
			if !s.PlaceLightning(0, f.X, f.Y, h) {
				t.Fatal("marker placement failed")
			}
			s.ActivateLightning(0, h)
			assert := func(want snapshot) {
				t.Helper()
				ref := 0
				if s.MarkerSlots[0] > 0 {
					ref = 20800 + (s.MarkerSlots[0]-1)*32
				}
				if ref != int(want.MarkerReference) || uint32(h.rng) != want.RNG {
					t.Fatalf("tick%d marker/RNG differs %d/%d %x/%x", want.Tick, ref, want.MarkerReference, uint32(h.rng), want.RNG)
				}
				for _, raw := range want.Actors {
					bytes, err := hex.DecodeString(raw.Raw)
					if err != nil {
						t.Fatal(err)
					}
					id := raw.Slot
					var x, y, phase, animation, owner, life, first, parent, random int
					if marker := s.Markers[id]; marker.Active || bytes[0] == 0x28 {
						x, y, life = marker.X, marker.Y, marker.Life
						phase, animation = 22, 1760+marker.Frame*4
						if marker.Phase == LightningSteady {
							animation = 1784 + marker.Frame*4
						}
						if marker.Phase == LightningDisappearing {
							phase, animation = 26, 1824+marker.Frame*4
						}
						if marker.Active {
							owner = int(marker.Owner) + 1
						}
						if marker.FirstBolt > 0 {
							first = 20800 + (marker.FirstBolt-1)*32
						}
					} else if bytes[0] == 0x2a {
						bolt := s.Bolts[id]
						x, y, phase, random = bolt.X, bolt.Y, 24, int(bolt.Random)
						if bolt.Active {
							owner = int(bolt.Owner) + 1
						}
						if bolt.Next > 0 {
							first = 20800 + (bolt.Next-1)*32
						}
						if bolt.Marker > 0 {
							parent = 20800 + (bolt.Marker-1)*32
						}
					} else {
						continue
					}
					word := func(at int) int { return int(binary.BigEndian.Uint16(bytes[at:])) }
					if uint16(x) != uint16(word(6)) || uint16(y) != uint16(word(8)) || phase != int(bytes[22]) || animation != word(10) || owner != int(bytes[12]) || uint16(life) != uint16(word(24)) || first != word(26) || parent != word(28) || random != word(30) {
						t.Fatalf("tick%d slot%d semanticstate differs: xy%d,%d phase%d frame%d owner%d life%d link%d/%d random%d; original%s", want.Tick, id, x, y, phase, animation, owner, life, first, parent, random, raw.Raw)
					}
				}
			}
			assert(f.Initial)
			for _, step := range f.Trace {
				if f.Name == "center-xp32" && step.Tick == 30 {
					s.PlaceLightning(0, 35, 34, h)
				}
				for id := 0; id < EffectCapacity; id++ {
					if h.pool.Slots[id].Kind == EffectLightning {
						s.TickLightning(id, h)
					}
				}
				assert(step)
			}
		})
	}
}

func TestLightningMarkerPlacementAndDismissalKeepLifetimeAndIndependentSlots(t *testing.T) {
	h, s := newAirTestHabitat(), &AirEffects{}
	if !s.PlaceLightning(0, 32, 32, h) || h.randomAt != 0 {
		t.Fatal("marker placement consumed randomness")
	}
	s.Markers[0].Life = 77
	if !s.PlaceLightning(0, 35, 34, h) || s.Markers[0].Life != 77 {
		t.Fatal("marker relocation restarted its lifetime")
	}
	if got := s.ActivateLightning(0, h); got != 2 || h.randomAt != 2 {
		t.Fatal("base volley creation differs")
	}
	if got := s.ActivateLightning(0, h); got != 0 || h.randomAt != 2 {
		t.Fatal("existing volley was duplicated")
	}
	s.DismissLightning(0, h)
	if s.MarkerSlots[0] != 0 || s.Bolts[1].Active || s.Bolts[2].Active || !s.Markers[0].Active {
		t.Fatal("dismissal failed to remove bolts while retaining marker outro")
	}
	for range 5 {
		s.TickLightning(0, h)
	}
	if s.Markers[0].Active || h.pool.Slots[0].Kind != EffectNone {
		t.Fatal("marker outro failed to release shared slot")
	}
}

func TestWhirlwindMovementUsesDownhillRoutingAndTwoDrawTimer(t *testing.T) {
	h, s := newAirTestHabitat(), &AirEffects{}
	h.experience = 255
	if !s.CreateWhirlwind(0, 32, 32, h) || h.randomAt != 0 || s.Whirlwinds[0].Life != 455 {
		t.Fatal("whirlwind creation changed experience or RNG")
	}
	h.random = []uint16{0, 120, 1}
	h.parcels[33+33*MapSize].Altitude = 0
	s.TickWhirlwind(0, h)
	if s.Whirlwinds[0].Frame != 1 || s.Whirlwinds[0].Life != 455 {
		t.Fatal("whirlwind emergence consumed lifetime")
	}
	s.TickWhirlwind(0, h)
	e := s.Whirlwinds[0]
	if e.Phase != WhirlwindMoving || e.Life != 454 || e.Timer != 120 || h.randomAt != 3 || e.VX != 24 || e.VY != 24 || h.lifts != 1 {
		t.Fatalf("whirlwind routing/move/interaction differs: %+v draws%d", e, h.randomAt)
	}
}

func TestLightningVictimPopulationAndPhasesMatchOriginalNumericTraces(t *testing.T) {
	type record struct {
		Kind, Owner, Flags, State uint8
		Animation                 uint16
		Population                int32
		Reference                 uint16
		Calls                     []string
	}
	var catalog struct {
		VictimFixtures []struct {
			Input struct {
				Name, Mode                string
				Kind, State, Flags, Owner uint8
				Hero                      int
				Population                int32
				BoltOwner, BoltKind       uint8
				Animation                 uint16
				Ticks                     int
			}
			Initial record
			Trace   []record
		}
	}
	data, err := os.ReadFile("../populous2/testdata/lightning_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, f := range catalog.VictimFixtures {
		if f.Input.Mode == "bind" || f.Initial.State != 0x1c && f.Initial.State != 0x1e {
			continue
		}
		t.Run(f.Input.Name, func(t *testing.T) {
			hero := HeroNone
			if f.Input.Flags&2 != 0 && f.Input.Hero >= 0 {
				hero = HeroKind(f.Input.Hero + 1)
			}
			v := LightningVictimState{Phase: LightningVictimWalkingHit, Bolt: 1, Frames: 2, Hero: hero}
			if f.Initial.State == 0x1e {
				v.Phase = LightningVictimTownHit
			}
			base := lightningTestAnimationBase(v, f.Initial.State == 0x1e)
			v.Frame = (int(f.Initial.Animation) - base) / 4
			if v.Frame < 0 || v.Frame >= 2 {
				return
			}
			population := int(f.Initial.Population)
			for tick, want := range f.Trace {
				transition := TickLightningVictim(&v, &population, int8(f.Input.BoltOwner) > 0)
				state := uint8(0x1c)
				if f.Initial.State == 0x1e {
					state = 0x1e
				}
				if v.Phase == LightningVictimDeath {
					state = 0x20
				}
				if v.Phase == LightningVictimRecovery {
					state = 0x22
				}
				if transition == LightningVictimResume {
					state = 2
				}
				animation := lightningTestAnimationBase(v, f.Initial.State == 0x1e) + v.Frame*4
				if transition == LightningVictimRemove {
					population = 0
				}
				if transition == LightningVictimReformTown {
					if hero != HeroNone {
						state = 2
						animation = 0
					} else {
						state = 6
					}
				}
				if int32(population) != want.Population || state != want.State || uint16(animation) != want.Animation {
					t.Fatalf("tick%d population%d phase%x animation%x / original%d %x %x", tick+1, population, state, animation, want.Population, want.State, want.Animation)
				}
			}
			checked++
		})
	}
	if checked < 100 {
		t.Fatalf("too few original victim timelines checked: %d", checked)
	}
}

func lightningTestAnimationBase(v LightningVictimState, town bool) int {
	if v.Sequence == LightningDeathSequence {
		if v.Hero != HeroNone {
			return 11220
		}
		return 1848
	}
	if v.Sequence == LightningRecoverySequence {
		if v.Hero == HeroNone {
			return 1872
		}
		return [6]int{11052, 11180, 11092, 11136, 0, 11008}[v.Hero-1]
	}
	if town {
		return 1860
	}
	if v.Hero == HeroNone {
		return 1848
	}
	return [6]int{10996, 10972, 3712, 0, 10984, 10960}[v.Hero-1]
}

func TestWhirlwindMovementMatchesOriginalNumericTraces(t *testing.T) {
	type actor struct {
		Kind, Owner, Phase, Speed             uint8
		FixedX, FixedY, Animation             uint16
		VelocityX, VelocityY, MoveTimer, Life int16
	}
	var catalog struct {
		Fixtures []struct {
			Name       string
			Seed       uint32
			Experience uint8
			Target     [2]int
			Initial    actor
			InitialRNG uint32
			Trace      []struct {
				Tick  int
				Actor actor
				RNG   uint32
			}
		}
	}
	data, err := os.ReadFile("../populous2/testdata/whirlwind_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Fixtures) != 12 {
		t.Fatal("original whirlwind motion traces incomplete")
	}
	for _, f := range catalog.Fixtures {
		t.Run(f.Name, func(t *testing.T) {
			h, s := newAirTestHabitat(), &AirEffects{}
			h.experience, h.rng, h.useRNG = f.Experience, randomState(f.Seed), true
			for i := range h.parcels {
				h.parcels[i].Altitude = 0
				h.parcels[i].ExtraRise = true
			}
			if f.Name == "water" {
				for i := range h.parcels {
					h.parcels[i] = FireParcel{Water: true}
				}
			}
			if f.Name == "uphill" {
				for y := 0; y < MapSize; y++ {
					for x := 0; x < MapSize; x++ {
						height := 1 + max(0, min(7, x-28))
						rise := true
						if x < 28 || x >= 35 {
							height--
						}
						h.parcels[x+y*MapSize].Altitude = uint8(height)
						h.parcels[x+y*MapSize].ExtraRise = rise
					}
				}
			}
			if !s.CreateWhirlwind(0, f.Target[0], f.Target[1], h) {
				t.Fatal("whirlwind cast failed")
			}
			assert := func(tick int, want actor, rng uint32) {
				t.Helper()
				e := s.Whirlwinds[0]
				phase, animation := uint8(8), 1224+e.Frame*4
				if e.Phase == WhirlwindMoving {
					phase = 10
				}
				if e.Phase == WhirlwindDisappearing {
					phase, animation = 12, 1740+e.Frame*4
				}
				owner := uint8(0)
				if e.Active {
					owner = 1
				}
				got := actor{Kind: 32, Owner: owner, Phase: phase, Speed: 24, FixedX: uint16(e.X), FixedY: uint16(e.Y), Animation: uint16(animation), VelocityX: int16(e.VX), VelocityY: int16(e.VY), MoveTimer: int16(e.Timer), Life: int16(e.Life)}
				if got != want || uint32(h.rng) != rng {
					t.Fatalf("tick%d %+v/original%+v RNG%x/%x", tick, got, want, uint32(h.rng), rng)
				}
			}
			assert(0, f.Initial, f.InitialRNG)
			for _, step := range f.Trace {
				s.TickWhirlwind(0, h)
				assert(step.Tick, step.Actor, step.RNG)
			}
		})
	}
}

func TestStormCreationMatchesOriginalNumericCases(t *testing.T) {
	var catalog struct {
		Cases []struct {
			Input struct {
				Name, Mode                   string
				Owner                        uint16
				X, Y                         uint8
				Seed                         uint32
				FreeStart, FreeCount, Caller int
				Initial                      []struct {
					Address, Width int
					Value          uint32
				}
			}
			Admitted              bool
			RandomDraws, Attempts int
			Changes               []struct {
				Address int
				Value   uint8
			}
			RNG uint32
		}
	}
	data, err := os.ReadFile("../populous2/testdata/storm_native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, f := range catalog.Cases {
		input := f.Input
		if input.Mode != "create" || input.Owner < 1 || input.Owner > 3 {
			continue
		}
		t.Run(input.Name, func(t *testing.T) {
			h, s := newAirTestHabitat(), &AirEffects{}
			h.rng, h.useRNG = randomState(input.Seed), true
			for id := 0; id < EffectCapacity; id++ {
				if id < input.FreeStart || id >= input.FreeStart+input.FreeCount {
					h.pool.Slots[id] = EffectReservation{Kind: EffectFireColumn, Owner: 0}
				}
			}
			for _, p := range input.Initial {
				if p.Address == 0xe76a+int(input.Owner)*314+0x55 {
					h.experience = uint8(p.Value)
				}
			}
			admitted := s.CreateStorm(uint8(input.Owner-1), int(input.X), int(input.Y), h)
			if admitted != f.Admitted || h.randomAt != f.RandomDraws || uint32(h.rng) != f.RNG {
				t.Fatalf("creation admission/RNG differs %v/%v draws%d/%d state%x/%x", admitted, f.Admitted, h.randomAt, f.RandomDraws, uint32(h.rng), f.RNG)
			}
			memory := make([]byte, EffectCapacity*32)
			for id := 0; id < EffectCapacity; id++ {
				for off := 0; off < 32; off++ {
					memory[id*32+off] = uint8(id*32 + off + 7)
				}
				memory[id*32+12] = 1
				if id >= input.FreeStart && id < input.FreeStart+input.FreeCount {
					memory[id*32+12] = 0
				}
			}
			for _, p := range input.Initial {
				at := p.Address - 0xc800
				if at >= 0 && at+p.Width <= len(memory) {
					switch p.Width {
					case 1:
						memory[at] = uint8(p.Value)
					case 2:
						binary.BigEndian.PutUint16(memory[at:], uint16(p.Value))
					case 4:
						binary.BigEndian.PutUint32(memory[at:], p.Value)
					}
				}
			}
			for _, change := range f.Changes {
				at := change.Address - 0xc800
				if at >= 0 && at < len(memory) {
					memory[at] = change.Value
				}
			}
			for id, e := range s.Storms {
				if !e.Active {
					continue
				}
				raw := memory[id*32:]
				if uint16(e.X) != binary.BigEndian.Uint16(raw[6:]) || uint16(e.Y) != binary.BigEndian.Uint16(raw[8:]) || uint16(e.Life) != binary.BigEndian.Uint16(raw[24:]) || uint16(3300+e.Frame*4) != binary.BigEndian.Uint16(raw[10:]) || raw[12] != uint8(input.Owner) {
					t.Fatalf("cloud%d namedcreation differs %+v", id, e)
				}
			}
			checked++
		})
	}
	if checked != 164 {
		t.Fatalf("insufficient original storm creation cases: %d", checked)
	}
}

func TestStormStrikeCooldownAndEmptyImpactRemainDistinct(t *testing.T) {
	h, s := newAirTestHabitat(), &AirEffects{}
	s.Storms[0] = StormEffect{Active: true, Owner: 0, X: 32*256 + 128, Y: 32*256 + 128, Life: 200}
	h.random = []uint16{97, 1, 1}
	s.TickStorm(0, h)
	if s.Storms[0].Timer != 2 || !s.Storms[0].ImpactActive || len(h.damaged) != 2 || len(h.scorched) != 1 || h.randomAt != 2 {
		t.Fatal("successful empty strike lost original cooldown or double scan")
	}
	s.TickStorm(0, h)
	if s.Storms[0].Timer != 1 || len(h.damaged) != 3 || h.randomAt != 2 {
		t.Fatal("positive cooldown failed to scan without a random strike draw")
	}
	s.TickStorm(0, h)
	if s.Storms[0].Timer != 0 || len(h.damaged) != 4 || h.randomAt != 2 {
		t.Fatal("last cooldown pass failed to scan")
	}
	s.TickStorm(0, h)
	if h.randomAt != 3 || len(h.damaged) != 4 {
		t.Fatal("failed thunder draw incorrectly damaged victims")
	}
}
