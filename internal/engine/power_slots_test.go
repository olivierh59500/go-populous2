package engine

import "testing"

func TestCanonicalFirePowerSlotsKeepDistinctPanelAndCreatorPrices(t *testing.T) {
	if Achilles != 26 || Volcano != 27 {
		t.Fatal("physical fire power slots do not match original icons")
	}
	w := testFlatWorld()
	if w.PanelPowerCost(0, Achilles) != 40000 || w.PowerCost(0, Achilles) != 80000 || w.PanelPowerCost(0, Volcano) != 80000 || w.PowerCost(0, Volcano) != 40000 {
		t.Fatal("panel and creator prices were incorrectly unified")
	}
	w.RecordPowerUse(0, Achilles)
	if w.Players[0].Statistics.WeightedPowerUse != 4 {
		t.Fatal("Achilles statistics used icon rather than command cost slot")
	}
	w.RecordPowerUse(0, Volcano)
	if w.Players[0].Statistics.WeightedPowerUse != 7 {
		t.Fatal("cast statistics no longer use actual command cost slots")
	}
}

func TestLegacySnapshotsMigrateSemanticPowerIDsWithoutMovingNativePermissions(t *testing.T) {
	w := testFlatWorld()
	w.Players[0].RallyX, w.Players[0].RallyY = 32, 32
	w.Players[1].RallyX, w.Players[1].RallyY = 32, 32
	w.Level.Players[0].Powers[26] = true
	w.Level.Players[0].Powers[27] = false
	for _, version := range []int{1, 2} {
		snapshot := w.Snapshot()
		snapshot.Version = version
		snapshot.World.AI[0].PreparedPower = 26
		snapshot.World.AI[0].Order = AIOrder{Kind: AICastPower, Power: 27}
		snapshot.World.AI[0].Choices[0] = AIPowerChoice{Power: 26, Target: AITargetTown}
		snapshot.World.AI[0].Choices[1] = AIPowerChoice{Power: 27, Target: AITargetLeader}
		restored, err := snapshot.Restore()
		if err != nil {
			t.Fatal(err)
		}
		if restored.AI[0].PreparedPower != Volcano || restored.AI[0].Order.Power != Achilles || restored.AI[0].Choices[0].Power != Volcano || restored.AI[0].Choices[1].Power != Achilles {
			t.Fatal("old semantic power IDs lost their meaning")
		}
		if restored.Level.Players[0].Powers != w.Level.Players[0].Powers {
			t.Fatal("native physical permission slots were swapped")
		}
		current, err := restored.Snapshot().Restore()
		if err != nil {
			t.Fatal(err)
		}
		if current.AI != restored.AI {
			t.Fatal("current canonical save migrated twice")
		}
	}
}

func TestAIUsesSourceCommandPermissionSlotsInsteadOfCrossedIcons(t *testing.T) {
	w := testFlatWorld()
	w.Level.Players[1].Powers[26] = true
	w.Level.Players[1].Powers[27] = false
	w.compileAIPowers(1)
	found := false
	for _, choice := range w.AI[1].Choices[:w.AI[1].ChoiceCount] {
		found = found || choice.Power == Volcano
	}
	if !found || w.AI[1].LeaderChoiceCount != 0 {
		t.Fatal("AI command flags were treated as physical icon flags")
	}
	w.Level.Players[1].Powers[26], w.Level.Players[1].Powers[27] = false, true
	w.compileAIPowers(1)
	if w.AI[1].LeaderChoiceCount != 1 || w.AI[1].Choices[w.AI[1].ChoiceCount].Power != Achilles {
		t.Fatal("AI Achilles command no longer uses source flag27")
	}
}

func TestLegacySnapshotRestoresAssignedInspectionIdentityOnly(t *testing.T) {
	w := testFlatWorld()
	w.Players[0].RallyX, w.Players[0].RallyY = 32, 32
	w.Players[1].RallyX, w.Players[1].RallyY = 32, 32
	if err := w.CastFireColumn(0, 30, 30); err != nil {
		t.Fatal(err)
	}
	s := w.Snapshot()
	s.Version = 2
	s.Reservations[0].InspectionClass = InspectUnspecified
	restored, err := s.Restore()
	if err != nil {
		t.Fatal(err)
	}
	if restored.effects.Slots[0].InspectionClass != InspectFireColumn {
		t.Fatal("old assigned effect identity was not reconstructed")
	}
	s = w.Snapshot()
	s.Reservations[0].InspectionClass = 255
	if _, err := s.Restore(); err == nil {
		t.Fatal("unknown semantic inspection identity accepted")
	}
}
