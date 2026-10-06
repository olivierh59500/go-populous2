package gamcodec

import (
	"reflect"
	"testing"

	"go-populous2/internal/engine"
)

func TestGAMEffectFamiliesPreserveTypedControllerFields(t *testing.T) {
	w := &engine.World{}
	w.Fire.Columns[0] = engine.FireEffect{Active: true, Owner: 0, X: 8320, Y: 8576, VX: 16, VY: -16, Phase: engine.FireMoving, Frame: 1, Timer: 30, Life: 177}
	w.Air.Whirlwinds[1] = engine.WhirlwindEffect{Active: true, Owner: 1, X: 8320, Y: 8576, VX: -24, VY: 24, Phase: engine.WhirlwindMoving, Frame: 1, Timer: 120, Life: 200}
	w.Fire.Rain[2] = engine.FireEffect{Active: true, Owner: 0, X: 8320, Y: 8576, Phase: engine.MeteorFalling, Frame: 7, Timer: 0, Life: 17}
	w.Air.Storms[3] = engine.StormEffect{Active: true, Owner: 1, X: 8320, Y: 8576, Frame: 2, Timer: 1, Life: 123, ImpactActive: true, ImpactFrame: 1, WaterImpact: true}
	w.Fire.Volcano[4] = engine.FireEffect{Active: true, Owner: 0, X: 8192, Y: 8192, Phase: engine.VolcanoGrowing, Stage: 6}
	w.Water.Basalt[5] = engine.BasaltEffect{Active: true, Owner: 0, X: 32, Y: 33, Direction: 1, Life: 80, Delay: 17, Frame: 2}
	w.Water.Whirlpools[6] = engine.WhirlpoolEffect{Active: true, Owner: 1, X: 32, Y: 33, Life: 200, Delay: 12, Frame: 3}
	w.Wind[7] = engine.WindEffect{Active: true, Owner: 0, X: 32, Direction: 1, Life: 77}
	w.Air.Markers[8] = engine.LightningMarker{Active: true, Owner: 0, X: 8320, Y: 8576, Life: 150, Phase: engine.LightningSteady, Frame: 7, FirstBolt: 10}
	w.Air.Bolts[9] = engine.LightningBolt{Active: true, Owner: 0, X: 8377, Y: 8576, Marker: 9, Random: 17733}
	cases := []engine.EffectKind{engine.EffectFireColumn, engine.EffectWhirlwind, engine.EffectFireRain, engine.EffectStorm, engine.EffectVolcano, engine.EffectBasalt, engine.EffectWhirlpool, engine.EffectHurricane, engine.EffectLightning, engine.EffectLightning}
	var restored engine.Snapshot
	for id, kind := range cases {
		record, err := encodeEffect(w, id, kind, Catalog{})
		if err != nil {
			t.Fatal(err)
		}
		if err := decodeEffect(record[:], id, &restored, Catalog{}); err != nil {
			t.Fatal(err)
		}
		if restored.Reservations[id].Kind != kind {
			t.Fatal("effect family changed during file conversion")
		}
	}
	got := engine.World(restored.World)
	for name, pair := range map[string][2]any{"column": {w.Fire.Columns[0], got.Fire.Columns[0]}, "whirlwind": {w.Air.Whirlwinds[1], got.Air.Whirlwinds[1]}, "rain": {w.Fire.Rain[2], got.Fire.Rain[2]}, "storm": {w.Air.Storms[3], got.Air.Storms[3]}, "basalt": {w.Water.Basalt[5], got.Water.Basalt[5]}, "whirlpool": {w.Water.Whirlpools[6], got.Water.Whirlpools[6]}, "wind": {w.Wind[7], got.Wind[7]}, "marker": {w.Air.Markers[8], got.Air.Markers[8]}, "bolt": {w.Air.Bolts[9], got.Air.Bolts[9]}} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			t.Fatalf("%s typed state changed: %+v/%+v", name, pair[0], pair[1])
		}
	}
	if got.Fire.Volcano[4].Stage != 6 || got.Fire.Volcano[4].Phase != engine.VolcanoGrowing {
		t.Fatal("volcano stage changed")
	}
}

func TestGAMUnsupportedEffectAndWallAnimationsRejectExplicitly(t *testing.T) {
	record := make([]byte, 32)
	record[12], record[0], record[22] = 1, 127, 127
	if err := decodeEffect(record, 0, &engine.Snapshot{}, Catalog{}); err == nil {
		t.Fatal("unknown active effect was silently imported")
	}
	wall := make([]byte, 16)
	wall[0], wall[1], wall[12] = 28, 2, 1
	if _, err := decodeWall(wall, Catalog{}); err == nil {
		t.Fatal("unmapped broken-wall animation was silently imported")
	}
}

func TestGAMFungusQuakeAndTidalControllersPreserveNamedState(t *testing.T) {
	w := &engine.World{}
	w.Nature.Fungi[0] = engine.FungusController{Active: true, Collecting: true, Owner: 0, Wait: 77, Period: 8, MinX: 20, MinY: 21, MaxX: 28, MaxY: 29, AgeMinX: 19, AgeMinY: 20, AgeMaxX: 27, AgeMaxY: 30}
	w.Earth.Quakes[1] = engine.QuakeEffect{Active: true, Owner: 1, X: 32, Y: 33, Direction: 6, Descriptor: 3, Life: 17, Delay: 3, Phase: engine.QuakeWaiting}
	w.Water.Waves[2] = engine.TidalEffect{Active: true, Newborn: true, Owner: 0, X: 32 * 256, Y: 33 * 256, Direction: 1, Frame: 4}
	catalog := Catalog{AnimationRoles: map[uint16][]AnimationRole{7777: {{Name: "tidal/east", Frame: 4}}}}
	var restored engine.Snapshot
	for id, kind := range []engine.EffectKind{engine.EffectFungus, engine.EffectEarthquake, engine.EffectTidalWave} {
		record, err := encodeEffect(w, id, kind, catalog)
		if err != nil {
			t.Fatal(err)
		}
		if err := decodeEffect(record[:], id, &restored, catalog); err != nil {
			t.Fatal(err)
		}
	}
	got := engine.World(restored.World)
	if !reflect.DeepEqual(w.Nature.Fungi[0], got.Nature.Fungi[0]) || !reflect.DeepEqual(w.Earth.Quakes[1], got.Earth.Quakes[1]) || !reflect.DeepEqual(w.Water.Waves[2], got.Water.Waves[2]) {
		t.Fatal("advanced effect file state changed")
	}
}
