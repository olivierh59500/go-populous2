package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"os"
	"path/filepath"

	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func exportSpellHelp(source *populous2.Bundle, output string) error {
	source, err := interfaceSource(source)
	if err != nil {
		return err
	}
	d, frames, err := decodeSpellHelp(source)
	if err != nil {
		return err
	}
	atlas, regions, err := pack("spell-help.png", frames, nil)
	if err != nil {
		return err
	}
	for slot := range d.Icons {
		d.Icons[slot] = regions[d.Icons[slot].X]
	}
	for land := range d.Sequences {
		for slot := range d.Sequences[land] {
			for i := range d.Sequences[land][slot].Frames {
				frame := &d.Sequences[land][slot].Frames[i]
				frame.Image = regions[frame.Image.X]
			}
		}
	}
	if err := writePNG(output, "spell-help.png", atlas); err != nil {
		return err
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(output, visualassets.SpellHelpArtName), append(data, '\n'), 0644)
}

func decodeSpellHelp(source *populous2.Bundle) (*visualassets.SpellHelpDescriptor, []*image.RGBA, error) {
	rules, err := populous2.DecodeNativeInGameRequesterRules(source.Executable)
	if err != nil {
		return nil, nil, err
	}
	palette, err := populous2.NativeWorldPalette(source)
	if err != nil {
		return nil, nil, err
	}
	r, err := rules.Presentation.Compile(populous2.NativeMenuSpellHelp, nil)
	if err != nil {
		return nil, nil, err
	}
	layout, err := decodeNamedRequester("spell-help", rules.Presentation.Requesters, r, palette, map[int]string{2: "return"}, nil)
	if err != nil {
		return nil, nil, err
	}
	d := &visualassets.SpellHelpDescriptor{Version: 1, FramesPerSecond: 10, Layout: *layout, IconPosition: image.Pt(layout.OriginX+16, layout.OriginY+32), DescriptionPosition: image.Pt(layout.OriginX+16, layout.OriginY+64)}
	code := source.Executable.Hunks[0].Data
	images := []*image.RGBA{}
	cache := map[[32]byte]int{}
	add := func(img *image.RGBA) int {
		keyData := make([]byte, 8+len(img.Pix))
		binary.BigEndian.PutUint32(keyData, uint32(img.Bounds().Dx()))
		binary.BigEndian.PutUint32(keyData[4:], uint32(img.Bounds().Dy()))
		copy(keyData[8:], img.Pix)
		key := sha256.Sum256(keyData)
		if index, ok := cache[key]; ok {
			return index
		}
		index := len(images)
		images = append(images, img)
		cache[key] = index
		return index
	}
	for slot := range d.Icons {
		icon, err := rules.WorldIcon(slot, palette)
		if err != nil {
			return nil, nil, err
		}
		d.Icons[slot] = visualassets.Region{X: add(icon)}
		if binary.BigEndian.Uint16(code[0x21102+slot*2:]) == 0 {
			continue
		}
		start := int(binary.BigEndian.Uint32(code[0x58ba+slot*4:]))
		if start <= 0 || start >= len(code) {
			return nil, nil, fmt.Errorf("spell help %d has no description", slot)
		}
		end := bytes.IndexByte(code[start:], 0)
		if end < 0 || end > 1024 {
			return nil, nil, fmt.Errorf("spell help %d description lacks a bounded terminator", slot)
		}
		d.Descriptions[slot] = string(code[start : start+end])
	}
	animationRules, err := populous2.DecodeNativeSpellHelpAnimationRules(source.Executable)
	if err != nil {
		return nil, nil, err
	}
	for land := range d.Sequences {
		sprites, err := populous2.DecodeNativeSpriteBitmapBank(source, land)
		if err != nil {
			return nil, nil, err
		}
		decodedSprites := make([]*image.RGBA, len(sprites.Sprites))
		for i, sprite := range sprites.Sprites {
			if sprite.Width == 0 {
				continue
			}
			decodedSprites[i], err = populous2.DecodeNativeMaskedPlanes(sprite.Planes, sprite.Width, sprite.Height, palette)
			if err != nil {
				return nil, nil, err
			}
		}
		tiles, err := populous2.DecodeTiles(source.Raw[fmt.Sprintf("block%d.pak", land)], palette)
		if err != nil {
			return nil, nil, err
		}
		for slot := range d.Sequences[land] {
			sequence := &d.Sequences[land][slot]
			state := animationRules.NewState()
			if err := animationRules.Begin(&state, uint16(slot*2)); err != nil {
				return nil, nil, err
			}
			seen := map[uint16]int{}
			registers := [8]uint32{}
			for count := 0; count <= 256; count++ {
				before := state.Image.AudioBank
				plan, err := animationRules.Advance(&state, source.Raw[fmt.Sprintf("block%d.pak", land)], &registers)
				if err != nil {
					return nil, nil, err
				}
				phase := helpVisualPhase(code, slot, state.Frame)
				if first, ok := seen[phase]; ok {
					sequence.LoopStart = first
					break
				}
				if count == 256 {
					return nil, nil, fmt.Errorf("spell help %d animation exceeds its bounded loop", slot)
				}
				seen[phase] = len(sequence.Frames)
				canvas, err := paintHelpPlan(plan, decodedSprites, tiles)
				if err != nil {
					return nil, nil, err
				}
				cropped, position := cropHelpFrame(canvas)
				frame := visualassets.HelpFrameDescriptor{Image: visualassets.Region{X: add(cropped)}, Position: position}
				for cue := 1; cue < 133; cue++ {
					at := cue * 10
					if binary.BigEndian.Uint16(before[at:]) != binary.BigEndian.Uint16(state.Image.AudioBank[at:]) {
						frame.SoundCues = append(frame.SoundCues, cue)
					}
				}
				sequence.Frames = append(sequence.Frames, frame)
			}
		}
	}
	return d, images, nil
}

// helpVisualPhase normalizes only the importer's known graphic counter forms.
// It is not serialized; runtime playback uses explicit image frames and loops.
func helpVisualPhase(code []byte, slot int, counter uint16) uint16 {
	body := 0x52ac + int(int16(binary.BigEndian.Uint16(code[0x52ac+slot*2:])))
	switch body {
	case 0x5526, 0x53cc, 0x53e0, 0x5314, 0x546e, 0x5476:
		return 0
	case 0x531c, 0x5336, 0x537a, 0x53ec:
		return counter & 3
	case 0x5358:
		return counter & 15
	default:
		return counter
	}
}

func paintHelpPlan(plan populous2.NativeSpellHelpAnimationFrame, sprites, tiles []*image.RGBA) (*image.RGBA, error) {
	canvas := image.NewRGBA(image.Rect(0, 0, 320, 200))
	for _, sprite := range plan.Sprites {
		if sprite.Sprite < 0 || sprite.Sprite >= len(sprites) || sprites[sprite.Sprite] == nil {
			return nil, fmt.Errorf("spell help sprite is outside artwork bank")
		}
		img := sprites[sprite.Sprite]
		draw.Draw(canvas, img.Bounds().Add(image.Pt(int(sprite.X), int(sprite.Y))), img, image.Point{}, draw.Over)
	}
	for _, tile := range plan.Tiles {
		if int(tile.Tile) >= len(tiles) || tile.ByteOffset < 0 {
			return nil, fmt.Errorf("spell help tile is outside artwork bank")
		}
		img := tiles[tile.Tile]
		position := image.Pt(int(tile.ByteOffset%40)*8, int(tile.ByteOffset/40))
		draw.Draw(canvas, img.Bounds().Add(position), img, image.Point{}, draw.Over)
	}
	return canvas, nil
}

func cropHelpFrame(canvas *image.RGBA) (*image.RGBA, image.Point) {
	bounds := image.Rectangle{}
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			if canvas.RGBAAt(x, y).A != 0 {
				p := image.Rect(x, y, x+1, y+1)
				if bounds.Empty() {
					bounds = p
				} else {
					bounds = bounds.Union(p)
				}
			}
		}
	}
	if bounds.Empty() {
		return image.NewRGBA(image.Rect(0, 0, 1, 1)), image.Point{}
	}
	cropped := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(cropped, cropped.Bounds(), canvas, bounds.Min, draw.Src)
	return cropped, bounds.Min
}
