package populous2

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
)

type NativeDeityAction struct {
	Kind            string
	Element         Element
	Part, Direction int
}

type NativeDeityPresentation struct {
	Requester *NativeRequester
	Image     *image.RGBA
	Password  string
}

func (p *NativePresentation) DeityAction(action int, code []byte) (NativeDeityAction, error) {
	if action < 0 || action > 66 || action&1 != 0 || len(code) < 0xb8c6 {
		return NativeDeityAction{}, fmt.Errorf("native deity action outside source table")
	}
	target := 0xb882 + int(int16(binary.BigEndian.Uint16(code[0xb882+action:])))
	switch target {
	case 0xb7ec:
		return NativeDeityAction{}, nil
	case 0xb8c6:
		return NativeDeityAction{Kind: "name"}, nil
	case 0xb8d6, 0xb8dc, 0xb8e2, 0xb8e8, 0xb8ee, 0xb8f4:
		return NativeDeityAction{Kind: "allocate", Element: Element((target - 0xb8d6) / 6)}, nil
	case 0xb932, 0xb936, 0xb93a:
		return NativeDeityAction{Kind: "face", Part: (target - 0xb932) / 4, Direction: -1}, nil
	case 0xb93e, 0xb942, 0xb946:
		return NativeDeityAction{Kind: "face", Part: (target - 0xb93e) / 4, Direction: 1}, nil
	case 0xb99e:
		return NativeDeityAction{Kind: "password"}, nil
	case 0xba50:
		return NativeDeityAction{Kind: "continue"}, nil
	}
	return NativeDeityAction{}, fmt.Errorf("native deity action has an unhandled source target%x", target)
}

// DeityScreen composes the real requester, XP strips, face backing and masked
// parts. Its active palette comes from CODE33844, not LAND0 or the startup.
func (p *NativePresentation) DeityScreen(bundle *Bundle, deity Deity, nameField, codeField []byte) (NativeDeityPresentation, error) {
	var out NativeDeityPresentation
	if p == nil || bundle == nil || bundle.Executable == nil {
		return out, fmt.Errorf("native deity presentation assets missing")
	}
	code := bundle.Executable.Hunks[0].Data
	var palette [16]color.RGBA
	for i := range palette {
		palette[i] = AmigaColor(binary.BigEndian.Uint16(code[0x33844+i*2:]))
	}
	art, err := DecodeDeityArt(bundle.Executable, bundle.Raw["faces.pak"], palette)
	if err != nil {
		return out, err
	}
	if nameField == nil {
		nameField = []byte(deity.Name)
	}
	out.Password = deity.nativePassword()
	if codeField == nil {
		codeField = []byte(out.Password)
	}
	textAt := 0x96be
	if deity.Bolts != 0 {
		textAt = 0x96c6 + 14 - 2*(int(min(deity.Bolts, uint16(8)))-1)
	}
	textEnd := textAt
	for textEnd < len(code) && code[textEnd] != 0 {
		textEnd++
	}
	if textEnd == len(code) {
		return out, fmt.Errorf("native bolt text lacks terminator")
	}
	out.Requester, err = p.Compile(NativeMenuDeity, [][]byte{nameField, code[textAt:textEnd], codeField})
	if err != nil {
		return out, err
	}
	indices := make([]byte, 320*200)
	if err := p.Font.DrawIndices(indices, string(out.Requester.Text), out.Requester.Column, out.Requester.Row); err != nil {
		return out, err
	}
	if err := p.Widgets.PaintDeity(indices, deity.Experience); err != nil {
		return out, err
	}
	base, err := nativeIndexedImage(indices, palette)
	if err != nil {
		return out, err
	}
	out.Image = image.NewRGBA(base.Bounds())
	draw.Draw(out.Image, out.Image.Bounds(), base, image.Point{}, draw.Src)
	layers, err := p.Widgets.FaceLayers(deity.FaceParts)
	if err != nil {
		return out, err
	}
	for _, layer := range layers {
		part := art.Parts[layer.Part][layer.Variant]
		draw.Draw(out.Image, image.Rect(layer.X, layer.Y, layer.X+layer.Width, layer.Y+layer.Height), part, image.Point{}, draw.Over)
	}
	return out, nil
}
