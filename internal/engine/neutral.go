package engine

import "fmt"

type NeutralKind uint8

const (
	NeutralNone NeutralKind = iota
	NeutralRoadMaker
	NeutralLandLowerer
	NeutralWhirlwindMaker
	NeutralTreePlanter
	NeutralFireMaker
	NeutralMonster
)

type NeutralState struct {
	Kind       NeutralKind
	Paired     bool
	VictimTime int
}

// CreateNeutral uses the same 399 follower slots as ordinary people. These
// inventions are neutral, have zero population, and own their motion/effects.
func (w *World) CreateNeutral(kind NeutralKind, x, y int) (int, error) {
	if kind <= NeutralNone || kind > NeutralMonster || !inside(x, y) {
		return 0, fmt.Errorf("invalid neutral invention")
	}
	f := Follower{Owner: 2, X: uint8(x), Y: uint8(y), State: Walking, Neutral: NeutralState{Kind: kind}, positionSet: true, positionX: x * 256, positionY: y * 256}
	switch kind {
	case NeutralRoadMaker:
		f.positionY += 128
		f.MovementSpeed = 16
		f.velocityX = 16
	case NeutralLandLowerer:
		f.positionX += 128
		f.positionY += 127
		f.MovementSpeed = 0
		f.velocityY = -32
	case NeutralWhirlwindMaker:
		f.positionX += 128
		f.MovementSpeed = 48
		f.velocityY = 48
	case NeutralTreePlanter:
		f.positionY += 128
		f.MovementSpeed = 32
		f.velocityX = -32
	case NeutralFireMaker:
		f.positionY += 128
		f.MovementSpeed = 48
		f.velocityX = 48
	case NeutralMonster:
		f.positionX += 128
		f.positionY += 128
		f.MovementSpeed = 32
		f.velocityX = 32
		f.velocityY = 32
	}
	id := w.allocate(f)
	if id == 0 {
		return 0, fmt.Errorf("follower pool exhausted")
	}
	w.linkFollower(id)
	if kind == NeutralLandLowerer && y < 63 {
		f.Y++
		f.positionY += 256
		f.Neutral.Paired = true
		if second := w.allocate(f); second != 0 {
			w.linkFollower(second)
		}
	}
	return id, nil
}

func (w *World) tickNeutral(id int) {
	f := &w.Followers[id]
	if f.Neutral.Kind == NeutralNone {
		return
	}
	frames := 6
	if f.Neutral.Kind == NeutralLandLowerer {
		frames = 4
	}
	f.Frame = (f.Frame + 1) % uint16(frames)
	f.initialisePosition()
	x, y := f.positionX+f.velocityX, f.positionY+f.velocityY
	if x < 0 || y < 0 || x >= MapSize*256 || y >= MapSize*256 {
		w.remove(id)
		return
	}
	oldX, oldY := int(f.X), int(f.Y)
	nx, ny := x/256, y/256
	if nx != oldX || ny != oldY {
		w.moveFollowerCell(id, nx, ny)
	}
	f.positionX, f.positionY = x, y
	w.Actors.Move(ActorRef{Kind: ActorFollower, Index: uint16(id)}, x, y)
	switch f.Neutral.Kind {
	case NeutralRoadMaker:
		if w.Cell(nx, ny).Shape == 15 {
			w.Earth.Roads[nx+ny*MapSize] = RoadParcel{Active: true, Code: 174, Owner: 2}
		}
	case NeutralLandLowerer:
		w.directFireTerrain(nx, ny, false)
		if nx+1 < MapSize {
			w.directFireTerrain(nx+1, ny, false)
		}
	case NeutralWhirlwindMaker:
		if w.random.next()&31 == 0 {
			w.Air.CreateWhirlwind(2, nx, ny, worldAirHabitat{w})
		}
	case NeutralTreePlanter:
		if w.Cell(nx, ny).Shape != 0 && w.Nature.sceneryAt(nx, ny) < 0 && w.Actors.Heads[nx+ny*MapSize].Kind != ActorNone {
			for slot := range w.Nature.Scenery {
				if w.Nature.Scenery[slot].Kind == SceneryNone {
					w.Nature.Scenery[slot] = SceneryActor{Kind: SceneryTree, X: uint8(nx), Y: uint8(ny), Age: 24}
					break
				}
			}
		}
	case NeutralFireMaker:
		if w.Cell(nx, ny).Code != 0 && w.random.next()&31 == 0 {
			w.Fire.createColumn(2, nx, ny, false, worldFireHabitat{w})
		}
	case NeutralMonster:
		for _, d := range [3][2]int{{1, 1}, {0, 1}, {1, 0}} {
			tx, ty := nx+d[0], ny+d[1]
			if !inside(tx, ty) {
				continue
			}
			var victims [FollowerCapacity]int
			for _, other := range victims[:w.FollowersAt(tx, ty, victims[:])] {
				v := &w.Followers[other]
				if v.Owner > 1 || v.State != Walking && v.State != Town {
					continue
				}
				if v.State == Town {
					w.clearTownFarms(other)
				}
				v.State = Ruin
				v.Population = 0
				v.Neutral.VictimTime = 400
				v.Frame = 0
				w.clearHeroLinks(other)
			}
		}
	}
}

func (w *World) advanceNeutralVictim(id int) bool {
	f := &w.Followers[id]
	if f.Neutral.VictimTime <= 0 {
		return false
	}
	f.Neutral.VictimTime--
	if f.Neutral.VictimTime == 0 {
		w.remove(id)
	}
	return true
}
