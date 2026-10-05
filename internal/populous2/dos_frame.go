package populous2

import "fmt"

// NativeDOSLibraryCall carries the actual library vector and native arguments.
// The absent Amiga DOS binary is an explicit port boundary; a port may mutate
// every surviving register, and the enclosing source MOVEM masks decide what
// reaches the caller. D0 is supplied separately from asynchronous completion.
type NativeDOSLibraryCall struct {
	Vector       int
	Frame        *NativeFrameRegisterContext
	A            *[7]NativeRequesterAddress
	Path         []byte
	Handle       uint32
	Buffer       NativeRequesterAddress
	Count        uint32
	Code, Memory FollowerCleanupMemory
	CodeBase     uint32
}
type NativeDOSLibraryResult struct {
	Complete bool
	D0       uint32
}
type NativeDOSPort struct {
	Call func(NativeDOSLibraryCall, *uint32) (NativeDOSLibraryResult, error)
}
type NativeDOSFrameCallbacks struct {
	Code, Memory FollowerCleanupMemory
	CodeBase     uint32
	Frame        *NativeFrameRegisterContext
	Port         NativeDOSPort
	Call         func(NativeFileFrameCall, *uint32) (NativeCommandFrameResult, error)
}
type NativeDOSFrameStep struct {
	Complete, Waiting bool
	PC                int
	Library           []int
	Children          []int
}

// NativeDOSFrameState translates $19936/$19afc/$19c1c. Transfers use the raw
// DD6..EB48 bytes only. The surrounding $3f92 caller owns F32/F36 rebasing.
type NativeDOSFrameState struct {
	Started, Finished                bool
	Routine, PC                      int
	Registers, Outer, LibrarySaved   [8]uint32
	A, LibrarySavedA                 [7]NativeRequesterAddress
	LibraryActive, LibraryPrepared   bool
	LibraryVector                    int
	LibraryPhase                     uint32
	LibraryMaskD, LibraryMaskA       uint8
	LibraryCall                      NativeDOSLibraryCall
	ChildActive                      bool
	ChildRoutine                     int
	ChildPhase                       uint32
	Handle, Transferred, ChildSaved0 uint32
	ListCursor, ListLimit            uint32
	failed                           error
}

func (s *NativeDOSFrameState) Advance(routine int, arguments [7]NativeRequesterAddress, cb NativeDOSFrameCallbacks) (out NativeDOSFrameStep, failure error) {
	if s == nil || cb.Frame == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) {
		return out, fmt.Errorf("native DOS frame backing missing")
	}
	if s.Started && s.Routine != routine {
		return out, fmt.Errorf("native DOS routine changed across suspension")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		if routine != 0x19936 && routine != 0x19afc && routine != 0x19c1c {
			return out, fmt.Errorf("native DOS routine%x unsupported", routine)
		}
		s.Started = true
		s.Routine = routine
		s.PC = routine
		s.Registers = cb.Frame.D
		s.Outer = cb.Frame.D
		s.A = arguments
	}
	cb.Frame.D = s.Registers
	defer func() {
		s.Registers = cb.Frame.D
		out.PC = s.PC
		if failure != nil {
			s.failed = failure
		}
	}()
	if s.Finished {
		out.Complete = true
		return out, nil
	}
	c, code, m := cb.Frame, cb.Code, cb.Memory
	b := nativeRequesterFrameBacking{Code: code, Memory: m, CodeBase: cb.CodeBase, Frame: c}
	caddr := func(at int) NativeRequesterAddress {
		return NativeRequesterAddress{Address: cb.CodeBase + uint32(at), Code: true}
	}
	baddr := func(at int) NativeRequesterAddress {
		return NativeRequesterAddress{Address: c.AddressBase + uint32(at)}
	}
	text := func(p NativeRequesterAddress) ([]byte, error) {
		v := []byte{}
		for {
			ch, e := b.read(p)
			if e != nil {
				return nil, e
			}
			p.Address++
			if ch == 0 {
				return v, nil
			}
			v = append(v, ch)
		}
	}
	saveCursor := func() error {
		v, e := code.Read16(0xa2a)
		if e != nil {
			return e
		}
		if e = m.Write16(0x3ba, v); e != nil {
			return e
		}
		return code.Write16(0xa2a, 0x120)
	}
	restoreCursor := func() error {
		v, e := m.Read16(0x3ba)
		if e != nil {
			return e
		}
		return code.Write16(0xa2a, v)
	}
	restore := func(mask uint8, d [8]uint32) {
		for i := range d {
			if mask&(1<<i) != 0 {
				c.D[i] = d[i]
			}
		}
	}
	library := func(vector, next int, maskD, maskA uint8) (bool, error) {
		if cb.Port.Call == nil {
			return false, fmt.Errorf("native DOS library vector%d missing", vector)
		}
		if !s.LibraryActive {
			s.LibraryActive = true
			s.LibraryVector = vector
			if !s.LibraryPrepared {
				s.LibrarySaved = c.D
				s.LibrarySavedA = s.A
			}
			s.LibraryPrepared = false
			s.LibraryMaskD = maskD
			s.LibraryMaskA = maskA
			base, e := m.Read32(0x14c)
			if e != nil {
				return false, e
			}
			if routine == 0x19936 {
				s.A[5] = NativeRequesterAddress{Address: base, Absolute: true}
			} else {
				s.A[6] = NativeRequesterAddress{Address: base, Absolute: true}
			}
			call := NativeDOSLibraryCall{Vector: vector, Frame: c, A: &s.A, Code: code, Memory: m, CodeBase: cb.CodeBase}
			switch vector {
			case -84, -30:
				p := s.A[0]
				if vector == -84 {
					p = NativeRequesterAddress{Address: c.D[1], Code: s.A[0].Code}
				}
				path, e := text(p)
				if e != nil {
					return false, e
				}
				call.Path = path
			case -102, -108:
				call.Handle = c.D[1]
				call.Buffer = NativeRequesterAddress{Address: c.D[2], Code: true}
			case -90, -36:
				call.Handle = c.D[1]
			case -48, -42:
				call.Handle = c.D[1]
				call.Buffer = NativeRequesterAddress{Address: c.D[2]}
				call.Count = c.D[3]
			}
			s.LibraryCall = call
			out.Library = append(out.Library, vector)
		}
		if s.LibraryVector != vector {
			return false, fmt.Errorf("native DOS vector changed across suspension")
		}
		s.LibraryCall.Frame = c
		s.LibraryCall.A = &s.A
		result, e := cb.Port.Call(s.LibraryCall, &s.LibraryPhase)
		if e != nil {
			return false, e
		}
		if !result.Complete {
			out.Waiting = true
			return false, nil
		}
		c.D[0] = result.D0
		restore(s.LibraryMaskD, s.LibrarySaved)
		for i := range s.A {
			if s.LibraryMaskA&(1<<i) != 0 {
				s.A[i] = s.LibrarySavedA[i]
			}
		}
		s.LibraryActive = false
		s.LibraryPhase = 0
		s.PC = next
		return true, nil
	}
	child := func(at, next int) (bool, error) {
		if cb.Call == nil {
			return false, fmt.Errorf("native DOS child%x missing", at)
		}
		if !s.ChildActive {
			s.ChildActive = true
			s.ChildRoutine = at
			out.Children = append(out.Children, at)
		}
		if s.ChildRoutine != at {
			return false, fmt.Errorf("native DOS child changed across suspension")
		}
		mask := uint8(0)
		if at == 0x341e {
			mask = 1
		}
		result, e := cb.Call(NativeFileFrameCall{Routine: at, Arguments: mask, A: s.A, Frame: c}, &s.ChildPhase)
		if e != nil {
			return false, e
		}
		if !result.Complete {
			out.Waiting = true
			return false, nil
		}
		s.ChildActive = false
		s.ChildPhase = 0
		s.PC = next
		return true, nil
	}
	prepareLibrary := func() { s.LibrarySaved = c.D; s.LibrarySavedA = s.A; s.LibraryPrepared = true }
	finish := func() {
		if routine != 0x19936 {
			restore(0xfe, s.Outer)
		}
		s.Finished = true
		out.Complete = true
	}
	for transitions := 0; transitions < 65536; transitions++ {
		switch s.PC {
		case 0x19936:
			if e := saveCursor(); e != nil {
				return out, e
			}
			c.Word(7, 0)
			c.D[1] = s.A[0].Address
			c.D[2] = 0xfffffffe
			s.PC = 0x19962
		case 0x19962:
			done, e := library(-84, 0x1996a, 6, 2)
			if e != nil || !done {
				return out, e
			}
		case 0x1996a:
			c.D[1] = c.D[0]
			s.Handle = c.D[1]
			if c.D[1] == 0 {
				s.PC = 0x199dc
			} else {
				s.A[2] = caddr(0x199f8)
				c.D[2] = s.A[2].Address
				s.PC = 0x1997a
			}
		case 0x1997a:
			done, e := library(-102, 0x19982, 6, 2)
			if e != nil || !done {
				return out, e
			}
		case 0x19982:
			if c.D[0] == 0 {
				s.PC = 0x199d8
			} else {
				s.PC = 0x1998a
			}
		case 0x1998a:
			done, e := library(-108, 0x19992, 6, 2)
			if e != nil || !done {
				return out, e
			}
		case 0x19992:
			if uint16(c.D[0]) == 0 {
				s.PC = 0x199d8
				break
			}
			typ, e := code.Read16(int(int64(s.A[2].Address)-int64(cb.CodeBase)) + 4)
			if e != nil {
				return out, e
			}
			if int16(typ) >= 0 || int32(s.A[1].Address) >= int32(s.A[4].Address) {
				s.PC = 0x1998a
				break
			}
			s.A[3] = s.A[2]
			s.A[3].Address += 8
			for {
				v, e := b.read(s.A[3])
				if e != nil {
					return out, e
				}
				s.A[3].Address++
				if v == 0 {
					break
				}
			}
			s.A[3].Address--
			s.A[6] = caddr(0xab49)
			match := true
			for {
				v, e := b.read(s.A[6])
				if e != nil {
					return out, e
				}
				s.A[6].Address++
				c.Byte(0, v)
				if v == 0 {
					break
				}
				s.A[3].Address--
				got, e := b.read(s.A[3])
				if e != nil {
					return out, e
				}
				if got != v {
					match = false
					break
				}
			}
			if match {
				s.A[3] = s.A[2]
				s.A[3].Address += 8
				for {
					v, e := b.read(s.A[3])
					if e != nil {
						return out, e
					}
					s.A[3].Address++
					c.Byte(0, v)
					if int8(v) > 0x40 {
						c.Byte(0, v&0x5f)
					}
					at := int(int64(s.A[1].Address) - int64(c.AddressBase))
					if e := m.Write8(at, uint8(c.D[0])); e != nil {
						return out, e
					}
					s.A[1].Address++
					if uint8(c.D[0]) == 0 {
						break
					}
				}
				c.Word(7, uint16(c.D[7])+1)
			}
			s.PC = 0x1998a
		case 0x199d8:
			done, e := library(-90, 0x199dc, 0, 0)
			if e != nil || !done {
				return out, e
			}
		case 0x199dc:
			if e := code.Write16(0x443e, uint16(c.D[7])); e != nil {
				return out, e
			}
			c.Word(0, uint16(c.D[7]))
			if e := restoreCursor(); e != nil {
				return out, e
			}
			finish()
			return out, nil
		case 0x19afc, 0x19c1c:
			if e := saveCursor(); e != nil {
				return out, e
			}
			prepareLibrary()
			c.D[1] = s.A[0].Address
			c.D[2] = 1005
			if routine == 0x19afc {
				s.PC = 0x19b2e
			} else {
				s.PC = 0x19c4e
			}
		case 0x19b2e, 0x19c4e:
			next := 0x19b36
			if routine == 0x19c1c {
				next = 0x19c56
			}
			done, e := library(-30, next, 0xfe, 0x7f)
			if e != nil || !done {
				return out, e
			}
		case 0x19b36:
			c.D[7] = c.D[0]
			s.Handle = c.D[7]
			if int32(c.D[7]) <= 0 {
				s.PC = 0x19b88
			} else {
				prepareLibrary()
				c.D[1] = c.D[7]
				s.PC = 0x19b46
			}
		case 0x19b46:
			done, e := library(-36, 0x19b54, 0xfe, 0x7f)
			if e != nil || !done {
				return out, e
			}
		case 0x19b54:
			if e := restoreCursor(); e != nil {
				return out, e
			}
			s.PC = 0x19b62
		case 0x19b62:
			done, e := child(0x341e, 0x19b68)
			if e != nil || !done {
				return out, e
			}
		case 0x19b68:
			if e := saveCursor(); e != nil {
				return out, e
			}
			if uint16(c.D[0]) == 0 {
				c.D[0] = 0xffffffff
				s.PC = 0x19c0c
			} else {
				s.PC = 0x19b88
			}
		case 0x19b88:
			prepareLibrary()
			c.D[1] = s.A[0].Address
			c.D[2] = 1006
			s.PC = 0x19ba4
		case 0x19ba4:
			done, e := library(-30, 0x19bac, 0xfe, 0x7f)
			if e != nil || !done {
				return out, e
			}
		case 0x19bac, 0x19c56:
			c.D[7] = c.D[0]
			s.Handle = c.D[7]
			if int32(c.D[7]) <= 0 {
				if routine == 0x19afc {
					s.PC = 0x19c04
				} else {
					s.PC = 0x19cae
				}
			} else {
				prepareLibrary()
				c.D[1] = c.D[7]
				c.D[2] = baddr(NativeGAMStart).Address
				c.D[3] = NativeGAMSize
				if routine == 0x19afc {
					s.PC = 0x19bd6
				} else {
					s.PC = 0x19c80
				}
			}
		case 0x19bd6, 0x19c80:
			vector, next := -48, 0x19bde
			if routine == 0x19c1c {
				vector, next = -42, 0x19c88
			}
			done, e := library(vector, next, 0xfe, 0x7f)
			if e != nil || !done {
				return out, e
			}
		case 0x19bde, 0x19c88:
			s.Transferred = c.D[0]
			prepareLibrary()
			c.D[1] = c.D[7]
			if routine == 0x19afc {
				s.PC = 0x19bec
			} else {
				s.PC = 0x19c96
			}
		case 0x19bec, 0x19c96:
			next := 0x19bfa
			if routine == 0x19c1c {
				next = 0x19ca4
			}
			done, e := library(-36, next, 0xfe, 0x7f)
			if e != nil || !done {
				return out, e
			}
		case 0x19bfa, 0x19ca4:
			if s.Transferred == NativeGAMSize {
				c.Word(0, 1)
				if routine == 0x19afc {
					s.PC = 0x19c0c
				} else {
					s.PC = 0x19cb6
				}
			} else if routine == 0x19afc {
				s.PC = 0x19c04
			} else {
				s.PC = 0x19cae
			}
		case 0x19c04:
			c.D[0] = 0
			s.PC = 0x19c0c
		case 0x19cae:
			c.Word(0, 0)
			s.PC = 0x19cb6
		case 0x19c0c:
			if e := restoreCursor(); e != nil {
				return out, e
			}
			finish()
			return out, nil
		case 0x19cb6:
			if e := restoreCursor(); e != nil {
				return out, e
			}
			s.PC = 0x19cc2
		case 0x19cc2:
			if !s.ChildActive {
				s.ChildSaved0 = c.D[0]
			}
			done, e := child(0x1a32a, 0x19cca)
			if e != nil || !done {
				return out, e
			}
			c.D[0] = s.ChildSaved0
		case 0x19cca:
			finish()
			return out, nil
		default:
			return out, fmt.Errorf("native DOS PC%x unavailable", s.PC)
		}
	}
	return out, fmt.Errorf("native DOS body exceeded source transition budget")
}
