package populous2

import "testing"

func TestNativeMagnetSharesMixedMapChainAndRelocatesWithoutPressure(t *testing.T) {
	var graph NativeWorldOccupancy
	const source = 20 + 21*64
	const destination = 23 + 24*64
	graph.Grid.Cells[source].Header = 0xb1
	graph.Grid.Cells[destination].Header = 0xd2
	follower, _ := NativeWorldReference(NativeFollowerPool, 0)
	marker, _ := NativeMagnetReference(1)
	if err := graph.Place(follower, 20*256+64, 21*256+192); err != nil {
		t.Fatal(err)
	}
	if err := graph.Place(marker, 20*256+128, 21*256+128); err != nil {
		t.Fatal(err)
	}
	if graph.Grid.Cells[source].Head != marker || graph.Followers[0].Record.Previous != marker || graph.Magnets[1].Record.Next != follower {
		t.Fatal("native magnet was not prepended to the mixed actor chain")
	}
	if err := graph.Remove(marker); err != nil {
		t.Fatal(err)
	}
	if err := graph.Place(marker, 23*256+128, 24*256+128); err != nil {
		t.Fatal(err)
	}
	if graph.Grid.Cells[source].Head != follower || graph.Followers[0].Record.Previous != 0 || graph.Grid.Cells[destination].Head != marker || graph.Grid.Cells[source].Header != 0xb1 || graph.Grid.Cells[destination].Header != 0xd2 {
		t.Fatal("leader marker relocation changed actor membership or pressure")
	}
	if err := graph.Validate(); err != nil {
		t.Fatal(err)
	}
	graph.Magnets[1].Linked = false
	if err := graph.Validate(); err == nil {
		t.Fatal("validator accepted a mapped marker without membership")
	}
}

func TestNativeMagnetLookupKeepsActorImageBoundary(t *testing.T) {
	for owner := uint8(0); owner < 3; owner++ {
		ref, _ := NativeMagnetReference(owner)
		location, ok := LocateNativeMagnet(ref)
		if !ok || location.Index != int(owner) || location.Stride != 14 {
			t.Fatal("native marker lookup differs")
		}
		if _, ok := LocateNativeRecord(ref); ok {
			t.Fatal("marker was classified inside the four actor pools")
		}
		if _, ok := LocateNativeMagnet(ref + 1); ok {
			t.Fatal("unaligned marker entity reference accepted")
		}
	}
}
