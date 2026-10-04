package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type EarthquakeRules struct {
	BaseStrength, InitialTimer, FadeLife uint16
	Speed                                uint8
	Directions                           [4]uint8
	Branches                             [16][6]uint8
	Offsets                              [16][2]int8
	Fade                                 [25]uint8
	Raster                               [256]uint8
}

func DecodeEarthquakeRules(exe *amiga.Executable) (EarthquakeRules, error) {
	var r EarthquakeRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33612 {
		return r, fmt.Errorf("native earthquake tables missing")
	}
	c := exe.Hunks[0].Data
	r.BaseStrength = binary.BigEndian.Uint16(c[0x20d66:])
	r.InitialTimer = binary.BigEndian.Uint16(c[0x20d68:])
	r.Speed = c[0x20d6b]
	r.FadeLife = binary.BigEndian.Uint16(c[0x20d6c:])
	copy(r.Directions[:], c[0x17962:0x17966])
	copy(r.Fade[:], c[0x20eb6:0x20ecf])
	copy(r.Raster[:], c[0x33512:0x33612])
	for i := range r.Branches {
		copy(r.Branches[i][:], c[0x20e36+i*8:0x20e36+i*8+6])
		r.Offsets[i] = [2]int8{int8(c[0x20e36+i*8+6]), int8(c[0x20e36+i*8+7])}
	}
	return r, nil
}

type EarthquakeCallbacks struct {
	Memory       FollowerCleanupMemory
	Tile, Header func(int, int) (uint8, error)
	SetTile      func(int, int, uint8) error
	// Lower is the unpriced original $d7f0 primitive, including recursive
	// propagation and its map/header reconstruction, not player admission.
	Lower            func(int, int) error
	Random           func() int
	Dirty, Shake     func() error
	CameraX, CameraY uint8
}

type EarthquakeCreation struct {
	Address           int
	Allocated, Active bool
}
type EarthquakeStep struct {
	Handled, Removed bool
	ChildAddress     int
	Branched         bool
}

func quakeCallbacksValid(cb EarthquakeCallbacks) bool {
	return cb.Memory.Read8 != nil && cb.Memory.Read16 != nil && cb.Memory.Write8 != nil && cb.Memory.Write16 != nil && cb.Tile != nil && cb.Header != nil && cb.SetTile != nil && cb.Lower != nil
}

// Create translates $165da. CallerAddress is incoming A0, needed because an
// invalid coordinate returns that exact pointer. A full scan returns $e740,
// which the caller's pointer-order compensation may subsequently modify.
// Byte0, flags, velocities, links and other stale fields are not reset.
func (r *EarthquakeRules) Create(owner, x, y, direction uint8, strength uint16, callerAddress int, cb EarthquakeCallbacks) (EarthquakeCreation, error) {
	result := EarthquakeCreation{Address: callerAddress}
	if r == nil || !quakeCallbacksValid(cb) {
		return result, fmt.Errorf("native earthquake callbacks missing")
	}
	if int8(x) < 0 || int8(y) < 0 || x >= 64 || y >= 64 {
		return result, nil
	}
	if direction >= 16 {
		return result, fmt.Errorf("native earthquake direction outside table")
	}
	m := cb.Memory
	address := 0xc800
	for ; address < 0xe740; address += 32 {
		owner, err := m.Read8(address + 12)
		if err != nil {
			return result, err
		}
		if owner == 0 {
			break
		}
	}
	result.Address = address
	if address == 0xe740 {
		return result, nil
	}
	result.Allocated = true
	for _, field := range []struct {
		offset int
		value  uint8
	}{{12, owner}, {6, x}, {8, y}, {7, 0}, {9, 0}, {22, 0x22}, {26, direction}} {
		if err := m.Write8(address+field.offset, field.value); err != nil {
			return result, err
		}
	}
	for _, field := range []struct {
		offset int
		value  uint16
	}{{10, uint16(direction >> 1)}, {20, r.InitialTimer}} {
		if err := m.Write16(address+field.offset, field.value); err != nil {
			return result, err
		}
	}
	if err := m.Write8(address+18, r.Speed); err != nil {
		return result, err
	}
	if err := m.Write16(address+24, strength); err != nil {
		return result, err
	}
	height, err := r.surface(address, cb)
	if err != nil {
		return result, err
	}
	if height == 0 {
		if cb.Dirty != nil {
			if err := cb.Dirty(); err != nil {
				return result, err
			}
		}
		if err := cb.SetTile(int(x), int(y), 0xac+(direction>>1)); err != nil {
			return result, err
		}
	}
	active, err := m.Read8(address + 12)
	result.Active = active != 0
	return result, err
}

// surface translates $16694. A nonflat height>1 is lowered once but the
// original pre-lowering height is returned. Flat cells paint the current
// crack descriptor, while an existing $ac..$c4 crack returns zero untouched.
func (r *EarthquakeRules) surface(address int, cb EarthquakeCallbacks) (int, error) {
	m := cb.Memory
	x, err := m.Read8(address + 6)
	if err != nil {
		return 0, err
	}
	y, err := m.Read8(address + 8)
	if err != nil {
		return 0, err
	}
	tile, err := cb.Tile(int(x), int(y))
	if err != nil {
		return 0, err
	}
	if tile >= 0xac && tile <= 0xc4 {
		return 0, nil
	}
	if r.Raster[tile]&15 == 15 {
		descriptor, err := m.Read8(address + 11)
		if err != nil {
			return 0, err
		}
		return 0, cb.SetTile(int(x), int(y), descriptor+0xac)
	}
	header, err := cb.Header(int(x), int(y))
	if err != nil {
		return 0, err
	}
	height := int(header&7) + int(r.Raster[tile]&1)
	if height == 0 {
		return -1, m.Write8(address+12, 0)
	}
	if height > 1 {
		if err := cb.Lower(int(x), int(y)); err != nil {
			return 0, err
		}
	}
	return height, nil
}

func quakeCamera(address int, cb EarthquakeCallbacks) error {
	if cb.Shake == nil {
		return nil
	}
	x, err := cb.Memory.Read8(address + 6)
	if err != nil {
		return err
	}
	y, err := cb.Memory.Read8(address + 8)
	if err != nil {
		return err
	}
	dx, dy := int(cb.CameraX)-int(x), int(cb.CameraY)-int(y)
	if dx <= 0 && dy <= 0 && dx+8 > 0 && dy+8 > 0 {
		return cb.Shake()
	}
	return nil
}

func quakeDecrement(address, offset int, m FollowerCleanupMemory) (uint16, bool, error) {
	old, err := m.Read16(address + offset)
	if err != nil {
		return 0, false, err
	}
	next := old - 1
	return next, int16(old) > 1, m.Write16(address+offset, next)
}

// Tick translates $152ea/$153a4/$153f0 for raw states$22/$24/$26. The native
// pool dispatcher uses state+$16, not stale kind byte0. Map-based crack art
// remains after the actor expires; no allocation or occupancy unlink occurs.
func (r *EarthquakeRules) Tick(address int, cb EarthquakeCallbacks) (EarthquakeStep, error) {
	var step EarthquakeStep
	if r == nil || !quakeCallbacksValid(cb) {
		return step, fmt.Errorf("native earthquake callbacks missing")
	}
	if address < 0xc800 || address >= 0xe740 || address&31 != 0 {
		return step, fmt.Errorf("native earthquake record outside pool")
	}
	m := cb.Memory
	owner, err := m.Read8(address + 12)
	if err != nil {
		return step, err
	}
	if owner == 0 {
		return step, nil
	}
	state, err := m.Read8(address + 22)
	if err != nil {
		return step, err
	}
	if state != 0x22 && state != 0x24 && state != 0x26 {
		return step, nil
	}
	step.Handled = true
	if err := quakeCamera(address, cb); err != nil {
		return step, err
	}
	life, positive, err := quakeDecrement(address, 24, m)
	if err != nil {
		return step, err
	}
	if state != 0x26 && !positive {
		if err := m.Write16(address+24, r.FadeLife); err != nil {
			return step, err
		}
		if err := m.Write8(address+22, 0x26); err != nil {
			return step, err
		}
		state = 0x26
		if err := quakeCamera(address, cb); err != nil {
			return step, err
		}
		life, positive, err = quakeDecrement(address, 24, m)
		if err != nil {
			return step, err
		}
	}
	_ = life
	if state == 0x26 && !positive {
		step.Removed = true
		return step, m.Write8(address+12, 0)
	}
	if state == 0x24 {
		_, err := r.surface(address, cb)
		return step, err
	}
	_, timerPositive, err := quakeDecrement(address, 20, m)
	if err != nil {
		return step, err
	}
	if timerPositive {
		return step, nil
	}
	speed, err := m.Read8(address + 18)
	if err != nil {
		return step, err
	}
	if err := m.Write8(address+21, speed); err != nil {
		return step, err
	}
	height, err := r.surface(address, cb)
	if err != nil {
		return step, err
	}
	if state == 0x26 {
		if height != 0 {
			return step, nil
		}
		descriptor, err := m.Read16(address + 10)
		if err != nil {
			return step, err
		}
		if int(descriptor) >= len(r.Fade) {
			return step, fmt.Errorf("native earthquake descriptor outside fade table")
		}
		next := r.Fade[descriptor]
		if uint16(next) == descriptor {
			return step, nil
		}
		if err := m.Write8(address+11, next); err != nil {
			return step, err
		}
		if cb.Dirty != nil {
			if err := cb.Dirty(); err != nil {
				return step, err
			}
		}
		x, err := m.Read8(address + 6)
		if err != nil {
			return step, err
		}
		y, err := m.Read8(address + 8)
		if err != nil {
			return step, err
		}
		return step, cb.SetTile(int(x), int(y), next+0xac)
	}
	if height < 0 || height > 1 {
		return step, nil
	}
	if err := m.Write8(address+22, 0x24); err != nil {
		return step, err
	}
	direction, err := m.Read8(address + 26)
	if err != nil {
		return step, err
	}
	if direction >= 16 || cb.Random == nil {
		return step, fmt.Errorf("native earthquake branch direction/RNG missing")
	}
	childDirection := r.Branches[direction][uint16(cb.Random())%6]
	x, err := m.Read8(address + 6)
	if err != nil {
		return step, err
	}
	y, err := m.Read8(address + 8)
	if err != nil {
		return step, err
	}
	strength, err := m.Read16(address + 24)
	if err != nil {
		return step, err
	}
	child, err := r.Create(owner, x+uint8(r.Offsets[direction][0]), y+uint8(r.Offsets[direction][1]), childDirection, strength, address, cb)
	if err != nil {
		return step, err
	}
	step.Branched, step.ChildAddress = true, child.Address
	// Native CMPA compares absolute pointers. Failed edge creation returns
	// the parent and increments it; a full pool returns $e740, so the same
	// write aliases a magnet record. Neither behavior is silently repaired.
	if child.Address >= address {
		value, err := m.Read16(child.Address + 24)
		if err != nil {
			return step, err
		}
		if err := m.Write16(child.Address+24, value+1); err != nil {
			return step, err
		}
	}
	return step, nil
}
