package engine

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestPlagueAffectsOnlyOpposingGroupsAndResetsSeparateOverlay(t *testing.T) {
	w := testFlatWorld()
	w.random = 4311
	blue := addFollower(w, 32, 32, 0, 100, Walking)
	red := addFollower(w, 32, 32, 1, 200, Town)
	w.Followers[red].Hero.Kind = HeroHeracles
	w.Followers[red].Frame = 3
	if err := w.CastPlague(0, 32, 32); err != nil {
		t.Fatal(err)
	}
	if w.Followers[blue].Disease.Infected || !w.Followers[red].Disease.Infected || w.random != 4311 {
		t.Fatal("plague ownership or random admission differs")
	}
	for range 27 {
		if w.tickDisease(red) {
			t.Fatal("positive population was killed by zero original plague damage")
		}
	}
	if f := w.Followers[red]; f.Disease.Frame != 5 || f.Population != 200 || f.Frame != 3 {
		t.Fatal("plague clock altered ordinary animation/population")
	}
	if err := w.CastPlague(0, 32, 32); err != nil || w.Followers[red].Disease.Frame != 0 {
		t.Fatal("plague recast did not restart overlay")
	}
}

func TestDiseaseInheritsOnlyFromInfectedMergeAndTownBirth(t *testing.T) {
	w := testFlatWorld()
	a := addFollower(w, 32, 32, 0, 100, Walking)
	b := addFollower(w, 32, 32, 0, 100, Walking)
	w.Followers[a].Disease = DiseaseState{Infected: true, Frame: 17}
	w.InheritDiseaseMerge(a, b)
	if w.Followers[b].Disease != w.Followers[a].Disease {
		t.Fatal("merge did not copy infected source phase")
	}
	w.Followers[a].Disease = DiseaseState{}
	w.InheritDiseaseMerge(a, b)
	if !w.Followers[b].Disease.Infected || w.Followers[b].Disease.Frame != 17 {
		t.Fatal("healthy source cleared existing infection")
	}
	w = testFlatWorld()
	parent := addFollower(w, 32, 32, 0, 4000, Town)
	w.Followers[parent].Disease = DiseaseState{Infected: true, Frame: 12}
	w.Followers[parent].Work = 7
	w.stepFollower(parent)
	child := 0
	for id, f := range w.Followers {
		if id != parent && f.State != Inactive {
			child = id
		}
	}
	if child == 0 || !w.Followers[child].Disease.Infected || w.Followers[child].Disease.Frame != w.Followers[parent].Disease.Frame {
		t.Fatal("emigrant lost parent's disease phase")
	}
}

func TestDiseaseMergeDoesNotCopyRetainedDeathState(t *testing.T) {
	w := testFlatWorld()
	a := addFollower(w, 32, 32, 0, 0, Ruin)
	b := addFollower(w, 32, 32, 0, 100, Walking)
	w.Followers[a].Disease = DiseaseState{Infected: true, Frame: 7, Dying: true, DeathFrame: 1}
	w.InheritDiseaseMerge(a, b)
	if d := w.Followers[b].Disease; !d.Infected || d.Frame != 7 || d.Dying || d.DeathFrame != 0 {
		t.Fatal("plague merge copied fields outside infection/overlay", d)
	}
}

func TestZeroPopulationPlagueRetainsItsDeathFrames(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 32, 32, 0, 0, Walking)
	w.Players[0].Leader = id
	w.Followers[id].Disease.Infected = true
	if !w.tickDisease(id) || w.Followers[id].State != Ruin || w.Players[0].Leader != 0 {
		t.Fatal("plague zero-population cleanup did not retain death")
	}
	w.tickDisease(id)
	if w.Followers[id].State == Inactive {
		t.Fatal("plague actor disappeared before final frame")
	}
	w.tickDisease(id)
	if w.Followers[id].State != Inactive {
		t.Fatal("plague death did not leave follower pool")
	}
}

func TestArmageddonConvertsOrderedHealthyGroupsAndRemovesInfected(t *testing.T) {
	w := testFlatWorld()
	w.random = 4311
	ids := []int{addFollower(w, 20, 20, 0, 100, Walking), addFollower(w, 24, 24, 1, 200, Town), addFollower(w, 28, 28, 0, 300, Fighting), addFollower(w, 32, 32, 1, 400, Walking)}
	w.Followers[ids[2]].Disease.Infected = true
	positions := w.Followers
	rng := randomState(4311)
	want := map[int]HeroKind{}
	for _, id := range ids {
		if id != ids[2] {
			want[id] = HeroKind(1 + rng.next()%4)
		}
	}
	if err := w.CastArmageddon(0); err != nil {
		t.Fatal(err)
	}
	if !w.Armageddon || w.random != rng || w.Followers[ids[2]].State != Inactive {
		t.Fatal("Armageddon admission/infection/random sequence differs")
	}
	for id, kind := range want {
		f := w.Followers[id]
		if f.Hero.Kind != kind || f.X != positions[id].X || f.Y != positions[id].Y {
			t.Fatal("Armageddon hero selection or position changed", id)
		}
	}
	before := w.Followers
	beforeRNG := w.random
	if err := w.CastArmageddon(1); err != nil || w.Followers != before || w.random != beforeRNG {
		t.Fatal("repeated Armageddon converted actors again")
	}
}

func TestArmageddonExcludesRetainedDeathsAndAirborneGroups(t *testing.T) {
	w := testFlatWorld()
	ruin := addFollower(w, 20, 20, 0, 100, Ruin)
	airborne := addFollower(w, 24, 24, 0, 100, Airborne)
	if err := w.CastArmageddon(0); err != nil {
		t.Fatal(err)
	}
	if w.Followers[ruin].IsHero() || w.Followers[airborne].IsHero() {
		t.Fatal("Armageddon converted ineligible retained states")
	}
}

func TestPrivateArmageddonGameplayStates(t *testing.T) {
	path := os.Getenv("POPULOUS2_ARMAGEDDON_TRACE")
	if path == "" {
		t.Skip("set POPULOUS2_ARMAGEDDON_TRACE for private Armageddon comparison")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Input struct {
				Name, Mode     string
				Owner, Enabled uint16
				Seed           uint32
				Actors         []struct {
					Slot                                    int
					Owner, Kind, State, Flags, Stage, Speed uint8
					Population                              uint32
				}
			}
			Calls []struct {
				Kind       string
				Ref, Value uint16
			}
			RNG         uint32
			RandomDraws int
		}
	}
	if err = json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, c := range catalog.Cases {
		i := c.Input
		if i.Owner < 1 || i.Owner > 2 {
			continue
		}
		mapped := true
		for _, a := range i.Actors {
			if a.Owner < 1 || a.Owner > 2 || a.Slot <= 0 || a.Slot >= FollowerCapacity || a.State != 2 && a.State != 4 && a.State != 6 && a.State != 8 && a.State != 22 && a.State != 24 {
				mapped = false
			}
		}
		if !mapped {
			continue
		}
		t.Run(i.Name, func(t *testing.T) {
			w := testFlatWorld()
			w.random = randomState(i.Seed)
			w.Armageddon = i.Enabled != 0
			for _, a := range i.Actors {
				state := Walking
				switch a.State {
				case 6:
					state = Town
				case 8:
					state = Ruin
				case 22:
					state = Drowning
				case 24:
					state = Ruin
				}
				w.Followers[a.Slot] = Follower{Owner: a.Owner - 1, X: 32, Y: 33, State: state, Population: int(int32(a.Population)), MovementSpeed: a.Speed, Stage: a.Stage, Disease: DiseaseState{Infected: a.Flags&16 != 0}}
			}
			if err := w.CastArmageddon(int(i.Owner - 1)); err != nil {
				t.Fatal(err)
			}
			if uint32(w.random) != c.RNG || !w.Armageddon {
				t.Fatalf("Armageddon RNG/globalstate differs %d expected%d", w.random, c.RNG)
			}
			for _, call := range c.Calls {
				id := int(call.Ref) / 52
				if id <= 0 || id >= FollowerCapacity {
					continue
				}
				switch call.Kind {
				case "convert":
					if w.Followers[id].Hero.Kind != HeroKind(1+call.Value/2) {
						t.Fatal("Armageddon hero selection differs", id)
					}
				case "cleanup":
					if w.Followers[id].State != Inactive {
						t.Fatal("infected Armageddon group survived", id)
					}
				}
			}
		})
		checked++
	}
	if checked == 0 {
		t.Fatal("private Armageddon catalog supplied no mapped gameplay cases")
	}
}

func TestBaptismSamplesGroundWithoutOverwritingOccupants(t *testing.T) {
	w := testFlatWorld()
	w.random = 4311
	w.Players[0].Experience[Water] = 255
	id := addFollower(w, 32, 32, 0, 100, Walking)
	if err := w.CastBaptism(0, 32, 32); err != nil {
		t.Fatal(err)
	}
	placed := 0
	for at, p := range w.Nature.Ground {
		if p.Mark == GroundBaptism {
			placed++
			if w.Occupants[at] != 0 || w.Cell(at%MapSize, at/MapSize).Code != 143 {
				t.Fatal("font did not preserve occupants or artwork")
			}
		}
	}
	if placed == 0 || w.Occupants[32+32*MapSize] != uint16(id) {
		t.Fatal("baptism did not create sampled parcels")
	}
}

func TestBaptismDelayedFlipPreservesPopulationAndVelocityFractions(t *testing.T) {
	for _, reference := range []struct {
		owner  int
		hero   HeroKind
		length int
	}{{0, HeroNone, 12}, {1, HeroNone, 11}, {0, HeroPerseus, 11}, {1, HeroHelen, 11}} {
		w := testFlatWorld()
		id := addFollower(w, 32, 32, reference.owner, 100, Walking)
		w.Players[reference.owner].Leader = id
		f := &w.Followers[id]
		f.Hero.Kind = reference.hero
		f.velocityX = 16
		f.velocityY = -16
		w.Nature.Ground[32+32*MapSize] = GroundParcel{Mark: GroundBaptism, Owner: 0}
		for frame := 1; frame < reference.length; frame++ {
			if !w.advanceConversion(id) || f.Owner != uint8(reference.owner) || f.State != Converting {
				t.Fatal("font flipped before completing its frames")
			}
		}
		if !w.advanceConversion(id) || f.Owner != uint8(reference.owner^1) || f.State != Walking || f.Population != 100 || f.X != 33 || f.Y != 31 || f.positionX&255 != 128 || f.positionY&255 != 128 {
			t.Fatal("font completion did not preserve original conversion fields", *f)
		}
		if w.Players[reference.owner].Leader != 0 || w.Occupants[33+31*MapSize] != uint16(id) {
			t.Fatal("font leader/graph handoff incomplete")
		}
	}
}

func TestBaptismConversionDoesNotWrapAtWorldEdges(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 0, 63, 0, 100, Walking)
	f := &w.Followers[id]
	f.velocityX = -20
	f.velocityY = 20
	w.Nature.Ground[0+63*MapSize] = GroundParcel{Mark: GroundBaptism}
	for range 12 {
		w.advanceConversion(id)
	}
	if f.X != 0 || f.Y != 63 || f.Owner != 1 {
		t.Fatal("font completion wrapped outside finite world")
	}
}

func TestPrivateBaptismGroundCreation(t *testing.T) {
	path := os.Getenv("POPULOUS2_BAPTISM_TRACE")
	if path == "" {
		t.Skip("set POPULOUS2_BAPTISM_TRACE for private baptism map comparison")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	type patch struct {
		Address, Width int
		Value          uint32
	}
	var catalog struct {
		Cases []struct {
			Input struct {
				Name       string
				Owner      uint16
				X, Y       uint8
				Seed       uint32
				Tile       uint8
				Pattern    int
				Initial    []patch
				Links      []uint16
				Operations []struct {
					Kind    string
					Repeat  int
					Initial []patch
				}
			}
			Frames []struct {
				RNG     uint32
				Changes []struct {
					Address int
					Value   uint8
				}
			}
		}
	}
	if err = json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, c := range catalog.Cases {
		i := c.Input
		if !strings.HasPrefix(i.Name, "cast-font-") || i.Owner < 1 || i.Owner > 2 || !inside(int(i.X), int(i.Y)) || i.Pattern != 0 || len(i.Links) != 0 || len(i.Operations) != 1 || i.Operations[0].Kind != "font" || i.Operations[0].Repeat != 1 || len(c.Frames) != 1 {
			continue
		}
		w := testFlatWorld()
		w.random = randomState(i.Seed)
		for at := range w.Tiles {
			w.Tiles[at].Code = i.Tile
		}
		unsupported := false
		for _, p := range i.Initial {
			if p.Address == 0xe76a+int(i.Owner)*314+0x57 && p.Width == 1 {
				w.Players[i.Owner-1].Experience[Water] = uint8(p.Value)
			} else {
				unsupported = true
			}
		}
		if unsupported {
			continue
		}
		t.Run(i.Name, func(t *testing.T) {
			if err := w.CastBaptism(int(i.Owner-1), int(i.X), int(i.Y)); err != nil {
				t.Fatal(err)
			}
			if uint32(w.random) != c.Frames[0].RNG {
				t.Fatal("baptism random sequence differs")
			}
			expected := [MapSize * MapSize]byte{}
			for at := range expected {
				expected[at] = i.Tile
			}
			for _, change := range c.Frames[0].Changes {
				if change.Address >= 0xf45 && change.Address < 0x4f44 && (change.Address-0xf45)%4 == 0 {
					expected[(change.Address-0xf45)/4] = change.Value
				}
			}
			for at, code := range expected {
				if got := w.Cell(at%MapSize, at/MapSize).Code; got != code {
					t.Fatalf("baptism parcel%d code%d expected%d", at, got, code)
				}
			}
		})
		checked++
	}
	if checked == 0 {
		t.Fatal("baptism fixture supplied no normal sampling cases")
	}
}
