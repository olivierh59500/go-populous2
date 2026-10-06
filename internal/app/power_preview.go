package app

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"

	"go-populous2/internal/engine"
)

type PowerPreview struct {
	World            *engine.World
	Power            engine.PowerID
	Updates          int
	Return           Screen
	Failure          string
	CameraX, CameraY int
}

func newPowerPreview(assets *Assets, id engine.PowerID) (*PowerPreview, error) {
	if assets == nil || len(assets.Levels) == 0 {
		return nil, fmt.Errorf("preview assets missing")
	}
	if _, ok := engine.PowerByID(id); !ok {
		return nil, fmt.Errorf("unknown preview power")
	}
	level := assets.Levels[0]
	level.WorldParameters = [60]byte{}
	for owner := range level.Players {
		p := &level.Players[owner]
		p.Groups = 0
		p.Population = 100
		p.Mana = 1000000
		p.MovementSpeed = 16
		p.Attrition = 0
		p.Scenario = engine.ScenarioOptions{BuildAnywhere: true}
		p.Extra[0] = 1
		for power := range p.Powers {
			p.Powers[power] = true
		}
	}
	w, err := engine.NewWorld(level, assets.Landscapes[level.Landscape])
	if err != nil {
		return nil, err
	}
	// Use an unoccupied stage rather than the live campaign's objects or RNG.
	for _, a := range w.Nature.Scenery {
		if a.Kind != engine.SceneryNone {
			if err := w.EditorClearCell(int(a.X), int(a.Y)); err != nil {
				return nil, err
			}
		}
	}
	var heights [engine.CornerSize * engine.CornerSize]uint8
	for y := 27; y <= 36; y++ {
		for x := 27; x <= 36; x++ {
			heights[x+y*engine.CornerSize] = 1
		}
	}
	if err := w.EditorSetTerrain(heights); err != nil {
		return nil, err
	}
	for _, placement := range []struct{ owner, x, y, pop int }{{0, 30, 31, 5000}, {1, 33, 31, 900}, {1, 33, 33, 300}, {0, 29, 32, 100}} {
		if err := w.EditorPlaceFollower(placement.owner, placement.x, placement.y, placement.pop); err != nil {
			return nil, err
		}
	}
	for owner := range w.Players {
		w.Players[owner].Computer = false
		w.Players[owner].Mana = 1000000
	}
	preview := &PowerPreview{World: w, Power: id, CameraX: 27, CameraY: 27}
	target := engine.PowerTarget{X: 31, Y: 31, Direction: 1}
	switch id {
	case engine.Trees, engine.Flowers, engine.Swamp, engine.Fungus, engine.Baptism:
		target.X, target.Y = 31, 33
	case engine.Plague:
		target.X, target.Y = 33, 31
	case engine.Basalt:
		target.X, target.Y = 25, 29
		preview.CameraX = 24
	case engine.Whirlpool:
		target.X, target.Y = 24, 29
		preview.CameraX = 23
	case engine.Tsunami:
		target.X, target.Y = 27, 31
	case engine.Wall:
		if err := w.Cast(0, id, engine.PowerTarget{X: 31, Y: 30}); err != nil {
			return nil, err
		}
		target.X, target.Y = 32, 30
	case engine.Road:
		for _, p := range [][2]int{{30, 30}, {31, 30}, {32, 30}} {
			if err := w.Cast(0, id, engine.PowerTarget{X: p[0], Y: p[1]}); err != nil {
				return nil, err
			}
		}
		target.X, target.Y = 32, 31
	case engine.Lightning:
		target.X, target.Y = 33, 31
	case engine.PapalMagnet:
		target.X, target.Y = 33, 32
	}
	if err := w.Cast(0, id, target); err != nil {
		return nil, fmt.Errorf("preview cast: %w", err)
	}
	if _, hero := engine.HeroKindForPower(id); hero {
		w.SetMode(0, engine.Fight)
		w.SetMode(1, engine.Settle)
	}
	if id == engine.Lightning {
		if err := w.ActivateLightning(0); err != nil {
			return nil, err
		}
	}
	return preview, nil
}

func (g *Game) openPowerHelp(id engine.PowerID) error {
	p, err := newPowerPreview(g.Assets, id)
	if err != nil {
		return err
	}
	p.Return = g.Screen
	g.PowerPreview = p
	g.Screen = PowerHelpScreen
	return nil
}

func (g *Game) updatePowerHelp(x, y int, clicked bool) error {
	p := g.PowerPreview
	if p == nil {
		return fmt.Errorf("power help has no preview")
	}
	if clicked && y >= 178 {
		g.Screen = p.Return
		g.PowerPreview = nil
		return nil
	}
	p.Updates++
	if p.Updates%4 == 0 {
		p.World.Step()
	}
	if p.Updates >= 500 {
		fresh, err := newPowerPreview(g.Assets, p.Power)
		if err != nil {
			return err
		}
		fresh.Return = p.Return
		g.PowerPreview = fresh
	}
	return nil
}

func (g *Game) drawPowerHelp() {
	if g.PowerPreview == nil {
		return
	}
	p := g.PowerPreview
	previewGame := *g
	previewGame.World = p.World
	previewGame.Network = nil
	previewGame.music = nil
	previewGame.audio = nil
	previewGame.CameraX, previewGame.CameraY = p.CameraX, p.CameraY
	previewGame.PickingPower = false
	previewGame.messageUntil = 0
	previewGame.drawWorld()
	draw.Draw(g.framebuffer, image.Rect(0, 0, 320, 24), image.NewUniform(color.RGBA{40, 45, 18, 255}), image.Point{}, draw.Src)
	power, _ := engine.PowerByID(p.Power)
	g.text(strings.ToUpper(power.Name), 8, 5)
	draw.Draw(g.framebuffer, image.Rect(0, 159, 320, 200), image.NewUniform(color.RGBA{40, 45, 18, 255}), image.Point{}, draw.Src)
	for row, line := range powerDescriptions[p.Power] {
		g.text(line, 8, 162+row*9)
	}
	g.button("BACK", 120, 181, 80)
}

var powerDescriptions = map[engine.PowerID][2]string{
	engine.RaiseLower:  {"SHAPE LAND TO SUPPORT YOUR PEOPLE.", "LEFT RAISES; RIGHT LOWERS TERRAIN."},
	engine.PapalMagnet: {"PLACE A MAGNET AND CALL YOUR LEADER.", "RALLY FOLLOWERS INTO AN EXPEDITION."},
	engine.Perseus:     {"PERSEUS HUNTS OPPOSING GROUPS.", "USE A STRONG LEADER FOR THE HERO."},
	engine.Plague:      {"INFECT OPPOSING PEOPLE AND TOWNS.", "INFECTION FOLLOWS MERGES AND BIRTHS."},
	engine.Armageddon:  {"CONVERT ELIGIBLE GROUPS INTO HEROES.", "HEROES CAN THEN FORCE LAND RAISING."},
	engine.Trees:       {"PLANT A CLUSTER OF TREES ON LAND.", "MATURE TREES CAN SPREAD FIRE."},
	engine.Flowers:     {"RENEW SAMPLED DAMAGED PARCELS.", "RESTORE LAND FOR NEW SETTLEMENTS."},
	engine.Swamp:       {"PLACE SWAMPS ON EMPTY FERTILE LAND.", "ENTERING GROUPS SINK INTO THE GROUND."},
	engine.Fungus:      {"PLANT SEEDS; RECAST TO EXTEND THEM.", "MATURE FUNGUS SPREADS AND IS DEADLY."},
	engine.Adonis:      {"ADONIS ATTRACTS OPPOSING PEOPLE.", "HE IS IMMUNE TO SWAMP AND FUNGUS."},
	engine.Road:        {"JOIN ROADS ACROSS LAND AND RAMPS.", "NEIGHBOURS CONNECT AUTOMATICALLY."},
	engine.Wall:        {"BUILD WALLS CONNECTED TO YOUR CHAIN.", "ROADS CREATE GATES THROUGH WALLS."},
	engine.Earthquake:  {"SEND A BRANCHING CRACK ACROSS LAND.", "DIRECTION CONTROLS ITS FIRST FRONT."},
	engine.Batholith:   {"RAISE A SAMPLED POINT OR ADD A ROCK.", "ROCKS BLOCK FOLLOWER MOVEMENT."},
	engine.Heracles:    {"HERACLES DOUBLES THE GROUP'S PEOPLE.", "HIS STRENGTH HELPS BREAK OPPONENTS."},
	engine.Lightning:   {"PLACE THE MARKER, THEN FIRE A VOLLEY.", "WALLS STOP THE LIGHTNING'S SCAN."},
	engine.Whirlwind:   {"A MOVING WHIRLWIND CARRIES PEOPLE.", "CARRIED GROUPS EVENTUALLY LAND."},
	engine.Storm:       {"A STORM STRIKES ITS LOCAL PARCELS.", "LAND AND WATER HAVE DIFFERENT HITS."},
	engine.Odysseus:    {"ODYSSEUS PURSUES OPPOSING GROUPS.", "WHIRLWINDS CANNOT LIFT THIS HERO."},
	engine.Wind:        {"SEND WIND ALONG A CHOSEN DIRECTION.", "THE FRONT PUSHES GROUPS IT CROSSES."},
	engine.FireColumn:  {"A COLUMN OF FIRE MOVES ACROSS LAND.", "IT BURNS PEOPLE, TREES AND TOWNS."},
	engine.FireRain:    {"METEORS FALL ACROSS SAMPLED PARCELS.", "IMPACTS CAN ALTER THE LANDSCAPE."},
	engine.Volcano:     {"A VOLCANO GROWS AND EMITS LAVA.", "FLOWS FOLLOW SUPPORTED SLOPES."},
	engine.Achilles:    {"ACHILLES FIGHTS OPPOSING GROUPS.", "FIRE DOES NOT HARM THIS HERO."},
	engine.Basalt:      {"SEND A LINE OF BASALT ACROSS WATER.", "PERMANENT BASALT STOPS WATER FRONTS."},
	engine.Whirlpool:   {"START ON FOUR CLEAR WATER PARCELS.", "A MOVING WHIRLPOOL ERODES THE SHORE."},
	engine.Baptism:     {"PLACE FONTS TO CONVERT ENTERING MEN.", "CONVERSION FINISHES ITS ANIMATION."},
	engine.Helen:       {"HELEN CAPTURES OPPOSING PEOPLE.", "SHE CAN CROSS WATER SAFELY."},
	engine.Tsunami:     {"FOUR WATER FRONTS EXPAND FROM LAND.", "WAVES LOWER SHALLOW SHORE PARCELS."},
}
