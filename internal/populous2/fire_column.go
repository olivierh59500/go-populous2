package populous2

import (
	"encoding/binary"
	"fmt"
	"go-populous2/internal/amiga"
)

const NativeEffectCapacity = 250

// NativeEffectActor retains the simulation fields of a native 32-byte effect
// record. Position and velocity are signed 8.8 values, not whole tile deltas.
type NativeEffectActor struct {
	Active      bool
	Kind        uint8
	Player      uint8
	X, Y        int16
	VX, VY      int16
	Speed       uint8
	Timer, Life int16
	State       uint8
	Animation   int
}

type FireColumnRules struct {
	BaseLife        int
	Speed           uint8
	Jitter          [9][2]int
	Neighbors       [16][2]int
	Vectors         [16][2]int16
	Frames          map[int]AnimationFrame
	SequenceLengths map[int]int
	HeroDeath       [6]int
	TownDeath       [TownStages]int
	TownClearCount  [TownStages]uint8
}

func DecodeFireColumnRules(exe *amiga.Executable) (FireColumnRules, error) {
	var r FireColumnRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x23d1a+0x2bd4+4 {
		return r, fmt.Errorf("native fire column tables missing")
	}
	code := exe.Hunks[0].Data
	r.BaseLife = int(binary.BigEndian.Uint16(code[0x20d44:]))
	r.Speed = code[0x20d47]
	if r.BaseLife < 1 || r.BaseLife > 32000 || r.Speed == 0 {
		return r, fmt.Errorf("invalid native fire column lifetime/speed")
	}
	decodeOffset := func(at int) [2]int {
		n := int(int16(binary.BigEndian.Uint16(code[at:])))
		x := int(int8(byte(n)))
		return [2]int{x, (n - x) / 256}
	}
	for i := range r.Jitter {
		r.Jitter[i] = decodeOffset(0x20d7a + i*2)
	}
	for i := range r.Neighbors {
		r.Neighbors[i] = decodeOffset(0x20dd6 + i*2)
		r.Vectors[i] = [2]int16{int16(binary.BigEndian.Uint16(code[0x20df6+i*4:])), int16(binary.BigEndian.Uint16(code[0x20df6+i*4+2:]))}
	}
	r.Frames = make(map[int]AnimationFrame)
	r.SequenceLengths = make(map[int]int)
	for _, start := range []int{0x1a0, 0x4b8, 0x660} {
		frames, err := DecodeAnimation(exe, start)
		if err != nil {
			return FireColumnRules{}, err
		}
		r.SequenceLengths[start] = len(frames)
		for i, frame := range frames {
			r.Frames[start+i*4] = frame
		}
	}
	for i := range r.HeroDeath {
		r.HeroDeath[i] = int(binary.BigEndian.Uint16(code[0x20a24+i*2:]))
	}
	for i := range r.TownDeath {
		r.TownDeath[i] = int(binary.BigEndian.Uint16(code[0x20b3a+i*2:]))
		r.TownClearCount[i] = code[0x13654+i] + 1
	}
	for _, start := range append([]int{0x178, 0x2bd4}, r.TownDeath[:]...) {
		if _, ok := r.SequenceLengths[start]; ok {
			continue
		}
		frames, err := DecodeAnimation(exe, start)
		if err != nil {
			return FireColumnRules{}, err
		}
		r.SequenceLengths[start] = len(frames)
		for i, frame := range frames {
			r.Frames[start+i*4] = frame
		}
	}
	return r, nil
}

// castFireColumn follows $15b7c: a jittered 3x3 position, first free shared
// slot and two random draws on success. Occupied or water tiles are valid.
func (w *World) castFireColumn(player, x, y int) bool {
	d := w.FireColumns.Jitter[(w.random()%18)/2]
	x += d[0]
	y += d[1]
	if !inside(x, y) {
		return false
	}
	for i := range w.NativeEffects {
		a := &w.NativeEffects[i]
		if a.Active {
			continue
		}
		// Creation leaves velocity bytes untouched until the first active
		// reroute, including when reusing an expired native record.
		vx, vy := a.VX, a.VY
		*a = NativeEffectActor{Active: true, Kind: 0x22, Player: uint8(player), X: int16(x*256 + 128), Y: int16(y*256 + 128), VX: vx, VY: vy, State: 2, Animation: 0x1a0, Timer: 1, Speed: w.FireColumns.Speed, Life: int16(w.FireColumns.BaseLife + int(w.Experience[player][Fire]))}
		w.random()
		w.linkEffect(i)
		return true
	}
	return false
}

func (w *World) tickNativeEffects() {
	for i := range w.NativeEffects {
		a := &w.NativeEffects[i]
		if !a.Active {
			continue
		}
		wasLinked, _ := w.Occupancy.Linked(nativeActorReference(NativeEffectPool, i))
		switch a.Kind {
		case 0x22:
			w.tickFireColumn(a)
		case 0x20:
			w.tickWhirlwind(a)
		case FungusActorKind:
			w.tickFungus(i)
		case WhirlpoolActorKind:
			w.tickWhirlpool(a)
		case BasaltActorKind:
			w.tickBasalt(i)
		case 0x28, 0x2a:
			if err := w.LightningRules.Tick(&w.LightningState, &w.NativeEffects, i, w.lightningCallbacks()); err != nil {
				panic(err)
			}
			if a.Active {
				w.moveActor(NativeEffectPool, i, uint16(a.X), uint16(a.Y))
			}
		}
		if wasLinked && (a.Kind == 0x20 || a.Kind == 0x22) {
			if !a.Active {
				w.unlinkActor(NativeEffectPool, i)
			} else {
				w.moveActor(NativeEffectPool, i, uint16(a.X), uint16(a.Y))
			}
		}
		w.projectNativeEffectRecord(i)
	}
}

func (w *World) tickFireColumn(a *NativeEffectActor) {
	r := &w.FireColumns
	if a.State == 2 {
		if a.Animation+4 < 0x1a0+r.SequenceLengths[0x1a0]*4 {
			a.Animation += 4
			return
		}
		a.State = 4
		a.Animation = 0x4b8
	}
	if a.State == 4 {
		a.Life--
		if a.Life <= 0 {
			a.State = 6
			a.Animation = 0x660
		} else {
			a.Animation += 4
			if a.Animation >= 0x4b8+r.SequenceLengths[0x4b8]*4 {
				a.Animation = 0x4b8
			}
			x, y := int(a.X)>>8, int(a.Y)>>8
			if !inside(x, y) {
				a.Active = false
				return
			}
			cell := w.TerrainCell(x, y)
			if w.GroundRules.Properties[cell.Code]&8 != 0 {
				a.State = 6
				a.Animation = 0x660
			} else {
				a.Timer--
				if a.Timer <= 0 {
					w.routeFireColumn(a, x, y, cell.BaseAltitude)
				}
				nextX, nextY := a.X+a.VX, a.Y+a.VY
				if nextX < 0 || nextY < 0 || nextX >= 0x4000 || nextY >= 0x4000 {
					a.Active = false
					return
				}
				a.X, a.Y = nextX, nextY
				for index := range w.NativeEffects {
					if a == &w.NativeEffects[index] {
						w.moveActor(NativeEffectPool, index, uint16(a.X), uint16(a.Y))
						break
					}
				}
				w.burnFireCell(int(a.X)>>8, int(a.Y)>>8)
				return
			}
		}
	}
	if a.State == 6 {
		next := a.Animation + 4
		if next >= 0x660+r.SequenceLengths[0x660]*4 {
			a.Active = false
		} else {
			a.Animation = next
		}
	}
}

func (w *World) routeFireColumn(a *NativeEffectActor, x, y, height int) {
	bits := uint16(w.random())
	start := int(bits&0xe) / 2
	selected := 0
	for i := 0; i < 8; i++ {
		index := start + i
		d := w.FireColumns.Neighbors[index]
		xx, yy := x+d[0], y+d[1]
		if !inside(xx, yy) {
			continue
		}
		neighbor := w.TerrainCell(xx, yy).BaseAltitude
		if neighbor < height {
			continue
		}
		if neighbor == height {
			accept := bits&1 != 0
			bits >>= 1
			if !accept {
				continue
			}
		}
		height = neighbor
		selected = index * 4
	}
	// Offset zero is also the native no-selection sentinel.
	if selected == 0 {
		selected = int(bits & 0x3c)
	}
	vector := w.FireColumns.Vectors[selected/4]
	a.VX = vector[0] * int16(a.Speed)
	a.VY = vector[1] * int16(a.Speed)
	a.Timer = int16(2 * (255 / int(a.Speed)))
}
