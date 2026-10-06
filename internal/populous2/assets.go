package populous2

import (
	"fmt"
	"image"
	"io/fs"
	"strings"

	embedded "go-populous2/assets"
	"go-populous2/internal/amiga"
)

type Bundle struct {
	Executable        *amiga.Executable
	Resources         []Resource
	Raw               map[string][]byte
	Landscapes        [4]Landscape
	Tiles             [4][]*image.RGBA
	Sprites           [4][]Sprite
	Background        *image.RGBA
	Levels            []Level
	Spells            []Spell
	ManaRules         ManaRules
	HeroRules         HeroRules
	FollowerMotion    FollowerMotionRules
	FollowerDecision  FollowerDecisionRules
	TownEvaluator     NativeTownEvaluator
	TownCenterArt     NativeTownCenterArt
	MagnetRules       NativeMagnetRules
	FollowerEntry     FollowerEntryRules
	FollowerCombat    FollowerCombatRules
	TownCombat        TownCombatRules
	FollowerAftermath FollowerAftermathRules
	CommonPrepass     CommonPrepassRules
	FollowerHero      FollowerHeroRules
	FollowerTerrain   FollowerTerrainRules
	FollowerMagnet    FollowerMagnetRules
	FollowerRuin      FollowerRuinVictimRules
	FollowerCrossing  FollowerCrossingRules
	PrimitiveCreators NativePrimitiveCreatorRules
	NeutralRules      NativeNeutralRules
	EarthquakeRules   EarthquakeRules
	VolcanoRules      VolcanoRules
	LavaRules         NativeLavaRules
	StormRules        StormRules
	FireRainRules     FireRainRules
	HurricaneRules    HurricaneRules
	TsunamiRules      TsunamiRules
	PlagueRules       PlagueRules
	ArmageddonRules   ArmageddonRules
	ForestNative      ForestNativeRules
	RenewNative       RenewNativeRules
	CampaignResult    CampaignResultRules
	NativeAI          NativeAIRules
	Audio             *AudioBank
	Actions           []NativeAction
	GroundRules       GroundEffectRules
	NativeGround      NativeGroundRules
	HillParameters    [4][4]int
	PlagueAnimation   []AnimationFrame
	RoadRules         RoadRules
	Scenery           *SceneryBank
	BatholithRange    int
	DeityArt          *DeityArt
	FireColumns       FireColumnRules
	NativeFireColumn  NativeFireColumnRules
	SceneryRender     SceneryRenderRules
	Whirlwinds        WhirlwindRules
	NativeWhirlwind   NativeWhirlwindRules
	WhirlwindFollower WhirlwindFollowerRules
	Whirlpools        WhirlpoolRules
	BasaltRules       BasaltRules
	LightningRules    LightningRules
	FungusRules       FungusRules
	NativeFungus      NativeFungusRules
	FungusHazards     FungusHazardRules
	WallRules         WallRules
}

func Load() (*Bundle, error) {
	files, err := embedded.DataFS()
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("Populous II game data are missing: run go run ./cmd/import-assets with your original ADF files and compatible executable (see docs/ASSET_SETUP.md), or set POPULOUS2_DATA_DIR")
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
	b.FollowerMotion, err = DecodeFollowerMotionRules(exe)
	if err != nil {
		return nil, err
	}
	b.FollowerDecision, err = DecodeFollowerDecisionRules(exe)
	if err != nil {
		return nil, err
	}
	b.TownEvaluator, err = DecodeNativeTownEvaluator(exe)
	if err != nil {
		return nil, err
	}
	b.TownCenterArt, err = DecodeNativeTownCenterArt(exe)
	if err != nil {
		return nil, err
	}
	b.MagnetRules, err = DecodeNativeMagnetRules(exe)
	if err != nil {
		return nil, err
	}
	b.FollowerEntry, err = DecodeFollowerEntryRules(exe)
	if err != nil {
		return nil, err
	}
	b.FollowerCombat, err = DecodeFollowerCombatRules(exe)
	if err != nil {
		return nil, err
	}
	b.TownCombat, err = DecodeTownCombatRules(exe)
	if err != nil {
		return nil, err
	}
	b.FollowerAftermath, err = DecodeFollowerAftermathRules(exe)
	if err != nil {
		return nil, err
	}
	b.CommonPrepass, err = DecodeCommonPrepassRules(exe)
	if err != nil {
		return nil, err
	}
	b.FollowerHero, err = DecodeFollowerHeroRules(exe)
	if err != nil {
		return nil, err
	}
	b.FollowerTerrain, err = DecodeFollowerTerrainRules(exe)
	if err != nil {
		return nil, err
	}
	b.FollowerMagnet, err = DecodeFollowerMagnetRules(exe)
	if err != nil {
		return nil, err
	}
	b.FollowerRuin, err = DecodeFollowerRuinVictimRules(exe)
	if err != nil {
		return nil, err
	}
	b.FollowerCrossing, err = DecodeFollowerCrossingRules(exe)
	if err != nil {
		return nil, err
	}
	b.PrimitiveCreators, err = DecodeNativePrimitiveCreatorRules(exe)
	if err != nil {
		return nil, err
	}
	b.NeutralRules, err = DecodeNativeNeutralRules(exe)
	if err != nil {
		return nil, err
	}
	b.EarthquakeRules, err = DecodeEarthquakeRules(exe)
	if err != nil {
		return nil, err
	}
	b.VolcanoRules, err = DecodeVolcanoRules(exe)
	if err != nil {
		return nil, err
	}
	b.LavaRules, err = DecodeNativeLavaRules(exe)
	if err != nil {
		return nil, err
	}
	b.StormRules, err = DecodeStormRules(exe)
	if err != nil {
		return nil, err
	}
	b.FireRainRules, err = DecodeFireRainRules(exe)
	if err != nil {
		return nil, err
	}
	b.HurricaneRules, err = DecodeHurricaneRules(exe)
	if err != nil {
		return nil, err
	}
	b.TsunamiRules, err = DecodeTsunamiRules(exe)
	if err != nil {
		return nil, err
	}
	b.PlagueRules, err = DecodePlagueRules(exe)
	if err != nil {
		return nil, err
	}
	b.ArmageddonRules, err = DecodeArmageddonRules(exe)
	if err != nil {
		return nil, err
	}
	b.ForestNative, err = DecodeForestNativeRules(exe)
	if err != nil {
		return nil, err
	}
	b.RenewNative, err = DecodeRenewNativeRules(exe)
	if err != nil {
		return nil, err
	}
	b.CampaignResult, err = DecodeCampaignResultRules(exe)
	if err != nil {
		return nil, err
	}
	b.NativeAI, err = DecodeNativeAIRules(exe)
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
	b.NativeGround, err = DecodeNativeGroundRules(exe)
	if err != nil {
		return nil, err
	}
	b.HillParameters, err = DecodeHillParameters(exe)
	if err != nil {
		return nil, err
	}
	b.PlagueAnimation, err = DecodeAnimation(exe, 0xddc)
	if err != nil {
		return nil, err
	}
	b.RoadRules, err = DecodeRoadRules(exe)
	if err != nil {
		return nil, err
	}
	b.Scenery, err = DecodeScenery(exe)
	if err != nil {
		return nil, err
	}
	b.BatholithRange, err = DecodeBatholithRange(exe)
	if err != nil {
		return nil, err
	}
	b.WallRules, err = DecodeWallRules(exe)
	if err != nil {
		return nil, err
	}
	b.DeityArt, err = DecodeDeityArt(exe, b.Raw["faces.pak"], b.Landscapes[0].Palettes[0])
	if err != nil {
		return nil, err
	}
	b.FireColumns, err = DecodeFireColumnRules(exe)
	if err != nil {
		return nil, err
	}
	b.NativeFireColumn, err = DecodeNativeFireColumnRules(exe)
	if err != nil {
		return nil, err
	}
	b.SceneryRender, err = DecodeSceneryRenderRules(exe)
	if err != nil {
		return nil, err
	}
	b.Whirlwinds, err = DecodeWhirlwindRules(exe)
	if err != nil {
		return nil, err
	}
	b.NativeWhirlwind, err = DecodeNativeWhirlwindRules(exe)
	if err != nil {
		return nil, err
	}
	b.WhirlwindFollower, err = DecodeWhirlwindFollowerRules(exe)
	if err != nil {
		return nil, err
	}
	b.Whirlpools, err = DecodeWhirlpoolRules(exe)
	if err != nil {
		return nil, err
	}
	b.BasaltRules, err = DecodeBasaltRules(exe)
	if err != nil {
		return nil, err
	}
	b.LightningRules, err = DecodeLightningRules(exe)
	if err != nil {
		return nil, err
	}
	b.FungusRules, err = DecodeFungusRules(exe)
	if err != nil {
		return nil, err
	}
	b.NativeFungus, err = DecodeNativeFungusRules(exe)
	if err != nil {
		return nil, err
	}
	b.FungusHazards, err = DecodeFungusHazardRules(exe)
	if err != nil {
		return nil, err
	}
	return b, nil
}
