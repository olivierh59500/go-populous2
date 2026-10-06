package engine

import "fmt"

type FollowerSelectionTransfer struct {
	FromFollower, ToFollower uint16
}

// FollowerSelectionTransfers reports changes of actor identity without owning
// a player's local selection. The ordinary ascending 399-actor pass fits two
// transfers per visit. If redispatch produces more, an exact composed mapping
// replaces the ordered log; no selection change is dropped or allocated.
type FollowerSelectionTransfers struct {
	Transfers [FollowerCapacity * 2]FollowerSelectionTransfer
	Count     uint16
	Composed  bool
}

// Resolve follows a selection through this simulation pass. It is read-only:
// callers consume each batch once, immediately after a new world tick.
func (b *FollowerSelectionTransfers) Resolve(id int) int {
	if id <= 0 || id >= FollowerCapacity {
		return 0
	}
	if b.Composed {
		return int(b.Transfers[id-1].ToFollower)
	}
	for _, transfer := range b.Transfers[:int(b.Count)] {
		if id == int(transfer.FromFollower) {
			id = int(transfer.ToFollower)
		}
	}
	return id
}

func (b *FollowerSelectionTransfers) reset() { *b = FollowerSelectionTransfers{} }

func (b *FollowerSelectionTransfers) add(from, to int) {
	if from <= 0 || from >= FollowerCapacity || to <= 0 || to >= FollowerCapacity || from == to {
		panic("invalid follower selection transfer")
	}
	if int(b.Count) == len(b.Transfers) {
		var destinations [FollowerCapacity]uint16
		for id := 1; id < FollowerCapacity; id++ {
			destinations[id] = uint16(id)
		}
		for _, transfer := range b.Transfers {
			for id := 1; id < FollowerCapacity; id++ {
				if destinations[id] == transfer.FromFollower {
					destinations[id] = transfer.ToFollower
				}
			}
		}
		b.reset()
		b.Composed = true
		b.Count = FollowerCapacity - 1
		for id := 1; id < FollowerCapacity; id++ {
			b.Transfers[id-1] = FollowerSelectionTransfer{uint16(id), destinations[id]}
		}
	}
	if b.Composed {
		for i := 0; i < int(b.Count); i++ {
			if b.Transfers[i].ToFollower == uint16(from) {
				b.Transfers[i].ToFollower = uint16(to)
			}
		}
		return
	}
	b.Transfers[b.Count] = FollowerSelectionTransfer{uint16(from), uint16(to)}
	b.Count++
}

func (b *FollowerSelectionTransfers) validate() error {
	if int(b.Count) > len(b.Transfers) || b.Composed && b.Count != FollowerCapacity-1 {
		return fmt.Errorf("snapshot selection transfer count is invalid")
	}
	for i, transfer := range b.Transfers {
		if i >= int(b.Count) {
			if transfer != (FollowerSelectionTransfer{}) {
				return fmt.Errorf("snapshot has inactive selection transfers")
			}
			continue
		}
		if transfer.FromFollower == 0 || transfer.FromFollower >= FollowerCapacity || transfer.ToFollower == 0 || transfer.ToFollower >= FollowerCapacity || b.Composed && int(transfer.FromFollower) != i+1 {
			return fmt.Errorf("snapshot selection transfer actor is invalid")
		}
	}
	return nil
}
