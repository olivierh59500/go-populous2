package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

const (
	FungusActorKind  = uint8(0x26)
	FungusCollecting = uint8(0x12)
	FungusEvolving   = uint8(0x14)
)

// FungusBounds retains bytes 26..29 of the native controller. Extents are
// inclusive: an extent of two visits three cells. X/Y in NativeEffectActor
// instead hold packed minimum/working coordinates, not 8.8 positions.
type FungusBounds struct {
	DX, DY, WorkDX, WorkDY uint8
}

// FungusState is the additional state the caller must save beside its shared
// actor pool. Pending entries are collecting slots plus one; zero means none.
// A side can have several evolving controllers after its collecting slot clears.
type FungusState struct {
	Pending [2]uint16
	Bounds  [NativeEffectCapacity]FungusBounds
}

type FungusRules struct {
	CollectWait int16
	BasePeriod  uint8
	Neighbors   [8]int // Signed byte offsets in the native four-byte tile grid.
	Properties  [256]uint16
}

func DecodeFungusRules(exe *amiga.Executable) (FungusRules, error) {
	var rules FungusRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33512 {
		return rules, fmt.Errorf("native fungus tables missing")
	}
	code := exe.Hunks[0].Data
	rules.CollectWait = int16(binary.BigEndian.Uint16(code[0x20d50:]))
	rules.BasePeriod = code[0x20d53]
	// All eight unsigned experience tiers must leave a nonzero /3 divisor.
	if rules.CollectWait < 1 || rules.BasePeriod < 10 || rules.BasePeriod > 127 {
		return FungusRules{}, fmt.Errorf("invalid native fungus wait/period")
	}
	for i := range rules.Neighbors {
		n := int(int16(binary.BigEndian.Uint16(code[0x20f28+i*2:])))
		if n%4 != 0 || n < -260 || n > 260 {
			return FungusRules{}, fmt.Errorf("invalid native fungus neighbor offset")
		}
		rules.Neighbors[i] = n
	}
	if int16(binary.BigEndian.Uint16(code[0x20f28+16:])) != -99 {
		return FungusRules{}, fmt.Errorf("native fungus neighbor terminator missing")
	}
	for i := range rules.Properties {
		rules.Properties[i] = binary.BigEndian.Uint16(code[0x33312+i*2:])
	}
	return rules, nil
}

// FungusPlacement distinguishes planting from allocating a controller. Native
// $15fda writes tile 145 before allocation, and its input handler does not make
// mana consumption conditional on allocation success. The caller owns mana.
type FungusPlacement struct {
	Planted bool
	Reused  bool
	Slot    int // -1 when no controller could be allocated.
}

// Create translates $15fda. Tile callbacks receive linear indices 0..4095.
// A reused collecting controller retains its original wait and period.
func (rules *FungusRules) Create(pool *[NativeEffectCapacity]NativeEffectActor, state *FungusState, player, x, y int, experience uint8, read func(int) uint8, write func(int, uint8)) FungusPlacement {
	result := FungusPlacement{Slot: -1}
	if rules == nil || pool == nil || state == nil || read == nil || write == nil || player < 0 || player > 1 || !inside(x, y) {
		return result
	}
	pos := x + y*64
	if rules.Properties[read(pos)]&0x27 == 0 {
		return result
	}
	write(pos, 145)
	result.Planted = true
	if ref := state.Pending[player]; ref != 0 {
		index := int(ref) - 1
		// A malformed saved reference must not index beyond the shared pool.
		if index < 0 || index >= len(pool) {
			return result
		}
		actor, bounds := &pool[index], &state.Bounds[index]
		minX, workX := fungusCoordinates(actor.X)
		minY, workY := fungusCoordinates(actor.Y)
		if uint8(x) <= minX {
			bounds.DX += minX - uint8(x)
			minX = uint8(x)
		} else if n := uint8(x) - minX + 2; n > bounds.DX {
			bounds.DX = n
		}
		// $16064 uses signed BGE, unlike the unsigned X comparison.
		if int8(uint8(y)) < int8(minY) {
			bounds.DY += minY - uint8(y)
			minY = uint8(y)
		} else if n := uint8(y) - minY + 2; n > bounds.DY {
			bounds.DY = n
		}
		workX, workY = minX, minY
		bounds.WorkDX, bounds.WorkDY = bounds.DX, bounds.DY
		actor.X, actor.Y = packFungusCoordinates(minX, workX), packFungusCoordinates(minY, workY)
		result.Reused, result.Slot = true, index
		return result
	}
	period := rules.BasePeriod - (experience >> 5)
	if period < 3 || period > 127 || rules.CollectWait < 1 {
		return result
	}
	for index := range pool {
		actor := &pool[index]
		if actor.Active {
			continue
		}
		// Native creation does not clear unrelated bytes of a recycled slot.
		actor.Active, actor.Kind, actor.Player = true, FungusActorKind, uint8(player)
		minX, minY := uint8(x-1), uint8(y-1)
		actor.X, actor.Y = packFungusCoordinates(minX, minX), packFungusCoordinates(minY, minY)
		actor.Timer, actor.Speed, actor.State = rules.CollectWait, period, FungusCollecting
		state.Bounds[index] = FungusBounds{DX: 2, DY: 2, WorkDX: 2, WorkDY: 2}
		state.Pending[player] = uint16(index + 1)
		result.Slot = index
		return result
	}
	return result
}

type FungusStep struct {
	Started, Aged, Generated, Finished bool
	OutsideReads, OutsideWrites        int
}

// Tick translates $14e16..$150ba. Timer is initially native word 20. During
// evolution only its low byte (native signed byte 21) is decremented/reset;
// its high byte is preserved. No unrelated shared actors are modified.
//
// Access uses native linear row aliases, including neighbors across row edges.
// The native bottom clamp typo is retained in Bounds. For memory safety, tile
// indices outside 0..4095 read tile zero and writes are ignored and counted.
// This explicitly differs from an Amiga accessing adjacent BSS; this engine
// does not claim full memory-emulation parity after an out-of-map access.
func (rules *FungusRules) Tick(actor *NativeEffectActor, state *FungusState, index int, read func(int) uint8, write func(int, uint8)) FungusStep {
	var step FungusStep
	if rules == nil || actor == nil || state == nil || !actor.Active || actor.Kind != FungusActorKind || actor.Player > 1 || index < 0 || index >= len(state.Bounds) || read == nil || write == nil || actor.Speed < 3 || actor.Speed > 127 {
		return step
	}
	if actor.State == FungusCollecting {
		if state.Pending[actor.Player] != 0 {
			actor.Timer--
			if actor.Timer > 0 {
				return step
			}
		}
		state.Pending[actor.Player] = 0
		actor.State, actor.Timer = FungusEvolving, int16(actor.Speed)
		step.Started = true
	}
	if actor.State != FungusEvolving {
		return step
	}
	phase := int8(uint8(actor.Timer)) - 1
	actor.Timer = int16(uint16(actor.Timer)&0xff00 | uint16(uint8(phase)))
	surface := fungusSurface{read: read, write: write, step: &step}
	if phase >= 0 {
		if uint8(phase)%(actor.Speed/3) == 0 {
			rules.age(actor, &state.Bounds[index], &surface)
			step.Aged = true
		}
		return step
	}
	actor.Timer = int16(uint16(actor.Timer)&0xff00 | uint16(actor.Speed))
	rules.generation(actor, &state.Bounds[index], &surface)
	step.Generated = true
	step.Finished = !actor.Active
	return step
}

type fungusSurface struct {
	read  func(int) uint8
	write func(int, uint8)
	step  *FungusStep
}

func (surface *fungusSurface) tile(offset int) uint8 {
	index := offset / 4
	if index < 0 || index >= 4096 {
		surface.step.OutsideReads++
		return 0
	}
	return surface.read(index)
}

func (surface *fungusSurface) set(offset int, tile uint8) {
	index := offset / 4
	if index < 0 || index >= 4096 {
		surface.step.OutsideWrites++
		return
	}
	surface.write(index, tile)
}

func fungusCoordinates(packed int16) (uint8, uint8) {
	return uint8(uint16(packed) >> 8), uint8(packed)
}

func packFungusCoordinates(current, work uint8) int16 {
	return int16(uint16(current)<<8 | uint16(work))
}

// The native grid scales only the low X byte, then sign-extends the word.
func fungusOffset(x, y uint8) int {
	return int(int16(uint16(y)<<8 | uint16(x<<2)))
}

func (rules *FungusRules) age(actor *NativeEffectActor, bounds *FungusBounds, surface *fungusSurface) {
	_, x := fungusCoordinates(actor.X)
	_, y := fungusCoordinates(actor.Y)
	start := fungusOffset(x, y)
	for row := 0; row <= int(bounds.WorkDY); row++ {
		for col := 0; col <= int(bounds.WorkDX); col++ {
			offset := start + row*256 + col*4
			tile := surface.tile(offset)
			switch {
			case tile >= 145 && tile <= 148:
				surface.set(offset, tile+1)
			case tile >= 150 && tile <= 151:
				next := tile + 1
				if next == 152 {
					next = 15
				}
				surface.set(offset, next)
			}
		}
	}
}

func (rules *FungusRules) generation(actor *NativeEffectActor, bounds *FungusBounds, surface *fungusSurface) {
	oldX, _ := fungusCoordinates(actor.X)
	oldY, _ := fungusCoordinates(actor.Y)
	start := fungusOffset(oldX, oldY)
	minX, maxX, minY, maxY := uint8(255), uint8(0), 0x4100, -1
	for row := 0; row <= int(bounds.DY); row++ {
		for col := 0; col <= int(bounds.DX); col++ {
			offset := start + row*256 + col*4
			properties := rules.Properties[surface.tile(offset)] & 0x37
			if properties == 0 {
				continue
			}
			neighbors := 0
			for _, delta := range rules.Neighbors {
				if rules.Properties[surface.tile(offset+delta)] == 0x10 {
					neighbors++
				}
			}
			if properties&0x10 != 0 {
				if neighbors != 2 && neighbors != 3 {
					surface.set(offset, 150)
					continue
				}
			} else {
				if neighbors != 3 {
					continue
				}
				surface.set(offset, 145)
			}
			x, y := uint8(offset), int(int16(uint16(offset)&0xff00))
			if x <= minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if minX == 255 {
		actor.Active = false // $17446: the controller is not map-linked.
		return
	}
	newX := minX >> 2
	if newX > 0 {
		newX--
	}
	workX := newX
	if newX > oldX {
		workX = oldX
	}
	dx := (maxX >> 2) - newX + 1
	workDX := dx
	if dx <= bounds.DX {
		workDX = bounds.DX
	}
	if int8(newX+dx) >= 64 {
		dx = 63 - newX
	}
	newY := uint8(minY >> 8)
	if newY > 0 {
		newY--
	}
	workY := newY
	if newY > oldY {
		workY = oldY
	}
	dy := uint8(maxY>>8) - newY + 1
	workDY := dy
	if dy <= bounds.DY {
		workDY = bounds.DY
	}
	if int8(newY+dy) >= 64 {
		// $150b0 subtracts byte6 (X), not byte8 (Y). Preserve the bytes;
		// fungusSurface above bounds every subsequent memory access.
		dy = 63 - newX
	}
	actor.X, actor.Y = packFungusCoordinates(newX, workX), packFungusCoordinates(newY, workY)
	bounds.DX, bounds.DY, bounds.WorkDX, bounds.WorkDY = dx, dy, workDX, workDY
}
