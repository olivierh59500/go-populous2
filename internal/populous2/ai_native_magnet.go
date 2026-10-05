package populous2

import "fmt"

// MagnetMode translates $13dde/$13f9e. Town counts, raw marker/target/leader
// references and the native cached leader-power index drive its commands.
// The opponent-town comparison before MOVE.W leader is overwritten by that
// MOVE's flags; no additional population/radius condition is introduced.
func (r *NativeAIRules) MagnetMode(god, command int, cb NativeAICallbacks) (bool, error) {
	m := cb.Memory
	if r == nil || !winMemoryValid(m) {
		return false, fmt.Errorf("native AI magnet memory missing")
	}
	identity, err := m.Read16(god + 0x18)
	if err != nil {
		return false, err
	}
	enemy := 0xe8a4
	if identity == 1 {
		enemy = 0xe9de
	}
	minimum, err := r.codeWord(0x207e8)
	if err != nil {
		return false, err
	}
	frame := cb.Frame
	if frame != nil {
		frame.Word(0, minimum)
	}
	towns, err := m.Read16(god + 0x24)
	if err != nil {
		return false, err
	}
	mode, err := m.Read16(god + 0xc)
	if err != nil {
		return false, err
	}
	switchMode := func() (bool, error) {
		timer, err := r.codeWord(0x207f4)
		if err != nil {
			return false, err
		}
		if err := m.Write16(god+0x28, timer); err != nil {
			return false, err
		}
		lowPopulation, err := m.Read8(god + 7)
		if err != nil {
			return false, err
		}
		index := int(lowPopulation & 3)
		kind := uint8(14)
		if index == 2 {
			kind = 18
		}
		if index == 3 {
			kind = 20
		}
		if frame != nil {
			frame.D[0] = 1
		}
		return true, m.Write8(command+1, kind)
	}
	if int16(towns) <= int16(minimum) {
		if mode == 16 {
			return switchMode()
		}
		if frame != nil {
			frame.D[0] = 0
		}
		return false, nil
	}
	leader, err := m.Read16(god + 8)
	if err != nil {
		return false, err
	}
	if frame != nil {
		frame.Word(0, leader)
	}
	join := false
	if leader == 0 {
		if frame != nil {
			choice, e := m.Read16(god + 0x26)
			if e != nil {
				return false, e
			}
			frame.Word(0, choice)
			if choice != 0 {
				frame.Word(0, choice>>2)
			}
		}
		if err := r.clearLeaderChoice(god, m); err != nil {
			return false, err
		}
		join = mode != 16
	} else {
		actor := aiReferenceAddress(leader)
		target, err := m.Read16(god + 0x22)
		if err != nil {
			return false, err
		}
		marker, err := m.Read16(god + 0xa)
		if err != nil {
			return false, err
		}
		population, err := m.Read32(actor + 26)
		if err != nil {
			return false, err
		}
		if frame != nil {
			frame.D[3] = 4096
		}
		useEnemy := false
		if int32(population) >= 4096 {
			leaderChoices, err := m.Read16(god + 0x96)
			if err != nil {
				return false, err
			}
			if frame != nil {
				frame.Word(1, leaderChoices)
			}
			if leaderChoices != 0 {
				if cb.Random == nil {
					return false, fmt.Errorf("native AI leader choice RNG missing")
				}
				firstCount, err := m.Read16(god + 0x94)
				if err != nil {
					return false, err
				}
				bits := cb.Random()
				choice := (bits%leaderChoices + firstCount) * 4
				if frame != nil {
					frame.D[0] = uint32(bits)
					_ = frameDivide(frame, 0, leaderChoices)
					frame.Swap(0)
					frame.Word(0, (uint16(frame.D[0])+firstCount)*4)
				}
				old, err := m.Read16(god + 0x26)
				if err != nil {
					return false, err
				}
				if frame != nil {
					frame.Word(1, old)
					if old != 0 {
						frame.Word(1, old>>2)
					}
				}
				if old != 0 && old>>2 >= firstCount {
					useEnemy = true
				} else {
					if err := m.Write16(god+0x26, choice); err != nil {
						return false, err
					}
				}
			}
			if !useEnemy {
				if frame != nil {
					frame.D[3] = 8192
				}
				useEnemy = int32(population) >= 8192
			}
		}
		if useEnemy {
			available, err := m.Read16(enemy + 0x20)
			if err != nil {
				return false, err
			}
			if available == 0 {
				if frame != nil {
					frame.D[0] = 0
				}
				return false, nil
			}
			target, err = m.Read16(enemy + 0x22)
			if err != nil {
				return false, err
			}
		}
		targetActor, markerActor := aiReferenceAddress(target), aiReferenceAddress(marker)
		x, err := m.Read8(targetActor + 6)
		if err != nil {
			return false, err
		}
		y, err := m.Read8(targetActor + 8)
		if err != nil {
			return false, err
		}
		mx, err := m.Read8(markerActor + 6)
		if err != nil {
			return false, err
		}
		my, err := m.Read8(markerActor + 8)
		if err != nil {
			return false, err
		}
		if frame != nil {
			frame.Byte(0, mx)
			if x == mx {
				frame.Byte(0, my)
			}
		}
		if x != mx || y != my {
			if frame != nil {
				frame.D[0] = 1
			}
			return true, r.commandAtActor(command, 8, targetActor, m)
		}
		timer, err := m.Read16(god + 0x28)
		if err != nil {
			return false, err
		}
		if err := m.Write16(god+0x28, timer-1); err != nil {
			return false, err
		}
		if mode == 16 {
			if int16(timer) <= 1 {
				lx, err := m.Read8(actor + 6)
				if err != nil {
					return false, err
				}
				ly, err := m.Read8(actor + 8)
				if err != nil {
					return false, err
				}
				tile, err := m.Read8(0xf44 + int(int16(uint16(ly)<<8|uint16(uint8(lx<<2)))) + 1)
				if err != nil {
					return false, err
				}
				if r.Properties[tile]&1 == 0 {
					return switchMode()
				}
				if frame != nil {
					frame.Word(0, uint16(tile)*2)
				}
			}
		} else {
			if int16(timer) > 1 {
				if frame != nil {
					frame.D[0] = 0
				}
				return false, nil
			}
			join = true
		}
	}
	if join {
		timer, err := r.codeWord(0x207f2)
		if err != nil {
			return false, err
		}
		if err := m.Write16(god+0x28, timer); err != nil {
			return false, err
		}
		if err := m.Write8(command+1, 16); err != nil {
			return false, err
		}
	}
	marker, err := m.Read16(god + 0xa)
	if err != nil {
		return false, err
	}
	at := aiReferenceAddress(marker)
	x, err := m.Read8(at + 6)
	if err != nil {
		return false, err
	}
	y, err := m.Read8(at + 8)
	if err != nil {
		return false, err
	}
	tile, err := m.Read8(0xf44 + int(int16(uint16(y)<<8|uint16(uint8(x<<2)))) + 1)
	if err != nil {
		return false, err
	}
	if r.Properties[tile]&0x790 != 0 {
		if err := r.commandAtActor(command, 2, at, m); err != nil {
			return false, err
		}
		if err := m.Write16(god+0x28, 0); err != nil {
			return false, err
		}
	}
	if frame != nil {
		frame.Word(1, r.Properties[tile]&0x790)
	}
	pending, err := m.Read8(command + 1)
	return pending != 0, err
}

func (r *NativeAIRules) clearLeaderChoice(god int, m FollowerCleanupMemory) error {
	choice, err := m.Read16(god + 0x26)
	if err != nil {
		return err
	}
	if choice == 0 {
		return nil
	}
	count, err := m.Read16(god + 0x94)
	if err != nil {
		return err
	}
	if choice>>2 >= count {
		return m.Write16(god+0x26, 0)
	}
	return nil
}
