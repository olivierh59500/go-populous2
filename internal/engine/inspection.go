package engine

// EffectInspectionClass names the retained presentation identity used by the
// original effect-icon scanner. A controller's simulation family can differ:
// volcano, quake and wind allocation deliberately preserve a reused identity.
type EffectInspectionClass uint8

const (
	InspectUnspecified EffectInspectionClass = iota
	InspectWhirlwind
	InspectFireColumn
	InspectWhirlpool
	InspectFungus
	InspectLightningMarker
	InspectLightningBolt
	InspectFireRain
	InspectEarthquake
	InspectBasalt
	InspectTidalWave
	InspectVolcano
	InspectStorm
	InspectLava
	InspectHurricane
)

func inspectionClassAssigned(kind EffectKind) (EffectInspectionClass, bool) {
	switch kind {
	case EffectFireColumn:
		return InspectFireColumn, true
	case EffectFireRain:
		return InspectFireRain, true
	case EffectLava:
		return InspectLava, true
	case EffectFungus:
		return InspectFungus, true
	case EffectLightning:
		return InspectLightningMarker, true
	case EffectWhirlwind:
		return InspectWhirlwind, true
	case EffectStorm:
		return InspectStorm, true
	case EffectBasalt:
		return InspectBasalt, true
	case EffectTidalWave:
		return InspectTidalWave, true
	case EffectWhirlpool:
		return InspectWhirlpool, true
	}
	return InspectUnspecified, false
}

// EffectiveInspectionClass also reads older/manual named snapshots whose
// explicit class predates this field. Only class-setting families infer their
// mandatory identity; source-preserving controllers keep Unspecified.
func (w *World) EffectiveInspectionClass(id int) EffectInspectionClass {
	if w == nil || id < 0 || id >= EffectCapacity {
		return InspectUnspecified
	}
	r := w.effects.Slots[id]
	if r.InspectionClass != InspectUnspecified {
		return r.InspectionClass
	}
	if r.Kind == EffectLightning && w.Air.Bolts[id].Active {
		return InspectLightningBolt
	}
	if class, assigned := inspectionClassAssigned(r.Kind); assigned {
		return class
	}
	return InspectUnspecified
}

// EffectInspectionPoint is a read-only controller query, including controllers
// that never join the map painter's actor chains. Coordinates are map cells.
func (w *World) EffectInspectionPoint(id, owner int, class EffectInspectionClass) (int, int, bool) {
	if w == nil || id < 0 || id >= EffectCapacity || owner < 0 || owner > 1 {
		return 0, 0, false
	}
	r := w.effects.Slots[id]
	if r.Kind == EffectNone || int(r.Owner) != owner || w.EffectiveInspectionClass(id) != class {
		return 0, 0, false
	}
	switch r.Kind {
	case EffectFireColumn:
		a := w.Fire.Columns[id]
		return a.X >> 8, a.Y >> 8, a.Active
	case EffectFireRain:
		a := w.Fire.Rain[id]
		return a.X >> 8, a.Y >> 8, a.Active
	case EffectVolcano:
		a := w.Fire.Volcano[id]
		return a.X >> 8, a.Y >> 8, a.Active
	case EffectLava:
		a := w.Fire.Lava[id]
		return a.X >> 8, a.Y >> 8, a.Active
	case EffectFungus:
		a := w.Nature.Fungi[id]
		return a.MinX, a.MinY, a.Active
	case EffectLightning:
		if a := w.Air.Markers[id]; a.Active {
			return a.X >> 8, a.Y >> 8, true
		}
		a := w.Air.Bolts[id]
		return a.X >> 8, a.Y >> 8, a.Active
	case EffectWhirlwind:
		a := w.Air.Whirlwinds[id]
		return a.X >> 8, a.Y >> 8, a.Active
	case EffectStorm:
		a := w.Air.Storms[id]
		return a.X >> 8, a.Y >> 8, a.Active
	case EffectBasalt:
		a := w.Water.Basalt[id]
		return a.X, a.Y, a.Active
	case EffectTidalWave:
		a := w.Water.Waves[id]
		return a.X >> 8, a.Y >> 8, a.Active
	case EffectWhirlpool:
		a := w.Water.Whirlpools[id]
		return a.X, a.Y, a.Active
	case EffectEarthquake:
		a := w.Earth.Quakes[id]
		return a.X, a.Y, a.Active
	case EffectHurricane:
		a := w.Wind[id]
		return a.X, a.Y, a.Active
	}
	return 0, 0, false
}
