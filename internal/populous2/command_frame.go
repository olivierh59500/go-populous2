package populous2

import "fmt"

// NativeCommandFrameResult keeps source CCR.Z separate from completion of a
// real modal/I/O child. These UI handlers ignore Z, while their actual child
// register outputs still reach the later side command.
type NativeCommandFrameResult struct{ Complete, Zero bool }

type NativeCommandFrameCall struct {
	NativeCommandCall
	Handler              int
	PaletteA2, PaletteA3 uint32 // Original CODE labels $33844/$3361a for102e4.
	TargetA0             uint32 // Actual BSS$22 pointer supplied to the $d8cc child.
}

type NativeCommandFrameCallbacks struct {
	Immediate   NativeCommandCallbacks
	ReadCode16  func(int) (uint16, error)
	WriteCode16 func(int, uint16) error
	// Call owns the actual resumable child body and its operation phase. A
	// missing child is an explicit boundary, never a completed no-op.
	Call func(NativeCommandFrameCall, *uint32) (NativeCommandFrameResult, error)
}

// NativeCommandFrameState freezes the source caller/dispatch and retains the
// complete mutable D continuation and actual child phase across suspension.
// Ordinary immediate commands invoke the already proven Execute only once.
type NativeCommandFrameState struct {
	Started, Finished                             bool
	Caller, Handler                               int
	Registers                                     NativeCommandRegisterContext
	Phase                                         uint8
	ChildPhase                                    uint32
	ChildActive, LastZero                         bool
	ChildRoutine                                  int
	ChildPaletteA2, ChildPaletteA3, ChildTargetA0 uint32
	Step                                          NativeCommandStep
	failed                                        error
}

func commandFrameUI(handler int) bool {
	switch handler {
	case 0x17d18, 0x17d22, 0x17d3e, 0x17d58, 0x17d6a, 0x17dda:
		return true
	}
	return false
}

func (s *NativeCommandFrameState) ExecuteFrame(r *NativeCommandRules, caller int, c *NativeCommandRegisterContext, cb NativeCommandFrameCallbacks) (step NativeCommandStep, complete bool, failure error) {
	if s == nil || r == nil || c == nil || !winMemoryValid(cb.Immediate.Memory) {
		return step, false, fmt.Errorf("native command frame state/rules/backing missing")
	}
	if s.Started && s.Caller != caller {
		return s.Step, false, fmt.Errorf("native command frame caller changed during continuation")
	}
	if s.failed != nil {
		return s.Step, false, s.failed
	}
	if s.Finished {
		*c = s.Registers
		return s.Step, true, nil
	}
	defer func() {
		if s.Started {
			*c = s.Registers
		}
		if failure != nil {
			s.failed = failure
		}
	}()
	m := cb.Immediate.Memory
	if !s.Started {
		command, e := m.Read8(caller + 1)
		if e != nil {
			return step, false, e
		}
		handler := 0
		if command != 0 {
			pause, probeError := m.Read16(0xf3c)
			if probeError == nil && (pause == 0 || int16(int8(command)) > 102) {
				offset, probeError := r.word(0x17524 + int(int16(int8(command))))
				if probeError == nil {
					handler = 0x17524 + int(int16(offset))
				}
			}
		}

		s.Started = true
		s.Caller = caller
		s.Handler = handler
		s.Registers = *c
		s.Step = NativeCommandStep{Command: command, Calls: []int{}}
		if !commandFrameUI(handler) {
			// No prefix is applied before delegation: existing Execute owns the one
			// actual dispatcher/admission/body/debit sequence, including its errors.
			s.Step, e = r.Execute(caller, &s.Registers, cb.Immediate)
			if e != nil {
				return s.Step, false, e
			}
			s.Finished = true
			return s.Step, true, nil
		}
		commandByte(&s.Registers, 0, command)
		commandExtend(&s.Registers, 0)
		offset, e := r.word(0x17524 + int(int16(uint16(s.Registers.D[0]))))
		if e != nil {
			return s.Step, false, e
		}
		commandWord(&s.Registers, 0, offset)
	}
	child := func(routine int, palette bool, target uint32) (bool, error) {
		if cb.Call == nil {
			return false, fmt.Errorf("native command frame child%x callback missing", routine)
		}
		if !s.ChildActive {
			s.Step.Calls = append(s.Step.Calls, routine)
			s.ChildActive = true
			s.ChildRoutine, s.ChildTargetA0 = routine, target
			s.ChildPaletteA2, s.ChildPaletteA3 = 0, 0
			if palette {
				s.ChildPaletteA2, s.ChildPaletteA3 = 0x33844, 0x3361a
			}
		} else if s.ChildRoutine != routine {
			return false, fmt.Errorf("native command frame child changed during continuation")
		}
		call := NativeCommandFrameCall{NativeCommandCall: NativeCommandCall{Routine: s.ChildRoutine, Caller: s.Caller, Context: &s.Registers}, Handler: s.Handler, TargetA0: s.ChildTargetA0, PaletteA2: s.ChildPaletteA2, PaletteA3: s.ChildPaletteA3}

		result, e := cb.Call(call, &s.ChildPhase)
		if e != nil {
			return false, e
		}
		if !result.Complete {
			return false, nil
		}
		s.LastZero = result.Zero
		s.ChildActive = false
		s.ChildPhase = 0
		s.Phase++
		return true, nil
	}
	codeWrite := func(at int, value uint16) error {
		if cb.WriteCode16 == nil {
			return fmt.Errorf("native command frame CODE word%x callback missing", at)
		}
		return cb.WriteCode16(at, value)
	}
	for !s.Finished {
		switch s.Handler {
		case 0x17d18, 0x17d22:
			switch s.Phase {
			case 0:
				if s.Handler == 0x17d18 {
					seed, e := m.Read32(0xeb24)
					if e != nil {
						return s.Step, false, e
					}
					if e := m.Write32(0xeb28, seed); e != nil {
						return s.Step, false, e
					}
				}
				s.Phase++
			case 1:
				done, e := child(0x102e4, true, 0)
				if e != nil || !done {
					return s.Step, false, e
				}
			case 2:
				done, e := child(0x10ad8, false, 0)
				if e != nil || !done {
					return s.Step, false, e
				}
			default:
				s.Finished = true
			}
		case 0x17d3e, 0x17d58:
			switch s.Phase {
			case 0:
				if s.Handler == 0x17d3e {
					if e := codeWrite(0x3f90, 0); e != nil {
						return s.Step, false, e
					}
					if e := codeWrite(0x4468, 1); e != nil {
						return s.Step, false, e
					}
				} else {
					if e := codeWrite(0x4468, 0); e != nil {
						return s.Step, false, e
					}
				}
				s.Phase++
			case 1:
				done, e := child(0x3f92, false, 0)
				if e != nil || !done {
					return s.Step, false, e
				}
			default:
				s.Finished = true
			}
		case 0x17d6a:
			switch s.Phase {
			case 0:
				done, e := child(0x102e4, true, 0)
				if e != nil || !done {
					return s.Step, false, e
				}
			case 1:
				done, e := child(0x1842e, false, 0)
				if e != nil || !done {
					return s.Step, false, e
				}
			case 2:
				if cb.ReadCode16 == nil {
					return s.Step, false, fmt.Errorf("native command frame CODEa2a reader missing")
				}
				value, e := cb.ReadCode16(0xa2a)
				if e != nil {
					return s.Step, false, e
				}
				if e := m.Write16(0xddc, value); e != nil {
					return s.Step, false, e
				}
				if e := codeWrite(0xa2a, 0); e != nil {
					return s.Step, false, e
				}
				s.Phase++
			case 3:
				done, e := child(0x10a8c, false, 0)
				if e != nil || !done {
					return s.Step, false, e
				}
			default:
				s.Finished = true
			}
		case 0x17dda:
			switch s.Phase {
			case 0:
				value, e := m.Read16(0xeb22)
				if e != nil {
					return s.Step, false, e
				}
				value++
				if value == 4 {
					value = 0
				}
				if e := m.Write16(0xeb22, value); e != nil {
					return s.Step, false, e
				}
				s.Phase++
			case 1:
				done, e := child(0x102e4, true, 0)
				if e != nil || !done {
					return s.Step, false, e
				}
			case 2:
				done, e := child(0x1a32a, false, 0)
				if e != nil || !done {
					return s.Step, false, e
				}
			case 3:
				target := s.ChildTargetA0
				if !s.ChildActive {
					var e error
					target, e = m.Read32(0x22)
					if e != nil {
						return s.Step, false, e
					}
				}
				done, e := child(0xd8cc, false, target)
				if e != nil || !done {
					return s.Step, false, e
				}
			default:
				s.Finished = true
			}
		default:
			return s.Step, false, fmt.Errorf("native resumable command handler%x missing", s.Handler)
		}
	}
	return s.Step, true, nil
}
