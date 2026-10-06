package engine

import "testing"

func TestEffectsShareCapacityAndRetainGenerationOnReuse(t *testing.T) {
	var pool effectPool
	for index := 0; index < EffectCapacity; index++ {
		kind := EffectFireColumn
		if index%2 == 0 {
			kind = EffectFungus
		}
		if id := pool.allocate(kind, uint8(index%3)); id != index {
			t.Fatal("effect slot order changed", index, id)
		}
	}
	if id := pool.allocate(EffectStorm, 0); id != -1 {
		t.Fatal("new effect exceeded shared pool", id)
	}
	pool.Slots[17].LastVelocityX = 16
	pool.Slots[17].LastVelocityY = -16
	pool.free(17)
	if id := pool.allocate(EffectWhirlwind, 1); id != 17 {
		t.Fatal("released effect not reused", id)
	}
	if slot := pool.Slots[17]; slot.Generation != 2 || slot.LastVelocityX != 16 || slot.LastVelocityY != -16 || slot.Kind != EffectWhirlwind {
		t.Fatal("reused effect lost reservation identity", slot)
	}
	pool.free(-1)
	pool.free(EffectCapacity)
	if id := pool.allocate(EffectNone, 0); id != -1 {
		t.Fatal("empty effect reserved a slot")
	}
	if id := pool.allocate(EffectPlague, 3); id != -1 {
		t.Fatal("invalid owner reserved a slot")
	}
}

func TestWorldEffectPassAdvancesCrossFamilyNewbornOnlyInLaterSlot(t *testing.T) {
	for _, lowerSlot := range []bool{false, true} {
		t.Run(map[bool]string{false: "later", true: "earlier"}[lowerSlot], func(t *testing.T) {
			w := &World{}
			parent := 0
			if lowerSlot {
				for range 6 {
					w.allocateEffect(EffectLightning, 0)
				}
				w.releaseEffect(0)
				parent = 5
				w.effects.Slots[parent].Kind = EffectLava
			} else {
				parent = w.allocateEffect(EffectLava, 0)
			}
			// A supported lava parcel has a water parcel directly to its east.
			w.Tiles[20+20*MapSize] = Cell{Corners: [4]uint8{1, 1, 1, 1}, Shape: 15, Code: 15}
			w.Fire.Lava[parent] = FireEffect{Active: true, Owner: 0, X: 20 * 256, Y: 20 * 256, Direction: 1, Timer: 1, Life: 10, Phase: LavaFlowing}
			w.Step()
			child := -1
			for id, reservation := range w.effects.Slots {
				if reservation.Kind == EffectBasalt {
					child = id
					break
				}
			}
			if child < 0 {
				t.Fatal("lava did not create its basalt child")
			}
			expected := 99
			if lowerSlot {
				expected = 100
			}
			if got := w.Water.Basalt[child].Life; got != expected {
				t.Fatalf("child slot%d parent%d life%d; want%d", child, parent, got, expected)
			}
		})
	}
}

func TestWorldProcessesFollowersBeforeFireAndSceneryAfterEffects(t *testing.T) {
	w := testFlatWorld()
	id := addFollower(w, 20, 20, 0, 100, Walking)
	slot := w.allocateEffect(EffectFireRain, 1)
	w.Fire.Rain[slot] = FireEffect{Active: true, Owner: 1, X: 20 * 256, Y: 20 * 256, Phase: MeteorFalling, Life: 3, Frame: 0}
	w.Step()
	if w.Followers[id].State != Ruin || w.FireDamage.Deaths[id].Frame != 0 {
		t.Fatal("fire victim advanced before the follower phase ended")
	}
	if w.Followers[id].Frame != 0 {
		t.Fatal("fire death should begin after the follower update")
	}
}
