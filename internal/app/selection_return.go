package app

import "go-populous2/internal/engine"

// FollowerSelectionReturn is local presentation state. Temporary hero/editor
// inspection uses 100 main-frame presentations; simulation passes, Draw calls
// and unrelated menus do not advance this countdown.
type FollowerSelectionReturn struct {
	BackupFollower int
	FramesLeft     uint16
}

func (s *FollowerSelectionReturn) begin(current, target int) int {
	if current != 0 {
		// The source replaces its backup only when a previous temporary
		// inspection is already active. An empty selection has no timeout.
		if s.FramesLeft != 0 {
			s.BackupFollower = current
		}
		s.FramesLeft = 100
	}
	return target
}

func (s *FollowerSelectionReturn) checked(current int, valid func(int) bool) int {
	if current == 0 {
		return 0
	}
	if valid(current) {
		return current
	}
	if s.FramesLeft != 0 {
		s.FramesLeft = 0
		if valid(s.BackupFollower) {
			return s.BackupFollower
		}
	}
	return 0
}

func (s *FollowerSelectionReturn) advance(current int, valid func(int) bool) int {
	if s.FramesLeft != 0 {
		s.FramesLeft--
		if s.FramesLeft == 0 {
			current = s.BackupFollower
		}
	}
	return s.checked(current, valid)
}

func selectedFollowerExists(w *engine.World, id int) bool {
	return w != nil && id > 0 && id < engine.FollowerCapacity && w.Followers[id].State != engine.Inactive
}

// selectFollowerTemporarily is used by the original hero/editor scan actions.
// Clicking a visible map actor sets SelectedFollower directly instead.
func (g *Game) selectFollowerTemporarily(id int) {
	if !selectedFollowerExists(g.World, id) {
		return
	}
	g.SelectedFollower = g.SelectionReturn.begin(g.SelectedFollower, id)
}

func (g *Game) advanceSelectionPresentation() {
	g.SelectedFollower = g.SelectionReturn.advance(g.SelectedFollower, func(id int) bool {
		return selectedFollowerExists(g.World, id)
	})
}
