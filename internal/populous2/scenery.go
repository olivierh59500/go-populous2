package populous2

import (
	"encoding/binary"
	"fmt"
	"go-populous2/internal/amiga"
)

const SceneryCapacity = 200

type SceneryKind uint8

const (
	SceneryTree    SceneryKind = 0x16
	SceneryBoulder SceneryKind = 0x18
)

// SceneryActor corresponds to the native fourteen-byte tree/boulder records.
// Its age byte is signed: positive values age normally, negative values mark
// burial/destruction, and zero selects the removal animation/state.
type SceneryActor struct {
	Kind      SceneryKind
	Active    bool
	X, Y      int
	Age       int8
	Animation int
	Frame     int
	Removing  bool
}

type SceneryRules struct {
	ClusterCount, Attempts, InitialAge, AgeMask int
	Offsets                                     [45][2]int
	Animations                                  [4]int
}

type SceneryBank struct {
	Trees, Boulders          SceneryRules
	Frames                   map[int][]AnimationFrame
	RemovalStart, RemovalEnd int
}

func DecodeScenery(exe *amiga.Executable) (*SceneryBank, error) {
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x20fa4 {
		return nil, fmt.Errorf("native scenery tables missing")
	}
	code := exe.Hunks[0].Data
	b := &SceneryBank{Frames: make(map[int][]AnimationFrame)}
	b.RemovalStart = 0xad8
	b.RemovalEnd = 0xae0
	for _, entry := range []struct {
		rules                                              *SceneryRules
		ageMask, age, attempts, count, animations, offsets int
	}{
		{&b.Trees, 0x20f3a, 0x20f3c, 0x20f3e, 0x20f40, 0x20f42, 0x20f4a},
		{&b.Boulders, 0xddca, 0xddcc, 0xddce, 0xddd0, 0xddd2, 0xddda},
	} {
		r := entry.rules
		r.AgeMask = int(binary.BigEndian.Uint16(code[entry.ageMask:]))
		r.InitialAge = int(binary.BigEndian.Uint16(code[entry.age:]))
		r.Attempts = int(binary.BigEndian.Uint16(code[entry.attempts:]))
		r.ClusterCount = int(binary.BigEndian.Uint16(code[entry.count:]))
		if r.InitialAge < 1 || r.InitialAge > 127 || r.Attempts < 1 || r.ClusterCount < 1 {
			return nil, fmt.Errorf("invalid native scenery parameters")
		}
		for i := range r.Animations {
			r.Animations[i] = int(binary.BigEndian.Uint16(code[entry.animations+i*2:]))
			frames, err := DecodeAnimation(exe, r.Animations[i])
			if err != nil {
				return nil, err
			}
			b.Frames[r.Animations[i]] = frames
		}
		for i := range r.Offsets {
			n := int(int16(binary.BigEndian.Uint16(code[entry.offsets+i*2:])))
			x := int(int8(byte(n)))
			r.Offsets[i] = [2]int{x, (n - x) / 256}
		}
	}
	for _, offset := range []int{b.RemovalStart, b.RemovalEnd} {
		frames, err := DecodeAnimation(exe, offset)
		if err != nil {
			return nil, err
		}
		b.Frames[offset] = frames
	}
	frames, err := DecodeAnimation(exe, 0xf10)
	if err != nil {
		return nil, err
	}
	b.Frames[0xf10] = frames
	return b, nil
}

func (w *World) sceneryAt(pos int) int {
	if pos < 0 || pos >= len(w.sceneryIndex) {
		return -1
	}
	return int(w.sceneryIndex[pos]) - 1
}

func (w *World) rebuildSceneryIndex() {
	w.sceneryIndex = [4096]uint16{}
	for i, actor := range w.Scenery {
		if actor.Active && inside(actor.X, actor.Y) {
			w.sceneryIndex[actor.X+actor.Y*64] = uint16(i + 1)
		}
	}
}

func (w *World) allocateScenery(kind SceneryKind, x, y, animation, age int) bool {
	pos := x + y*64
	if !inside(x, y) || w.Core.MapWho[pos] != 0 || w.sceneryAt(pos) >= 0 {
		return false
	}
	for i := range w.Scenery {
		if w.Scenery[i].Active {
			continue
		}
		w.Scenery[i] = SceneryActor{Kind: kind, Active: true, X: x, Y: y, Age: int8(age), Animation: animation}
		w.sceneryIndex[pos] = uint16(i + 1)
		return true
	}
	return false
}

// plantScenery follows the common random sampling used at $da0a/$dc04.
// Duplicate occupied cells skip the allocation and its extra random draw.
func (w *World) plantScenery(kind SceneryKind, player, x, y int) int {
	r := w.SceneryBank.Trees
	if kind == SceneryBoulder {
		r = w.SceneryBank.Boulders
	}
	random := w.random()
	variant := (random % 8) / 2
	count := random % r.Attempts
	if kind == SceneryTree && player >= 0 && player < 2 {
		count += int(w.Experience[player][Plants] >> 4)
	}
	if kind == SceneryTree && player == -99 && w.Scenery[45].Active {
		// $cd22 leaves signed D2=-99. $d9d8 preserves it, and $da0a's
		// signed deity check aliases BSS:$6e4f: slot 45's Y fraction byte.
		// Scenery is centered at fraction $80, adding eight attempts.
		count += 8
	}
	planted := 0
	for attempt := 0; attempt <= count; attempt++ {
		d := r.Offsets[(w.random()%90)/2]
		xx, yy := x+d[0], y+d[1]
		if !inside(xx, yy) {
			continue
		}
		pos := xx + yy*64
		cell := w.TerrainCell(xx, yy)
		if cell.Code == 0 || w.GroundRules.Properties[cell.Code]&0x40 != 0 || w.Core.MapWho[pos] != 0 || w.sceneryAt(pos) >= 0 {
			continue
		}
		free := false
		for _, actor := range w.Scenery {
			if !actor.Active {
				free = true
				break
			}
		}
		if !free {
			break
		}
		animation := r.Animations[variant]
		// The native rare-variant draw changes the pointer only when its
		// modulo-90 result is zero; preserve the draw even when unchanged.
		if rare := w.random() % 90; rare == 0 {
			animation = r.Animations[(rare&0xfe)/2]
		}
		if w.allocateScenery(kind, xx, yy, animation, r.InitialAge) {
			planted++
		}
	}
	return planted
}

func (w *World) initializeScenery() {
	for _, kind := range []SceneryKind{SceneryTree, SceneryBoulder} {
		r := w.SceneryBank.Trees
		if kind == SceneryBoulder {
			r = w.SceneryBank.Boulders
		}
		clusters := w.random()%r.ClusterCount + r.ClusterCount/2
		for i := 0; i <= clusters; i++ {
			point := w.random() & 0x3f3f
			w.plantScenery(kind, -99, point&63, (point>>8)&63)
		}
	}
}

// tickScenery follows $de36, including the signed byte burial counters.
func (w *World) tickScenery() {
	for i := range w.Scenery {
		a := &w.Scenery[i]
		if !a.Active {
			continue
		}
		r := w.SceneryBank.Trees
		if a.Kind == SceneryBoulder {
			r = w.SceneryBank.Boulders
		}
		if a.Removing {
			if a.Age > 0 {
				a.Age--
				a.Animation = w.SceneryBank.RemovalEnd
				a.Frame = 0
				continue
			}
			if a.Age < 0 {
				age := -int(a.Age) + 1
				if age == w.SceneryBank.Trees.InitialAge {
					w.removeScenery(i)
					continue
				}
				a.Age = int8(-age)
				continue
			}
			a.Frame++
			if a.Frame >= len(w.SceneryBank.Frames[a.Animation]) {
				x, y := a.X, a.Y
				a.Age = -1
				a.Animation = w.SceneryBank.RemovalStart
				a.Frame = 0
				w.spreadTreeFire(x, y)
			}
			continue
		}
		code := w.TerrainCell(a.X, a.Y).Code
		buried := w.GroundRules.Properties[code]&0x88 != 0
		if a.Age < 0 {
			age := -int(a.Age) + 1
			if age == w.SceneryBank.Trees.InitialAge {
				w.removeScenery(i)
				continue
			}
			a.Age = int8(-age)
			if !buried {
				a.Age = -a.Age
			}
			continue
		}
		if a.Age > 0 && w.Core.GameTurn&r.AgeMask == 0 {
			a.Age--
		}
		if buried {
			a.Age = -(a.Age + 1)
		}
	}
}

func (w *World) removeScenery(index int) {
	a := &w.Scenery[index]
	w.sceneryIndex[a.X+a.Y*64] = 0
	a.Active = false
}
