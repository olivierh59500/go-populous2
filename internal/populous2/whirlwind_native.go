package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

// NativeWhirlwindRules retains the original word tables and animation markers.
// Its controllers mutate retained records, including recycled and aliased bytes.
type NativeWhirlwindRules struct {
	Neighbors       [16]uint16
	Vectors         [16][2]int16
	Raster          [256]uint8
	ReleaseOffsets  [8]uint16
	Whirlpool       WhirlpoolRules
	WhirlpoolChance uint16
	code            []byte
}

func DecodeNativeWhirlwindRules(exe *amiga.Executable) (NativeWhirlwindRules, error) {
	var r NativeWhirlwindRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33612 {
		return r, fmt.Errorf("native whirlwind tables missing")
	}
	r.code = exe.Hunks[0].Data
	var err error
	r.Whirlpool, err = DecodeWhirlpoolRules(exe)
	if err != nil {
		return NativeWhirlwindRules{}, err
	}
	r.WhirlpoolChance = binary.BigEndian.Uint16(r.code[0x20d78:])
	if r.WhirlpoolChance == 0 {
		return NativeWhirlwindRules{}, fmt.Errorf("native whirlwind water modulus is zero")
	}
	copy(r.Raster[:], r.code[0x33512:0x33612])
	for i := range r.Neighbors {
		r.Neighbors[i] = binary.BigEndian.Uint16(r.code[0x20dd6+i*2:])
		r.Vectors[i] = [2]int16{int16(binary.BigEndian.Uint16(r.code[0x20df6+i*4:])), int16(binary.BigEndian.Uint16(r.code[0x20df8+i*4:]))}
	}
	for i := range r.ReleaseOffsets {
		r.ReleaseOffsets[i] = binary.BigEndian.Uint16(r.code[0x20efa+i*2:])
	}
	return r, nil
}

type NativeWhirlwindCallbacks struct {
	Memory FollowerCleanupMemory
	Frame  *NativeFrameRegisterContext
	Random func() uint16
	// SourceD2 preserves the dispatcher's incoming low word. A reroute replaces
	// it with DBF's $ffff; water birth replaces only its low owner byte.
	SourceD2   uint16
	Move       func(NativeRecordReference, uint16, uint16) (bool, error)
	Unlink     func(NativeRecordReference) error
	ClearFarms func(NativeRecordReference, uint8) error
	// Cleanup executes complete $124a2 and returns its actual D1. Leader
	// relocation can replace D1's low byte with Y; release retains that value.
	Cleanup func(NativeRecordReference, FollowerCleanupRegisters) (FollowerCleanupStep, error)
}

type NativeWhirlwindStep struct {
	Advanced, Routed, Moved, Expired, Removed bool
	RandomDraws, Visited, Lifted, Towns       int
	WaterAttempt, WaterCreated                bool
	WaterReference                            NativeRecordReference
	WaterOwnerWord                            uint16
	Release                                   WhirlwindReleaseResult
}

// nativeWhirlwindMemory keeps the first failed raw access. Later writes cannot
// turn a failed native address into a fabricated zero-filled record.
type nativeWhirlwindMemory struct {
	m   FollowerCleanupMemory
	err error
}

func (m *nativeWhirlwindMemory) byte(at int) uint8 {
	if m.err != nil {
		return 0
	}
	v, err := m.m.Read8(at)
	m.err = err
	return v
}
func (m *nativeWhirlwindMemory) word(at int) uint16 {
	if m.err != nil {
		return 0
	}
	v, err := m.m.Read16(at)
	m.err = err
	return v
}
func (m *nativeWhirlwindMemory) putByte(at int, v uint8) {
	if m.err == nil {
		m.err = m.m.Write8(at, v)
	}
}
func (m *nativeWhirlwindMemory) putWord(at int, v uint16) {
	if m.err == nil {
		m.err = m.m.Write16(at, v)
	}
}

func (r *NativeWhirlwindRules) codeWord(at int) (int16, error) {
	if r == nil || at < 0 || at&1 != 0 || at+2 > len(r.code) {
		return 0, fmt.Errorf("native whirlwind CODE word outside retained image: %x", at)
	}
	return int16(binary.BigEndian.Uint16(r.code[at:])), nil
}

func nativeWhirlwindGrid(tile uint16) int {
	return 0xf44 + int(int16(tile&0xff00|uint16(uint8(tile)<<2)))
}

func (r *NativeWhirlwindRules) advance(m *nativeWhirlwindMemory, at int, loop bool, frame *NativeFrameRegisterContext) (bool, error) {
	next := m.word(at+10) + 4
	if frame != nil {
		frame.Word(0, next)
	}
	marker, err := r.codeWord(0x23d1a + int(int16(next)))
	if m.err != nil {
		return false, m.err
	}
	if err != nil {
		return false, err
	}
	if marker < 0 {
		if !loop {
			return false, nil
		}
		next += uint16(marker)
		if frame != nil {
			frame.Word(0, next)
		}
	}
	m.putWord(at+10, next)
	return true, m.err
}

// CreateWaterChild is original $15cc8, including its four exact-zero parcels,
// owner-byte allocation, stale record preservation and initial terrain stamp.
// The new kind 24 record is deliberately absent from the occupancy graph.
func (r *NativeWhirlwindRules) CreateWaterChild(owner uint16, x, y uint8, memory FollowerCleanupMemory) (NativePrimitiveCreation, error) {
	return r.createWaterChild(owner, x, y, memory, nil)
}

func (r *NativeWhirlwindRules) createWaterChild(owner uint16, x, y uint8, memory FollowerCleanupMemory, frame *NativeFrameRegisterContext) (NativePrimitiveCreation, error) {
	var result NativePrimitiveCreation
	if r == nil || !winMemoryValid(memory) {
		return result, fmt.Errorf("native whirlwind water memory missing")
	}
	m := nativeWhirlwindMemory{m: memory}
	origin := uint16(y)<<8 | uint16(x)
	if frame != nil {
		frame.Byte(3, y)
		frame.Word(3, uint16(frame.D[3])<<8)
		frame.Byte(3, x)
	}
	for _, offset := range r.Whirlpool.Footprint {
		parcel := origin + offset
		if frame != nil {
			frame.Word(4, parcel)
			frame.Word(5, parcel&0xc0c0)
		}
		if parcel&0xc0c0 != 0 {
			return result, nil
		}
		if frame != nil {
			frame.Byte(4, uint8(parcel)*4)
		}
		if m.byte(nativeWhirlwindGrid(parcel)+1) != 0 {
			return result, m.err
		}
	}
	if frame != nil {
		frame.Word(4, 0xff9d)
	}
	if m.err != nil {
		return result, m.err
	}
	at, err := primitiveFreeRecord(memory, 0xc800, 0xe740, 32)
	if err != nil || at == 0 {
		return result, err
	}
	for _, write := range []struct {
		offset int
		value  uint8
	}{{12, uint8(owner)}, {6, x}, {7, 0}, {8, y}, {9, 0}, {0, 0x24}, {22, 0x0e}} {
		m.putByte(at+write.offset, write.value)
	}
	xp := uint8(0)
	if int16(owner) <= 2 {
		xp = m.byte(primitiveDeityAddress(owner) + 0x57)
	}
	m.putWord(at+24, uint16(r.Whirlpool.BaseLife)+uint16(xp))
	m.putByte(at+18, r.Whirlpool.Speed)
	m.putWord(at+20, uint16(r.Whirlpool.Speed))
	m.putWord(at+10, 0x97)
	if frame != nil {
		frame.Word(3, origin)
	}
	for i, offset := range r.Whirlpool.Footprint {
		grid := nativeWhirlwindGrid(origin + offset)
		if r.Whirlpool.TileProperties[m.byte(grid+1)]&8 != 0 {
			m.putByte(grid+1, uint8(0x98+i))
		}
	}
	result.Reference = NativeRecordReference(uint16(at - 0x76c0))
	result.Created = m.err == nil
	return result, m.err
}

func (r *NativeWhirlwindRules) route(at int, cb NativeWhirlwindCallbacks, step *NativeWhirlwindStep) error {
	m := nativeWhirlwindMemory{m: cb.Memory}
	origin := m.word(at+8)&0xff00 | uint16(m.byte(at+6))
	height := uint16(m.byte(nativeWhirlwindGrid(origin)) & 7)
	frame := cb.Frame
	if frame != nil {
		frame.Word(6, origin)
		frame.Word(3, uint16(nativeWhirlwindGrid(origin)-0xf44))
		frame.Byte(3, uint8(height))
	}
	if m.err != nil {
		return m.err
	}
	bits := cb.Random()
	step.RandomDraws++
	if frame != nil {
		frame.D[0] = uint32(bits)
		frame.Word(4, bits)
		frame.Word(0, bits&0x0e)
		frame.Word(0, uint16(frame.D[0])*2)
		frame.Word(5, uint16(frame.D[0]))
		frame.D[2] = 7
		frame.D[7] = 0
	}
	start, selected := int(bits&0x0e)/2, uint16(0)
	for i := 0; i < 8; i++ {
		parcel := origin + r.Neighbors[start+i]
		if frame != nil {
			frame.Word(1, parcel)
			frame.Word(0, parcel&0xc0c0)
			frame.Word(5, uint16((start+i+1)*4))
		}
		if parcel&0xc0c0 != 0 {
			continue
		}
		grid := nativeWhirlwindGrid(parcel)
		tile := m.byte(grid + 1)
		candidate := uint16(m.byte(grid)&7) + uint16(r.Raster[tile]&1)
		if frame != nil {
			frame.Byte(1, uint8(parcel)*4)
			frame.Byte(0, m.byte(grid)&7)
			frame.Byte(7, tile)
			if r.Raster[tile]&1 != 0 {
				frame.Word(0, uint16(frame.D[0])+1)
			}
		}
		if candidate > height {
			continue
		}
		if candidate == height {
			accept := bits&1 != 0
			bits >>= 1
			if frame != nil {
				frame.Word(4, bits)
			}
			if !accept {
				continue
			}
		}
		height, selected = candidate, uint16((start+i)*4)
		if frame != nil {
			frame.Word(3, uint16(frame.D[0]))
		}
	}
	if frame != nil {
		frame.Word(2, 0xffff)
	}
	if selected == 0 {
		selected = bits & 0x3c
		if frame != nil {
			frame.Word(4, selected)
		}
	}
	speed := m.byte(at + 18)
	if m.err != nil {
		return m.err
	}
	if speed == 0 {
		return fmt.Errorf("native whirlwind route division by zero")
	}
	vector := r.Vectors[selected/4]
	if frame != nil {
		frame.D[1] = uint32(speed)
		frame.D[0] = uint32(int32(vector[1]) * int32(speed))
		frame.D[0] = 255
		if err := frameDivide(frame, 0, uint16(speed)); err != nil {
			return err
		}
	}
	m.putWord(at+14, uint16(int32(vector[0])*int32(speed)))
	m.putWord(at+16, uint16(int32(vector[1])*int32(speed)))
	// $14b34's 255/speed is immediately replaced by the second random draw.
	randomTimer := cb.Random()
	if frame != nil {
		frame.D[0] = uint32(randomTimer)
		frame.Word(0, randomTimer&0x78)
	}
	m.putWord(at+20, randomTimer&0x78)
	step.RandomDraws++
	step.Routed = true
	return m.err
}

// Lift is $16132. The hero selector is a signed byte offset into retained CODE,
// so valid native adjacent-table reads are preserved rather than normalized.
func (r *NativeWhirlwindRules) Lift(effect, target NativeRecordReference, memory FollowerCleanupMemory) (bool, error) {
	if r == nil || !winMemoryValid(memory) {
		return false, fmt.Errorf("native whirlwind lift memory missing")
	}
	m := nativeWhirlwindMemory{m: memory}
	at := cleanupRecordAddress(target)
	if m.byte(at+22) == 0x3a {
		return false, m.err
	}
	animation := uint16(0x4d4)
	if m.byte(at+13)&2 != 0 {
		selector := m.word(at + 40)
		word, err := r.codeWord(0x20a6c + int(int16(selector)))
		if m.err != nil {
			return false, m.err
		}
		if err != nil {
			return false, err
		}
		animation = uint16(word)
		if animation == 0 {
			return false, nil
		}
	}
	m.putWord(at+10, animation)
	m.putByte(at, 8)
	m.putByte(at+22, 0x14)
	m.putByte(at+25, 1)
	m.putWord(at+32, uint16(effect))
	return m.err == nil, m.err
}

func (r *NativeWhirlwindRules) pickup(effect NativeRecordReference, cb NativeWhirlwindCallbacks, step *NativeWhirlwindStep) error {
	m := nativeWhirlwindMemory{m: cb.Memory}
	at := cleanupRecordAddress(effect)
	tile := m.word(at+8)&0xff00 | uint16(m.byte(at+6))
	head, a2 := m.word(nativeWhirlwindGrid(tile)+2), 0xf44
	if cb.Frame != nil {
		cb.Frame.Word(0, head)
	}
	seen := map[uint16]bool{}
	for head != 0 && m.err == nil {
		if seen[head] {
			return fmt.Errorf("cyclic native whirlwind pickup chain")
		}
		seen[head] = true
		ref, target := NativeRecordReference(head), cleanupRecordAddress(NativeRecordReference(head))
		step.Visited++
		kind := m.byte(target)
		if kind == 2 || kind == 0x0a {
			lifted, err := r.Lift(effect, ref, cb.Memory)
			if err != nil {
				return err
			}
			if lifted {
				step.Lifted++
			}
		}
		if m.byte(target) == 4 && m.byte(a2+22) != 0x30 {
			a2 = target
			if cb.Frame != nil {
				cb.Frame.Byte(2, 15)
			}
			if cb.ClearFarms == nil {
				return fmt.Errorf("native whirlwind town cleanup missing")
			}
			if err := cb.ClearFarms(ref, 15); err != nil {
				return err
			}
			step.Towns++
			lifted, err := r.Lift(effect, ref, cb.Memory)
			if err != nil {
				return err
			}
			if lifted {
				step.Lifted++
			}
		}
		// Native lift preserves A2. Later towns therefore test the previous
		// town's state, and the first town tests the raw grid-prefix byte22.
		head = m.word(target + 2)
		if cb.Frame != nil {
			cb.Frame.Word(0, head)
		}
	}
	return m.err
}

// Release is $14c20. It clears/unlinks the effect before saving the old cell's
// head and scans every kind 8, independently of its owner or source reference.
func (r *NativeWhirlwindRules) Release(effect NativeRecordReference, cb NativeWhirlwindCallbacks) (WhirlwindReleaseResult, error) {
	var result WhirlwindReleaseResult
	if r == nil || !winMemoryValid(cb.Memory) || cb.Random == nil || cb.Move == nil || cb.Unlink == nil || cb.Cleanup == nil {
		return result, fmt.Errorf("native whirlwind release callbacks missing")
	}
	m := nativeWhirlwindMemory{m: cb.Memory}
	at := cleanupRecordAddress(effect)
	base := m.word(at+8)&0xff00 | uint16(m.byte(at+6))
	if cb.Frame != nil {
		cb.Frame.Word(1, base)
	}
	result.CoordinateBase, result.Source = NativePackedTile(base), WhirlwindWordSource{Kind: WhirlwindReleaseTable}
	m.putByte(at+12, 0)
	if m.err != nil {
		return result, m.err
	}
	if err := cb.Unlink(effect); err != nil {
		return result, err
	}
	head := m.word(nativeWhirlwindGrid(base) + 2)
	if cb.Frame != nil {
		cb.Frame.Word(0, head)
	}
	seen := map[uint16]bool{}
	for head != 0 && m.err == nil {
		if seen[head] {
			return result, fmt.Errorf("cyclic native whirlwind release chain")
		}
		seen[head] = true
		ref, target := NativeRecordReference(head), cleanupRecordAddress(NativeRecordReference(head))
		next := m.word(target + 2)
		if cb.Frame != nil {
			cb.Frame.Word(5, next)
		}
		result.Visited++
		if m.byte(target) == 8 {
			selector := cb.Random() & 0x0e
			if cb.Frame != nil {
				cb.Frame.D[0] = uint32(selector)
			}
			offset := uint16(0)
			switch result.Source.Kind {
			case WhirlwindReleaseTable:
				offset = r.ReleaseOffsets[selector/2]
			case WhirlwindOccupancyPrefix:
				offset = m.word(0xf46 + int(selector))
			case WhirlwindRecordPrefix:
				offset = m.word(cleanupRecordAddress(result.Source.Reference) + int(selector))
			}
			destination := uint16(result.CoordinateBase) + offset
			if cb.Frame != nil {
				cb.Frame.Word(0, destination)
				cb.Frame.Word(2, destination&0xc0c0)
			}
			if m.err != nil {
				return result, m.err
			}
			if destination&0xc0c0 != 0 {
				result.Source = WhirlwindWordSource{Kind: WhirlwindRecordPrefix, Reference: ref}
				registers := FollowerCleanupRegisters{D1: uint32(result.CoordinateBase), D2: uint32(destination & 0xc0c0)}
				if cb.Frame != nil {
					cb.Frame.D[0] = 0
					registers = FollowerCleanupRegisters{D0: cb.Frame.D[0], D1: cb.Frame.D[1], D2: cb.Frame.D[2]}
				}
				cleanup, err := cb.Cleanup(ref, registers)
				if err != nil {
					return result, err
				}
				if cb.Frame != nil {
					cb.Frame.D[0], cb.Frame.D[1], cb.Frame.D[2] = cleanup.Registers.D0, cleanup.Registers.D1, cleanup.Registers.D2
				}
				result.CoordinateBase = NativePackedTile(uint16(cleanup.Registers.D1))
				result.Removed++
			} else {
				x, y := uint16(uint8(destination))<<8|128, destination&0xff00|128
				if cb.Frame != nil {
					cb.Frame.Byte(6, uint8(destination))
					cb.Frame.Word(6, uint16(cb.Frame.D[6])<<8)
					cb.Frame.Byte(6, 128)
					cb.Frame.Word(7, destination)
					cb.Frame.Byte(7, 128)
					if err := cb.Frame.ObserveMove(ref, x, y, cb.Memory); err != nil {
						return result, err
					}
				}
				changed, err := cb.Move(ref, x, y)
				if err != nil {
					return result, err
				}
				if changed {
					result.Source = WhirlwindWordSource{Kind: WhirlwindOccupancyPrefix}
				}
				m.putByte(target+22, 0x1a)
				m.putWord(target+10, 0x68c)
				result.Released++
			}
		}
		head = next
		if cb.Frame != nil {
			cb.Frame.Word(0, next)
		}
	}
	return result, m.err
}

// Tick dispatches $14a24/$14a50/$14c02. Expiry advances its first outro frame
// through $149fa; the later terminal frame alone performs follower release.
func (r *NativeWhirlwindRules) Tick(ref NativeRecordReference, cb NativeWhirlwindCallbacks) (NativeWhirlwindStep, error) {
	var step NativeWhirlwindStep
	if r == nil || !winMemoryValid(cb.Memory) || cb.Random == nil || cb.Move == nil || cb.Unlink == nil {
		return step, fmt.Errorf("native whirlwind controller callbacks missing")
	}
	m := nativeWhirlwindMemory{m: cb.Memory}
	at := cleanupRecordAddress(ref)
	if m.byte(at+12) == 0 {
		return step, m.err
	}
	state := m.byte(at + 22)
	if state == 8 {
		advanced, err := r.advance(&m, at, false, cb.Frame)
		if err != nil {
			return step, err
		}
		if advanced {
			step.Advanced = true
			return step, nil
		}
		m.putWord(at+10, 0x4c8)
		m.putByte(at+22, 10)
		state = 10
	}
	if state == 10 {
		life := m.word(at + 24)
		m.putWord(at+24, life-1)
		// SUBI.W/BGT tests the signed value before subtraction, including $8000.
		if int16(life) <= 1 {
			step.Expired = true
			m.putByte(at+22, 12)
			m.putWord(at+10, 0x6cc)
			advanced, err := r.advance(&m, at, false, cb.Frame)
			if err != nil {
				return step, err
			}
			step.Advanced = advanced
			if !advanced {
				m.putByte(at+12, 0)
				if m.err != nil {
					return step, m.err
				}
				step.Removed = true
				return step, cb.Unlink(ref)
			}
			return step, m.err
		}
		advanced, err := r.advance(&m, at, true, cb.Frame)
		if err != nil {
			return step, err
		}
		step.Advanced = advanced
		timer := m.word(at + 20)
		m.putWord(at+20, timer-1)
		if m.err != nil {
			return step, m.err
		}
		if int16(timer) <= 1 {
			if err := r.route(at, cb, &step); err != nil {
				return step, err
			}
		}
		x, y := m.word(at+6)+m.word(at+14), m.word(at+8)+m.word(at+16)
		if m.err != nil {
			return step, m.err
		}
		if cb.Frame != nil {
			cb.Frame.Word(6, x)
			cb.Frame.Word(7, m.word(at+8))
			if int16(x) >= 0 {
				cb.Frame.Word(7, y)
			}
		}
		if int16(x) < 0 || int16(y) < 0 || int16(x) >= 0x4000 || int16(y) >= 0x4000 {
			step.Release, err = r.Release(ref, cb)
			step.Removed = true
			step.RandomDraws += step.Release.Released + step.Release.Removed
			return step, err
		}
		if cb.Frame != nil {
			if err := cb.Frame.ObserveMove(ref, x, y, cb.Memory); err != nil {
				return step, err
			}
		}
		_, err = cb.Move(ref, x, y)
		if err != nil {
			return step, err
		}
		step.Moved = true
		bits := cb.Random()
		step.RandomDraws++
		if cb.Frame != nil {
			cb.Frame.D[0] = uint32(bits)
			if err := frameDivide(cb.Frame, 0, r.WhirlpoolChance); err != nil {
				return step, err
			}
			cb.Frame.Swap(0)
		}
		if bits%r.WhirlpoolChance == 0 {
			step.WaterAttempt = true
			sourceD2 := cb.SourceD2
			if cb.Frame != nil {
				sourceD2 = uint16(cb.Frame.D[2])
			}
			ownerWord := sourceD2&0xff00 | uint16(m.byte(at+12))
			if step.Routed {
				ownerWord = 0xff00 | uint16(m.byte(at+12))
			}
			step.WaterOwnerWord = ownerWord
			if cb.Frame != nil {
				cb.Frame.Byte(0, m.byte(at+6))
				cb.Frame.Byte(1, m.byte(at+8))
				cb.Frame.Byte(2, m.byte(at+12))
			}
			child, err := r.createWaterChild(ownerWord, m.byte(at+6), m.byte(at+8), cb.Memory, cb.Frame)
			if err != nil {
				return step, err
			}
			step.WaterCreated, step.WaterReference = child.Created, child.Reference
		}
		return step, r.pickup(ref, cb, &step)
	}
	if state == 12 {
		advanced, err := r.advance(&m, at, false, cb.Frame)
		if err != nil {
			return step, err
		}
		step.Advanced = advanced
		if !advanced {
			step.Release, err = r.Release(ref, cb)
			step.Removed = true
			step.RandomDraws += step.Release.Released + step.Release.Removed
			return step, err
		}
		return step, m.err
	}
	return step, fmt.Errorf("unknown native whirlwind state %02x", state)
}

type NativeWhirlwindFollowerStep struct{ Moving, ReadyNextUpdate bool }

// TickFollower is $11c72/$11ce8. Transport reads raw source words without an
// owner/kind guard and moves on every call. Landing makes a walker whose
// ordinary decision starts on the following update; it retains weapon 1.
func (r *NativeWhirlwindRules) TickFollower(ref NativeRecordReference, cb NativeWhirlwindCallbacks) (NativeWhirlwindFollowerStep, error) {
	var step NativeWhirlwindFollowerStep
	if r == nil || !winMemoryValid(cb.Memory) || cb.Move == nil {
		return step, fmt.Errorf("native lifted follower callbacks missing")
	}
	m := nativeWhirlwindMemory{m: cb.Memory}
	at := cleanupRecordAddress(ref)
	if m.byte(at+12) == 0 {
		return step, m.err
	}
	switch m.byte(at + 22) {
	case 0x14:
		if _, err := r.advance(&m, at, true, cb.Frame); err != nil {
			return step, err
		}
		source := cleanupRecordAddress(NativeRecordReference(m.word(at + 32)))
		x, y := m.word(source+6), m.word(source+8)
		if m.err != nil {
			return step, m.err
		}
		if cb.Frame != nil {
			cb.Frame.Word(6, x)
			cb.Frame.Word(7, y)
			if err := cb.Frame.ObserveMove(ref, x, y, cb.Memory); err != nil {
				return step, err
			}
		}
		if _, err := cb.Move(ref, x, y); err != nil {
			return step, err
		}
		step.Moving = true
	case 0x1a:
		advanced, err := r.advance(&m, at, false, cb.Frame)
		if err != nil {
			return step, err
		}
		if !advanced {
			m.putByte(at, 2)
			m.putWord(at+10, 0)
			m.putByte(at+22, 2)
			step.ReadyNextUpdate = true
		}
	}
	return step, m.err
}
