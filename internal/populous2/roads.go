package populous2

import (
	"encoding/binary"
	"fmt"
	"go-populous2/internal/amiga"
)

type RoadRules struct {
	Offsets                 [4][2]int
	Tiles                   [16]uint8
	Connections             [20]uint8
	ReverseBits             [4]uint8
	SlopeShapes, SlopeTiles [4]uint8
}

func DecodeRoadRules(exe *amiga.Executable) (RoadRules, error) {
	var r RoadRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x16938 {
		return r, fmt.Errorf("native road tables missing")
	}
	code := exe.Hunks[0].Data
	copy(r.ReverseBits[:], code[0x168f0:0x168f4])
	copy(r.Tiles[:], code[0x168fc:0x1690c])
	copy(r.Connections[:], code[0x1691c:0x16930])
	copy(r.SlopeShapes[:], code[0x16930:0x16934])
	copy(r.SlopeTiles[:], code[0x16934:0x16938])
	for i := range r.Offsets {
		n := int(int16(binary.BigEndian.Uint16(code[0x168f4+i*2:])))
		x := int(int8(byte(n)))
		r.Offsets[i] = [2]int{x, (n - x) / 256}
	}
	return r, nil
}

// castRoad follows CODE:$1677a. Native connection bits run from bit three for
// the first neighbor to bit zero for the last. The four permitted slope shapes
// have their own road art rather than a flat-line overlay.
func (w *World) castRoad(player, x, y int) bool {
	cell := w.TerrainCell(x, y)
	property := w.GroundRules.Properties[cell.Code]
	if property&0x400 != 0 {
		return false
	}
	mask := uint16(0x23)
	if player == 1 {
		mask = 0x25
	}
	tile := uint8(0)
	if property&mask == 0 {
		for i, shape := range w.RoadRules.SlopeShapes {
			if cell.Code == shape {
				tile = w.RoadRules.SlopeTiles[i]
				break
			}
		}
		if tile == 0 {
			return false
		}
	} else {
		connections := uint8(0)
		for i, d := range w.RoadRules.Offsets {
			xx, yy := x+d[0], y+d[1]
			if inside(xx, yy) && w.isRoad(w.TerrainCell(xx, yy).Code) {
				connections |= 1 << uint(3-i)
			}
		}
		tile = w.RoadRules.Tiles[connections]
	}
	w.setRoadMark(player, x, y, tile)
	connections := w.RoadRules.Connections[int(tile)-197]
	for i, d := range w.RoadRules.Offsets {
		if connections&(1<<uint(3-i)) == 0 {
			continue
		}
		xx, yy := x+d[0], y+d[1]
		if !inside(xx, yy) {
			continue
		}
		neighbor := w.TerrainCell(xx, yy).Code
		if !w.isRoad(neighbor) || neighbor < 201 || neighbor > 216 {
			continue
		}
		mask := w.RoadRules.Connections[int(neighbor)-197] | 1<<w.RoadRules.ReverseBits[3-i]
		w.setRoadMark(w.Marks[xx+yy*64].Player, xx, yy, w.RoadRules.Tiles[mask])
	}
	return true
}

func (w *World) isRoad(code uint8) bool { return w.GroundRules.Properties[code]&0x40 != 0 }

func (w *World) setRoadMark(player, x, y int, tile uint8) {
	w.Marks[x+y*64] = Mark{Spell: Road, Player: player, Life: 1, Persistent: true, NativeTile: tile}
}

// RemoveRoad implements the uncharged right-button action at CODE:$16744.
func (w *World) RemoveRoad(x, y int) bool {
	if !inside(x, y) || !w.isRoad(w.TerrainCell(x, y).Code) {
		return false
	}
	w.Marks[x+y*64] = Mark{}
	return true
}
