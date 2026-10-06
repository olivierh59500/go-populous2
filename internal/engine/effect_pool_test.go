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
