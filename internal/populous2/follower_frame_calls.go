package populous2

import "fmt"

// CleanupFollowerWithFrame composes the existing complete cleanup body with
// its full caller context. $124a2 changes D0-D2; its nested farm and hero-link
// wrappers preserve the remaining data registers.
func CleanupFollowerWithFrame(ref NativeRecordReference, frame *NativeFrameRegisterContext, cb FollowerCleanupCallbacks) (FollowerCleanupStep, error) {
	if frame == nil {
		return FollowerCleanupStep{}, fmt.Errorf("native cleanup frame missing")
	}
	step, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: frame.D[0], D1: frame.D[1], D2: frame.D[2]}, cb)
	frame.D[0], frame.D[1], frame.D[2] = step.Registers.D0, step.Registers.D1, step.Registers.D2
	return step, err
}

// ClearFollowerLeaderWithFrame preserves the complete saved D0 and keeps the
// actual owner-word/Y-byte writes in D2/D1 from $140ae visible to the caller.
func ClearFollowerLeaderWithFrame(ref NativeRecordReference, frame *NativeFrameRegisterContext, cb FollowerLeaderCallbacks) (FollowerLeaderStep, error) {
	if frame == nil {
		return FollowerLeaderStep{}, fmt.Errorf("native leader frame missing")
	}
	step, err := ClearFollowerLeader(ref, FollowerCleanupRegisters{D0: frame.D[0], D1: frame.D[1], D2: frame.D[2]}, cb)
	frame.D[0], frame.D[1], frame.D[2] = step.Registers.D0, step.Registers.D1, step.Registers.D2
	return step, err
}

// PrepareFollowerContactWithFrame retains $12ade's saved D0 and its actual
// Helen target-reference D1 / town-farm D2 byte outputs. Original graph,
// association and capture mutations remain in the complete contact body.
func PrepareFollowerContactWithFrame(source, target NativeRecordReference, frame *NativeFrameRegisterContext, cb FollowerContactCallbacks) (FollowerContactStep, error) {
	if frame == nil {
		return FollowerContactStep{}, fmt.Errorf("native contact frame missing")
	}
	clear := cb.ClearFarms
	cb.ClearFarms = func(ref NativeRecordReference, tile uint8) error {
		frame.Byte(2, tile)
		if clear == nil {
			return fmt.Errorf("native contact farm callback missing")
		}
		return clear(ref, tile)
	}
	step, err := PrepareFollowerContact(source, target, cb)
	if step.Captured {
		frame.D[1] = uint32(int32(int16(step.Defender)))
	}
	return step, err
}

// MergeWithFrame exposes $128fc's full saved-register behavior. Its complete
// body changes D0 internally but restores it; unlink preserves D0/D2 and
// hero-link cleanup preserves D0, so all incoming data longs survive.
func (rules FollowerEntryRules) MergeWithFrame(source, target NativeRecordReference, frame *NativeFrameRegisterContext, cb FollowerEntryCallbacks) error {
	if frame == nil {
		return fmt.Errorf("native merge frame missing")
	}
	return rules.merge(source, target, cb)
}
