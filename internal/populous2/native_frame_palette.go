package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

// NativeFramePaletteBank retains sixteen raw words and their actual native
// address. $102e4 compares target address identity, never color equality.
type NativeFramePaletteBank struct {
	Address uint32
	Words   [16]uint16
}

func DecodeNativeFramePaletteBank(exe *amiga.Executable, offset, codeBase uint32) (NativeFramePaletteBank, error) {
	var bank NativeFramePaletteBank
	if exe == nil || len(exe.Hunks) == 0 || offset&1 != 0 || uint64(offset)+32 > uint64(len(exe.Hunks[0].Data)) {
		return bank, fmt.Errorf("native palette bank outside aligned CODE")
	}
	bank.Address = codeBase + offset
	for i := range bank.Words {
		bank.Words[i] = binary.BigEndian.Uint16(exe.Hunks[0].Data[int(offset)+i*2:])
	}
	return bank, nil
}

// NativeFramePaletteState is the resumable original $102e4 frame. D0 is
// saved once at entry; the other seven data registers survive each wait and
// retain their final arithmetic outputs. Each phase consumes a real ready
// word, and updates both actual Copper palettes rather than an image cache.
type NativeFramePaletteState struct {
	Source, Target    NativeFramePaletteBank
	CodeBase          uint32
	Phase             uint16
	savedD0           uint32
	started, complete bool
}

func NewNativeFramePaletteState(source, target NativeFramePaletteBank, codeBase uint32) *NativeFramePaletteState {
	return &NativeFramePaletteState{Source: source, Target: target, CodeBase: codeBase}
}

// Advance runs to the next original $786 wait or the final RTS. It does not
// synthesize an interrupt or advance F40. Callers feed VBlank into the real
// presentation producer and resume this same frame afterward.
func (s *NativeFramePaletteState) Advance(p *NativeFramePresentationState, c *NativeFrameRegisterContext, memory FollowerCleanupMemory) (bool, error) {
	if s == nil || p == nil || c == nil || memory.Write16 == nil || len(p.Chip) != NativeFrameChipBytes {
		return false, fmt.Errorf("native palette frame backing missing")
	}
	if s.complete {
		return true, nil
	}
	if !s.started {
		s.savedD0 = c.D[0]
		gate := uint16(1)
		if s.Target.Address == s.CodeBase+0x3361a {
			gate = 0
		}
		if err := memory.Write16(0x3b0, gate); err != nil {
			return false, err
		}
		c.D[7] = 0
		s.started = true
	}
	for {
		c.D[6] = 15
		if p.Input.word(0xa) == 0 {
			return false, nil
		}
		for i := 0; i < 16; i++ {
			c.Word(0, s.Source.Words[i])
			c.Word(1, s.Target.Words[i])
			c.Word(2, uint16(c.D[0]))
			c.Word(3, uint16(c.D[1]))
			c.Word(2, uint16(c.D[2])>>8)
			c.Word(3, uint16(c.D[3])>>8)
			c.Word(3, uint16(c.D[3])-uint16(c.D[2]))
			c.D[3] = uint32(int32(int16(c.D[3])) * int32(int16(c.D[7])))
			c.D[5] = 0
			c.Byte(5, uint8(c.D[3]))
			c.Word(5, uint16(c.D[5])<<4)
			c.Word(5, uint16(c.D[5])+uint16(c.D[0]))
			c.Word(2, uint16(c.D[0]))
			c.Word(3, uint16(c.D[1]))
			c.Word(2, uint16(c.D[2])>>4)
			c.Word(3, uint16(c.D[3])>>4)
			c.D[4] = 15
			c.Word(2, uint16(c.D[2])&uint16(c.D[4]))
			c.Word(3, uint16(c.D[3])&uint16(c.D[4]))
			c.Word(3, uint16(c.D[3])-uint16(c.D[2]))
			c.D[3] = uint32(int32(int16(c.D[3])) * int32(int16(c.D[7])))
			c.Byte(5, uint8(c.D[3]))
			c.Byte(5, uint8(c.D[5])+uint8(c.D[0]))
			c.Word(0, uint16(c.D[0])&uint16(c.D[4]))
			c.Word(2, uint16(c.D[0]))
			c.Word(1, uint16(c.D[1])&uint16(c.D[4]))
			c.Word(1, uint16(c.D[1])-uint16(c.D[0]))
			c.D[1] = uint32(int32(int16(c.D[1])) * int32(int16(c.D[7])))
			c.Word(1, uint16(c.D[1])>>4)
			c.Word(1, uint16(c.D[1])+uint16(c.D[2]))
			c.Word(1, uint16(c.D[1])&uint16(c.D[4]))
			c.Word(4, ^uint16(c.D[4]))
			c.Word(5, uint16(c.D[5])&uint16(c.D[4]))
			c.Word(5, uint16(c.D[5])|uint16(c.D[1]))
			// The low-byte additions and earlier red carry are deliberate
			// native behavior, including raw words outside canonical RGB4.
			binary.BigEndian.PutUint16(p.Chip[0x36+i*4:], uint16(c.D[5]))
			binary.BigEndian.PutUint16(p.Chip[0x236+i*4:], uint16(c.D[5]))
			c.Word(6, uint16(c.D[6])-1)
		}
		p.Input.setWord(0xa, 0)
		c.Word(7, uint16(c.D[7])+1)
		s.Phase++
		if uint16(c.D[7]) == 17 {
			c.D[0] = s.savedD0
			s.complete = true
			return true, nil
		}
	}
}
