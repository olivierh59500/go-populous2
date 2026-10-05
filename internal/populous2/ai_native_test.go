package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type aiNativeInput struct {
	Name, Mode string
	Side       int
	Initial    []nativeHeroPatch
	Seed       uint32
}
type aiNativePolicy struct {
	Kind         string
	God, Command int
	Before       string
	Changes      []nativeHeroChange
	Result       bool
}
type aiNativeFixture struct {
	Input       aiNativeInput
	Hash        string
	RNG         uint32
	RandomDraws int
	Result      bool
	Changes     []nativeHeroChange
	Policies    []aiNativePolicy
}

func aiNativeFixtureMemory(input aiNativeInput) *scenarioScriptMemory {
	m := &scenarioScriptMemory{}
	for i := range 4096 {
		m[0xf44+i*4] = 0xa8
		m[0xf45+i*4] = 15
	}
	_ = m.write32(0xeb28, input.Seed)
	for side := 1; side <= 2; side++ {
		god, command := 0xe76a+side*314, 0xeb56+(side-1)*10
		_ = m.write16(god+0x18, uint16(side))
		_ = m.write16(god+0xc, 14)
		m[command] = uint8(side)
		m[0x76f4], m[0x7700] = 4, uint8(input.Side)
		_ = m.write16(0x76fa, 0x2080)
		_ = m.write16(0x76fc, 0x2080)
	}
	for _, p := range input.Initial {
		switch p.Width {
		case 1:
			_ = m.write8(p.Address, uint8(p.Value))
		case 2:
			_ = m.write16(p.Address, uint16(p.Value))
		case 4:
			_ = m.write32(p.Address, p.Value)
		}
	}
	return m
}

func TestNativeAIInitialPoliciesAgainstOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/ai_native_initial.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []aiNativeFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 1112 {
		t.Fatal("native AI initial fixture catalog incomplete")
	}
	r, err := DecodeNativeAIRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	if r.ExpansionDelay != 2 || len(r.ExpansionOffsets) != 60 {
		t.Fatal("native AI expansion constants differ")
	}
	counts := map[string]int{}
	for _, f := range catalog.Cases {
		counts[f.Input.Mode]++
		t.Run(f.Input.Name, func(t *testing.T) {
			m := aiNativeFixtureMemory(f.Input)
			before := *m
			draws, policyIndex := 0, 0
			cb := NativeAICallbacks{Memory: m.callbacks(), Random: func() uint16 {
				draws++
				rng, _ := m.read32(0xeb28)
				if rng == 0 {
					rng = 0xbc614e
				}
				rng *= 0xbb40e62d
				_ = m.write32(0xeb28, rng)
				return uint16(rng >> 8 & 0x7fff)
			},
				Policy: func(kind NativeAIPolicy, god, command int) (bool, error) {
					if policyIndex >= len(f.Policies) {
						return false, fmt.Errorf("unexpected external native AI policy")
					}
					p := f.Policies[policyIndex]
					policyIndex++
					name := "offensive"
					if kind == NativeAIMagnetMode {
						name = "magnet"
					}
					if p.Kind != name || p.God != god || p.Command != command || fmt.Sprintf("%x", sha256.Sum256(m[:])) != p.Before {
						return false, fmt.Errorf("native AI policy boundary/order differs")
					}
					for _, change := range p.Changes {
						m[change.Address] = change.Value
					}
					return p.Result, nil
				},
			}
			god, command := 0xe76a+f.Input.Side*314, 0xeb56+(f.Input.Side-1)*10
			result := false
			switch f.Input.Mode {
			case "urgent":
				result, err = r.Urgent(god, command, cb)
			case "expand":
				result, err = r.Expand(god, command, cb)
			case "release":
				result, err = r.ReleaseTown(god, command, cb)
			case "choices":
				err = r.CompileChoices(god, m.callbacks())
			case "tick":
				_, err = r.Tick(cb)
			default:
				t.Fatal("unknown native AI scope")
			}
			if err != nil {
				t.Fatal(err)
			}
			if f.Input.Mode != "tick" && f.Input.Mode != "choices" && result != f.Result {
				t.Fatal("native AI decision condition differs")
			}
			rng, _ := m.read32(0xeb28)
			if rng != f.RNG || draws != f.RandomDraws || fmt.Sprintf("%x", sha256.Sum256(m[:])) != f.Hash {
				t.Fatal("complete original AI BSS/RNG/counters differs")
			}
			if policyIndex != len(f.Policies) {
				t.Fatal("native AI skipped actual external policy")
			}
			changes := []nativeHeroChange{}
			for i, v := range before {
				if v != m[i] {
					changes = append(changes, nativeHeroChange{Address: i, Value: m[i]})
				}
			}
			if !reflect.DeepEqual(changes, f.Changes) {
				t.Fatal("native AI changed byte ranges differ")
			}
		})
	}
	if !reflect.DeepEqual(counts, map[string]int{"urgent": 384, "expand": 240, "release": 384, "tick": 96, "choices": 8}) {
		t.Fatalf("native AI coverage differs: %v", counts)
	}
}
