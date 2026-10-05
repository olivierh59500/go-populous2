package populous2

import "fmt"

// NativeTransportHandshakeState retains the complete $17eec controller.
// Only D0-D2 belong to its outer MOVEM; genuine requester/reset outputs in
// D3-D7 remain visible to the serial requester after the handshake returns.
type NativeTransportHandshakeState struct {
	Started, Finished             bool
	PC                            int
	Registers, Saved, DialogSaved [8]uint32
	Transfer                      NativeTransportIOState
	TransferTarget                NativeRequesterAddress
	Data                          []byte
	Deadline                      uint32
	Target                        int
	Zero, Negative, FlagsKnown    bool
	ChildActive                   bool
	ChildRoutine                  int
	ChildPhase                    uint32
	DialogPhase                   uint8
	DialogFlag                    uint16
	DialogMessage                 uint32
	Failure                       error
	failed                        error
}

func (s *NativeTransportHandshakeState) Advance(cb NativeTransportFrameCallbacks) (out NativeTransportFrameStep, failure error) {
	if s == nil || cb.Frame == nil || cb.Presentation == nil || cb.Bitmap == nil || !winMemoryValid(cb.Code) || !winMemoryValid(cb.Memory) || !validNativeSerialPort(cb.Port) {
		return out, fmt.Errorf("native handshake backing/port missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		s.Started = true
		s.PC = 0x17eec
		s.Registers = cb.Frame.D
		s.Saved = cb.Frame.D
	}
	cb.Frame.D = s.Registers
	defer func() {
		s.Registers = cb.Frame.D
		out.PC = s.PC
		out.Failure = s.Failure
		out.Zero, out.Negative, out.FlagsKnown = s.Zero, s.Negative, s.FlagsKnown && out.Complete
		if failure != nil {
			s.failed = failure
		}
	}()
	if s.Finished {
		out.Complete = true
		return out, nil
	}
	c, code, m := cb.Frame, cb.Code, cb.Memory
	b := nativeRequesterFrameBacking{Code: code, Memory: m, CodeBase: cb.CodeBase, Frame: c, Bitmap: cb.Bitmap, Sound: cb.Sound}
	caddr := func(at int) NativeRequesterAddress {
		return NativeRequesterAddress{Address: cb.CodeBase + uint32(at), Code: true}
	}
	baddr := func(at int) NativeRequesterAddress {
		return NativeRequesterAddress{Address: c.AddressBase + uint32(at)}
	}
	compile := func(parameter uint32) error {
		c.Word(3, 1)
		return b.compile(cb.CodeBase+0x91d0, []NativeRequesterAddress{{Address: parameter, Code: true}})
	}
	draw := func() error {
		target, e := m.Read32(0x1e)
		if e != nil {
			return e
		}
		start, e := code.Read16(0xab4e)
		if e != nil {
			return e
		}
		col, e := code.Read16(0xab50)
		if e != nil {
			return e
		}
		row, e := code.Read16(0xab52)
		if e != nil {
			return e
		}
		c.Word(0, col)
		c.Word(1, row)
		if e := b.text(target, 0xab4e+int(int16(start))); e != nil {
			return e
		}
		return fileFrameSwap(b, cb.Presentation)
	}
	prepare := func(target NativeRequesterAddress, count int, receive bool) error {
		s.TransferTarget = target
		s.Data = make([]byte, count)
		if !receive {
			p := target
			for i := range s.Data {
				v, e := b.read(p)
				if e != nil {
					return e
				}
				s.Data[i] = v
				p.Address++
			}
		}
		s.Transfer = NativeTransportIOState{}
		return nil
	}
	transfer := func(routine int) (NativeSerialFrameChildResult, error) {
		before := s.Transfer.Position
		result, e := s.Transfer.Advance(routine, s.Data, cb)
		if e != nil {
			return result, e
		}
		if routine == 0xbbe {
			p := s.TransferTarget
			p.Address += uint32(before)
			for i := before; i < s.Transfer.Position; i++ {
				at := int(int64(p.Address) - int64(c.AddressBase))
				memory := m
				if p.Code {
					memory = code
					at = int(int64(p.Address) - int64(cb.CodeBase))
				}
				if e := memory.Write8(at, s.Data[i]); e != nil {
					return result, e
				}
				p.Address++
			}
		}
		if s.Transfer.Failure != nil {
			s.Failure = s.Transfer.Failure
		}
		return result, nil
	}
	poll := func() (bool, error) {
		if _, e := b.click(); e != nil {
			return false, e
		}
		return uint16(c.D[0]) != 0, nil
	}
	child := func(routine, next int) (bool, error) {
		if cb.Call == nil && cb.CallTransport == nil {
			return false, fmt.Errorf("native handshake child%x missing", routine)
		}
		if !s.ChildActive {
			s.ChildActive = true
			s.ChildRoutine = routine
		}
		if s.ChildRoutine != routine {
			return false, fmt.Errorf("native handshake child changed across wait")
		}
		call := NativeFileFrameCall{Routine: routine, Frame: c}
		if routine == 0x102e4 {
			first, second := 0x3361a, 0x33844
			if s.DialogPhase == 3 {
				first, second = second, first
			}
			call.Arguments = 1<<2 | 1<<3
			call.A[2], call.A[3] = caddr(first), caddr(second)
		}
		var result NativeSerialFrameChildResult
		var e error
		if cb.CallTransport != nil {
			result, e = cb.CallTransport(call, &s.ChildPhase)
			if result.Complete {
				s.Zero, s.Negative, s.FlagsKnown = result.Zero, result.Negative, true
			}
		} else {
			legacy, err := cb.Call(call, &s.ChildPhase)
			result.Complete, result.Zero, e = legacy.Complete, legacy.Zero, err
			s.FlagsKnown = false
		}
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
	for transitions := 0; transitions < 2048; transitions++ {
		switch s.PC {
		case 0x17eec:
			if cb.Port.Configure == nil {
				return out, fmt.Errorf("native handshake baud configuration missing")
			}
			baud, e := m.Read16(0x15a)
			if e != nil {
				return out, e
			}
			if e := cb.Port.Configure(baud); e != nil {
				return out, e
			}
			if e := m.Write16(0x3a8, 0); e != nil {
				return out, e
			}
			if e := code.Write16(0x182ca, 0xfff6); e != nil {
				return out, e
			}
			if e := code.Write8(0x182c1, 0); e != nil {
				return out, e
			}
			if e := code.Write32(0x182c6, 0); e != nil {
				return out, e
			}
			s.PC = 0x17f18
		case 0x17f18:
			if e := code.Write16(0x181be, 0); e != nil {
				return out, e
			}
			if e := cb.Port.Flush(); e != nil {
				return out, e
			}
			if e := transportFrameSyncReceive(cb); e != nil {
				return out, e
			}
			s.PC = 0x17f24
		case 0x17f24:
			attempt, e := code.Read16(0x181be)
			if e != nil {
				return out, e
			}
			attempt++
			if e := code.Write16(0x181be, attempt); e != nil {
				return out, e
			}
			c.Word(0, attempt)
			c.D[0] = uint32(int32(int16(c.D[0])))
			at := 0xa9ca
			if int32(c.D[0]) >= 0 {
				digits := fmt.Sprintf("%d", int32(c.D[0]))
				for _, v := range []byte(digits) {
					if e := code.Write8(at, v); e != nil {
						return out, e
					}
					at++
				}
			}
			if e := code.Write8(at, 0); e != nil {
				return out, e
			}
			parameter, e := code.Read32(0x181ba)
			if e != nil {
				return out, e
			}
			if e := compile(parameter); e != nil {
				return out, e
			}
			if e := draw(); e != nil {
				return out, e
			}
			cancel, e := poll()
			if e != nil {
				return out, e
			}
			if cancel {
				s.Zero, s.Negative, s.FlagsKnown = false, int16(c.D[0]) < 0, true
				s.PC = 0x181ac
				continue
			}
			c.D[0] = 6
			counter, e := m.Read32(0x12)
			if e != nil {
				return out, e
			}
			c.D[0] += counter
			s.Deadline = c.D[0]
			s.PC = 0x17f74
		case 0x17f74:
			counter, e := m.Read32(0x12)
			if e != nil {
				return out, e
			}
			if int32(s.Deadline) >= int32(counter) {
				out.Waiting = true
				return out, nil
			}
			c.D[0] = 1
			if e := prepare(caddr(0x182c0), 1, false); e != nil {
				return out, e
			}
			s.PC = 0x17f7e
		case 0x17f7e, 0x17fdc, 0x18036, 0x1808c, 0x1809c, 0x180e8:
			result, e := transfer(0xc2a)
			if e != nil {
				return out, e
			}
			if !result.Complete {
				out.Waiting = true
				return out, nil
			}
			switch s.PC {
			case 0x17f7e:
				s.PC = 0x17f84
			case 0x17fdc:
				s.PC = 0x17fe2
			case 0x18036:
				s.PC = 0x1803c
			case 0x1808c:
				c.Word(0, 236)
				if e := prepare(baddr(0xe8f2), 236, false); e != nil {
					return out, e
				}
				s.PC = 0x1809c
			case 0x1809c, 0x180e8:
				s.PC = 0x180ee
			}
		case 0x17f84:
			result, e := transportFrameAvailable(cb)
			if result.Failure != nil {
				s.Failure = result.Failure
			}
			if e != nil {
				return out, e
			}
			if !result.Complete {
				out.Waiting = true
				return out, nil
			}
			if result.Negative {
				s.Zero, s.Negative, s.FlagsKnown = result.Zero, result.Negative, true
				s.PC = 0x181ac
			} else if result.Zero {
				s.PC = 0x17f24
			} else {
				c.D[0] = 1
				if e := prepare(caddr(0x182c1), 1, true); e != nil {
					return out, e
				}
				s.PC = 0x17f98
			}
		case 0x17f98:
			result, e := transfer(0xbbe)
			if e != nil {
				return out, e
			}
			if !result.Complete {
				out.Waiting = true
				return out, nil
			}
			if !result.Zero {
				s.Zero, s.Negative, s.FlagsKnown = result.Zero, result.Negative, true
				s.PC = 0x181ac
				continue
			}
			received, e := code.Read8(0x182c1)
			if e != nil {
				return out, e
			}
			c.Byte(0, received)
			token, e := code.Read8(0x182c0)
			if e != nil {
				return out, e
			}
			if received != token {
				s.PC = 0x17f24
			} else {
				s.PC = 0x17fb2
			}
		case 0x17fb2, 0x17fc6:
			if cb.WaitCPU == nil {
				return out, fmt.Errorf("native handshake CPU loop continuation missing")
			}
			ready, e := cb.WaitCPU(uint32(s.PC), 100000)
			if e != nil {
				return out, e
			}
			if !ready {
				out.Waiting = true
				return out, nil
			}
			c.D[0] = 0
			if s.PC == 0x17fb2 {
				s.PC = 0x17fc0
			} else {
				c.D[0] = 4
				if e := prepare(caddr(0x182c2), 4, false); e != nil {
					return out, e
				}
				s.PC = 0x17fdc
			}
		case 0x17fc0:
			if e := cb.Port.Flush(); e != nil {
				return out, e
			}
			if e := transportFrameSyncReceive(cb); e != nil {
				return out, e
			}
			s.PC = 0x17fc6
		case 0x17fe2, 0x1803c, 0x180aa, 0x18108:
			saved := c.D[1]
			cancel, e := poll()
			if s.PC == 0x18108 {
				c.D[1] = saved
			}
			if e != nil {
				return out, e
			}
			if cancel {
				s.Zero, s.Negative, s.FlagsKnown = false, int16(c.D[0]) < 0, true
				s.PC = 0x181ac
				continue
			}
			next := 0x17fee
			if s.PC == 0x1803c {
				next = 0x18048
			} else if s.PC == 0x180aa {
				next = 0x180ba
			} else if s.PC == 0x18108 {
				next = 0x1811c
			}
			s.PC = next
		case 0x17fee, 0x18048, 0x180ba, 0x1811c:
			result, e := transportFrameAvailable(cb)
			if result.Failure != nil {
				s.Failure = result.Failure
			}
			if e != nil {
				return out, e
			}
			if !result.Complete {
				out.Waiting = true
				return out, nil
			}
			if result.Negative {
				s.Zero, s.Negative, s.FlagsKnown = result.Zero, result.Negative, true
				s.PC = 0x181ac
				continue
			}
			switch s.PC {
			case 0x17fee:
				if int16(c.D[0]) < 4 {
					s.PC = 0x17fe2
					out.Waiting = true
					return out, nil
				}
				c.D[0] = 4
				if e := prepare(caddr(0x182c6), 4, true); e != nil {
					return out, e
				}
				s.PC = 0x18006
			case 0x18048:
				if int16(c.D[0]) < 1 {
					s.PC = 0x1803c
					out.Waiting = true
					return out, nil
				}
				c.D[0] = 1
				if e := prepare(caddr(0x182c1), 1, true); e != nil {
					return out, e
				}
				s.PC = 0x18060
			case 0x180ba, 0x1811c:
				if result.Zero {
					if s.PC == 0x180ba {
						s.PC = 0x180aa
					} else {
						s.PC = 0x18108
					}
					out.Waiting = true
					return out, nil
				}
				c.D[0] = 1
				if e := prepare(baddr(s.Target), 1, true); e != nil {
					return out, e
				}
				if s.PC == 0x180ba {
					s.PC = 0x180c8
				} else {
					s.PC = 0x1812a
				}
			}
		case 0x18006, 0x18060, 0x180c8, 0x1812a:
			result, e := transfer(0xbbe)
			if e != nil {
				return out, e
			}
			if !result.Complete {
				out.Waiting = true
				return out, nil
			}
			if !result.Zero {
				s.Zero, s.Negative, s.FlagsKnown = result.Zero, result.Negative, true
				s.PC = 0x181ac
				continue
			}
			switch s.PC {
			case 0x18006:
				signature, e := code.Read32(0x182c6)
				if e != nil {
					return out, e
				}
				c.D[0] = signature
				expected, e := code.Read32(0x182c2)
				if e != nil {
					return out, e
				}
				if signature != expected {
					budget, e := code.Read16(0x182ca)
					if e != nil {
						return out, e
					}
					budget++
					if e := code.Write16(0x182ca, budget); e != nil {
						return out, e
					}
					if budget == 0 {
						s.PC = 0x18186
					} else {
						s.PC = 0x17f18
					}
				} else {
					c.D[0] = 1
					if e := prepare(baddr(0xeb43), 1, false); e != nil {
						return out, e
					}
					s.PC = 0x18036
				}
			case 0x18060:
				profile, e := m.Read8(0xeb43)
				if e != nil {
					return out, e
				}
				c.Byte(1, profile)
				peer, e := code.Read8(0x182c1)
				if e != nil {
					return out, e
				}
				if profile == peer {
					s.PC = 0x1819a
				} else if profile == 2 {
					s.Target = 0xeb22
					s.PC = 0x180aa
				} else {
					c.D[0] = 14
					if e := prepare(baddr(0xeb22), 14, false); e != nil {
						return out, e
					}
					s.PC = 0x1808c
				}
			case 0x180c8:
				s.Target += int(int16(c.D[0]))
				if s.Target < 0xeb30 {
					s.PC = 0x180aa
				} else {
					c.Word(0, 236)
					if e := prepare(baddr(0xea2c), 236, false); e != nil {
						return out, e
					}
					s.PC = 0x180e8
				}
			case 0x1812a:
				s.Target += int(int16(c.D[0]))
				c.Word(1, uint16(c.D[1])-1)
				if uint16(c.D[1]) != 0xffff {
					s.PC = 0x18108
				} else {
					s.PC = 0x1813a
				}
			}
		case 0x180ee:
			profile, e := m.Read16(0xeb42)
			if e != nil {
				return out, e
			}
			s.Target = 0xe8f2
			if profile == 1 {
				s.Target = 0xea2c
			}
			c.Word(1, 235)
			s.PC = 0x18108
		case 0x1813a:
			first, second := uint8(6), uint8(8)
			profile, e := m.Read16(0xeb42)
			if e != nil {
				return out, e
			}
			if profile != 1 {
				first, second = second, first
			}
			if e := m.Write8(0xeb5e, first); e != nil {
				return out, e
			}
			if e := m.Write8(0xeb68, second); e != nil {
				return out, e
			}
			if e := m.Write16(0x3a8, 1); e != nil {
				return out, e
			}
			if e := m.Write16(0xeb44, 6); e != nil {
				return out, e
			}
			seed, e := m.Read32(0xeb24)
			if e != nil {
				return out, e
			}
			if e := m.Write32(0xeb28, seed); e != nil {
				return out, e
			}
			s.PC = 0x1817e
		case 0x1817e:
			done, e := child(0x10ad8, 0x181ac)
			if e != nil || !done {
				return out, e
			}
		case 0x18186, 0x1819a:
			at := 0x181b2
			if s.PC == 0x1819a {
				at = 0x181b6
			}
			message, e := code.Read32(at)
			if e != nil {
				return out, e
			}
			s.DialogMessage = message
			s.DialogSaved = c.D
			s.DialogPhase = 0
			s.PC = 0x33b2
		case 0x33b2:
			switch s.DialogPhase {
			case 0:
				c.D[3] = 1
				if e := compile(s.DialogMessage); e != nil {
					return out, e
				}
				front, e := m.Read32(0x1a)
				if e != nil {
					return out, e
				}
				back, e := m.Read32(0x1e)
				if e != nil {
					return out, e
				}
				source, e := cb.Bitmap(front)
				if e != nil {
					return out, e
				}
				target, e := cb.Bitmap(back)
				if e != nil {
					return out, e
				}
				if len(source) < 32000 || len(target) < 32000 {
					return out, fmt.Errorf("native handshake dialog screen unavailable")
				}
				for at := 0; at < 32000; at += 32 {
					var block [32]byte
					copy(block[:], source[at:at+32])
					copy(target[at:at+32], block[:])
				}
				flag, e := m.Read16(0x3b0)
				if e != nil {
					return out, e
				}
				s.DialogFlag = flag
				s.DialogPhase = 1
			case 1:
				if s.DialogFlag == 0 {
					done, e := child(0x102e4, 0x33b2)
					if e != nil || !done {
						return out, e
					}
				}
				s.DialogPhase = 2
			case 2:
				if e := draw(); e != nil {
					return out, e
				}
				if _, e := b.click(); e != nil {
					return out, e
				}
				offset, e := code.Read16(0x341a + int(int16(c.D[0])))
				if e != nil {
					return out, e
				}
				c.Word(0, offset)
				target := 0x341a + int(int16(offset))
				if target == 0x33ea {
					out.Waiting = true
					return out, nil
				}
				if target != 0x33fe {
					return out, fmt.Errorf("native handshake error dialog dispatch invalid")
				}
				s.DialogPhase = 3
			case 3:
				if s.DialogFlag == 0 {
					done, e := child(0x102e4, 0x33b2)
					if e != nil || !done {
						return out, e
					}
				}
				c.D = s.DialogSaved
				if s.DialogFlag != 0 {
					s.Zero, s.Negative, s.FlagsKnown = false, int16(s.DialogFlag) < 0, true
				}
				s.PC = 0x181ac
			}
		case 0x181ac:
			for i := 0; i < 3; i++ {
				c.D[i] = s.Saved[i]
			}
			s.Finished = true
			out.Complete = true
			return out, nil
		default:
			return out, fmt.Errorf("native handshake branch unsupported")
		}
	}
	out.Waiting = true
	return out, nil
}
