package populous2

import (
	"encoding/binary"
	"fmt"
	"go-populous2/internal/amiga"
)

type NativeFollowerNeutralFrameRules struct{ code []byte }

func DecodeNativeFollowerNeutralFrameRules(exe *amiga.Executable) (NativeFollowerNeutralFrameRules, error) {
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33612 {
		return NativeFollowerNeutralFrameRules{}, fmt.Errorf("native neutral follower frame CODE missing")
	}
	return NativeFollowerNeutralFrameRules{code: exe.Hunks[0].Data}, nil
}
func (r *NativeFollowerNeutralFrameRules) word(at int) (uint16, error) {
	if r == nil || at < 0 || at&1 != 0 || at+2 > len(r.code) {
		return 0, fmt.Errorf("native neutral follower CODE word unavailable")
	}
	return binary.BigEndian.Uint16(r.code[at:]), nil
}
func (r *NativeFollowerNeutralFrameRules) byte(at int) (uint8, error) {
	if r == nil || at < 0 || at >= len(r.code) {
		return 0, fmt.Errorf("native neutral follower CODE byte unavailable")
	}
	return r.code[at], nil
}

type NativeFollowerNeutralFrameCallbacks struct {
	Memory        FollowerCleanupMemory
	Frame         *NativeFrameRegisterContext
	Random        func() uint16
	Move, Cleanup func(NativeRecordReference, *NativeFrameRegisterContext) error
	Unlink        func(NativeRecordReference) error
	// Call executes the actual direct source leaf, retaining its full D output.
	Call func(uint32, *NativeFrameRegisterContext) error
}

// Tick translates state44/$12190 and all seven $121e4 subtype targets. BLT
// follows ADD.W's signed-overflow flags before the separate stored-word bounds
// comparisons. Raw subtype words and mixed linked target records stay intact.
func (r *NativeFollowerNeutralFrameRules) Tick(ref NativeRecordReference, cb NativeFollowerNeutralFrameCallbacks) (uint32, error) {
	if r == nil || cb.Frame == nil || !winMemoryValid(cb.Memory) || cb.Move == nil || cb.Unlink == nil || cb.Call == nil || cb.Random == nil || cb.Cleanup == nil {
		return 0, fmt.Errorf("native neutral follower frame callbacks missing")
	}
	m, c := nativeWhirlwindMemory{m: cb.Memory}, cb.Frame
	at := cleanupRecordAddress(ref)
	c.Word(0, m.word(at+10)+4)
	marker, e := r.word(0x23d1a + int(int16(uint16(c.D[0]))))
	if e != nil {
		return 0, e
	}
	if int16(marker) < 0 {
		c.Word(0, uint16(c.D[0])+marker)
	}
	m.putWord(at+10, uint16(c.D[0]))
	x, y, vx, vy := m.word(at+6), m.word(at+8), m.word(at+14), m.word(at+16)
	c.RestoreWord(6, x)
	c.RestoreWord(7, y)
	nx, ny := x+vx, y+vy
	c.Word(6, nx)
	outside := int32(int16(x))+int32(int16(vx)) < 0
	if !outside {
		c.Word(7, ny)
		outside = int32(int16(y))+int32(int16(vy)) < 0
	}
	if !outside {
		outside = int16(nx) >= 0x4000 || int16(ny) >= 0x4000
	}
	if outside {
		m.putByte(at+12, 0)
		if e := m.m.Write32(at+26, 0); e != nil {
			return 0, e
		}
		if m.err != nil {
			return 0, m.err
		}
		if e := cb.Unlink(ref); e != nil {
			return 0, e
		}
		return 0x12462, nil
	}
	if m.err != nil {
		return 0, m.err
	}
	if e := cb.Move(ref, c); e != nil {
		return 0, e
	}
	subtype := m.word(at + 40)
	c.Word(0, subtype)
	offset, e := r.word(0x121e4 + int(int16(subtype)))
	if e != nil {
		return 0, e
	}
	c.Word(0, offset)
	target := 0x121e4 + int(int16(offset))
	packed := func() uint16 { return m.word(at+8)&0xff00 | uint16(m.byte(at+6)) }
	call := func(routine uint32) error {
		if m.err != nil {
			return m.err
		}
		return cb.Call(routine, c)
	}
	switch target {
	case 0x12224:
	case 0x121f2:
		p := packed()
		c.Word(1, p&0xff00|uint16(uint8(p)*4))
		grid := 0xf44 + int(int16(uint16(c.D[1])))
		tile := m.byte(grid + 1)
		c.D[0] = uint32(tile)
		raster, e := r.byte(0x33512 + int(tile))
		if e != nil {
			return 0, e
		}
		c.Byte(0, raster)
		c.Word(0, uint16(c.D[0])&15)
		if uint16(c.D[0]) == 15 {
			m.putByte(grid+1, 0xae)
		}
	case 0x12228:
		c.D[0], c.D[1] = uint32(m.byte(at+6)), uint32(m.byte(at+8))
		if e := call(0xd7f0); e != nil {
			return 0, e
		}
		c.Byte(0, m.byte(at+6))
		c.Word(0, uint16(c.D[0])+1)
		if int16(uint16(c.D[0])) < 0x40 {
			c.Byte(1, m.byte(at+8))
			if e := call(0xd7f0); e != nil {
				return 0, e
			}
		}
	case 0x12266:
		c.D[0] = uint32(cb.Random())
		c.Word(0, uint16(c.D[0])&0x1f)
		if uint16(c.D[0]) == 0 {
			c.Byte(0, m.byte(at+6))
			c.Byte(1, m.byte(at+8))
			c.Word(2, 3)
			if e := call(0x15c3e); e != nil {
				return 0, e
			}
		}
	case 0x1228a:
		p := packed()
		c.Word(0, p&0xff00|uint16(uint8(p)*4))
		grid := 0xf44 + int(int16(uint16(c.D[0])))
		tile := m.byte(grid + 1)
		c.D[0] = uint32(tile)
		raster, e := r.byte(0x33512 + int(tile))
		if e != nil {
			return 0, e
		}
		if raster != 0 {
			head := m.word(grid + 2)
			c.Word(0, head)
			tree := false
			hadHead := head != 0
			seen := map[uint16]bool{}
			for head != 0 {
				if seen[head] {
					return 0, fmt.Errorf("cyclic native neutral tree admission chain")
				}
				seen[head] = true
				node := cleanupRecordAddress(NativeRecordReference(head))
				if m.byte(node) == 0x16 {
					tree = true
					break
				}
				head = m.word(node + 2)
				c.Word(0, head)
			}
			if hadHead && !tree && uint16(c.D[0]) == 0 {
				c.Byte(0, m.byte(at+6))
				c.Byte(1, m.byte(at+8))
				c.Word(2, 3)
				if e := call(0xdb26); e != nil {
					return 0, e
				}
			}
		}
	case 0x122e0:
		p := packed()
		c.Word(0, p&0xff00|uint16(uint8(p)*4))
		grid := 0xf44 + int(int16(uint16(c.D[0])))
		if m.byte(grid+1) != 0 {
			c.D[0] = uint32(cb.Random())
			c.Word(0, uint16(c.D[0])&0x1f)
			if uint16(c.D[0]) == 0 {
				c.Byte(0, m.byte(at+6))
				c.Byte(1, m.byte(at+8))
				c.Word(2, 3)
				if e := call(0x15b7c); e != nil {
					return 0, e
				}
			}
		}
	case 0x1231e:
		origin := packed()
		c.Word(2, origin)
		for cursor := 0x1239a; ; cursor += 2 {
			offset, e := r.word(cursor)
			if e != nil {
				return 0, e
			}
			c.Word(1, offset)
			if offset == 0xff9d {
				break
			}
			p := origin + offset
			c.Word(1, p)
			c.Word(0, p&0xc0c0)
			if uint16(c.D[0]) != 0 {
				continue
			}
			c.Byte(1, uint8(p)*4)
			head := m.word(0xf46 + int(int16(uint16(c.D[1]))))
			c.Word(0, head)
			seen := map[uint16]bool{}
			for head != 0 {
				if seen[head] {
					return 0, fmt.Errorf("cyclic native neutral ruin chain")
				}
				seen[head] = true
				node := cleanupRecordAddress(NativeRecordReference(head))
				kind := m.byte(node)
				if kind == 2 || kind == 4 {
					c.D[0] = 1
					saved2 := c.D[2]
					if e := cb.Cleanup(NativeRecordReference(head), c); e != nil {
						return 0, e
					}
					c.D[2] = saved2
					timer, e := r.word(0x20d72)
					if e != nil {
						return 0, e
					}
					m.putWord(node+20, timer)
					m.putByte(node+22, 0x46)
					m.putWord(node+10, 0x2c34)
				}
				head = m.word(node + 2)
				c.Word(0, head)
			}
		}
	default:
		return 0, fmt.Errorf("native neutral subtype%04x target%05x outside family", subtype, target)
	}
	return 0x12462, m.err
}
