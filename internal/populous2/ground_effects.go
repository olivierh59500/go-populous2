package populous2

import (
	"encoding/binary"
	"fmt"
	"go-populous2/internal/amiga"
	legacy "go-populous2/internal/legacy"
)

type GroundEffectRules struct {
	Offsets                              [45][2]int
	FontCount, SwampCount, GreeneryCount int
	Properties                           [256]uint16
}

func DecodeGroundEffectRules(exe *amiga.Executable) (GroundEffectRules, error) {
	var r GroundEffectRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x33512 {
		return r, fmt.Errorf("ground effect tables missing")
	}
	code := exe.Hunks[0].Data
	r.FontCount = int(binary.BigEndian.Uint16(code[0x21002:]))
	r.SwampCount = int(binary.BigEndian.Uint16(code[0x20fa6:]))
	r.GreeneryCount = int(binary.BigEndian.Uint16(code[0x20fa4:]))
	for _, n := range []int{r.FontCount, r.SwampCount, r.GreeneryCount} {
		if n < 1 || n > 256 {
			return GroundEffectRules{}, fmt.Errorf("invalid ground effect count")
		}
	}
	for i := range r.Offsets {
		n := int(int16(binary.BigEndian.Uint16(code[0x20fa8+i*2:])))
		x := int(int8(byte(n)))
		y := (n - x) / 256
		if abs(x) > 4 || abs(y) > 4 {
			return GroundEffectRules{}, fmt.Errorf("invalid ground effect offset")
		}
		r.Offsets[i] = [2]int{x, y}
	}
	for i := range r.Properties {
		r.Properties[i] = binary.BigEndian.Uint16(code[0x33312+i*2:])
	}
	return r, nil
}

// castGroundEffect follows $16938 (fonts), $169cc (swamps) and $16a62
// (greenery). The sampled positions and number of attempts come from the
// native tables, rather than a filled radius-two circle. Empty attempts still
// consume the cast: these three native command handlers do not test success.
func (w *World) castGroundEffect(player int, id SpellID, x, y int) bool {
	r := w.GroundRules
	count, tile, mask := 0, uint8(0), uint16(0)
	switch id {
	case Baptism:
		base := r.FontCount + int(w.Experience[player][Water]>>5)
		count = w.random()%base + base/2
		tile, mask = 143, 0x67
	case Swamp:
		base := r.SwampCount
		count = w.random()%base + base/2
		tile, mask = 168, 0x27
	case Flowers:
		base := r.GreeneryCount
		count = w.random()%base + base/2 + int(w.Experience[player][Plants]>>5)
		tile = 245
	default:
		return false
	}
	for attempt := 0; attempt <= count; attempt++ {
		// Native DIVU #90 followed by clearing bit zero addresses words.
		// Modulo 45 would consume the same RNG stream but choose other cells.
		d := r.Offsets[(w.random()%90)/2]
		xx, yy := x+d[0], y+d[1]
		if !inside(xx, yy) {
			continue
		}
		pos := xx + yy*64
		cell := w.TerrainCell(xx, yy)
		if id == Flowers {
			if cell.Shape != 15 {
				continue
			}
		} else if w.Core.MapWho[pos] != 0 || r.Properties[cell.Code]&mask == 0 {
			continue
		}
		w.Marks[pos] = Mark{Spell: id, Player: player, Life: 1, Persistent: true, NativeTile: tile}
		if id == Flowers && w.Core.MapBlk[pos] == legacy.BadLand {
			w.Core.MapBlk[pos] = legacy.FlatBlock
		}
	}
	return true
}

func (w *World) applyGroundEffects() {
	for i := range w.Core.Peeps {
		p := &w.Core.Peeps[i]
		if p.Population <= 0 || p.AtPos < 0 || p.AtPos >= len(w.Marks) {
			continue
		}
		mark := w.Marks[p.AtPos]
		if !mark.Persistent || mark.Spell != Baptism {
			p.InFont = false
		}
		if !mark.Persistent {
			continue
		}
		switch mark.Spell {
		case Swamp:
			if w.Heroes[i].Active && w.Heroes[i].Spell == Heracles {
				continue
			}
			if p.Flags&legacy.InTown == 0 {
				w.Core.DamagePeep(i, p.Population)
			}
		case Baptism:
			if !p.InFont || p.LastFontTile != p.AtPos {
				player := int(p.Player) ^ 1
				if w.Core.ConvertPeep(i, player) {
					p = &w.Core.Peeps[i]
					p.InFont = true
					p.LastFontTile = p.AtPos
					if w.Heroes[i].Active {
						w.Heroes[i].Player = player
					}
				}
			}
		}
	}
}
