package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

// AudioSample preserves Paula's first DMA block and following loop block.
// These lengths are stored in words, not bytes, in the native bank.
type AudioSample struct{ Initial, Loop []byte }

type AudioCue struct {
	Flags              uint16
	Channel, Volume    uint8
	Pattern, Companion int
}

// AudioBank contains the original bytecode score and signed eight-bit samples.
// The driver is embedded in CODE ($1926c-$19932), separate from FX.DAT.
type AudioBank struct {
	Cues            [133]AudioCue
	Samples         []AudioSample
	Patterns        [][]byte
	Channels        [4][]uint8
	VolumeEnvelopes [32][13]byte
	PeriodEnvelopes [32][13]byte
	Periods         [102]uint16
	Lengths         [16]uint8
	Tempo           uint8
}

func DecodeAudioBank(exe *amiga.Executable, fx []byte) (*AudioBank, error) {
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x3e40c || len(fx) < 20 {
		return nil, fmt.Errorf("Populous II audio data missing")
	}
	if uint64(binary.BigEndian.Uint32(fx[:4])) != uint64(len(fx)-4) {
		return nil, fmt.Errorf("FX.DAT length header mismatch")
	}
	samples := fx[4:]
	first := int(binary.BigEndian.Uint32(samples[:4]))
	if first < 16 || first%16 != 0 || first > len(samples) || first/16 > 32 {
		return nil, fmt.Errorf("invalid FX.DAT sample table")
	}
	b := &AudioBank{Samples: make([]AudioSample, first/16)}
	for i := range b.Samples {
		r := samples[i*16:]
		start, loop := int(binary.BigEndian.Uint32(r)), int(binary.BigEndian.Uint32(r[4:]))
		initialBytes, loopBytes := int(binary.BigEndian.Uint16(r[12:]))*2, int(binary.BigEndian.Uint16(r[14:]))*2
		if start < first || start > len(samples)-initialBytes || loop < first || loop > len(samples)-loopBytes {
			return nil, fmt.Errorf("sample %d exceeds FX.DAT", i)
		}
		b.Samples[i] = AudioSample{Initial: append([]byte(nil), samples[start:start+initialBytes]...), Loop: append([]byte(nil), samples[loop:loop+loopBytes]...)}
	}
	code := exe.Hunks[0].Data
	for i := range b.Cues {
		at := 0x185a8 + i*10
		cue := AudioCue{Flags: binary.BigEndian.Uint16(code[at+2:]), Channel: code[at+4], Volume: code[at+5], Pattern: int(binary.BigEndian.Uint16(code[at+6:])), Companion: int(binary.BigEndian.Uint16(code[at+8:]))}
		if cue.Channel > 3 || cue.Volume > 64 || cue.Pattern >= 133 || cue.Companion%10 != 0 || cue.Companion/10 >= 133 {
			return nil, fmt.Errorf("invalid audio cue %d", i)
		}
		cue.Companion /= 10
		b.Cues[i] = cue
	}
	for i := range b.Periods {
		b.Periods[i] = binary.BigEndian.Uint16(code[0x18dfa+i*2:])
	}
	copy(b.Lengths[:], code[0x18b4a:0x18b5a])
	// $18aee selects score zero from the table at $3e3f8. All offsets in the
	// selected header are relative to its own first byte, not the outer table.
	at := 0x3e3f8 + int(binary.BigEndian.Uint16(code[0x3e3f8:]))
	if at+18 > len(code) {
		return nil, fmt.Errorf("native score header exceeds CODE")
	}
	tempo := binary.BigEndian.Uint16(code[at:])
	if tempo == 0 || tempo > 255 {
		return nil, fmt.Errorf("invalid native score tempo")
	}
	b.Tempo = uint8(tempo)
	for i, table := range []*[32][13]byte{&b.VolumeEnvelopes, &b.PeriodEnvelopes} {
		start := at + int(binary.BigEndian.Uint16(code[at+2+i*2:]))
		if start+32*13 > len(code) {
			return nil, fmt.Errorf("envelope table exceeds CODE")
		}
		for n := range *table {
			copy(table[n][:], code[start+n*13:start+(n+1)*13])
		}
	}
	patterns := at + 16
	// The four channel sequences precede the pattern payloads. Their lowest
	// offset marks the end of the pattern pointer table.
	firstSequence := len(code) - at
	for channel := range b.Channels {
		firstSequence = min(firstSequence, int(binary.BigEndian.Uint16(code[at+6+channel*2:])))
	}
	count := (firstSequence - 16) / 2
	if firstSequence < 18 || (firstSequence-16)%2 != 0 || count > 256 || patterns+count*2 > len(code) {
		return nil, fmt.Errorf("invalid native pattern table")
	}
	b.Patterns = make([][]byte, count)
	for i := range b.Patterns {
		start := at + int(binary.BigEndian.Uint16(code[patterns+i*2:]))
		pattern, err := readAudioPattern(code, start)
		if err != nil {
			return nil, fmt.Errorf("pattern %d: %w", i, err)
		}
		b.Patterns[i] = pattern
	}
	for channel := range b.Channels {
		start := at + int(binary.BigEndian.Uint16(code[at+6+channel*2:]))
		if start < at+16 || start >= len(code) {
			return nil, fmt.Errorf("invalid native channel sequence")
		}
		for i := start; i < len(code) && len(b.Channels[channel]) < 4096; i++ {
			if code[i] == 255 {
				break
			}
			if int(code[i]) >= count {
				return nil, fmt.Errorf("channel %d refers to absent pattern", channel)
			}
			b.Channels[channel] = append(b.Channels[channel], code[i])
		}
	}
	return b, nil
}

func readAudioPattern(code []byte, start int) ([]byte, error) {
	if start < 0 || start >= len(code) {
		return nil, fmt.Errorf("pattern offset outside CODE")
	}
	for at := start; at < len(code) && at-start < 4096; at++ {
		command := code[at]
		if command == 255 {
			return append([]byte(nil), code[start:at+1]...), nil
		}
		if command == 0xe0 || command == 0xe1 || command == 0xe2 {
			at++
		}
		if command >= 0xe4 {
			return nil, fmt.Errorf("unsupported native audio command %x", command)
		}
	}
	return nil, fmt.Errorf("unterminated native pattern")
}
