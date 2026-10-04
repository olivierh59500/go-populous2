package populous2

import (
	"encoding/binary"
	"fmt"
	"go-populous2/internal/amiga"
)

func DecodeBatholithRange(exe *amiga.Executable) (int, error) {
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0xde36 {
		return 0, fmt.Errorf("native batholith range missing")
	}
	n := int(binary.BigEndian.Uint16(exe.Hunks[0].Data[0xde34:]))
	if n < 1 || n > 256 {
		return 0, fmt.Errorf("invalid batholith range")
	}
	return n, nil
}

// castBatholith translates CODE:$df68. Each held-button cast samples a point,
// then either raises it or allocates one original boulder. It does not create
// a filled raised disc or insert a first-game RockBlock overlay.
func (w *World) castBatholith(x, y int) bool {
	random := w.random()
	x += int(uint8(random)&7) - 4
	if x < 0 || x >= 64 {
		return false
	}
	// X uses the low three bits and Y uses the high byte of the same draw.
	y += int((uint32(random)>>8)&7) - 4
	if y < 0 || y >= 64 {
		return false
	}
	if w.random()%w.BatholithRange-w.BatholithRange/2 >= 0 {
		before := w.Core.Alt
		w.Core.PaintRaiseAt(x, y)
		w.clearChangedGround(before)
		return true
	}
	variant := (w.random() % 8) / 2
	cell := w.TerrainCell(x, y)
	pos := x + y*64
	if cell.Code == 0 || w.GroundRules.Properties[cell.Code]&0x40 != 0 || w.Core.MapWho[pos] != 0 || w.sceneryAt(pos) >= 0 {
		return true
	}
	free := false
	for _, actor := range w.Scenery {
		if !actor.Active {
			free = true
			break
		}
	}
	if !free {
		return true
	}
	animation := w.SceneryBank.Boulders.Animations[variant]
	if w.random()%90 == 0 {
		animation = w.SceneryBank.Boulders.Animations[0]
	}
	w.allocateScenery(SceneryBoulder, x, y, animation, w.SceneryBank.Boulders.InitialAge)
	return true
}
