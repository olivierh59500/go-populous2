package app

import (
	"image"
	"testing"

	"go-populous2/internal/engine"
	"go-populous2/internal/visualassets"
)

func selectionReturnGame() *Game {
	w := &engine.World{}
	for id := 1; id <= 3; id++ {
		w.Followers[id] = engine.Follower{State: engine.Walking, Population: 100}
	}
	return &Game{World: w, SelectedFollower: 1, Assets: &Assets{Visual: &visualassets.Bundle{}, SelectionPanel: &visualassets.SelectionPanel{PopulationSprite: -1}}, framebuffer: image.NewRGBA(image.Rect(0, 0, 320, 200))}
}

func TestTemporarySelectionUsesOriginalHundredMainFrameBoundary(t *testing.T) {
	g := selectionReturnGame()
	g.selectFollowerTemporarily(2)
	if g.SelectedFollower != 2 || g.SelectionReturn.FramesLeft != 100 || g.SelectionReturn.BackupFollower != 0 {
		t.Fatal("first source temporary inspection used an invented backup")
	}
	for range 100 {
		g.drawSelectionPanel()
	}
	if g.SelectionReturn.FramesLeft != 100 {
		t.Fatal("Draw advanced the temporary-selection clock")
	}
	for range 99 {
		g.advanceSelectionPresentation()
	}
	if g.SelectedFollower != 2 || g.SelectionReturn.FramesLeft != 1 {
		t.Fatal("temporary selection returned too early")
	}
	g.advanceSelectionPresentation()
	if g.SelectedFollower != 0 || g.SelectionReturn.FramesLeft != 0 {
		t.Fatal("source return-to-empty boundary differs")
	}
}

func TestRepeatedTemporarySelectionAndRemovalUseSourceBackup(t *testing.T) {
	g := selectionReturnGame()
	g.SelectionReturn = FollowerSelectionReturn{BackupFollower: 3, FramesLeft: 20}
	g.selectFollowerTemporarily(2)
	if g.SelectionReturn.BackupFollower != 1 || g.SelectionReturn.FramesLeft != 100 {
		t.Fatal("repeated temporary inspection did not save its old selection")
	}
	g.World.Followers[2] = engine.Follower{}
	g.advanceSelectionPresentation()
	if g.SelectedFollower != 1 || g.SelectionReturn.FramesLeft != 0 {
		t.Fatal("removed temporary actor did not immediately return to its valid backup")
	}
	g.SelectedFollower = 2
	g.SelectionReturn = FollowerSelectionReturn{BackupFollower: 3, FramesLeft: 2}
	g.World.Followers[2] = engine.Follower{State: engine.Walking}
	g.World.Followers[3] = engine.Follower{}
	g.advanceSelectionPresentation()
	g.advanceSelectionPresentation()
	if g.SelectedFollower != 0 || g.SelectionReturn.FramesLeft != 0 {
		t.Fatal("removed backup selected another actor")
	}
}

func TestEmptyTemporarySelectionPreservesExistingCountdown(t *testing.T) {
	g := selectionReturnGame()
	g.SelectedFollower = 0
	g.SelectionReturn = FollowerSelectionReturn{BackupFollower: 3, FramesLeft: 2}
	g.refreshSelectedFollower()
	if g.SelectedFollower != 0 || g.SelectionReturn.FramesLeft != 2 {
		t.Fatal("empty selection prematurely restored its pending backup")
	}
	g.selectFollowerTemporarily(2)
	if g.SelectionReturn.FramesLeft != 2 || g.SelectionReturn.BackupFollower != 3 {
		t.Fatal("selecting from empty state restarted the source timeout")
	}
	g.advanceSelectionPresentation()
	g.advanceSelectionPresentation()
	if g.SelectedFollower != 3 {
		t.Fatal("existing source countdown did not restore its backup")
	}
}

func TestSelectionTransfersLeaveReturnBackupAtOriginalIdentity(t *testing.T) {
	g := selectionReturnGame()
	g.SelectionReturn = FollowerSelectionReturn{BackupFollower: 1, FramesLeft: 73}
	g.World.Tick = 1
	g.World.SelectionTransfers.Count = 1
	g.World.SelectionTransfers.Transfers[0] = engine.FollowerSelectionTransfer{FromFollower: 1, ToFollower: 2}
	g.consumeSelectionTransfers()
	if g.SelectedFollower != 2 || g.SelectionReturn.BackupFollower != 1 || g.SelectionReturn.FramesLeft != 73 {
		t.Fatal("simulation identity transfer rewrote the source's independent return backup")
	}
}
