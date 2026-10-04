package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type nativeWhirlwindWaterBytes struct {
	Index int
	Raw   string
}

type nativeWhirlwindWaterSnapshot struct {
	Tick                   int
	Actors, Cells          []nativeWhirlwindWaterBytes
	PoolSHA256, GridSHA256 string
	RNG                    uint32
}

type nativeWhirlwindWaterAttempt struct {
	Tick, X, Y, Index int
	Created           bool
	AtCreationRaw     string
}

type nativeWhirlwindWaterFixture struct {
	Name                           string
	Seed                           uint32
	AirExperience, WaterExperience uint8
	Target                         [2]int
	Initial                        nativeWhirlwindWaterSnapshot
	Snapshots                      []nativeWhirlwindWaterSnapshot
	Attempts                       []nativeWhirlwindWaterAttempt
	Passes                         int
}

func nativeWhirlwindWaterFixtures(t *testing.T) []nativeWhirlwindWaterFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/whirlwind_water_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []nativeWhirlwindWaterFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 8 {
		t.Fatal("combined native water fixture catalog is incomplete")
	}
	return catalog.Cases
}

// Omitted entries are zero bytes, not absent actors/cells. This reconstructs
// the complete original pools, including stale bytes in inactive records.
func decodeNativeWhirlwindWaterSnapshot(t *testing.T, snapshot nativeWhirlwindWaterSnapshot) ([8000]byte, [16384]byte) {
	t.Helper()
	var pool [8000]byte
	var grid [16384]byte
	for _, list := range []struct {
		entries []nativeWhirlwindWaterBytes
		bytes   []byte
		stride  int
	}{
		{snapshot.Actors, pool[:], 32},
		{snapshot.Cells, grid[:], 4},
	} {
		previous := -1
		for _, entry := range list.entries {
			if entry.Index <= previous || entry.Index < 0 || (entry.Index+1)*list.stride > len(list.bytes) {
				t.Fatal("native sparse snapshot has an invalid or duplicate index")
			}
			raw, err := hex.DecodeString(entry.Raw)
			if err != nil || len(raw) != list.stride {
				t.Fatal("native sparse snapshot has invalid raw bytes")
			}
			copy(list.bytes[entry.Index*list.stride:], raw)
			previous = entry.Index
		}
	}
	if fmt.Sprintf("%x", sha256.Sum256(pool[:])) != snapshot.PoolSHA256 || fmt.Sprintf("%x", sha256.Sum256(grid[:])) != snapshot.GridSHA256 {
		t.Fatal("combined native snapshot does not reconstruct its complete pool/grid hashes")
	}
	return pool, grid
}

// These are complete original $1482e passes, not parent-only calls with the
// child slots artificially occupied. Every successful child is observed after
// its later shared slot has run within the same native simulation pass.
func TestCombinedNativeWaterFixtureCoverageAndSamePassBirths(t *testing.T) {
	passes, snapshots, births := 0, 0, 0
	for _, fixture := range nativeWhirlwindWaterFixtures(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			passes += fixture.Passes
			initial, grid := decodeNativeWhirlwindWaterSnapshot(t, fixture.Initial)
			if initial[0] != 0x20 || initial[12] != 1 || binary.BigEndian.Uint16(initial[24:]) != 200+uint16(fixture.AirExperience) || binary.BigEndian.Uint16(grid[(32+32*64)*4+2:]) != 0x5140 {
				t.Fatal("native creator did not produce the expected linked parent")
			}
			byTick := make(map[int]nativeWhirlwindWaterSnapshot)
			previous := 0
			for _, snapshot := range fixture.Snapshots {
				if snapshot.Tick <= previous || snapshot.Tick > fixture.Passes {
					t.Fatal("native pool snapshots are not ordered")
				}
				decodeNativeWhirlwindWaterSnapshot(t, snapshot)
				byTick[snapshot.Tick] = snapshot
				previous = snapshot.Tick
				snapshots++
			}
			last, ok := byTick[fixture.Passes]
			if !ok {
				t.Fatal("native final pass is missing")
			}
			pool, _ := decodeNativeWhirlwindWaterSnapshot(t, last)
			for index := range NativeEffectCapacity {
				if pool[index*32+12] != 0 {
					t.Fatal("native capture ended before all effect records expired")
				}
			}
			for _, attempt := range fixture.Attempts {
				snapshot, ok := byTick[attempt.Tick]
				if !ok {
					t.Fatal("native child-attempt pass was not sampled")
				}
				if !attempt.Created {
					continue
				}
				births++
				created, err := hex.DecodeString(attempt.AtCreationRaw)
				if err != nil || len(created) != 32 || attempt.Index <= 0 || attempt.Index >= NativeEffectCapacity {
					t.Fatal("native child creation record is invalid")
				}
				pool, _ := decodeNativeWhirlwindWaterSnapshot(t, snapshot)
				child := pool[attempt.Index*32 : (attempt.Index+1)*32]
				if created[0] != 0x24 || child[0] != 0x24 || child[12] != 1 || binary.BigEndian.Uint16(created[24:]) != 300+uint16(fixture.WaterExperience) || binary.BigEndian.Uint16(child[24:]) != 299+uint16(fixture.WaterExperience) || binary.BigEndian.Uint16(child[10:]) != 0x9b || binary.BigEndian.Uint16(child[20:]) != 15 || binary.BigEndian.Uint32(child[2:]) != 0 {
					t.Fatal("native later-slot child missed its birth-pass tick or was incorrectly linked")
				}
			}
		})
	}
	if passes != 5090 || snapshots != 644 || births != 74 {
		t.Fatalf("native combined coverage passes/snapshots/births = %d/%d/%d", passes, snapshots, births)
	}
}
