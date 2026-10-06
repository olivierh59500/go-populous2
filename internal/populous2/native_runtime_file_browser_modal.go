package populous2

import "fmt"

// NativeRuntimeFileBrowserModal keeps the full $4bba/$4c14 address cursor and
// $4d12 MOVEM frame across its real VBlank wait. The FF byte is native CODE.
type NativeRuntimeFileBrowserModal struct {
	NativeFileTextModal
	SavedA  [7]NativeRequesterAddress
	ClickA0 NativeRequesterAddress
}

func (s *NativeRuntimeFileBrowserModal) begin(b nativeRequesterFrameBacking, field, clickEnd int, a *[7]NativeRequesterAddress) error {
	if e := s.NativeFileTextModal.begin(b, field, clickEnd); e != nil {
		return e
	}
	a[0] = NativeRequesterAddress{Address: b.CodeBase + uint32(s.Display), Code: true}
	a[1] = NativeRequesterAddress{Address: b.CodeBase + uint32(field), Code: true}
	a[2] = NativeRequesterAddress{Address: b.CodeBase + uint32(s.Caret), Code: true}
	return nil
}

func (s *NativeRuntimeFileBrowserModal) prepare(b nativeRequesterFrameBacking, a *[7]NativeRequesterAddress) error {
	s.SavedA = *a
	source := s.Caret - int(int16(b.Frame.D[4]))
	for n := 0; n < int(s.Capacity); n++ {
		value, e := b.Code.Read8(source)
		if e != nil {
			return e
		}
		if value != 0 {
			source++
		}
	}
	if e := s.NativeFileTextModal.prepare(b); e != nil {
		return e
	}
	a[0] = NativeRequesterAddress{Address: s.Target, Chip: true}
	a[1] = s.SavedA[0]
	a[3] = NativeRequesterAddress{Address: b.CodeBase + uint32(source), Code: true}
	return nil
}

func (s *NativeRuntimeFileBrowserModal) advance(b nativeRequesterFrameBacking, presentation *NativeFramePresentationState, keys NativeInputRules, a *[7]NativeRequesterAddress) (bool, error) {
	if s == nil || !s.Active || presentation == nil {
		return false, fmt.Errorf("native file modal live state missing")
	}
	c := b.Frame
	for {
		if s.Waiting {
			ready, e := b.Memory.Read16(0xa)
			if e != nil {
				return false, e
			}
			if ready == 0 {
				return false, nil
			}
			if e := campaignRequesterText(b, a, s.Target, s.Display); e != nil {
				return false, e
			}
			if e := fileFrameSwap(b, presentation); e != nil {
				return false, e
			}
			if e := b.Code.Write8(s.Display+int(s.Capacity), s.Tail); e != nil {
				return false, e
			}
			c.D, *a = s.Saved, s.SavedA
			s.Waiting = false
			if s.Finish != 0 {
				if s.Finish == 1 {
					c.D[0] = 0
				}
				if s.Finish == 2 {
					a[0] = s.ClickA0
				}
				s.Active = false
				return true, nil
			}
		}
		left, e := b.Memory.Read16(0x140)
		if e != nil {
			return false, e
		}
		right, e := b.Memory.Read16(0x142)
		if e != nil {
			return false, e
		}
		if left != 0 || right != 0 {
			saved := c.D
			savedA := *a
			_, e := campaignRequesterClick(b, a)
			clickA0 := a[0]
			*a = savedA
			a[6] = clickA0
			if e != nil {
				return false, e
			}
			d0 := c.D[0]
			c.D = saved
			c.D[0] = d0
			if uint16(d0) != 0 {
				c.D[5] = 0xffffffff
				s.Finish = 2
				s.ClickA0 = a[6]
				if e := s.prepare(b, a); e != nil {
					return false, e
				}
				continue
			}
		}
		leftKey, e := b.Memory.Read8(0x93)
		if e != nil {
			return false, e
		}
		rightKey, e := b.Memory.Read8(0x95)
		if e != nil {
			return false, e
		}
		if leftKey != 0 {
			if e := b.Memory.Write8(0x93, 0); e != nil {
				return false, e
			}
			if s.Field != s.Caret {
				s.Caret--
				a[2] = NativeRequesterAddress{Address: b.CodeBase + uint32(s.Caret), Code: true}
				c.Word(4, uint16(c.D[4])-1)
				if int16(c.D[4]) < 0 {
					c.D[4] = 0
				}
			}
		} else if rightKey != 0 {
			if e := b.Memory.Write8(0x95, 0); e != nil {
				return false, e
			}
			v, e := b.Code.Read8(s.Caret)
			if e != nil {
				return false, e
			}
			if v != 0 {
				s.Caret++
				a[2] = NativeRequesterAddress{Address: b.CodeBase + uint32(s.Caret), Code: true}
				c.Word(4, uint16(c.D[4])+1)
				if uint16(c.D[4]) == uint16(c.D[3]) {
					c.Word(4, uint16(c.D[4])-1)
				}
			}
		} else {
			key, e := keys.Character(&presentation.Input, &c.D)
			if e != nil {
				return false, e
			}
			switch key {
			case 0:
			case 13:
				c.D[5] = 0xffffffff
				s.Finish = 1
				if e := s.prepare(b, a); e != nil {
					return false, e
				}
				continue
			case 8:
				if s.Field != s.Caret {
					from, to := s.Caret, s.Caret-1
					s.Caret--
					a[2] = NativeRequesterAddress{Address: b.CodeBase + uint32(s.Caret), Code: true}
					for {
						v, e := b.Code.Read8(from)
						if e != nil {
							return false, e
						}
						from++
						if e := b.Code.Write8(to, v); e != nil {
							return false, e
						}
						to++
						if v == 0 {
							break
						}
					}
					a[3], a[4] = NativeRequesterAddress{Address: b.CodeBase + uint32(from), Code: true}, NativeRequesterAddress{Address: b.CodeBase + uint32(to), Code: true}
					c.Word(4, uint16(c.D[4])-1)
					if int16(c.D[4]) < 0 {
						c.D[4] = 0
					}
				}
			default:
				end := s.Caret
				for {
					v, e := b.Code.Read8(end)
					if e != nil {
						return false, e
					}
					end++
					if v == 0 {
						break
					}
				}
				a[3] = NativeRequesterAddress{Address: b.CodeBase + uint32(end), Code: true}
				v, e := b.Code.Read8(end)
				if e != nil {
					return false, e
				}
				if v != 0xff {
					for from := end - 1; from >= s.Caret; from-- {
						v, e := b.Code.Read8(from)
						if e != nil {
							return false, e
						}
						if e := b.Code.Write8(from+1, v); e != nil {
							return false, e
						}
					}
					a[3] = NativeRequesterAddress{Address: b.CodeBase + uint32(s.Caret), Code: true}
					a[4] = NativeRequesterAddress{Address: b.CodeBase + uint32(s.Caret+1), Code: true}
					if e := b.Code.Write8(s.Caret, key); e != nil {
						return false, e
					}
					s.Caret++
					a[2] = NativeRequesterAddress{Address: b.CodeBase + uint32(s.Caret), Code: true}
					c.Word(4, uint16(c.D[4])+1)
					if uint16(c.D[4]) == uint16(c.D[3]) {
						c.Word(4, uint16(c.D[4])-1)
					}
				}
			}
		}
		c.Word(5, uint16(c.D[4]))
		if e := s.prepare(b, a); e != nil {
			return false, e
		}
	}
}
