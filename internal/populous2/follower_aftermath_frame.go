package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type NativeFollowerAftermathFrameRules struct {
	code []byte
}

func DecodeNativeFollowerAftermathFrameRules(exe *amiga.Executable) (NativeFollowerAftermathFrameRules, error) {
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33612 {
		return NativeFollowerAftermathFrameRules{}, fmt.Errorf("native aftermath frame tables missing")
	}
	return NativeFollowerAftermathFrameRules{code: exe.Hunks[0].Data}, nil
}

type NativeFollowerAftermathFrameCallbacks struct {
	Memory      FollowerCleanupMemory
	Frame       *NativeFrameRegisterContext
	Cleanup     func(NativeRecordReference, *NativeFrameRegisterContext) error
	ClearLeader func(NativeRecordReference, *NativeFrameRegisterContext) error
	Unlink      func(NativeRecordReference) error
	DestroyTown func(NativeRecordReference) error
}

func (r *NativeFollowerAftermathFrameRules) word(at int) (uint16, error) {
	if r == nil || at < 0 || at&1 != 0 || at+2 > len(r.code) {
		return 0, fmt.Errorf("native aftermath word outside CODE")
	}
	return binary.BigEndian.Uint16(r.code[at:]), nil
}

func (r *NativeFollowerAftermathFrameRules) neighbors(ref NativeRecordReference, cb NativeFollowerAftermathFrameCallbacks) error {
	m := nativeWhirlwindMemory{m: cb.Memory}
	at := cleanupRecordAddress(ref)
	origin := m.word(at+8)&0xff00 | uint16(m.byte(at+6))
	for index := 0; index < 64; index++ {
		offset, err := r.word(0x20bb6 + index*2)
		if err != nil {
			return err
		}
		if offset == 0xff9d {
			return m.err
		}
		packed := origin + offset
		if packed&0xc0c0 != 0 {
			continue
		}
		grid := 0xf44 + int(int16(packed&0xff00|uint16(uint8(packed)*4)))
		head := m.word(grid + 2)
		for count := 0; head != 0; count++ {
			if count >= 1053 {
				return fmt.Errorf("native destruction neighbor chain is unbounded")
			}
			target := cleanupRecordAddress(NativeRecordReference(head))
			if m.byte(target) == 2 && m.byte(target+13)&2 == 0 {
				m.putByte(target, 6)
				m.putByte(target+22, 8)
				m.putWord(target+10, 0x178)
				if m.err == nil {
					m.err = cb.Memory.Write32(target+26, 0)
				}
			}
			if m.byte(target) == 4 {
				if cb.DestroyTown == nil {
					return fmt.Errorf("native neighbor town continuation missing")
				}
				if err := cb.DestroyTown(NativeRecordReference(head)); err != nil {
					return err
				}
			}
			if m.byte(target) == 0x16 {
				m.putByte(target, 0x1e)
				m.putWord(target+10, 0xf10)
			}
			head = m.word(target + 2)
			if m.err != nil {
				return m.err
			}
		}
	}
	return fmt.Errorf("native destruction neighbor table lacks terminator")
}

func (r *NativeFollowerAftermathFrameRules) remove(ref NativeRecordReference, cb NativeFollowerAftermathFrameCallbacks) error {
	m := nativeWhirlwindMemory{m: cb.Memory}
	at := cleanupRecordAddress(ref)
	if m.byte(at+13)&1 != 0 {
		if cb.ClearLeader == nil {
			return fmt.Errorf("native aftermath leader frame continuation missing")
		}
		if err := cb.ClearLeader(ref, cb.Frame); err != nil {
			return err
		}
	}
	m.putByte(at+12, 0)
	if m.err == nil {
		m.err = cb.Memory.Write32(at+26, 0)
	}
	if m.err != nil {
		return m.err
	}
	if cb.Unlink == nil {
		return fmt.Errorf("native aftermath unlink continuation missing")
	}
	return cb.Unlink(ref)
}

// Tick starts at the actual dispatch target for the fourteen retained
// aftermath states. It ends before $123b4/$12462, leaving drawing/population
// to the caller. Raw animation words, signed timer flags and every data
// register remain authoritative; this call does not repeat the prepass.
func (r *NativeFollowerAftermathFrameRules) Tick(ref NativeRecordReference, cb NativeFollowerAftermathFrameCallbacks) (NativeFollowerPassFlow, error) {
	if r == nil || cb.Frame == nil || !winMemoryValid(cb.Memory) {
		return NativeFollowerNext, fmt.Errorf("native aftermath frame backing missing")
	}
	m, c := nativeWhirlwindMemory{m: cb.Memory}, cb.Frame
	at := cleanupRecordAddress(ref)
	state := m.byte(at + 22)
	family := aftermathHandler(state)
	if family == 0 {
		return NativeFollowerNext, fmt.Errorf("native aftermath state outside controller")
	}
	if state == 0x28 {
		timer, err := r.word(0x20d72)
		if err != nil {
			return NativeFollowerNext, err
		}
		m.putWord(at+20, timer)
		if err := r.neighbors(ref, cb); err != nil {
			return NativeFollowerNext, err
		}
		if m.err == nil {
			m.err = cb.Memory.Write32(at+26, 0)
		}
	}
	if state != 0x30 {
		c.Word(0, m.word(at+10))
		c.Word(0, uint16(c.D[0])+4)
		word, err := r.word(0x23d1a + int(int16(uint16(c.D[0]))))
		if err != nil {
			return NativeFollowerNext, err
		}
		if int16(word) >= 0 {
			m.putWord(at+10, uint16(c.D[0]))
		} else {
			switch family {
			case 8:
				c.D[0] = 0
				if cb.Cleanup == nil {
					return NativeFollowerNext, fmt.Errorf("native aftermath cleanup frame continuation missing")
				}
				return NativeFollowerNext, cb.Cleanup(ref, c)
			case 0x18:
				return NativeFollowerNext, r.remove(ref, cb)
			case 0x42:
				m.putByte(at, 2)
				m.putWord(at+10, 0)
				m.putByte(at+22, 2)
			case 0x28:
				m.putByte(at+22, 0x30)
			}
		}
		if family != 0x28 {
			flow := NativeFollowerNext
			if family == 8 || family == 0x42 {
				flow = NativeFollowerCount
			}
			return flow, m.err
		}
	}
	previous := int16(m.word(at + 20))
	m.putWord(at+20, uint16(previous)-1)
	if previous > 1 {
		c.Word(0, m.word(at+8))
		c.Byte(0, m.byte(at+6)*4)
		grid := 0xf44 + int(int16(uint16(c.D[0])))
		c.Byte(0, m.byte(grid+1))
		c.Word(0, uint16(c.D[0])&255)
		index := 0x33512 + int(int16(uint16(c.D[0])))
		if index < 0 || index >= len(r.code) {
			return NativeFollowerNext, fmt.Errorf("native ruin raster alias outside CODE")
		}
		c.Byte(0, r.code[index])
		c.Word(0, uint16(c.D[0])&15)
		if uint8(c.D[0]) == 15 {
			return NativeFollowerNext, m.err
		}
	}
	if m.err != nil {
		return NativeFollowerNext, m.err
	}
	return NativeFollowerNext, r.remove(ref, cb)
}
