package populous2

import "fmt"

type FollowerAttritionCallbacks struct {
	Read    func(NativeRecordReference) (FollowerEntryActor, error)
	Write   func(NativeRecordReference, FollowerEntryActor) error
	Cleanup func(NativeRecordReference, uint16) error
}

type FollowerAttritionStep struct {
	Died bool
	D0   uint32
}

// ApplyFollowerAttrition translates $130e4/$130fc. amount is the exact deity
// long at offset$14(decimal20) read by the caller's A1 context. SUB.L wraps,
// then MOVE.L stores the result and resets overflow before BGT, so the signed
// stored result determines survival. It is not an immediate legacy deletion.
func ApplyFollowerAttrition(ref NativeRecordReference, amount uint32, cb FollowerAttritionCallbacks) (FollowerAttritionStep, error) {
	var step FollowerAttritionStep
	if cb.Read == nil || cb.Write == nil {
		return step, fmt.Errorf("native attrition record callbacks missing")
	}
	a, err := cb.Read(ref)
	if err != nil {
		return step, err
	}
	a.Motion.Population = int32(uint32(a.Motion.Population) - amount)
	if err := cb.Write(ref, a); err != nil {
		return step, err
	}
	if a.Motion.Population > 0 {
		return step, nil
	}
	if cb.Cleanup == nil {
		return step, fmt.Errorf("native attrition retained cleanup missing")
	}
	if err := cb.Cleanup(ref, 1); err != nil {
		return step, err
	}
	a, err = cb.Read(ref)
	if err != nil {
		return step, err
	}
	a.Motion.Animation = 0x7f4
	if err := cb.Write(ref, a); err != nil {
		return step, err
	}
	// Re-read the hero flag after cleanup, which can change raw relationships.
	if a.Motion.Flags&2 != 0 {
		a.Motion.Animation = 0x9d4
		if err := cb.Write(ref, a); err != nil {
			return step, err
		}
		a.Motion.State = 0x40
	} else {
		a.Motion.State = 0x2c
	}
	if err := cb.Write(ref, a); err != nil {
		return step, err
	}
	step.Died, step.D0 = true, 1
	return step, nil
}
