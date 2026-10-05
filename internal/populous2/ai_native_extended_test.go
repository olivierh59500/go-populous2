package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestNativeAIOffensiveMagnetAndWaterAgainstOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/ai_native_extended.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			aiNativeFixture
			D4, D5 uint16
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 1008 {
		t.Fatal("native AI extended fixture catalog incomplete")
	}
	r, err := DecodeNativeAIRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, f := range catalog.Cases {
		counts[f.Input.Mode]++
		t.Run(f.Input.Name, func(t *testing.T) {
			m := aiNativeFixtureMemory(f.Input)
			before := *m
			draws := 0
			cb := NativeAICallbacks{Memory: m.callbacks(), Random: func() uint16 {
				draws++
				rng, _ := m.read32(0xeb28)
				if rng == 0 {
					rng = 0xbc614e
				}
				rng *= 0xbb40e62d
				_ = m.write32(0xeb28, rng)
				return uint16(rng >> 8 & 0x7fff)
			}}
			god, command := 0xe76a+f.Input.Side*314, 0xeb56+(f.Input.Side-1)*10
			context := NativeAIRegisterContext{D4: 0xccdd, D5: 0x6778}
			result := false
			switch f.Input.Mode {
			case "offensive":
				result, err = r.Offensive(god, command, &context, cb)
			case "magnet":
				result, err = r.MagnetMode(god, command, cb)
			case "water":
				result, err = r.WaterTarget(0x76f4, uint16(f.Input.Side), &context, m.callbacks())
			default:
				t.Fatal("unknown native AI extended scope")
			}
			if err != nil {
				t.Fatal(err)
			}
			if result != f.Result {
				t.Fatal("native AI extended condition differs")
			}
			rng, _ := m.read32(0xeb28)
			if rng != f.RNG || draws != f.RandomDraws || fmt.Sprintf("%x", sha256.Sum256(m[:])) != f.Hash {
				t.Fatal("complete native AI extended BSS/RNG/counters differs")
			}
			if (f.Input.Mode == "water" || f.Input.Mode == "offensive") && (context.D4 != f.D4 || context.D5 != f.D5) {
				t.Fatalf("native AI water coordinates differ: %+v want%x/%x", context, f.D4, f.D5)
			}
			changes := []nativeHeroChange{}
			for i, v := range before {
				if v != m[i] {
					changes = append(changes, nativeHeroChange{Address: i, Value: m[i]})
				}
			}
			if !reflect.DeepEqual(changes, f.Changes) {
				t.Fatal("native AI extended changed byte ranges differ")
			}
		})
	}
	if !reflect.DeepEqual(counts, map[string]int{"offensive": 216, "magnet": 768, "water": 24}) {
		t.Fatalf("native AI extended coverage differs: %v", counts)
	}
}
