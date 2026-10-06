package app

import "go-populous2/internal/engine"

// effectIconInspection records the actual secondary icon dispatch. Several
// ground-only powers intentionally consume secondary clicks without a scan.
func effectIconInspection(power engine.PowerID) (engine.EffectInspectionClass, bool) {
	switch power {
	case engine.Fungus, engine.Volcano:
		return engine.InspectUnspecified, true
	case engine.Lightning:
		return engine.InspectLightningMarker, true
	case engine.Whirlwind:
		return engine.InspectWhirlwind, true
	case engine.Storm:
		return engine.InspectStorm, true
	case engine.FireColumn:
		return engine.InspectFireColumn, true
	case engine.FireRain:
		return engine.InspectFireRain, true
	case engine.Basalt:
		return engine.InspectHurricane, true
	case engine.Whirlpool:
		return engine.InspectWhirlpool, true
	}
	return engine.InspectUnspecified, false
}

func (g *Game) inspectEffectIcon(power engine.PowerID, newPress bool) bool {
	wanted, scan := effectIconInspection(power)
	if !scan {
		return true
	}
	start := g.effectScanCursor
	if start < 0 || start >= engine.EffectCapacity {
		start = 0
	}
	id, advance := start, !newPress
	for visited := 0; visited < engine.EffectCapacity; visited++ {
		if advance {
			id = (id + 1) % engine.EffectCapacity
			if id == start {
				break
			}
		}
		advance = true
		x, y, found := g.World.EffectInspectionPoint(id, g.playerSide(), wanted)
		if found {
			g.effectScanCursor = id
			g.CameraX, g.CameraY = max(0, min(56, x-4)), max(0, min(56, y-4))
			return true
		}
	}
	return true
}
