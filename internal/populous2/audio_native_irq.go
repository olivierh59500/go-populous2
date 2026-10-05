package populous2

import (
	"encoding/binary"
	"fmt"
)

// MusicCommand is $1932c, operating on the four primary score records.
// Effects use Command/$190e4 and the secondary records instead.
func (d *NativeAudioDevice) MusicCommand(control, data uint16, input uint32) (uint32, error) {
	if d == nil {
		return 0, fmt.Errorf("native music device missing")
	}
	status := uint32(0xffff)
	switch control {
	case 0x800:
		d.Code[0x18b33] = uint8(data)
		return status, nil
	case 0x400:
		d.Code[0x18b31], d.Code[0x18b32] = uint8(data), uint8(data)
		return status, nil
	case 0x1000:
		d.Code[0x18b34] = uint8(data)
		return status, nil
	}
	mask := control & 15
	if mask == 0 {
		mask = 15
	}
	for channel := 0; channel < 4; channel++ {
		if mask&(1<<uint(channel)) == 0 {
			continue
		}
		at := 0x18b6a + channel*80
		value := uint8(data)
		if control&0x8000 != 0 {
			value &= 63
			d.Code[at+47] = value
		}
		if control&0x2000 != 0 {
			d.Code[at+43], d.Code[at+47] = value, value
		}
		if control&0x100 != 0 {
			d.Code[at+43] = 0
			status = status&0xffffff00 | uint32(d.Code[0x18b33])
			d.Code[at+47] = uint8(status)
		}
		if control&0x200 != 0 {
			d.Code[at+47] = 0
		}
		if control&0x40 != 0 {
			d.putWord(at+38, d.word(at+38)|0x40)
			d.audioStop(channel)
		}
		if control&0x80 != 0 {
			d.putWord(at+38, 0x81)
			d.Code[0x18b30] = 0xff
		}
		if control&0x20 != 0 {
			d.putWord(at+38, d.word(at+38)|0x20)
		}
		if control&0x10 != 0 && d.Code[at+39]&0x40 == 0 {
			v := uint32(d.word(at + 36))
			if v <= status {
				status = status&0xffff0000 | v
			}
		}
	}
	return status, nil
}

func (d *NativeAudioDevice) audioStop(channel int) {
	d.Hardware = append(d.Hardware, NativeFrameHardwareWrite{0x1987a, uint32(0xdff0a8 + channel*16), 0, 2}, NativeFrameHardwareWrite{0x19880, 0xdff096, uint32(1 << uint(channel)), 2})
}

func (d *NativeAudioDevice) audioEnvelope(at int, base uint16) (uint16, error) {
	address := d.long(at+4) + uint32(d.Code[at+10])
	read := func(offset uint32) (uint8, error) {
		p, err := d.codeAt(address+offset, 1)
		if err == nil {
			return d.Code[p], nil
		}
		if d.ReadAbsolute8 == nil {
			return 0, fmt.Errorf("native envelope absolute address %x requires retained backing", address+offset)
		}
		return d.ReadAbsolute8(address + offset)
	}
	flags := d.Code[at+11]
	d.Code[at+11] &^= 1
	if flags&1 != 0 {
		delta, err := read(1)
		if err != nil {
			return 0, err
		}
		d.putWord(at+2, d.word(at+2)+uint16(int16(int8(delta))))
	}
	value := base + d.word(at+2) - d.word(at)
	if value&0x8000 != 0 {
		value = 0
	}
	if d.Code[at+11]&4 != 0 {
		return value, nil
	}
	d.Code[at+12]++
	steps, err := read(2)
	if err != nil {
		return 0, err
	}
	if d.Code[at+12] != steps {
		return value, nil
	}
	d.Code[at+13]++
	d.Code[at+12] = 0
	repeats, err := read(0)
	if err != nil {
		return 0, err
	}
	if d.Code[at+13] != repeats {
		d.Code[at+11] |= 1
		return value, nil
	}
	d.Code[at+13] = 0
	segment := uint16(d.Code[at+10]) + 3
	if uint8(segment) == 12 {
		if d.Code[at+11] == 0 {
			d.Code[at+11] |= 4
			return value, nil
		}
		d.Code[at+13] = 0
		d.putWord(at, d.word(at)+uint16(int16(int8(d.Code[at+8]))))
		segment = 3
	}
	d.Code[at+10] = uint8(segment)
	d.Code[at+11] |= 1
	return value, nil
}

func (d *NativeAudioDevice) resetNativeEnvelope(at int) {
	d.Code[at+10] = 0
	d.putWord(at+2, 0)
	d.putWord(at, 0)
	d.Code[at+13], d.Code[at+12] = 0, 0
	d.Code[at+11] = d.Code[at+11]&^4 | 1
}

// audioVoice is complete $19612, including native sequence and envelope
// pointers. It writes the shared driver output words; the secondary voice
// can supersede its primary's output without adding another hardware voice.
func (d *NativeAudioDevice) audioNativeVoice(at int) error {
	if d.Code[at+39]&0x40 != 0 {
		return nil
	}
	d.putLong(0x18b2a, d.long(at+24))
	if d.Code[at+47] != d.Code[at+43] {
		d.Code[at+44]++
		if d.Code[at+44] > d.Code[0x18b34] {
			d.Code[at+44] = 0
			if d.Code[at+47] > d.Code[at+43] {
				d.Code[at+43]++
			} else {
				d.Code[at+43]--
			}
		}
	}
	newSequence := d.Code[at+39]&1 != 0
	d.Code[at+39] &^= 1
	if !newSequence && d.Code[0x18b31] == 0 {
		d.Code[at+40]--
	}
	needNote := newSequence || d.Code[at+40] == 0
	if needNote {
		if newSequence {
			d.putLong(at+12, d.long(at))
			d.putWord(at+36, 0)
		} else {
			d.Code[at+40] = d.Code[at+41]
		}
		readingSequence := newSequence
		for commands := 0; ; commands++ {
			if commands > 4096 {
				return fmt.Errorf("native audio bytecode exceeds bounded continuation")
			}
			var pointer int
			var err error
			if readingSequence {
				pointer, err = d.codeAt(d.long(at+12), 1)
			} else {
				pointer, err = d.codeAt(d.long(at+8), 1)
			}
			if err != nil {
				return err
			}
			command := d.Code[pointer]
			pointer++
			if command == 255 {
				if !readingSequence {
					readingSequence = true
					continue
				}
				if d.Code[at+39]&0x20 != 0 {
					d.putLong(at+12, d.long(at))
					d.putWord(at+36, 0)
					continue
				}
				parent, err := d.codeAt(d.long(at+4), 80)
				if err != nil {
					return err
				}
				if d.Code[parent+39]&2 != 0 && d.Code[at+38]&2 != 0 {
					d.putWord(parent+38, d.word(parent+38)|0x100)
					d.Code[parent+43] = 0
				}
				d.Code[parent+39] &^= 2
				d.putWord(at+38, d.word(at+38)|0x40)
				d.putWord(0x18b16, 0)
				return nil
			}
			if readingSequence {
				d.putLong(at+12, d.CodeBase+uint32(pointer))
				d.putWord(at+36, d.word(at+36)+1)
				table, err := d.codeAt(d.long(0x18b26), 2)
				if err != nil {
					return err
				}
				index := table + int(command)*2
				if index+2 > len(d.Code) {
					return fmt.Errorf("native audio pattern index outside CODE")
				}
				d.putLong(at+8, d.long(0x18b22)+uint32(int32(int16(d.word(index)))))
				readingSequence = false
				continue
			}
			d.putLong(at+8, d.CodeBase+uint32(pointer))
			if command < 128 {
				if command != 0 {
					command += d.Code[at+42]
				}
				periodAt := 0x18dfa + int(command)*2
				if periodAt+2 > len(d.Code) {
					return fmt.Errorf("native audio period alias outside CODE")
				}
				d.putWord(at+34, d.word(periodAt))
				d.putWord(at+32, d.word(periodAt))
				for _, offset := range []int{16, 20} {
					env, err := d.codeAt(d.long(at+offset), 16)
					if err != nil {
						return err
					}
					d.resetNativeEnvelope(env)
				}
				break
			}
			if command&0x40 == 0 {
				if command&0x20 != 0 {
					d.Code[0x18b2e] = command & 31
				} else {
					value := d.Code[0x18b4a+int(command&15)]
					d.Code[at+41], d.Code[at+40] = value, value
				}
				continue
			}
			if command&0x20 == 0 {
				bank, err := d.codeAt(d.long(0x18b1a), 13)
				if err != nil {
					return err
				}
				bank += int(command&31) * 13
				if bank+13 > len(d.Code) {
					return fmt.Errorf("native volume envelope outside CODE")
				}
				env, err := d.codeAt(d.long(at+16), 16)
				if err != nil {
					return err
				}
				d.Code[env+11] = d.Code[bank] & 0x80
				d.Code[env+8] = d.Code[bank] & 0x7f
				d.resetNativeEnvelope(env)
				d.putLong(env+4, d.CodeBase+uint32(bank+1))
				continue
			}
			switch command & 7 {
			case 0:
				d.Code[at+42] = d.Code[pointer]
				d.putLong(at+8, d.CodeBase+uint32(pointer+1))
			case 1, 2:
				index := d.Code[pointer]
				d.putLong(at+8, d.CodeBase+uint32(pointer+1))
				bank, err := d.codeAt(d.long(0x18b1e), 13)
				if err != nil {
					return err
				}
				bank += int(index) * 13
				if bank+13 > len(d.Code) {
					return fmt.Errorf("native period envelope outside CODE")
				}
				env, err := d.codeAt(d.long(at+20), 16)
				if err != nil {
					return err
				}
				d.Code[env+11], d.Code[env+8] = 0, 0
				if command&7 == 1 {
					d.Code[env+11] = 0x80
				}
				d.resetNativeEnvelope(env)
				d.putLong(env+4, d.CodeBase+uint32(bank+1))
			case 3:
			default:
				return fmt.Errorf("native audio extension %x requires its source continuation", command)
			}
		}
	}
	volumeEnv, err := d.codeAt(d.long(at+16), 16)
	if err != nil {
		return err
	}
	volume, err := d.audioEnvelope(volumeEnv, 0)
	if err != nil {
		return err
	}
	current, master := d.Code[at+43], d.Code[0x18b33]
	// CMP.B/BLE is signed, while the later subtraction uses word borrow.
	if int8(current) > int8(master) {
		current = master
	}
	reduction := uint16(63) - uint16(current)
	if volume < reduction {
		volume = 0
	} else {
		volume -= reduction
	}
	d.putWord(0x18b16, volume&255)
	periodEnv, err := d.codeAt(d.long(at+20), 16)
	if err != nil {
		return err
	}
	period, err := d.audioEnvelope(periodEnv, d.word(at+32))
	if err != nil {
		return err
	}
	d.putWord(0x18b18, period)
	return nil
}

// TickCIA translates $1942a. Internal score and envelope updates preserve
// the caller's complete D1-D7 and return D0=0, as the original IRQ wrapper.
func (d *NativeAudioDevice) TickCIA(c *NativeFrameRegisterContext) error {
	if d == nil || c == nil {
		return fmt.Errorf("native audio IRQ frame missing")
	}
	if d.Code[0x18b30] == 0 {
		c.D[0] = 0
		return nil
	}
	d.Code[0x18b30] = 0
	d.Code[0x18b31]--
	for channel := 0; channel < 4; channel++ {
		at := 0x18b6a + channel*80
		d.Code[0x18b2e] = 255
		d.putWord(0x18b16, 0)
		d.putWord(0x18b18, 0)
		if err := d.audioNativeVoice(at); err != nil {
			return err
		}
		if d.Code[at+39]&2 != 0 {
			d.Code[0x18b2e] = 255
			secondary, err := d.codeAt(d.long(at+28), 80)
			if err != nil {
				return err
			}
			if err = d.audioNativeVoice(secondary); err != nil {
				return err
			}
		}
		register := uint32(0xdff0a0 + channel*16)
		d.Hardware = append(d.Hardware, NativeFrameHardwareWrite{0x19492, register + 8, uint32(d.word(0x18b16)), 2}, NativeFrameHardwareWrite{0x19498, register + 6, uint32(d.word(0x18b18)), 2})
		phase := 0x194e2 + channel
		instrument := d.Code[0x18b2e]
		if instrument != 255 {
			d.Code[phase+4] = instrument
			offset := 4 + int(instrument)*16
			if offset+16 > len(d.FX) {
				return fmt.Errorf("native DMA sample descriptor outside FX.DAT")
			}
			location := d.long(0x18ec6) + binary.BigEndian.Uint32(d.FX[offset:])
			length := binary.BigEndian.Uint16(d.FX[offset+12:])
			d.Hardware = append(d.Hardware, NativeFrameHardwareWrite{0x194c6, register + 4, uint32(length), 2}, NativeFrameHardwareWrite{0x194cc, register, location, 4}, NativeFrameHardwareWrite{0x194d4, 0xdff096, uint32(1 << uint(channel)), 2})
			d.Code[phase] = 1
		} else if d.Code[phase] != 0 {
			if d.Code[phase] == 1 {
				d.Hardware = append(d.Hardware, NativeFrameHardwareWrite{0x19500, 0xdff096, 0x8200 | uint32(1<<uint(channel)), 2})
				d.Code[phase] = 2
			} else {
				offset := 4 + int(d.Code[phase+4])*16
				if offset+16 > len(d.FX) {
					return fmt.Errorf("native DMA loop descriptor outside FX.DAT")
				}
				location := d.long(0x18ec6) + binary.BigEndian.Uint32(d.FX[offset+4:])
				length := binary.BigEndian.Uint16(d.FX[offset+14:])
				d.Hardware = append(d.Hardware, NativeFrameHardwareWrite{0x19522, register + 4, uint32(length), 2}, NativeFrameHardwareWrite{0x19528, register, location, 4})
				d.Code[0x18b2e] = 255
				d.Code[phase] = 0
			}
		}
	}
	if d.Code[0x18b31] == 0 {
		d.Code[0x18b31] = d.Code[0x18b32]
	}
	d.Code[0x18b30] = 255
	c.D[0] = 0
	return nil
}
