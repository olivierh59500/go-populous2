package populous2

// advanceModal translates the actual $4c2e input loop with its surviving
// A3/A4 edit pointers. The existing $4d12 display body retains raw bytes,
// register saves, real VBlank waits and the original shared screen swap.
func (s *NativeCampaignSelectionFrameState) advanceModal(b nativeRequesterFrameBacking, p *NativeFramePresentationState, keys NativeInputRules) (bool, error) {
	c := b.Frame
	ca := func(at int) NativeRequesterAddress {
		return NativeRequesterAddress{Address: b.CodeBase + uint32(at), Code: true}
	}
	for {
		if s.Modal.Waiting {
			ready, e := b.Memory.Read16(0xa)
			if e != nil {
				return false, e
			}
			if ready == 0 {
				return false, nil
			}
			if e = b.text(s.Modal.Target, s.Modal.Display); e != nil {
				return false, e
			}
			if e = fileFrameSwap(b, p); e != nil {
				return false, e
			}
			if e = b.Code.Write8(s.Modal.Display+int(s.Modal.Capacity), s.Modal.Tail); e != nil {
				return false, e
			}
			c.D = s.Modal.Saved
			s.Modal.Waiting = false
			if s.Modal.Finish != 0 {
				if s.Modal.Finish == 1 {
					c.D[0] = 0
				}
				s.Modal.Active = false
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
			address := s.ModalA
			_, e := campaignRequesterClick(b, &address)
			if e != nil {
				return false, e
			}
			d0 := c.D[0]
			c.D = saved
			c.D[0] = d0
			s.ModalA[6] = address[0]
			if uint16(d0) != 0 {
				c.D[5] = 0xffffffff
				s.Modal.Finish = 2
				s.ModalA[0] = address[0]
				if e = s.Modal.prepare(b); e != nil {
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
			if e = b.Memory.Write8(0x93, 0); e != nil {
				return false, e
			}
			if s.Modal.Field != s.Modal.Caret {
				s.Modal.Caret--
				c.Word(4, uint16(c.D[4])-1)
				if int16(c.D[4]) < 0 {
					c.D[4] = 0
				}
			}
		} else if rightKey != 0 {
			if e = b.Memory.Write8(0x95, 0); e != nil {
				return false, e
			}
			v, e := b.Code.Read8(s.Modal.Caret)
			if e != nil {
				return false, e
			}
			if v != 0 {
				s.Modal.Caret++
				c.Word(4, uint16(c.D[4])+1)
				if uint16(c.D[4]) == uint16(c.D[3]) {
					c.Word(4, uint16(c.D[4])-1)
				}
			}
		} else {
			key, e := keys.Character(&p.Input, &c.D)
			if e != nil {
				return false, e
			}
			switch key {
			case 0:
			case 13:
				c.D[5] = 0xffffffff
				s.Modal.Finish = 1
				if e = s.Modal.prepare(b); e != nil {
					return false, e
				}
				continue
			case 8:
				if s.Modal.Field != s.Modal.Caret {
					from, to := s.Modal.Caret, s.Modal.Caret-1
					s.Modal.Caret--
					for {
						v, e := b.Code.Read8(from)
						if e != nil {
							return false, e
						}
						from++
						if e = b.Code.Write8(to, v); e != nil {
							return false, e
						}
						to++
						if v == 0 {
							break
						}
					}
					s.ModalA[3], s.ModalA[4] = ca(from), ca(to)
					c.Word(4, uint16(c.D[4])-1)
					if int16(c.D[4]) < 0 {
						c.D[4] = 0
					}
				}
			default:
				end := s.Modal.Caret
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
				s.ModalA[3] = ca(end)
				sentinel, e := b.Code.Read8(end)
				if e != nil {
					return false, e
				}
				if sentinel != 0xff {
					from, to := end, end+1
					for {
						from--
						to--
						v, e := b.Code.Read8(from)
						if e != nil {
							return false, e
						}
						if e = b.Code.Write8(to, v); e != nil {
							return false, e
						}
						if from == s.Modal.Caret {
							break
						}
					}
					s.ModalA[3], s.ModalA[4] = ca(from), ca(to)
					if e = b.Code.Write8(s.Modal.Caret, key); e != nil {
						return false, e
					}
					s.Modal.Caret++
					c.Word(4, uint16(c.D[4])+1)
					if uint16(c.D[4]) == uint16(c.D[3]) {
						c.Word(4, uint16(c.D[4])-1)
					}
				}
			}
		}
		c.Word(5, uint16(c.D[4]))
		if e = s.Modal.prepare(b); e != nil {
			return false, e
		}
	}
}
