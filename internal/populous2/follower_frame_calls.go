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
