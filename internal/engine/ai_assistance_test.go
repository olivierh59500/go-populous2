package engine

import "testing"

func TestHumanAssistanceExpandsWithoutComputerReactionOrUrgentOrders(t *testing.T) {
	w := testFlatWorld()
	town := w.allocate(Follower{Owner: 0, X: 20, Y: 20, State: Town, Population: 100, Stage: 9})
	w.linkFollower(town)
	w.Players[0].Assisted = true
	w.Players[0].Mana = 10000
	w.AI[0].ExpansionTown = town
	w.AI[0].Reaction = 300
	w.AI[0].TerrainRequestFollower = town
	w.AI[0].TerrainRequestX, w.AI[0].TerrainRequestY = 5, 5
	w.Heights[20+19*CornerSize] = 0
	before := w.Heights
	w.thinkAI(0)
	if got := w.AI[0].Order; got.Kind != AIRaise || got.X != 20 || got.Y != 19 || w.AI[0].Reaction != 300 {
		t.Fatal("human assistance used the full computer policy", got, w.AI[0].Reaction)
	}
	if w.Heights != before {
		t.Fatal("assistance changed terrain before the deferred command phase")
	}
	w.executeAIOrders()
	if w.Heights[20+19*CornerSize] != 1 {
		t.Fatal("assistance did not execute its expansion command")
	}
	w.AI[0] = AIState{Reaction: 300, Prepared: true, PreparedPower: Volcano, PreparedTarget: PowerTarget{X: 40, Y: 40}}
	w.thinkAI(0)
	if w.AI[0].Order.Kind != AINoOrder || w.AI[0].Reaction != 300 {
		t.Fatal("human assistance cast an offensive power or changed reaction state")
	}
	restored, err := w.Snapshot().Restore()
	if err != nil || !restored.Players[0].Assisted || restored.Players[0].Computer {
		t.Fatal("snapshot lost the distinct human assistance policy", err)
	}
}
