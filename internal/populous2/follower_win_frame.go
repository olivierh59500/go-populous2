package populous2

import "fmt"

type NativeFollowerWinFrameCallbacks struct {
	Memory      FollowerCleanupMemory
	Cleanup     func(NativeRecordReference, *NativeFrameRegisterContext) error
	ClearFarms  func(NativeRecordReference, uint8) error
	ReformTown  func(NativeRecordReference, NativeRecordReference) error
	DestroyTown func(NativeRecordReference) error
	PoolBlocked func() bool
	Insert      func(NativeRecordReference) error
}

type NativeFollowerWinFrameStep struct {
	FollowerWinStep
	ReturnedA3 NativeRecordReference
}

// WinWithFrame adds the actual $1298c data-register continuation to the
// complete existing winner body. Farm, reform, town destruction and Adonis
// wrappers preserve their saved registers; $12a98 leaves cleanup D0-D2.
// The returned A3 is the source loser pointer retained across those wrappers.
func (rules FollowerWinRules) WinWithFrame(winner, loser, originalA0 NativeRecordReference, frame *NativeFrameRegisterContext, cb NativeFollowerWinFrameCallbacks) (NativeFollowerWinFrameStep, error) {
	var out NativeFollowerWinFrameStep
	if frame == nil || !winMemoryValid(cb.Memory) || cb.Cleanup == nil {
		return out, fmt.Errorf("native winner frame backing missing")
	}
	m := cb.Memory
	wa, la := cleanupRecordAddress(winner), cleanupRecordAddress(loser)
	frame.D[0], frame.D[1] = 0, 0
	flags, err := m.Read8(la + 13)
	if err != nil {
		return out, err
	}
	frame.Byte(0, flags)
	kind, err := m.Read8(la)
	if err != nil {
		return out, err
	}
	stage, err := m.Read8(la + 1)
	if err != nil {
		return out, err
	}
	reward, err := rules.Reward(flags, kind, stage)
	if err != nil {
		return out, err
	}
	frame.Word(1, reward)
	if kind == 4 {
		frame.Byte(0, stage)
		frame.Word(0, uint16(frame.D[0])*2)
	}
	owner, err := m.Read8(wa + 12)
	if err != nil {
		return out, err
	}
	frame.Byte(0, owner)
	frame.D[0] = uint32(uint16(frame.D[0])) * 314
	frame.D[0] = 0
	loserOwner, err := m.Read8(la + 12)
	if err != nil {
		return out, err
	}
	frame.Byte(0, loserOwner)
	frame.D[0] = uint32(uint16(frame.D[0])) * 314
	callbacks := FollowerWinCallbacks{Memory: m, PoolBlocked: cb.PoolBlocked, Insert: cb.Insert,
		Cleanup: func(ref NativeRecordReference, mode uint16) error {
			frame.D[0] = uint32(mode)
			return cb.Cleanup(ref, frame)
		},
		ClearFarms: func(ref NativeRecordReference, tile uint8) error {
			frame.Byte(2, tile)
			if cb.ClearFarms == nil {
				return fmt.Errorf("native winner farm callback missing")
			}
			return cb.ClearFarms(ref, tile)
		},
		ReformTown: func(ref, original NativeRecordReference) error {
			if cb.ReformTown == nil {
				return fmt.Errorf("native winner reform callback missing")
			}
			return cb.ReformTown(ref, original)
		},
		DestroyTown: func(ref NativeRecordReference) error {
			if cb.DestroyTown == nil {
				return fmt.Errorf("native winner town callback missing")
			}
			return cb.DestroyTown(ref)
		},
	}
	step, err := rules.Win(winner, loser, originalA0, callbacks)
	out.FollowerWinStep = step
	out.ReturnedA3 = loser
	return out, err
}
