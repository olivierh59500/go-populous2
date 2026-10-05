package populous2

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
)

type nativeNeutralWaterFixture struct {
	nativeWhirlwindWaterFixture
	Owner uint8
}

// These complete 250-slot native passes use native owner 3 throughout, including
// spontaneous water children. Player XP varies without changing their lifetime.
func TestNeutralWhirlwindWaterChildrenAgainstCompleteNativePool(t *testing.T) {
	data, err := os.ReadFile("testdata/neutral_whirlwind_water_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativeNeutralWaterFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 4 {
		t.Fatal("neutral water catalog incomplete")
	}
	births, snapshots, passes := 0, 0, 0
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Name, func(t *testing.T) {
			if fixture.Owner != 3 {
				t.Fatal("native neutral owner differs")
			}
			w := oceanWorld(t, fixture.Seed)
			for player := range 2 {
				w.Experience[player][Air], w.Experience[player][Water] = fixture.AirExperience, fixture.WaterExperience
			}
			if err := w.runNativeFollowerCall(func() error {
				result, err := w.PrimitiveCreators.CreateWhirlwind(3, uint8(fixture.Target[0]), uint8(fixture.Target[1]), w.nativePrimitiveCallbacks())
				if !result.Created && err == nil {
					t.Fatal("neutral parent creation rejected")
				}
				return err
			}); err != nil {
				t.Fatal(err)
			}
			compare := func(snapshot nativeWhirlwindWaterSnapshot) {
				t.Helper()
				decodeNativeWhirlwindWaterSnapshot(t, snapshot)
				if worldEffectPoolHash(w) != snapshot.PoolSHA256 || worldNativeGridHash(w) != snapshot.GridSHA256 || w.Core.Snapshot().RNG != snapshot.RNG {
					t.Fatalf("neutral pass%d pool/grid/RNG differs", snapshot.Tick)
				}
			}
			initial, _ := decodeNativeWhirlwindWaterSnapshot(t, fixture.Initial)
			if initial[12] != 3 || binary.BigEndian.Uint16(initial[24:26]) != 200 {
				t.Fatal("neutral parent read opponent Air XP")
			}
			compare(fixture.Initial)
			next := 0
			byTick := map[int]nativeWhirlwindWaterSnapshot{}
			for _, snapshot := range fixture.Snapshots {
				byTick[snapshot.Tick] = snapshot
				snapshots++
			}
			for tick := 1; tick <= fixture.Passes; tick++ {
				w.tickNativeEffects()
				if next < len(fixture.Snapshots) && fixture.Snapshots[next].Tick == tick {
					compare(fixture.Snapshots[next])
					next++
				}
			}
			passes += fixture.Passes
			if next != len(fixture.Snapshots) {
				t.Fatal("neutral snapshots not all replayed")
			}
			for _, attempt := range fixture.Attempts {
				if !attempt.Created {
					continue
				}
				births++
				snapshot, ok := byTick[attempt.Tick]
				if !ok {
					t.Fatal("neutral child birth snapshot omitted")
				}
				pool, _ := decodeNativeWhirlwindWaterSnapshot(t, snapshot)
				child := pool[attempt.Index*32 : (attempt.Index+1)*32]
				if child[0] != 0x24 || child[12] != 3 || binary.BigEndian.Uint16(child[10:12]) != 0x9b || binary.BigEndian.Uint16(child[20:22]) != 15 || binary.BigEndian.Uint16(child[24:26]) != 299 {
					t.Fatal("neutral child XP bypass or same-pass update differs")
				}
			}
		})
	}
	if births != 34 || passes != 1974 || snapshots != 290 {
		t.Fatalf("neutral coverage differs: births%d passes%d snapshots%d", births, passes, snapshots)
	}
}
