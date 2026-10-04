package populous2

import (
	"fmt"
	"image"
	"io/fs"
	"os"
	"strings"

	embedded "go-populous2/assets"
	"go-populous2/internal/amiga"
)

type Bundle struct {
	Executable     *amiga.Executable
	Resources      []Resource
	Raw            map[string][]byte
	Landscapes     [4]Landscape
	Tiles          [4][]*image.RGBA
	Sprites        [4][]Sprite
	Background     *image.RGBA
	Levels         []Level
	Spells         []Spell
	ManaRules      ManaRules
	HeroRules      HeroRules
	Audio          *AudioBank
	Actions        []NativeAction
	GroundRules    GroundEffectRules
	HillParameters [4][4]int
}

func Load() (*Bundle, error) {
	files, err := fs.Sub(embedded.Files, "amiga")
	if err != nil {
		return nil, err
	}
	if dir := os.Getenv("POPULOUS2_DATA_DIR"); dir != "" {
		files = os.DirFS(dir)
	}
	return LoadFS(files)
}

func LoadFS(files fs.FS) (*Bundle, error) {
	paths, err := resourcePaths(files)
	if err != nil {
		return nil, err
	}
	name, found := paths["POPULOUS.II"]
	if !found {
		return nil, fmt.Errorf("Populous II executable populous.ii missing")
	}
	data, err := fs.ReadFile(files, name)
	if err != nil {
		return nil, err
	}
	exe, err := amiga.ParseExecutable(data)
	if err != nil {
		return nil, err
	}
	resources, err := ResourceTable(exe)
	if err != nil {
		return nil, err
	}
	b := &Bundle{Executable: exe, Resources: resources, Raw: make(map[string][]byte)}
	for _, resource := range resources {
		decoded, err := LoadResource(files, resource)
		if err != nil {
			return nil, err
		}
		b.Raw[strings.ToLower(resource.Name)] = decoded
	}
	for i := range b.Landscapes {
		land, err := DecodeLandscape(b.Raw[fmt.Sprintf("land%d.dat", i)])
		if err != nil {
			return nil, fmt.Errorf("landscape %d: %w", i, err)
		}
		b.Landscapes[i] = land
		tiles, err := DecodeTiles(b.Raw[fmt.Sprintf("block%d.pak", i)], land.Palettes[0])
		if err != nil {
			return nil, fmt.Errorf("landscape %d tiles: %w", i, err)
		}
		b.Tiles[i] = tiles
		s16, err := ApplySpriteDifference(b.Raw["s16-0.pak"], b.Raw[fmt.Sprintf("s16-%d.dif", i)])
		if err != nil {
			return nil, err
		}
		largeDiff := fmt.Sprintf("s32-%d.pif", i)
		if i == 0 {
			largeDiff = "s32-0.dif"
		}
		s32, err := ApplySpriteDifference(b.Raw["s32-0.pak"], b.Raw[largeDiff])
		if err != nil {
			return nil, err
		}
		sprites, err := DecodeSprites(exe, s16, s32, land.Palettes[0])
		if err != nil {
			return nil, err
		}
		b.Sprites[i] = sprites
	}
	b.Background, err = DecodeScreen(b.Raw["qaz.pak"], b.Landscapes[0].Palettes[0])
	if err != nil {
		return nil, err
	}
	b.Levels, err = DecodeCampaign(b.Raw["conquest.pak"])
	if err != nil {
		return nil, err
	}
	b.Spells, err = DecodeSpells(exe)
	if err != nil {
		return nil, err
	}
	b.ManaRules, err = DecodeManaRules(exe)
	if err != nil {
		return nil, err
	}
	b.HeroRules, err = DecodeHeroRules(exe)
	if err != nil {
		return nil, err
	}
	b.Audio, err = DecodeAudioBank(exe, b.Raw["fx.dat"])
	if err != nil {
		return nil, err
	}
	b.Actions, err = DecodeActions(exe)
	if err != nil {
		return nil, err
	}
	b.GroundRules, err = DecodeGroundEffectRules(exe)
	if err != nil {
		return nil, err
	}
	b.HillParameters, err = DecodeHillParameters(exe)
	if err != nil {
		return nil, err
	}
	return b, nil
}
