package populous

import "testing"

func TestNativeFollowerCapacityAndWideOccupancy(t *testing.T) {
	w := GenerateWorld(TutorialLevel())
	for len(w.Peeps) < MaxFollowers {
		if w.AllocateHeroClone(0) < 0 {
			t.Fatalf("allocation stopped at %d", len(w.Peeps))
		}
	}
	if got := w.AllocateHeroClone(0); got != -1 {
		t.Fatalf("reserved native record allocated: %d", got)
	}
	last := len(w.Peeps) - 1
	w.clearPeepMapRefs(last)
	w.Peeps[last].AtPos = 2000
	w.MapWho[2000] = uint16(last + 1)
	if w.MapWho[2000] != 399 {
		t.Fatal("occupant ID truncated to eight bits")
	}
	hash := w.StateHash()
	restored := WorldFromSnapshot(w.Snapshot(), w.Rules)
	if restored.MapWho[2000] != 399 || restored.StateHash() != hash {
		t.Fatal("snapshot lost wide references")
	}
	w.DamagePeep(last, w.Peeps[last].Population)
	if w.MapWho[2000] != 0 {
		t.Fatal("dead high-numbered follower retained an occupancy reference")
	}
	if got := w.AllocateHeroClone(0); got != last {
		t.Fatalf("dead record not recycled: %d", got)
	}
}
