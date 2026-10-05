package populous2

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
)

type NativeWorldScreen struct {
	Requester *NativeRequester
	Image     *image.RGBA
}

// NativeWorldPalette is the original world/deity requester palette at $33844.
func NativeWorldPalette(bundle *Bundle) ([16]color.RGBA, error) {
	var palette [16]color.RGBA
	if bundle == nil || bundle.Executable == nil || len(bundle.Executable.Hunks) == 0 || len(bundle.Executable.Hunks[0].Data) < 0x33864 {
		return palette, fmt.Errorf("native world screen palette missing")
	}
	for i := range palette {
		palette[i] = AmigaColor(binary.BigEndian.Uint16(bundle.Executable.Hunks[0].Data[0x33844+i*2:]))
	}
	return palette, nil
}

// WorldScreen composes $3cc4's requester and $3ee6's icons over the retained
// display buffer, using the world requester's own palette. codeField is nil
// outside text editing; an empty non-nil field still has opaque padding.
func (r *NativeInGameRequesterRules) WorldScreen(bundle *Bundle, state NativeWorldRequesterState, background []byte, codeField []byte) (NativeWorldScreen, error) {
	var out NativeWorldScreen
	plan, err := r.WorldPlan(state)
	if err != nil {
		return out, err
	}
	if codeField != nil {
		// Retain native world-plan checks and icon admission, replacing only
		// the first argument's text-field cells via the original compiler.
		value := codeField
		if len(value) == 0 {
			value = []byte{'k'}
		}
		plan.Requester, err = r.Presentation.Compile(NativeMenuWorld, [][]byte{value, r.Presentation.decimal(uint32(state.World)), r.landscapes[state.Landscape], r.opponents[state.World/32]})
		if err != nil {
			return out, err
		}
		bits := state.RuleBits
		for i, glyph := range plan.Requester.Text {
			if glyph == 'y' || glyph == 'z' {
				plan.Requester.Text[i] = 'z'
				if bits&1 != 0 {
					plan.Requester.Text[i] = 'y'
				}
				bits >>= 1
			}
		}
	}
	palette, err := NativeWorldPalette(bundle)
	if err != nil {
		return out, err
	}
	base, err := r.Presentation.Compose(background, plan.Requester, palette)
	if err != nil {
		return out, err
	}
	out.Requester = plan.Requester
	out.Image = image.NewRGBA(base.Bounds())
	draw.Draw(out.Image, out.Image.Bounds(), base, image.Point{}, draw.Src)
	for _, position := range plan.Icons {
		icon, err := r.WorldIcon(int(position.Slot), palette)
		if err != nil {
			return NativeWorldScreen{}, err
		}
		draw.Draw(out.Image, icon.Bounds().Add(image.Pt(int(position.X), int(position.Y))), icon, image.Point{}, draw.Over)
	}
	return out, nil
}

// OpponentScreen translates $af82's actual pointer choices, five biography
// lines and $baa8 face composition. In particular, reaction labels use a
// five-byte source stride, not a guessed fixed-string table index.
func (r *NativeInGameRequesterRules) OpponentScreen(bundle *Bundle, world, reaction, aggression uint16, background []byte) (NativeWorldScreen, error) {
	var out NativeWorldScreen
	if r == nil || world >= 1000 {
		return out, fmt.Errorf("native opponent world outside campaign")
	}
	palette, err := NativeWorldPalette(bundle)
	if err != nil {
		return out, err
	}
	code := bundle.Executable.Hunks[0].Data
	read := func(at int) ([]byte, int, error) {
		end := at
		for end >= 0 && end < len(code) && code[end] != 0 {
			end++
		}
		if at < 0 || end >= len(code) {
			return nil, 0, fmt.Errorf("native opponent text outside CODE")
		}
		return code[at:end], end + 1, nil
	}
	stage := int(world / 32)
	parameters := make([][]byte, 9)
	parameters[0] = r.opponents[stage]
	parameters[1], _, err = read(0x9896 + stage*23)
	if err != nil {
		return out, err
	}
	speed := 0xa877 + int(reaction)*5
	if int16(reaction) < 0 || speed >= 0xa8bd {
		speed = 0xa8b3
	}
	parameters[2], _, err = read(speed)
	if err != nil {
		return out, err
	}
	behavior := 0xa8be + int(aggression/5)*12
	if behavior >= 0xa91e {
		behavior = 0xa912
	}
	parameters[3], _, err = read(behavior)
	if err != nil {
		return out, err
	}
	at := 0x9b76
	for i := 0; i < stage*5; i++ {
		_, at, err = read(at)
		if err != nil {
			return out, err
		}
	}
	for i := 4; i < len(parameters); i++ {
		parameters[i], at, err = read(at)
		if err != nil {
			return out, err
		}
	}
	out.Requester, err = r.Presentation.Compile(NativeMenuOpponent, parameters)
	if err != nil {
		return out, err
	}
	indices := append([]byte(nil), background...)
	if len(indices) != 320*200 {
		return out, fmt.Errorf("native opponent background size differs")
	}
	if err := r.Presentation.Font.DrawIndices(indices, string(out.Requester.Text), out.Requester.Column, out.Requester.Row); err != nil {
		return out, err
	}
	backing := int(binary.BigEndian.Uint16(code[0xb12c:]))
	x, y := (backing%40)*8, backing/40
	for row := range 64 {
		copy(indices[(y+row)*320+x:], r.Presentation.Widgets.FaceBackdrop[row*48:(row+1)*48])
	}
	base, err := nativeIndexedImage(indices, palette)
	if err != nil {
		return out, err
	}
	out.Image = image.NewRGBA(base.Bounds())
	draw.Draw(out.Image, out.Image.Bounds(), base, image.Point{}, draw.Src)
	art, err := DecodeDeityArt(bundle.Executable, bundle.Raw["faces.pak"], palette)
	if err != nil {
		return out, err
	}
	for part := 2; part >= 0; part-- {
		variant := int(code[0x96d6+stage*14+11+part])
		if variant > 7 {
			return out, fmt.Errorf("native opponent face variant outside bank")
		}
		at := 0xb12e + (2-part)*6
		if int(binary.BigEndian.Uint16(code[at:])) != part*8 {
			return out, fmt.Errorf("native opponent face bank differs")
		}
		x, y := int(binary.BigEndian.Uint16(code[at+2:])), int(binary.BigEndian.Uint16(code[at+4:]))
		partImage := art.Parts[part][variant]
		y += max(0, 16-partImage.Bounds().Dy())
		draw.Draw(out.Image, partImage.Bounds().Add(image.Pt(x, y)), partImage, image.Point{}, draw.Over)
	}
	return out, nil
}

// SpellHelpBase reproduces $517a..$5278's requester, icon and original help
// text. The per-power preview dispatcher starts afterward and remains an
// explicit animation continuation, rather than an invented generic effect.
func (r *NativeInGameRequesterRules) SpellHelpBase(bundle *Bundle, state NativeWorldRequesterState, offset uint16, background []byte) (NativeWorldScreen, bool, error) {
	var out NativeWorldScreen
	if r == nil || offset&1 != 0 || offset >= 72 {
		return out, false, fmt.Errorf("native spell help table offset outside bank")
	}
	slot := int(offset / 2)
	if r.icons[slot].Descriptor == 0x214b2 || state.PowerFlags[slot] <= 0 {
		return out, false, nil
	}
	palette, err := NativeWorldPalette(bundle)
	if err != nil {
		return out, false, err
	}
	out.Requester, err = r.Presentation.Compile(NativeMenuSpellHelp, nil)
	if err != nil {
		return out, false, err
	}
	code := bundle.Executable.Hunks[0].Data
	at := int(binary.BigEndian.Uint32(code[0x58ba+slot*4:]))
	end := at
	for end > 0 && end < len(code) && code[end] != 0 {
		end++
	}
	if at <= 0 || end >= len(code) {
		return out, false, fmt.Errorf("native spell help description missing")
	}
	indices := append([]byte(nil), background...)
	if len(indices) != 320*200 {
		return out, false, fmt.Errorf("native spell help background size differs")
	}
	if err := r.Presentation.Font.DrawIndices(indices, string(out.Requester.Text), out.Requester.Column, out.Requester.Row); err != nil {
		return out, false, err
	}
	if err := r.Presentation.Font.DrawIndices(indices, string(code[at:end]), out.Requester.Column+2, out.Requester.Row+64); err != nil {
		return out, false, err
	}
	base, err := nativeIndexedImage(indices, palette)
	if err != nil {
		return out, false, err
	}
	out.Image = image.NewRGBA(base.Bounds())
	draw.Draw(out.Image, out.Image.Bounds(), base, image.Point{}, draw.Src)
	icon, err := r.WorldIcon(slot, palette)
	if err != nil {
		return out, false, err
	}
	draw.Draw(out.Image, icon.Bounds().Add(image.Pt((out.Requester.Column+2)*8, out.Requester.Row+32)), icon, image.Point{}, draw.Over)
	return out, true, nil
}
