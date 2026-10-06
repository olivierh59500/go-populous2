package engine

// ActorKind identifies a semantic game pool, independently of original record
// widths, addresses and memory layout. The empty reference is Kind ActorNone.
type ActorKind uint8

const (
	ActorNone ActorKind = iota
	ActorFollower
	ActorEffect
	ActorScenery
	ActorWall
	ActorMagnet
)

type ActorRef struct {
	Kind  ActorKind
	Index uint16
}
type ActorLink struct {
	Next, Previous ActorRef
	X, Y           int // Coordinates in 1/256-cell units.
	Linked         bool
}
type MagnetActor struct {
	X, Y  int
	Owner uint8
}

type ActorRegistry struct {
	Heads     [MapSize * MapSize]ActorRef
	Followers [FollowerCapacity]ActorLink
	Effects   [EffectCapacity]ActorLink
	Scenery   [SceneryCapacity]ActorLink
	Walls     [WallCapacity]ActorLink
	Magnets   [2]ActorLink
}

func (r *ActorRegistry) node(ref ActorRef) *ActorLink {
	i := int(ref.Index)
	switch ref.Kind {
	case ActorFollower:
		if i > 0 && i < len(r.Followers) {
			return &r.Followers[i]
		}
	case ActorEffect:
		if i >= 0 && i < len(r.Effects) {
			return &r.Effects[i]
		}
	case ActorScenery:
		if i >= 0 && i < len(r.Scenery) {
			return &r.Scenery[i]
		}
	case ActorWall:
		if i >= 0 && i < len(r.Walls) {
			return &r.Walls[i]
		}
	case ActorMagnet:
		if i >= 0 && i < len(r.Magnets) {
			return &r.Magnets[i]
		}
	}
	return nil
}
func actorCell(x, y int) (int, bool) {
	if x < 0 || y < 0 || x >= MapSize*256 || y >= MapSize*256 {
		return 0, false
	}
	return (x >> 8) + (y>>8)*MapSize, true
}

// Link prepends an actor to its parcel. Moving only within a parcel retains the
// existing chain order; crossing parcels unlinks and relinks the actor once.
func (r *ActorRegistry) Link(ref ActorRef, x, y int) bool {
	node := r.node(ref)
	at, inside := actorCell(x, y)
	if node == nil || !inside {
		return false
	}
	if node.Linked {
		r.Unlink(ref)
	}
	head := r.Heads[at]
	*node = ActorLink{Next: head, X: x, Y: y, Linked: true}
	if next := r.node(head); next != nil {
		next.Previous = ref
	}
	r.Heads[at] = ref
	return true
}
func (r *ActorRegistry) Unlink(ref ActorRef) bool {
	node := r.node(ref)
	if node == nil || !node.Linked {
		return false
	}
	at, inside := actorCell(node.X, node.Y)
	if !inside {
		return false
	}
	if previous := r.node(node.Previous); previous != nil {
		previous.Next = node.Next
	} else {
		r.Heads[at] = node.Next
	}
	if next := r.node(node.Next); next != nil {
		next.Previous = node.Previous
	}
	node.Next, node.Previous = ActorRef{}, ActorRef{}
	node.Linked = false
	return true
}
func (r *ActorRegistry) Move(ref ActorRef, x, y int) bool {
	node := r.node(ref)
	at, inside := actorCell(x, y)
	if node == nil || !inside {
		return false
	}
	previous, wasInside := actorCell(node.X, node.Y)
	if !node.Linked || !wasInside || previous != at {
		return r.Link(ref, x, y)
	}
	node.X, node.Y = x, y
	return true
}
func (r *ActorRegistry) Position(ref ActorRef) (int, int, bool) {
	node := r.node(ref)
	if node == nil || !node.Linked {
		return 0, 0, false
	}
	return node.X, node.Y, true
}
func (r *ActorRegistry) Next(ref ActorRef) ActorRef {
	if node := r.node(ref); node != nil {
		return node.Next
	}
	return ActorRef{}
}
