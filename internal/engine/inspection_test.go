package engine

import "testing"

func TestInspectionClassSurvivesOnlySourcePreservingSlotReuse(t *testing.T) {
	w := testFlatWorld()
	id := w.allocateEffect(EffectFireColumn, 0)
	w.Fire.Columns[id] = FireEffect{Active: true, Owner: 0, X: 20*256 + 123, Y: 30*256 + 254}
	if x, y, ok := w.EffectInspectionPoint(id, 0, InspectFireColumn); !ok || x != 20 || y != 30 {
		t.Fatal("assigned column inspection coordinates differ")
	}
	if _, _, ok := w.EffectInspectionPoint(id, 1, InspectFireColumn); ok {
		t.Fatal("enemy controller admitted by owned effect scan")
	}
	w.releaseEffect(id)
	next := w.allocateEffect(EffectVolcano, 0)
	w.Fire.Columns[id].Active = false
	w.Fire.Volcano[next] = FireEffect{Active: true, Owner: 0, X: 40*256 + 128, Y: 41*256 + 128}
	if next != id || w.EffectiveInspectionClass(next) != InspectFireColumn {
		t.Fatal("volcano erased source-preserved slot inspection identity")
	}
	if x, y, ok := w.EffectInspectionPoint(next, 0, InspectFireColumn); !ok || x != 40 || y != 41 {
		t.Fatal("retained identity did not query actual current controller")
	}
	s := w.Snapshot()
	restored, err := s.Restore()
	if err != nil {
		t.Fatal(err)
	}
	if restored.EffectiveInspectionClass(next) != InspectFireColumn {
		t.Fatal("snapshot lost retained inspection identity")
	}
	w.releaseEffect(next)
	w.allocateEffect(EffectWhirlwind, 0)
	if w.EffectiveInspectionClass(id) != InspectWhirlwind {
		t.Fatal("class-setting creator retained a prior identity")
	}
}

func TestInspectionUnspecifiedRemainsDistinctFromAssignedLegacyFallback(t *testing.T) {
	w := testFlatWorld()
	for _, kind := range []EffectKind{EffectVolcano, EffectEarthquake, EffectHurricane} {
		id := w.allocateEffect(kind, 0)
		if w.EffectiveInspectionClass(id) != InspectUnspecified {
			t.Fatal("unmapped creator invented an inspection class", kind)
		}
	}
	id := w.allocateEffect(EffectLightning, 0)
	w.effects.Slots[id].InspectionClass = InspectUnspecified
	w.Air.Bolts[id] = LightningBolt{Active: true, Owner: 0, X: 1000, Y: 2000}
	if w.EffectiveInspectionClass(id) != InspectLightningBolt {
		t.Fatal("legacy bolt inferred marker identity")
	}
}

func TestInspectionQueriesEveryPresentControllerFamily(t *testing.T) {
	for _, kind := range []EffectKind{EffectFireColumn, EffectFireRain, EffectVolcano, EffectLava, EffectFungus, EffectLightning, EffectWhirlwind, EffectStorm, EffectBasalt, EffectTidalWave, EffectWhirlpool, EffectEarthquake, EffectHurricane} {
		w := testFlatWorld()
		id := w.allocateEffect(kind, 0)
		fixedX, fixedY := 20*256+254, 30*256+123
		switch kind {
		case EffectFireColumn:
			w.Fire.Columns[id] = FireEffect{Active: true, X: fixedX, Y: fixedY}
		case EffectFireRain:
			w.Fire.Rain[id] = FireEffect{Active: true, X: fixedX, Y: fixedY, Phase: MeteorWaiting}
		case EffectVolcano:
			w.Fire.Volcano[id] = FireEffect{Active: true, X: fixedX, Y: fixedY}
		case EffectLava:
			w.Fire.Lava[id] = FireEffect{Active: true, X: fixedX, Y: fixedY}
		case EffectFungus:
			w.Nature.Fungi[id] = FungusController{Active: true, MinX: 20, MinY: 30}
		case EffectLightning:
			w.Air.Markers[id] = LightningMarker{Active: true, X: fixedX, Y: fixedY}
		case EffectWhirlwind:
			w.Air.Whirlwinds[id] = WhirlwindEffect{Active: true, X: fixedX, Y: fixedY}
		case EffectStorm:
			w.Air.Storms[id] = StormEffect{Active: true, X: fixedX, Y: fixedY}
		case EffectBasalt:
			w.Water.Basalt[id] = BasaltEffect{Active: true, X: 20, Y: 30}
		case EffectTidalWave:
			w.Water.Waves[id] = TidalEffect{Active: true, X: fixedX, Y: fixedY}
		case EffectWhirlpool:
			w.Water.Whirlpools[id] = WhirlpoolEffect{Active: true, X: 20, Y: 30}
		case EffectEarthquake:
			w.Earth.Quakes[id] = QuakeEffect{Active: true, X: 20, Y: 30}
		case EffectHurricane:
			w.Wind[id] = WindEffect{Active: true, X: 20, Y: 30}
		}
		if x, y, ok := w.EffectInspectionPoint(id, 0, w.EffectiveInspectionClass(id)); !ok || x != 20 || y != 30 {
			t.Fatal("controller query lost its actual map coordinate", kind, x, y, ok)
		}
	}
}
