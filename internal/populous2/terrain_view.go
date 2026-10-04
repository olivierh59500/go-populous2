package populous2

import legacy "go-populous2/internal/legacy"

// TerrainCell separates geometric corner heights from the inherited terrain
// IDs. Populous II's raised-slope graphics and animated sea-level graphics
// occupy different banks; the first game's sea-level +16 encoding is not a
// directly usable Populous II tile index.
type TerrainCell struct {
	Corners      [4]int // northwest, northeast, southeast, southwest
	BaseAltitude int
	Shape        uint8
	Code         uint8
}

func (cell TerrainCell) IsWater() bool {
	return cell.Corners == [4]int{}
}

func (w *World) isWaterAt(pos int) bool {
	return pos >= 0 && pos < 4096 && w.GroundRules.Properties[w.nativeTileAt(pos%64, pos/64)]&8 != 0
}

func (w *World) nativeTileAt(x, y int) uint8 { return w.TerrainCell(x, y).Code }

func (w *World) TerrainCell(x, y int) TerrainCell {
	if !inside(x, y) {
		return TerrainCell{}
	}
	a := x + y*legacy.EndWidth
	h := &w.Core.Alt
	cell := TerrainCell{Corners: [4]int{h[a], h[a+1], h[a+legacy.EndWidth+1], h[a+legacy.EndWidth]}}
	cell.BaseAltitude = min(cell.Corners[0], cell.Corners[1], cell.Corners[2], cell.Corners[3])
	for i, height := range cell.Corners {
		if height > cell.BaseAltitude {
			cell.Shape |= 1 << i
		}
	}
	if cell.Shape == 0 && cell.BaseAltitude > 0 {
		cell.BaseAltitude--
		cell.Shape = 15
	}
	cell.Code = cell.Shape
	switch w.Core.MapBlk[x+y*legacy.MapWidth] {
	case legacy.FarmBlock:
		cell.Code = 47
	case legacy.FarmBlock + 1:
		cell.Code = 63
	}
	if mark := w.Marks[x+y*legacy.MapWidth]; mark.Life > 0 && mark.NativeTile != 0 {
		cell.Code = mark.NativeTile
		if mark.Spell == Basalt && mark.NativeTile&0xf0 == 0xe0 {
			// Native terrain edits retain the basalt prefix and change only
			// the low corner-shape nibble, including raised flat geometry.
			cell.Code = 0xe0 | cell.Shape
		}
	}
	return cell
}

// Tile translates the graphics selection at CODE:$be5a-$bef4. Bank +16 is
// reserved for raised land, while water animation skips it and uses +32..128.
func (cell TerrainCell) Tile(tick, viewX, viewY int) int {
	code := int(cell.Code)
	if code == 143 {
		return code + ((tick + viewX + viewY) & 1)
	}
	if code == 168 {
		return code + ((tick + viewX + viewY) & 3)
	}
	if code <= 15 && cell.BaseAltitude > 0 {
		code += 16
	}
	if code < 15 {
		phase := (tick + viewX + viewY) & 7
		if phase != 0 {
			phase++
		}
		code += phase * 16
	}
	return code
}

// Surface contains the four projected corners within a 32x24 native tile.
// A raised corner moves up by eight pixels. Flat raised land consequently
// has its center eight pixels above the sea-level center.
func (cell TerrainCell) Surface() [4][2]int {
	points := [4][2]int{{16, 8}, {32, 16}, {16, 24}, {0, 16}}
	for i := range points {
		points[i][1] -= (cell.Corners[i] - cell.BaseAltitude) * 8
	}
	return points
}

func (cell TerrainCell) Contains(x, y int) bool {
	points := cell.Surface()
	positive, negative := false, false
	for i, a := range points {
		b := points[(i+1)%4]
		cross := (b[0]-a[0])*(y-a[1]) - (b[1]-a[1])*(x-a[0])
		positive = positive || cross > 0
		negative = negative || cross < 0
	}
	return !(positive && negative)
}

func (w *World) clearChangedGround(before [legacy.EndWidth * legacy.EndWidth]int) {
	minX, minY, maxX, maxY := 65, 65, -1, -1
	for i, h := range w.Core.Alt {
		if before[i] != h {
			minX = min(minX, i%65)
			maxX = max(maxX, i%65)
			minY = min(minY, i/65)
			maxY = max(maxY, i/65)
		}
	}
	if maxX < 0 {
		return
	}
	for y := max(0, minY-1); y <= min(63, maxY); y++ {
		for x := max(0, minX-1); x <= min(63, maxX); x++ {
			a := x + y*65
			if before[a] != w.Core.Alt[a] || before[a+1] != w.Core.Alt[a+1] || before[a+65] != w.Core.Alt[a+65] || before[a+66] != w.Core.Alt[a+66] {
				mark := w.Marks[x+y*64]
				if mark.Spell != Basalt || mark.NativeTile&0xf0 != 0xe0 {
					w.Marks[x+y*64] = Mark{}
				}
			}
		}
	}
}
