package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sort"

	"go-populous2/internal/engine"
	"go-populous2/internal/populous2"
	"go-populous2/internal/visualassets"
)

func exportPointers(output string, source *populous2.Bundle) error {
	rules, err := populous2.DecodeNativeRenderFrameRules(source.Executable)
	if err != nil {
		return err
	}
	raw := make([]byte, 0x11280)
	memory := populous2.FollowerCleanupMemory{
		Read8: func(at int) (uint8, error) { return raw[at], nil }, Read16: func(at int) (uint16, error) { return binary.BigEndian.Uint16(raw[at:]), nil }, Read32: func(at int) (uint32, error) { return binary.BigEndian.Uint32(raw[at:]), nil },
		Write8: func(at int, v uint8) error { raw[at] = v; return nil }, Write16: func(at int, v uint16) error { binary.BigEndian.PutUint16(raw[at:], v); return nil }, Write32: func(at int, v uint32) error { binary.BigEndian.PutUint32(raw[at:], v); return nil },
	}
	imageState := rules.Images.NewImageState()
	selectFrame := func(command uint16, tick int) (uint16, error) {
		binary.BigEndian.PutUint16(raw[0xeb18:], command)
		binary.BigEndian.PutUint16(raw[0xf42:], uint16(tick))
		input, err := populous2.NewNativeInputState(source.Executable)
		if err != nil {
			return 0, err
		}
		frame := populous2.NativeFrameRegisterContext{}
		_, err = rules.Cursor(populous2.NativeRenderFrameCallbacks{Memory: memory, Frame: &frame, Input: &input, Image: &imageState})
		return input.Mouse.Image, err
	}
	selectors := map[string][]uint16{"normal": {0}, "forbidden": {0x360}, "interface": {0x750}, "inspect": {0x5a0}}
	for _, power := range source.Spells {
		if power.ID == populous2.RaiseLower {
			continue
		}
		command := uint16(0)
		for _, action := range source.Actions {
			if action.Spell == power.ID {
				command = uint16(action.Command)
				break
			}
		}
		if command == 0 {
			return fmt.Errorf("pointer action missing for power%d", power.ID)
		}
		name := fmt.Sprintf("power/%d", engine.PowerFromCostSlot(engine.PowerID(power.ID)))
		for phase := 0; phase < 4; phase++ {
			selector, err := selectFrame(command, phase*8)
			if err != nil {
				return err
			}
			selectors[name] = append(selectors[name], selector)
		}
	}
	names := make([]string, 0, len(selectors))
	for name := range selectors {
		names = append(names, name)
	}
	sort.Strings(names)
	descriptor := visualassets.PointerDescriptor{Frames: make(map[string][]visualassets.Region), MapMarkerSprite: (0x219da - 0x21626) / 12}
	for shape := 0; shape < 16; shape++ {
		for vertex := 0; vertex < 4; vertex++ {
			at := 0x33194 + shape*16 + vertex*4
			descriptor.MapOutlines[shape][vertex] = [2]int{int(int16(binary.BigEndian.Uint16(source.Executable.Hunks[0].Data[at:]))), int(int16(binary.BigEndian.Uint16(source.Executable.Hunks[0].Data[at+2:])))}
		}
	}
	var images []*image.RGBA
	for _, name := range names {
		for _, selector := range selectors[name] {
			img := image.NewRGBA(image.Rect(0, 0, 16, 16))
			first := 0x27ec + int(selector)
			second := first + 72
			if second+68 > len(source.Executable.Hunks[3].Data) {
				return fmt.Errorf("pointer pixels outside graphic asset")
			}
			art := source.Executable.Hunks[3].Data
			colors := source.Executable.Hunks[0].Data[0x3361a:]
			for y := 0; y < 16; y++ {
				for x := 0; x < 16; x++ {
					words := [4]uint16{binary.BigEndian.Uint16(art[first+4+y*4:]), binary.BigEndian.Uint16(art[first+6+y*4:]), binary.BigEndian.Uint16(art[second+4+y*4:]), binary.BigEndian.Uint16(art[second+6+y*4:])}
					index := uint8(0)
					for plane, word := range words {
						index |= uint8(word>>uint(15-x)&1) << uint(plane)
					}
					if index != 0 {
						img.SetRGBA(x, y, populous2.AmigaColor(binary.BigEndian.Uint16(colors[(16+int(index))*2:])))
					}
				}
			}
			images = append(images, img)
		}
	}
	atlas, regions, err := pack("pointers.png", images, nil)
	if err != nil {
		return err
	}
	if err := writePNG(output, "pointers.png", atlas); err != nil {
		return err
	}
	at := 0
	for _, name := range names {
		for range selectors[name] {
			descriptor.Frames[name] = append(descriptor.Frames[name], regions[at])
			at++
		}
	}
	data, err := json.MarshalIndent(descriptor, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(output, visualassets.PointerFile), append(data, '\n'), 0644)
}
