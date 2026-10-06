package app

import (
	"reflect"
	"testing"
)

func TestOriginalAssistAndSideControlsRetainHumanInput(t *testing.T) {
	g := browserGame(t)
	g.CustomGame = true
	g.controlMode = 2
	g.applyControlMode()
	if err := g.applyInGameAction("assist"); err != nil {
		t.Fatal(err)
	}
	if g.controlMode != 18 || !g.World.Players[0].Assisted || g.World.Players[0].Computer {
		t.Fatal("computer assistance replaced the human with an offensive AI")
	}
	if err := g.applyInGameAction("profile"); err != nil {
		t.Fatal(err)
	}
	if g.playerSide() != 1 || !g.World.Players[1].Assisted || g.World.Players[1].Computer || !g.World.Players[0].Computer || g.World.Players[0].Assisted {
		t.Fatal("I AM switch kept computer assistance on the previous side")
	}
	if err := g.applyInGameAction("assist"); err != nil {
		t.Fatal(err)
	}
	if g.controlMode != 2 || g.World.Players[1].Computer || g.World.Players[1].Assisted || !g.World.Players[0].Computer {
		t.Fatal("disabling assistance did not restore ordinary human control")
	}
}

func TestControlActionsPreserveTheOtherImportedHumanDeity(t *testing.T) {
	g := browserGame(t)
	g.CustomGame = true
	for owner := range g.World.Players {
		g.World.Players[owner].Computer = false
		g.World.Players[owner].Assisted = false
	}
	g.controlMode = 4 // A stale presentation cache must not override the save.
	g.restoreControlMode()
	if _, assist, match := g.inGameControlLabels(); assist != "OFF" || match != "HUMAN V HUMAN      " {
		t.Fatal("two imported human deities were labeled as a computer game", assist, match)
	}
	other := g.World.Players[1]
	if err := g.applyInGameAction("assist"); err != nil {
		t.Fatal(err)
	}
	if !g.World.Players[0].Assisted || g.World.Players[0].Computer || g.World.Players[1] != other {
		t.Fatal("assistance changed the opposing imported deity")
	}
	if _, assist, match := g.inGameControlLabels(); assist != "ON " || match != "HUMAN V HUMAN      " {
		t.Fatal("assistance was confused with replacing human input", assist, match)
	}
	if err := g.applyInGameAction("opponent-control"); err != nil {
		t.Fatal(err)
	}
	if !g.World.Players[0].Computer || g.World.Players[0].Assisted || g.World.Players[1] != other {
		t.Fatal("computer-mode toggle changed the wrong deity")
	}
	if _, _, match := g.inGameControlLabels(); match != "COMPUTER V HUMAN   " {
		t.Fatal("one computer was mislabeled as two", match)
	}
	if err := g.applyInGameAction("opponent-control"); err != nil {
		t.Fatal(err)
	}
	if g.World.Players[0].Computer || g.World.Players[0].Assisted || g.World.Players[1] != other {
		t.Fatal("second toggle did not restore ordinary human control")
	}
}

func TestProfileHandoffSwapsControlsAndExperienceWithoutMovingTheWorld(t *testing.T) {
	g := browserGame(t)
	g.CustomGame = true
	g.World.Players[0].Computer, g.World.Players[0].Assisted = false, true
	g.World.Players[1].Computer, g.World.Players[1].Assisted = false, false
	g.World.Players[0].Experience = [6]uint8{1, 2, 3, 4, 5, 6}
	g.World.Players[1].Experience = [6]uint8{20, 21, 22, 23, 24, 25}
	profile := g.Profile
	before := g.World.Snapshot()
	if err := g.applyInGameAction("profile"); err != nil {
		t.Fatal(err)
	}
	if g.playerSide() != 1 || g.controlMode != 18 || !g.World.Players[1].Assisted || g.World.Players[0].Assisted || g.World.Players[0].Computer || g.World.Players[1].Computer {
		t.Fatal("I AM did not exchange the two actual control words")
	}
	if g.World.Players[1].Experience != before.World.Players[0].Experience || g.World.Players[0].Experience != before.World.Players[1].Experience || g.Profile != profile {
		t.Fatal("human experience was lost or the user profile changed")
	}
	after := g.World.Snapshot()
	for owner := range after.World.Players {
		after.World.Players[owner].Computer = before.World.Players[owner].Computer
		after.World.Players[owner].Assisted = before.World.Players[owner].Assisted
		after.World.Players[owner].Experience = before.World.Players[owner].Experience
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatal("profile control handoff moved followers, terrain, AI or game statistics")
	}
}
