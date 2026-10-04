package populous

import "testing"

func TestPerOwnerAttritionRunsThroughFollowerDispatch(t *testing.T) {
	w := GenerateWorld(TutorialLevel())
	w.Peeps = []Peep{{Player: 0, Population: 100, AtPos: 2000, Flags: IAmWaiting}, {Player: 1, Population: 100, AtPos: 2001, Flags: IAmWaiting}}
	w.MapWho = [4096]uint16{}
	for pos := range w.MapBlk {
		w.MapBlk[pos] = FlatBlock
	}
	w.FollowerAttrition = func(player int, water bool) int {
		if water {
			t.Fatal("land follower dispatch requested water attrition")
		}
		return []int{1, 7}[player]
	}
	w.TickWithComputer([2]bool{})
	if w.Peeps[0].Population != 99 || w.Peeps[1].Population != 93 {
		t.Fatalf("per-owner attrition did not reach walking/waiting states: %+v", w.Peeps)
	}
}

func TestFirstGameAttritionKeepsItsExistingDefaults(t *testing.T) {
	w := GenerateWorld(TutorialLevel())
	w.Rules.WalkDeath = 3
	if w.followerAttrition(0, false) != 3 || w.followerAttrition(1, true) != 6 {
		t.Fatal("second-game hook changed first-game default attrition")
	}
}
