package populous

import "testing"

func TestQueuedTerrainOrdersUseSecondGameAdmission(t *testing.T) {
	w := GenerateWorld(TutorialLevel())
	got := []Command{}
	w.TerrainCommand = func(player, x, y int, raise bool) bool {
		kind := CommandLower
		if raise {
			kind = CommandRaise
		}
		got = append(got, Command{Player: player, X: x, Y: y, Kind: kind})
		return false
	}
	before := w.Magnets
	if w.applyLegacyOrder(Command{Kind: CommandRaise, Player: 1, X: 12, Y: 13}) || w.applyLegacyOrder(Command{Kind: CommandLower, Player: 0, X: 14, Y: 15}) {
		t.Fatal("rejected native admission became accepted")
	}
	if len(got) != 2 || got[0].Player != 1 || got[0].Kind != CommandRaise || got[1].Kind != CommandLower || w.Magnets != before {
		t.Fatal("queued terrain order bypassed callback or charged inherited mana")
	}
}
