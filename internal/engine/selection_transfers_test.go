package engine

import "testing"

func TestSelectionTransfersFollowOriginalBirthAndFriendlyMerge(t *testing.T) {
	w := testFlatWorld()
	parent := addFollower(w, 32, 32, 0, 4000, Town)
	w.Followers[parent].Work = 7
	w.Step()
	child := w.SelectionTransfers.Resolve(parent)
	if child == parent || w.Followers[child].Population != 1336 || w.Followers[parent].Population != 2664 || w.SelectionTransfers.Count != 1 {
		t.Fatal("source-selected town did not transfer identity to its actual emigrant", parent, child)
	}
	if w.SelectionTransfers.Resolve(0) != 0 || w.SelectionTransfers.Resolve(399) != 399 {
		t.Fatal("birth selected an actor that was not already selected")
	}
	w.mergeFollowers(child, parent)
	if w.SelectionTransfers.Resolve(parent) != parent || w.SelectionTransfers.Resolve(child) != parent {
		t.Fatal("same-pass merge did not preserve chronological selection transfers")
	}
	if _, err := w.Snapshot().Restore(); err != nil {
		t.Fatal("selection batch could not survive a world snapshot", err)
	}
	w.Step()
	if w.SelectionTransfers.Count != 0 || w.SelectionTransfers.Resolve(parent) != parent {
		t.Fatal("prior-frame birth/merge transfers leaked into the next pass")
	}
}

func TestRejectedEmigrationDoesNotTransferSelection(t *testing.T) {
	w := testFlatWorld()
	parent := addFollower(w, 32, 32, 0, 4000, Town)
	w.Followers[parent].Work = 7
	for id := 2; id < FollowerCapacity; id++ {
		w.Followers[id] = Follower{Owner: 0, State: Walking, Population: 1}
	}
	w.stepFollower(parent)
	if w.SelectionTransfers.Count != 0 || w.SelectionTransfers.Resolve(parent) != parent {
		t.Fatal("full follower pool changed selection despite rejected birth")
	}
}

func TestSelectionTransferCompactionPreservesRepeatedSlotReuse(t *testing.T) {
	var batch FollowerSelectionTransfers
	var want [FollowerCapacity]int
	for id := range want {
		want[id] = id
	}
	for i := 0; i < 1602; i++ {
		from, to := 2, 3
		if i%3 == 1 {
			from, to = 1, 2
		}
		if i%3 == 2 {
			from, to = 3, 1
		}
		batch.add(from, to)
		for id := range want {
			if want[id] == from {
				want[id] = to
			}
		}
	}
	batch.add(1, 4)
	for id := range want {
		if want[id] == 1 {
			want[id] = 4
		}
	}
	if !batch.Composed || batch.Count != FollowerCapacity-1 {
		t.Fatal("full ordered batch did not use its bounded exact composition")
	}
	for id := range want {
		if got := batch.Resolve(id); got != want[id] {
			t.Fatalf("selection %d resolved to %d, original event order gives %d", id, got, want[id])
		}
	}
	if err := batch.validate(); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotRejectsMalformedSelectionTransferBatch(t *testing.T) {
	w := testFlatWorld()
	s := w.Snapshot()
	s.World.SelectionTransfers.Count = 801
	if _, err := s.Restore(); err == nil {
		t.Fatal("oversized selection batch accepted")
	}
	s = w.Snapshot()
	s.World.SelectionTransfers.Count = 1
	s.World.SelectionTransfers.Transfers[0] = FollowerSelectionTransfer{1, FollowerCapacity}
	if _, err := s.Restore(); err == nil {
		t.Fatal("out-of-range selection actor accepted")
	}
	s = w.Snapshot()
	s.World.SelectionTransfers.Composed = true
	if _, err := s.Restore(); err == nil {
		t.Fatal("incomplete composed selection batch accepted")
	}
}
