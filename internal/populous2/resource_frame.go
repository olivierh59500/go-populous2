package populous2

import (
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeResourceFrameRules struct{ Resources []Resource }

func DecodeNativeResourceFrameRules(exe *amiga.Executable) (NativeResourceFrameRules, error) {
	resources, err := ResourceTable(exe)
	return NativeResourceFrameRules{Resources: resources}, err
}

type NativeResourceFrameIOCall struct {
	Operation                  string
	Name                       string
	Handle, Destination, Limit uint32
	Frame                      *NativeFrameRegisterContext
}
type NativeResourceFrameIOResult struct {
	Complete bool
	Value    int32
	Data     []byte
}

type NativeResourceFrameCallbacks struct {
	RAM      FollowerCleanupMemory // Absolute relocated physical addresses, all actual HUNK spans.
	CodeBase uint32
	Frame    *NativeFrameRegisterContext
	Input    *NativeInputState // Shared mutable cursor selector, never a new per-call copy.
	IO       func(NativeResourceFrameIOCall, *uint32) (NativeResourceFrameIOResult, error)
	Call     func(NativeFileFrameCall, *uint32) (NativeCommandFrameResult, error)
}

type NativeResourceFrameStep struct {
	Complete, Waiting bool
	PC                int
	IO                string
	Calls             []int
}

type NativeResourceFrameState struct {
	Started, Finished bool
	PC                int
	Registers         [8]uint32
	Saved             [7]uint32
	Index             uint16
	Descriptor        uint32
	Handle            uint32
	IOActive          bool
	IOPhase           uint32
	ioCall            NativeResourceFrameIOCall
	ioSaved           [7]uint32
	ChildActive       bool
	ChildPhase        uint32
	ChildRoutine      int
	read              []byte
	failed            error
}

func (s *NativeResourceFrameState) Advance(r *NativeResourceFrameRules, cb NativeResourceFrameCallbacks) (out NativeResourceFrameStep, failure error) {
	if s == nil || r == nil || cb.Frame == nil || cb.Input == nil || !winMemoryValid(cb.RAM) {
		return out, fmt.Errorf("native resource physical backing/frame missing")
	}
	if s.failed != nil {
		return out, s.failed
	}
	if !s.Started {
		s.Started, s.PC, s.Registers = true, 0x19cd0, cb.Frame.D
		copy(s.Saved[:], cb.Frame.D[1:])
		s.Index = uint16(cb.Frame.D[0])
	}
	cb.Frame.D = s.Registers
	defer func() {
		s.Registers, out.PC = cb.Frame.D, s.PC
		if failure != nil {
			s.failed = failure
		}
	}()
	if s.Finished {
		out.Complete = true
		return out, nil
	}
	c, m := cb.Frame, cb.RAM
	bs, cs := int(c.AddressBase), int(cb.CodeBase)
	word := func(at int) (uint16, error) { return m.Read16(at) }
	cursor := func(value uint16) error { cb.Input.Mouse.Image = value; return m.Write16(cs+0xa2a, value) }
	io := func(operation string) (NativeResourceFrameIOResult, error) {
		if cb.IO == nil {
			return NativeResourceFrameIOResult{}, fmt.Errorf("native resource %s I/O missing", operation)
		}
		if !s.IOActive {
			s.IOActive = true
			copy(s.ioSaved[:], c.D[1:])
			s.ioCall = NativeResourceFrameIOCall{Operation: operation, Handle: s.Handle, Frame: c}
			if operation == "open" {
				name := []byte{}
				for at := int(s.Descriptor) + 18; ; at++ {
					v, err := m.Read8(at)
					if err != nil {
						return NativeResourceFrameIOResult{}, err
					}
					if v == 0 {
						break
					}
					name = append(name, v)
				}
				s.ioCall.Name = string(name)
				c.D[1] = s.Descriptor + 18
				c.D[2] = 1005
			} else if operation == "read" {
				destination, err := m.Read32(int(s.Descriptor))
				if err != nil {
					return NativeResourceFrameIOResult{}, err
				}
				limit, err := m.Read32(int(s.Descriptor) + 8)
				if err != nil {
					return NativeResourceFrameIOResult{}, err
				}
				s.ioCall.Destination, s.ioCall.Limit = destination, limit
				c.D[1], c.D[2], c.D[3] = s.Handle, destination, limit
			} else {
				c.D[1] = s.Handle
			}
		}
		if s.ioCall.Operation != operation {
			return NativeResourceFrameIOResult{}, fmt.Errorf("native resource I/O changed while waiting")
		}
		s.ioCall.Frame = c
		result, err := cb.IO(s.ioCall, &s.IOPhase)
		if err != nil {
			return result, err
		}
		if !result.Complete {
			out.Waiting, out.IO = true, operation
			return result, nil
		}
		copy(c.D[1:], s.ioSaved[:])
		c.D[0] = uint32(result.Value)
		s.IOActive, s.IOPhase = false, 0
		return result, nil
	}
	child := func(routine int) (bool, error) {
		if cb.Call == nil {
			return false, fmt.Errorf("native resource failure requester %#x missing", routine)
		}
		if !s.ChildActive {
			s.ChildActive, s.ChildRoutine = true, routine
			out.Calls = append(out.Calls, routine)
		}
		if s.ChildRoutine != routine {
			return false, fmt.Errorf("native resource requester changed while waiting")
		}
		call := NativeFileFrameCall{Routine: routine, Frame: c}
		if routine == 0x33b2 {
			call.Arguments = 6
			call.A[1] = NativeRequesterAddress{Address: cb.CodeBase + 0x91d0, Code: true}
			call.A[2] = NativeRequesterAddress{Address: cb.CodeBase + 0xa93c, Code: true}
		}
		result, err := cb.Call(call, &s.ChildPhase)
		if err != nil {
			return false, err
		}
		if !result.Complete {
			out.Waiting = true
			return false, nil
		}
		s.ChildActive, s.ChildPhase = false, 0
		return true, nil
	}
	for transitions := 0; transitions < 64; transitions++ {
		switch s.PC {
		case 0x19cd0:
			image, err := word(cs + 0xa2a)
			if err != nil {
				return out, err
			}
			if err := m.Write16(bs+0x3ba, image); err != nil {
				return out, err
			}
			if err := cursor(0x120); err != nil {
				return out, err
			}
			flags, err := m.Read32(bs + 0x3ac)
			if err != nil {
				return out, err
			}
			c.D[1] = flags
			if flags&(1<<uint(c.D[0]&31)) != 0 {
				c.D[0] = 2
				s.PC = 0x19e32
				continue
			}
			c.D[0] = uint32(uint16(c.D[0])) * 48
			s.Descriptor = cb.CodeBase + 0x19e4a + uint32(int32(int16(c.D[0])))
			s.PC = 0x19d00
		case 0x19d00:
			result, err := io("open")
			if err != nil || !result.Complete {
				return out, err
			}
			c.D[7] = c.D[0]
			if int32(c.D[7]) <= 0 {
				value, err := word(bs + 0x3ba)
				if err != nil {
					return out, err
				}
				if err := cursor(value); err != nil {
					return out, err
				}
				s.PC = 0x19d38
			} else {
				s.Handle = c.D[7]
				s.PC = 0x19d54
			}
		case 0x19d38:
			done, err := child(0x339e)
			if err != nil || !done {
				return out, err
			}
			s.PC = 0x19db4
		case 0x19d54:
			if !s.IOActive {
				limit, err := m.Read32(int(s.Descriptor) + 8)
				if err != nil {
					return out, err
				}
				c.D[1] = limit
			}
			result, err := io("read")
			if err != nil || !result.Complete {
				return out, err
			}
			if result.Value > 0 {
				if uint32(result.Value) > s.ioCall.Limit || len(result.Data) != int(result.Value) {
					return out, fmt.Errorf("native resource read count/payload differs")
				}
				s.read = append(s.read[:0], result.Data...)
				for i, v := range result.Data {
					if err := m.Write8(int(s.ioCall.Destination)+i, v); err != nil {
						return out, err
					}
				}
				if err := m.Write32(cs+0x19e46, c.D[0]); err != nil {
					return out, err
				}
				s.PC = 0x19dd0
			} else {
				s.PC = 0x19d7e
			}
		case 0x19d7e, 0x19dd0:
			failedRead := s.PC == 0x19d7e
			result, err := io("close")
			if err != nil || !result.Complete {
				return out, err
			}
			if failedRead {
				value, err := word(bs + 0x3ba)
				if err != nil {
					return out, err
				}
				if err := cursor(value); err != nil {
					return out, err
				}
				s.PC = 0x19dae
			} else {
				s.PC = 0x19dea
			}
		case 0x19dae:
			done, err := child(0x33b2)
			if err != nil || !done {
				return out, err
			}
			s.PC = 0x19db4
		case 0x19db4:
			value, err := word(cs + 0xa2a)
			if err != nil {
				return out, err
			}
			if err := m.Write16(bs+0x3ba, value); err != nil {
				return out, err
			}
			if err := cursor(0x120); err != nil {
				return out, err
			}
			s.PC = 0x19d00
		case 0x19dea:
			mask, err := m.Read32(int(s.Descriptor) + 4)
			if err != nil {
				return out, err
			}
			flags, err := m.Read32(bs + 0x3ac)
			if err != nil {
				return out, err
			}
			c.D[1] = flags &^ mask
			c.Word(0, s.Index)
			c.D[1] |= 1 << uint(c.D[0]&31)
			if err := m.Write32(bs+0x3ac, c.D[1]); err != nil {
				return out, err
			}
			packed, err := word(int(s.Descriptor) + 16)
			if err != nil {
				return out, err
			}
			destination, err := m.Read32(int(s.Descriptor))
			if err != nil {
				return out, err
			}
			if packed != 0 {
				decoded, err := DecodePacked(s.read)
				if err != nil {
					return out, err
				}
				for i, v := range decoded {
					if err := m.Write8(int(destination)+i, v); err != nil {
						return out, err
					}
				}
			}
			planar, err := m.Read32(int(s.Descriptor) + 12)
			if err != nil {
				return out, err
			}
			c.D[0] = planar
			if planar != 0 {
				if err := prepareNativeResourceFramePlanes(m, planar, uint32(bs+0x3be), c); err != nil {
					return out, err
				}
			}
			c.D[0] = 1
			s.PC = 0x19e32
		case 0x19e32:
			value, err := word(bs + 0x3ba)
			if err != nil {
				return out, err
			}
			if err := cursor(value); err != nil {
				return out, err
			}
			copy(c.D[1:], s.Saved[:])
			s.Finished, out.Complete, s.PC = true, true, 0x19e44
			return out, nil
		default:
			return out, fmt.Errorf("native resource source branch %#x missing", s.PC)
		}
	}
	return out, fmt.Errorf("native resource retries did not yield a real host boundary")
}

// PrepareNativeResourceFramePlanes executes$1069c against original physical
// descriptor/data pointers and retains its actual BSS scratch writes. It does
// not replace the mutable source bank with a separately prepared image.
func PrepareNativeResourceFramePlanes(m FollowerCleanupMemory, table, scratch uint32) error {
	return prepareNativeResourceFramePlanes(m, table, scratch, nil)
}

// PrepareNativeResourceFramePlanesWithRegisters also retains the direct
// caller's complete data/address ABI. A1-A6 remain unchanged when the initial
// table entry is already the negative sentinel; no address zeros are invented.
func PrepareNativeResourceFramePlanesWithRegisters(m FollowerCleanupMemory, table, scratch uint32, c *NativeFrameRegisterContext, a *[7]uint32) error {
	if c == nil || a == nil {
		return fmt.Errorf("native planar caller registers missing")
	}
	return prepareNativeResourceFramePlanes(m, table, scratch, c, a)
}

func prepareNativeResourceFramePlanes(m FollowerCleanupMemory, table, scratch uint32, c *NativeFrameRegisterContext, addresses ...*[7]uint32) error {
	var a *[7]uint32
	if len(addresses) != 0 {
		a = addresses[0]
	}
	for {
		pointer, err := m.Read32(int(table))
		if err != nil {
			return err
		}
		table += 4
		if a != nil {
			a[0] = table
		}
		if c != nil {
			c.D[7] = pointer
		}
		if int32(pointer) < 0 {
			return nil
		}
		half, err := m.Read16(int(table))
		if err != nil {
			return err
		}
		height, err := m.Read16(int(table) + 2)
		if err != nil {
			return err
		}
		table += 8
		plane := uint32(half>>2) * uint32(height)
		words := uint16(plane) >> 1
		if c != nil {
			c.D[6] = plane
			c.Word(4, words-1)
			c.Word(3, uint16(c.D[4]))
		}
		if words == 0 {
			return fmt.Errorf("native planar zero-sized descriptor requires65536 iterations")
		}
		if a != nil {
			stride := uint32(int32(int16(uint16(plane))))
			a[1] = scratch
			a[2] = scratch + stride
			a[3] = scratch + stride*2
			a[4] = scratch + stride*3
			a[5] = scratch + stride*4
			a[6] = pointer
		}
		for i := uint16(0); i < words; i++ {
			for channel := 0; channel < 5; channel++ {
				value, err := m.Read16(int(pointer) + int(i)*10 + channel*2)
				if err != nil {
					return err
				}
				if channel == 0 {
					value = ^value
					if c != nil {
						c.Word(0, value)
					}
				}
				address := scratch + uint32(int32(int16(uint16(plane)))*int32(channel)) + uint32(i)*2
				if err := m.Write16(int(address), value); err != nil {
					return err
				}
				if a != nil {
					a[1+channel] += 2
					a[6] += 2
				}
			}
			if c != nil {
				c.Word(4, uint16(c.D[4])-1)
			}
		}
		if a != nil {
			a[1], a[2] = scratch, pointer
		}
		for i := 0; i < int(words)*5; i++ {
			value, err := m.Read16(int(scratch) + i*2)
			if err != nil {
				return err
			}
			if err := m.Write16(int(pointer)+i*2, value); err != nil {
				return err
			}
			if a != nil {
				a[1] += 2
				a[2] += 2
			}
			if c != nil && i%5 == 4 {
				c.Word(3, uint16(c.D[3])-1)
			}
		}
	}
}
