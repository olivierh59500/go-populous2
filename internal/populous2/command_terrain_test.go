package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNativeCommandDirectTerrainAgainstCompleteCPU(t *testing.T) {
	data, e := os.ReadFile("testdata/command_terrain_native.json")
	if e != nil {
		t.Fatal(e)
	}
	var catalog struct{ Cases []commandNativeFixture }
	if e := json.Unmarshal(data, &catalog); e != nil {
		t.Fatal(e)
	}
	if len(catalog.Cases) != 462 {
		t.Fatal("native direct terrain full-memory corpus incomplete")
	}
	rules, e := DecodeNativeCommandRules(testBundle(t).Executable)
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			initial := commandNativeInitial(f.Input)
			for _, world := range []bool{false, true} {
				b := append([]byte(nil), initial...)
				m := commandNativeMemory(b)
				context := NativeCommandRegisterContext{D: f.Input.Context}
				var w *World
				if world {
					w = aiFixtureWorld(t, initial)
					m = w.nativeCleanupMemory()
				}
				_, e := rules.DirectTerrain(f.Input.Mode == "raise", &context, m)
				if e != nil {
					t.Fatal(e)
				}
				if world {
					b = aiFixtureWorldBytes(w, initial)
					copy(b[0xeb90:0x11280], w.NativeRedrawBytes[:])
				}
				if context.D != f.Registers {
					t.Fatalf("terrain registers world%t got%08x native%08x", world, context.D, f.Registers)
				}
				if fmt.Sprintf("%x", sha256.Sum256(b)) != f.Hash {
					changes := []nativeHeroChange{}
					for at, v := range b {
						if v != initial[at] {
							changes = append(changes, nativeHeroChange{Address: at, Value: v})
						}
					}
					t.Fatalf("terrain full memory world%t got%+v native%+v", world, changes, f.Changes)
				}
			}
		})
	}
}
