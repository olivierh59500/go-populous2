package populous2

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// Check the constructor's composed templates against the independent original
// startup corpus, rather than comparing two calls to the same Go helper.
func TestWorldConstructorTemplatesAgainstAllNativeCampaignReferences(t *testing.T) {
	data, err := os.ReadFile("testdata/ai_world_setup_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []aiWorldSetupFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, fixture := range catalog.Cases {
		if !strings.HasPrefix(fixture.Input.Name, "campaign-template-world") {
			continue
		}
		var number int
		if _, err := fmt.Sscanf(fixture.Input.Name, "campaign-template-world%d", &number); err != nil {
			t.Fatal(err)
		}
		w, err := NewWorld(testBundle(t), number, false)
		if err != nil {
			t.Fatal(err)
		}
		m := w.nativeCleanupMemory()
		expected := aiSetupFixtureRaw(fixture)
		for _, change := range fixture.Changes {
			expected[change.Address] = change.Value
		}
		for side := 0; side < 2; side++ {
			god := 0xe8a4 + side*314
			for offset := 0x5a; offset < 0x94; offset++ {
				value, err := m.Read8(god + offset)
				if err != nil || value != expected[god+offset] {
					t.Fatalf("world%d side%d original template byte%x differs", number, side, offset)
				}
			}
			for _, change := range fixture.Changes {
				if change.Address < god+0x94 || change.Address >= god+314 {
					continue
				}
				value, err := m.Read8(change.Address)
				if err != nil || value != change.Value {
					t.Fatalf("world%d original compiled policy byte%x differs", number, change.Address)
				}
			}
		}
		for offset := range 8 {
			value, _ := m.Read8(0xe9de + 0x52 + offset)
			if value != expected[0xe9de+0x52+offset] {
				t.Fatalf("world%d original opponent XP/bolt byte%d differs", number, offset)
			}
		}
		for address := 0xdde; address < 0xdde+60; address++ {
			value, _ := m.Read8(address)
			if value != expected[address] {
				t.Fatalf("world%d original script byte%x differs", number, address)
			}
		}
		checked++
	}
	if checked != 1000 {
		t.Fatalf("constructor native campaign coverage: %d worlds", checked)
	}
}

func TestWorldConstructorInitializesNativeCommandAndSessionFields(t *testing.T) {
	for _, custom := range []bool{false, true} {
		w, err := NewWorld(testBundle(t), 215, custom)
		if err != nil {
			t.Fatal(err)
		}
		m := w.nativeCleanupMemory()
		for _, field := range []struct {
			address int
			value   uint16
		}{{0xf0c, 8}, {0xf0a, 0}, {0xeb18, 2}, {0xeb42, 1}, {0xeb44, w.NativeGameMode}, {0xeb46, 215}, {0xe8b0, 14}, {0xe9ea, 14}, {0xe8be, 2}, {0xe9f8, 4}} {
			value, err := m.Read16(field.address)
			if err != nil || value != field.value {
				t.Fatalf("native initial field%x got%x want%x", field.address, value, field.value)
			}
		}
		for _, field := range []struct{ address, value int }{{0xeb56, 1}, {0xeb60, 2}, {0xeb5e, 2}, {0xeb68, 4}} {
			value, _ := m.Read8(field.address)
			if int(value) != field.value {
				t.Fatal("native command ownership/transport differs")
			}
		}
		seed, _ := m.Read32(0xeb24)
		rng, _ := m.Read32(0xeb28)
		if seed != w.Level.RandomSeed || rng != w.Core.RandomState() {
			t.Fatal("native initial RNG/session diverged")
		}
	}
}
