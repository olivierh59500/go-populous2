package populous2

import (
	"encoding/json"
	"os"
	"testing"
)

func TestNativeGameplayFamiliesAgainstOriginalCPU(t *testing.T) {
	const filename = "testdata/native_gameplay_family_native.json"
	runNativeGameplayCPUCorpus(t, filename, 144)
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct{ Cases []nativeGameplayCPUFixture }
	if err = json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	states := map[string]int{}
	breaks := 0
	for _, c := range corpus.Cases {
		breaks += c.WallBreakEntries
		for state, n := range c.FollowerStates {
			states[state] += n
		}
	}
	for _, state := range []string{"22", "24", "44", "50", "52", "66", "68", "70"} {
		if states[state] == 0 {
			t.Fatalf("native family state%s not entered", state)
		}
	}
	if breaks != 16 {
		t.Fatalf("actual transient11680 branchcoverage changed: %d", breaks)
	}
}
