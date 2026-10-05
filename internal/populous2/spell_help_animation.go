package populous2

import (
	"encoding/binary"
	"fmt"

	"go-populous2/internal/amiga"
)

type nativeHelpAnimationKind uint8

const (
	nativeHelpAnimationNone nativeHelpAnimationKind = iota
	nativeHelpAnimationImage
	nativeHelpAnimationTile
	nativeHelpAnimationTileSequence
	nativeHelpAnimationMappedTiles
	nativeHelpAnimationMosaic
	nativeHelpAnimationMaskOnly
)

type nativeHelpAnimationRoute struct {
	Routine     uint32
	Kind        nativeHelpAnimationKind
	First, Mask uint16
}

// NativeSpellHelpAnimationRules owns only $5278..$5286's preview dispatch.
// Requester/admission, VBlank wait, audio consumption and swapping are the
// surrounding caller stages, not synthesized inside this animation body.
type NativeSpellHelpAnimationRules struct {
	code                []byte
	routes              [36]nativeHelpAnimationRoute
	Images              NativeEditorCursorRules
	Mapped              [4]uint8
	MosaicFirst         [4]uint16
	MosaicDisplacements [4]int16
}

type NativeSpellHelpAnimationState struct {
	TableOffset, Frame, X, Y uint16 // Mutable CODE $5528/$552a/$552c/$552e.
	Image                    NativeImageRenderState
}
type NativeSpellHelpTile struct {
	Tile       uint16
	ByteOffset int32  // Actual destination byte displacement from BSS $1e.
	SourceHigh uint16 // Preserved D0 high word in BFAC's long source index.
}
type NativeSpellHelpAnimationFrame struct {
	Routine uint32
	Sprites []NativePresentationSprite
	Tiles   []NativeSpellHelpTile
}

func DecodeNativeSpellHelpAnimationRules(exe *amiga.Executable) (NativeSpellHelpAnimationRules, error) {
	var r NativeSpellHelpAnimationRules
	if exe == nil || len(exe.Hunks) == 0 || len(exe.Hunks[0].Data) < 0x58ba {
		return r, fmt.Errorf("native help animation tables missing")
	}
	r.code = exe.Hunks[0].Data
	var err error
	r.Images, err = DecodeNativeEditorCursorRules(exe)
	if err != nil {
		return r, err
	}
	w := func(a int) uint16 { return binary.BigEndian.Uint16(r.code[a:]) }
	for i := range r.routes {
		at := 0x52ac + int(w(0x52ac+i*2))
		v := nativeHelpAnimationRoute{Routine: uint32(at)}
		switch at {
		case 0x5526, 0x53cc, 0x53e0:
			v.Kind = nativeHelpAnimationNone
		case 0x531c, 0x5336, 0x5358:
			if w(at) != 0x0679 || w(at+2) != 1 || w(at+14) != 0x0242 || w(at+18) != 0x0642 {
				return r, fmt.Errorf("native help tile counter body differs")
			}
			v.Kind, v.Mask, v.First = nativeHelpAnimationTileSequence, w(at+16), w(at+20)
		case 0x537a:
			v.Kind, v.Mask = nativeHelpAnimationMappedTiles, w(at+16)
		case 0x53ec:
			v.Kind = nativeHelpAnimationMosaic
		case 0x5476:
			v.Kind, v.Mask, v.First = nativeHelpAnimationMaskOnly, w(at+2), w(at+20)
		default:
			if at < 0 || at+8 > len(r.code) || w(at) != 0x343c || w(at+4) != 0x6000 {
				return r, fmt.Errorf("native help dispatch body$%x unsupported", at)
			}
			v.First = w(at + 2)
			target := at + 6 + int(int16(w(at+6)))
			if target == 0x54e0 {
				v.Kind = nativeHelpAnimationImage
			} else if target == 0x54a2 {
				v.Kind = nativeHelpAnimationTile
			} else {
				return r, fmt.Errorf("native help preview target differs")
			}
		}
		r.routes[i] = v
	}
	copy(r.Mapped[:], r.code[0x5398:0x539c])
	for i := range r.MosaicFirst {
		offset := w(0x545c + i*2)
		if offset%12 != 0 {
			return r, fmt.Errorf("native help mosaic descriptor is unaligned")
		}
		r.MosaicFirst[i] = offset / 12
		r.MosaicDisplacements[i] = int16(w(0x5464 + i*2))
	}
	if w(0x546c) != 0xff9d {
		return r, fmt.Errorf("native help mosaic terminator differs")
	}
	return r, nil
}

func (r *NativeSpellHelpAnimationRules) NewState() NativeSpellHelpAnimationState {
	var s NativeSpellHelpAnimationState
	if r != nil && len(r.code) >= 0x5530 {
		s.TableOffset = binary.BigEndian.Uint16(r.code[0x5528:])
		s.Frame = binary.BigEndian.Uint16(r.code[0x552a:])
		s.X = binary.BigEndian.Uint16(r.code[0x552c:])
		s.Y = binary.BigEndian.Uint16(r.code[0x552e:])
		s.Image = r.Images.NewImageState()
	}
	return s
}

// Begin mirrors $51a6/$51ae after the existing caller has checked icon and
// positive power admission. It does not invent admission for reserved slots.
func (r *NativeSpellHelpAnimationRules) Begin(s *NativeSpellHelpAnimationState, tableOffset uint16) error {
	if r == nil || s == nil || tableOffset&1 != 0 || tableOffset >= 72 {
		return fmt.Errorf("native help slot outside36-entry table")
	}
	s.TableOffset, s.Frame = tableOffset, 0xffff
	return nil
}

func nativeHelpTileLastWord(block []byte, tile uint16) (uint16, error) {
	a := int(tile)*12 + 10
	if a < 0 || a+2 > len(block) {
		return 0, fmt.Errorf("native help tile descriptor outside actual BLOCK backing")
	}
	return binary.BigEndian.Uint16(block[a:]), nil
}

// Advance translates the actual36 source bodies. BLOCK is the current
// terrain's real descriptor bank; its last chunk word is part of D0's native
// output even though rendering is returned as a tile plan. No-op paths still
// retain the dispatcher's D0.W assignment, and image paths reuse exact EE32.
func (r *NativeSpellHelpAnimationRules) Advance(s *NativeSpellHelpAnimationState, block []byte, d *[8]uint32) (NativeSpellHelpAnimationFrame, error) {
	var plan NativeSpellHelpAnimationFrame
	if r == nil || s == nil || d == nil || s.TableOffset&1 != 0 || s.TableOffset >= 72 {
		return plan, fmt.Errorf("native help animation backing/context missing")
	}
	route := r.routes[s.TableOffset/2]
	plan.Routine = route.Routine
	d[0] = hudWord(d[0], s.TableOffset)
	d[0] = hudWord(d[0], uint16(route.Routine-0x52ac))
	if route.Kind == nativeHelpAnimationNone {
		return plan, nil
	}
	if route.Kind == nativeHelpAnimationImage {
		d[2] = hudWord(d[2], route.First)
		if s.Frame == 0xffff {
			s.Frame = uint16(d[2])
		}
		d[2] = hudWord(d[2], s.Frame+4)
		marker, err := r.Images.word(0x23d1a + int(int16(d[2])))
		if err != nil {
			return plan, err
		}
		if int16(marker) <= 0 {
			d[2] = hudWord(d[2], uint16(d[2])+marker)
		}
		s.Frame = uint16(d[2])
		d[0] = hudWord(d[0], s.X)
		d[1] = hudWord(d[1], s.Y+16)
		plan.Sprites, err = r.Images.DrawImage(&s.Image, d)
		return plan, err
	}
	var tile uint16
	switch route.Kind {
	case nativeHelpAnimationTile:
		tile = route.First
		d[2] = hudWord(d[2], tile)
	case nativeHelpAnimationTileSequence:
		s.Frame++
		d[2] = hudWord(d[2], s.Frame)
		d[2] = hudWord(d[2], uint16(d[2])&route.Mask)
		d[2] = hudWord(d[2], uint16(d[2])+route.First)
		tile = uint16(d[2])
	case nativeHelpAnimationMappedTiles:
		s.Frame++
		d[2] = hudWord(d[2], s.Frame)
		d[2] = hudWord(d[2], uint16(d[2])&route.Mask)
		if uint16(d[2]) >= uint16(len(r.Mapped)) {
			return plan, fmt.Errorf("native help mapped tile outside original table")
		}
		d[2] = d[2]&0xffffff00 | uint32(r.Mapped[uint16(d[2])])
		d[2] = hudWord(d[2], uint16(d[2])&0xff)
		tile = uint16(d[2])
	case nativeHelpAnimationMaskOnly:
		s.Frame &= route.Mask
		d[2] = hudWord(d[2], s.Frame)
		d[2] = hudWord(d[2], uint16(d[2])&route.Mask)
		d[2] = hudWord(d[2], uint16(d[2])+route.First)
		tile = uint16(d[2])
	case nativeHelpAnimationMosaic:
		s.Frame++
		d[2] = hudWord(d[2], s.Frame)
		d[2] = hudWord(d[2], uint16(d[2])&3)
		d[2] = hudWord(d[2], uint16(d[2])*2)
		phase := int(uint16(d[2]) / 2)
		tile = r.MosaicFirst[phase]
		d[2] = hudWord(d[2], tile*12)
		d[0] = hudWord(d[0], s.X)
		d[0] = hudWord(d[0], uint16(d[0])>>3)
		displacement := int32(int16(uint16(d[0])))
		d[0] = hudWord(d[0], s.Y)
		d[0] = uint32(uint16(d[0])) * 40
		displacement += int32(int16(d[0]))
		for i, delta := range r.MosaicDisplacements {
			d[0] = hudWord(d[0], uint16(delta))
			displacement += int32(delta)
			last, err := nativeHelpTileLastWord(block, tile+uint16(i))
			if err != nil {
				return plan, err
			}
			plan.Tiles = append(plan.Tiles, NativeSpellHelpTile{Tile: tile + uint16(i), ByteOffset: displacement, SourceHigh: uint16(d[0] >> 16)})
			d[0] = hudWord(d[0], last)
		}
		d[0] = hudWord(d[0], 0xff9d)
		return plan, nil
	default:
		return plan, fmt.Errorf("native help tile route invalid")
	}
	d[2] = uint32(uint16(d[2])) * 12
	d[0] = hudWord(d[0], s.X)
	d[0] = hudWord(d[0], uint16(d[0])>>3)
	displacement := int32(int16(uint16(d[0])))
	d[0] = hudWord(d[0], s.Y)
	d[0] = uint32(uint16(d[0])) * 40
	displacement += int32(int16(d[0]))
	last, err := nativeHelpTileLastWord(block, tile)
	if err != nil {
		return plan, err
	}
	d[0] = hudWord(d[0], last)
	plan.Tiles = append(plan.Tiles, NativeSpellHelpTile{Tile: tile, ByteOffset: displacement, SourceHigh: uint16(d[0] >> 16)})
	return plan, nil
}
