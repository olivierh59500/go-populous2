package populous2

import (
	"encoding/json"
	"os"
	"testing"
)

// These original-CPU cases use actual startup and command dispatch, followed
// by explicit controlled occupancy/actor seeds. Rendering and input remain
// separate boundaries; all raw simulation, swap and resource bytes are checked.
func TestNativeGameplayEnvironmentAgainstOriginalCPU(t *testing.T) {
	const filename = "testdata/native_gameplay_environment_native.json"
	runNativeGameplayCPUCorpus(t, filename, 136)
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct {
			nativeGameplayCPUFixture
			RoadBonusEntries int
		}
	}
	if err = json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	entered := map[string]int{}
	roadBonuses := 0
	for _, c := range corpus.Cases {
		if c.ErrorPC != 0 || c.ResultBoundary {
			t.Fatalf("environment case%s stopped at an unexpected source boundary", c.Input.Name)
		}
		for state, count := range c.FollowerStates {
			entered[state] += count
		}
		roadBonuses += c.RoadBonusEntries
	}
	// These decimal keys are the original dispatcher observations, rather
	// than a claim that a command name necessarily entered its intended state.
	for _, state := range []string{"8", "18", "20", "26", "34", "40", "46", "48", "58", "60"} {
		if entered[state] == 0 {
			t.Fatalf("native environment dispatch state%s was not entered", state)
		}
	}
	if roadBonuses != 24 {
		t.Fatalf("actual source11530 road-speed branch coverage changed: %d", roadBonuses)
	}
}
