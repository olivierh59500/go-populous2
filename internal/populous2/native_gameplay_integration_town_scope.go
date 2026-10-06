package populous2

import "fmt"

// evaluateFrameTown is the actual13352 child inside12BD8's saved-register
// frame. The ordered pass and every reform share its CODE cache/scratch;
// callers that do not borrow a native session retain the legacy adapter.
func (s *NativeFrameSession) evaluateFrameTown(ref NativeRecordReference) (int, error) {
	if s == nil || s.world == nil || s.followerRules == nil || s.followerRules.town == nil {
		return 0, fmt.Errorf("native town evaluation session missing")
	}
	saved := s.Frame.D
	defer func() { s.Frame.D = saved }()
	state := &s.Followers.Town
	state.OuterFlag13350 = s.Followers.Pass.TownCacheFlag
	state.MinimapVariant = s.Followers.Pass.MinimapVariant
	err := s.followerRules.town.Evaluate(ref, s.world.nativeTownFrameCallbacks(&s.Frame, state))
	s.Followers.Pass.TownCacheFlag = state.OuterFlag13350
	s.Followers.Pass.MinimapVariant = state.MinimapVariant
	return int(uint16(s.Frame.D[0])), err
}
