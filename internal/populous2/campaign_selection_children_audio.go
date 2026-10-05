package populous2

import "fmt"

// TickNativeCampaignAudio is literal $182ce/$183b6 over the one shared CODE
// bank, including negative descriptor assignment and unsaved A0/A1 outputs.
func TickNativeCampaignAudio(cb NativeCampaignFrameCallbacks, a *[7]NativeRequesterAddress, command func(uint16, uint16, uint32) (uint32, error)) error {
	if command == nil {
		return fmt.Errorf("native campaign audio device missing")
	}
	c := cb.Frame
	code := cb.Code
	read := func(at uint32) (uint16, error) { return code.Read16(int(at - cb.CodeBase)) }
	write := func(at uint32, v uint16) error { return code.Write16(int(at-cb.CodeBase), v) }
	device := func(control, data uint16) error {
		v, e := command(control, data, c.D[0])
		if e != nil {
			return e
		}
		c.D[0] = v
		return nil
	}
	var schedule func() error
	schedule = func() error {
		savedA0 := a[0]
		defer func() { a[0] = savedA0 }()
		c.D[0] = 0
		priority, e := code.Read8(int(a[0].Address-cb.CodeBase) + 4)
		if e != nil {
			return e
		}
		c.Byte(0, priority)
		c.Word(0, uint16(c.D[0])*2)
		a[1] = NativeRequesterAddress{Address: uint32(int64(cb.CodeBase+0x18426) + int64(int16(c.D[0]))), Code: true}
		c.D[0] = a[0].Address - (cb.CodeBase + 0x185a8)
		channel, e := read(a[1].Address)
		if e != nil {
			return e
		}
		if uint16(c.D[0]) == channel {
			return write(a[1].Address, -channel)
		}
		if channel != 0 {
			return nil
		}
		c.Word(0, -uint16(c.D[0]))
		if e = write(a[1].Address, uint16(c.D[0])); e != nil {
			return e
		}
		c.D[0], c.D[1] = 1, 0
		c.Byte(1, priority)
		c.Word(0, nativeAudioShiftWord(uint16(c.D[0]), uint8(c.D[1])))
		flags, e := read(a[0].Address + 2)
		if e != nil {
			return e
		}
		c.Word(0, uint16(c.D[0])|flags)
		data, e := read(a[0].Address + 6)
		if e != nil {
			return e
		}
		if e = device(uint16(c.D[0]), data); e != nil {
			return e
		}
		c.D[1], c.D[0] = uint32(priority), 1
		c.Word(0, nativeAudioShiftWord(uint16(c.D[0]), uint8(c.D[1]))|0x2000)
		control := uint16(c.D[0])
		value, e := code.Read8(int(a[0].Address-cb.CodeBase) + 5)
		if e != nil {
			return e
		}
		c.D[0] = 0
		c.Byte(0, value)
		return device(control, uint16(value))
	}
	a[0] = NativeRequesterAddress{Address: cb.CodeBase + 0x185b2, Code: true}
	for {
		flag, e := read(a[0].Address)
		if e != nil {
			return e
		}
		if flag != 0 {
			if e = write(a[0].Address, 0); e != nil {
				return e
			}
			if e = schedule(); e != nil {
				return e
			}
			linked, e := read(a[0].Address + 8)
			if e != nil {
				return e
			}
			c.Word(0, linked)
			if linked != 0 {
				saved := a[0]
				a[0] = NativeRequesterAddress{Address: uint32(int64(cb.CodeBase+0x185a8) + int64(int16(c.D[0]))), Code: true}
				if e = schedule(); e != nil {
					return e
				}
				a[0] = saved
			}
		}
		a[0].Address += 10
		if int32(a[0].Address) >= int32(cb.CodeBase+0x18ada) {
			break
		}
	}
	a[1] = NativeRequesterAddress{Address: cb.CodeBase + 0x18426, Code: true}
	for {
		channel, e := read(a[1].Address)
		if e != nil {
			return e
		}
		c.Word(0, channel)
		if channel != 0 {
			if int16(channel) < 0 {
				c.D[1] = a[1].Address - (cb.CodeBase + 0x18426)
				c.Word(1, uint16(c.D[1])>>1)
				c.D[0] = 1
				c.Word(0, nativeAudioShiftWord(uint16(c.D[0]), uint8(c.D[1]))|0x10)
				if e = device(uint16(c.D[0]), 0); e != nil {
					return e
				}
				if int16(c.D[0]) <= 0 {
					if e = write(a[1].Address, 0); e != nil {
						return e
					}
				}
			} else {
				a[0] = NativeRequesterAddress{Address: uint32(int64(cb.CodeBase+0x185a8) + int64(int16(c.D[0]))), Code: true}
				flags, e := read(a[0].Address + 2)
				if e != nil {
					return e
				}
				c.Word(0, flags&0x20)
				if e = write(a[1].Address, 0); e != nil {
					return e
				}
				c.D[0], c.D[1] = 1, 0
				priority, e := code.Read8(int(a[0].Address-cb.CodeBase) + 4)
				if e != nil {
					return e
				}
				c.Byte(1, priority)
				c.Word(0, nativeAudioShiftWord(uint16(c.D[0]), uint8(c.D[1]))|0x80)
				if e = device(uint16(c.D[0]), 0); e != nil {
					return e
				}
			}
		}
		current, e := read(a[1].Address)
		if e != nil {
			return e
		}
		if e = write(a[1].Address, -current); e != nil {
			return e
		}
		a[1].Address += 2
		if int32(a[1].Address) >= int32(cb.CodeBase+0x1842e) {
			return nil
		}
	}
}
