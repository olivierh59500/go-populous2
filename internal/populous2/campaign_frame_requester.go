package populous2

import "fmt"

// campaignRequesterCompile preserves $4eb6's address-register outputs while
// the shared proven compiler writes the actual workspace and data registers.
func campaignRequesterCompile(b nativeRequesterFrameBacking, a *[7]NativeRequesterAddress, definition, parameter uint32) error {
	original := *a
	at := definition + 4
	output := b.CodeBase + 0xab58
	if uint16(b.Frame.D[3]) == 0 {
		output = at
	}
	p := parameter
	last := a[3].Address
	params := []NativeRequesterAddress{}
	peekEnd := parameter
	read := func(address uint32) (byte, error) { return b.Code.Read8(int(int64(address) - int64(b.CodeBase))) }
	for steps := 0; steps < 65536; steps++ {
		v, e := read(at)
		if e != nil {
			return e
		}
		at++
		if v == 0 {
			for slot := parameter; slot < p || slot < peekEnd; slot += 4 {
				source, e := b.Code.Read32(int(slot - b.CodeBase))
				if e != nil {
					return e
				}
				params = append(params, NativeRequesterAddress{Address: source, Code: true})
			}
			if e = b.compile(definition, params); e != nil {
				return e
			}
			a[0] = original[0]
			a[1] = NativeRequesterAddress{Address: at, Code: true}
			a[2] = NativeRequesterAddress{Address: p, Code: true}
			a[3] = NativeRequesterAddress{Address: last, Code: true}
			a[4] = NativeRequesterAddress{Address: output, Code: true}
			return nil
		}
		if int8(v) < 0 {
			v -= 0x25
		}
		if v == '{' {
			source, e := b.Code.Read32(int(p - b.CodeBase))
			if e != nil {
				return e
			}
			p += 4

			if source == 0 {
				output++
				continue
			}
			last = source
			first, e := read(last)
			if e != nil {
				return e
			}
			if first == 0 {
				output++
				continue
			}
			at--
			for {
				v, e = read(last)
				if e != nil {
					return e
				}
				if v == 0 {
					break
				}
				last++
				output++
				at++
			}
			continue
		}
		output++
		if v == 'v' {
			source, e := b.Code.Read32(int(p - b.CodeBase))
			if e != nil {
				return e
			}
			if source == 0 {
				peekEnd = p + 4
				continue
			}
			p += 4

			width := 0
			for {
				v, e = read(at + uint32(width))
				if e != nil {
					return e
				}
				if v == 'w' || v == 0x9c {
					break
				}
				width++
			}
			length := 0
			for {
				v, e = read(source + uint32(length))
				if e != nil {
					return e
				}
				if v == 0 {
					break
				}
				length++
			}
			last = source
			if length > width {
				last += uint32(length - width)
			}
			at += uint32(width)
			for i := 0; i < width; i++ {
				v, e = read(last)
				if e != nil {
					return e
				}
				output++
				if v != 0 {
					last++
				}
			}
		}
	}
	return fmt.Errorf("native campaign compiler did not reach terminator")
}

// campaignRequesterText retains $509a's surviving A0/A1. Its A2-A4 outer
// MOVEM is supplied by the existing literal planar font body.
func campaignRequesterText(b nativeRequesterFrameBacking, a *[7]NativeRequesterAddress, target uint32, text int) error {
	x, y := uint16(b.Frame.D[0]), uint16(b.Frame.D[1])
	line := target + uint32(int32(int16(uint16(uint32(y)*40)))) + uint32(int32(int16(x)))
	cursor := line
	column := x
	at := text
	for steps := 0; steps < 65536; steps++ {
		v, e := b.Code.Read8(at)
		if e != nil {
			return e
		}
		at++
		if v == 0 {
			break
		}
		if v == '\n' {
			line += 320
			cursor = line
			column = x
		} else {
			if column >= 40 {
				break
			}
			cursor++
			column++
		}
	}
	if e := b.text(target, text); e != nil {
		return e
	}
	a[0] = NativeRequesterAddress{Address: cursor, Chip: true}
	a[1] = NativeRequesterAddress{Address: b.CodeBase + uint32(at), Code: true}
	return nil
}

func campaignRequesterCopy(b nativeRequesterFrameBacking, a *[7]NativeRequesterAddress) error {
	front, e := b.Memory.Read32(0x1a)
	if e != nil {
		return e
	}
	back, e := b.Memory.Read32(0x1e)
	if e != nil {
		return e
	}
	source, e := b.Bitmap(front)
	if e != nil {
		return e
	}
	target, e := b.Bitmap(back)
	if e != nil {
		return e
	}
	if len(source) < 32000 || len(target) < 32000 {
		return fmt.Errorf("native campaign copy outside screen RAM")
	}
	var last uint32
	for at := 0; at < 32000; at += 32 {
		var block [32]byte
		copy(block[:], source[at:at+32])
		copy(target[at:at+32], block[:])
		last = uint32(block[24])<<24 | uint32(block[25])<<16 | uint32(block[26])<<8 | uint32(block[27])
	}
	a[0] = NativeRequesterAddress{Address: front + 32000, Chip: true}
	a[1] = NativeRequesterAddress{Address: back + 32000, Chip: true}
	a[2] = NativeRequesterAddress{Address: last, Absolute: true}
	return nil
}

func campaignRequesterClick(b nativeRequesterFrameBacking, a *[7]NativeRequesterAddress) (uint32, error) {
	pressed, e := b.Memory.Read16(0x140)
	if e != nil {
		return 0, e
	}
	if pressed != 0 {
		x, e := b.Memory.Read16(0x134)
		if e != nil {
			return 0, e
		}
		y, e := b.Memory.Read16(0x136)
		if e != nil {
			return 0, e
		}
		col, e := b.Code.Read16(0xab50)
		if e != nil {
			return 0, e
		}
		row, e := b.Code.Read16(0xab52)
		if e != nil {
			return 0, e
		}
		width, e := b.Code.Read16(0xab54)
		if e != nil {
			return 0, e
		}
		height, e := b.Code.Read16(0xab56)
		if e != nil {
			return 0, e
		}
		dx, dy := uint16((x>>3)-col), uint16(y-row)
		if int16(dx) >= 0 && int16(dx-width) < 0 && int16(dy) >= 0 && int16(dy-height) <= 0 {
			start, e := b.Code.Read16(0xab4e)
			if e != nil {
				return 0, e
			}
			line := b.CodeBase + 0xab4e + uint32(int32(int16(start))) + uint32(uint16(uint32(dy/8)*uint32(width+1)))
			target := line + uint32(int32(int16(dx)))
			a[0] = NativeRequesterAddress{Address: b.CodeBase + 0xab4e + uint32(int32(int16(start))), Code: true}
			a[2] = NativeRequesterAddress{Address: target + 1, Code: true}
			a[3] = NativeRequesterAddress{Address: target, Code: true}
			selected := byte(0)
			for at := line; at <= target; at++ {
				v, e := b.Code.Read8(int(at - b.CodeBase))
				if e != nil {
					return 0, e
				}
				if int8(v) > 0x5a {
					m, e := b.Code.Read8(0x4e92 + int(v-0x5b))
					if e != nil {
						return 0, e
					}
					if m != 0 {
						selected = v
						if int8(m) < 0 {
							selected = 0
						}
					}
				}
			}
			if selected != 0 {
				a[1] = NativeRequesterAddress{Address: 0, Absolute: true}
				for at := a[0].Address; at <= target; at++ {
					v, e := b.Code.Read8(int(at - b.CodeBase))
					if e != nil {
						return 0, e
					}
					if int8(v) > 0x5a {
						m, e := b.Code.Read8(0x4e92 + int(v-0x5b))
						if e != nil {
							return 0, e
						}
						if int8(m) > 0 {
							a[1] = NativeRequesterAddress{Address: at, Code: true}
						}
					}
				}
				a[0] = NativeRequesterAddress{Address: target + 1, Code: true}
			}
		}
	}
	return b.click()
}

func campaignWorldCode(cb NativeCampaignFrameCallbacks, a *[7]NativeRequesterAddress, target uint32) error {
	value := uint16(uint32(uint16(cb.Frame.D[0]))*0x24a1+0x24df) & 0x7fff
	for value != 0 {
		offset := int(value&63) * 2
		for i := 0; i < 2; i++ {
			v, e := cb.Code.Read8(0x103f8 + offset + i)
			if e != nil {
				return e
			}
			if e = cb.Code.Write8(int(target-cb.CodeBase), v); e != nil {
				return e
			}
			target++
		}
		value >>= 6
	}
	if e := cb.Code.Write8(int(target-cb.CodeBase), 0); e != nil {
		return e
	}
	a[0] = NativeRequesterAddress{Address: target + 1, Code: true}
	return nil
}
