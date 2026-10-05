package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestNativeAICompleteDispatcherAgainstOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/ai_native_complete.json")
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
	if len(catalog.Cases) != 512 {
		t.Fatal("complete original AI dispatcher catalog incomplete")
	}
	r, err := DecodeNativeAIRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range catalog.Cases {
		t.Run(f.Input.Name, func(t *testing.T) {
			if f.Input.Mode != "tick" || len(f.Input.Initial) == 0 {
				t.Fatal("invalid complete dispatcher fixture")
			}
			m := aiNativeFixtureMemory(f.Input)
			before, draws := *m, 0
			context := NativeAIRegisterContext{D4: 0xccdd, D5: 0x6778}
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
			if _, err := r.TickComplete(&context, cb); err != nil {
				t.Fatal(err)
			}
			rng, _ := m.read32(0xeb28)
			if rng != f.RNG || draws != f.RandomDraws || fmt.Sprintf("%x", sha256.Sum256(m[:])) != f.Hash {
				t.Fatalf("complete native AI dispatcher BSS/RNG differs: draws%d/%d rng%x/%x commands%x/%x", draws, f.RandomDraws, rng, f.RNG, m[0xeb56:0xeb60], m[0xeb60:0xeb6a])
			}
			if context.D4 != f.D4 || context.D5 != f.D5 {
				t.Fatalf("native AI register continuation differs: %+v want %x/%x", context, f.D4, f.D5)
			}
			changes := []nativeHeroChange{}
			for i, value := range before {
				if value != m[i] {
					changes = append(changes, nativeHeroChange{Address: i, Value: m[i]})
				}
			}
			if !reflect.DeepEqual(changes, f.Changes) {
				t.Fatal("complete native AI changed ranges differ")
			}
		})
	}
}
