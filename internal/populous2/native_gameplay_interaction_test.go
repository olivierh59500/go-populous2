package populous2

import (
	"encoding/json"
	"os"
	"testing"
)

func TestNativeGameplayInteractionsAgainstOriginalCPU(t *testing.T) {
	const filename = "testdata/native_gameplay_interaction_native.json"
	runNativeGameplayCPUCorpus(t, filename, 104)
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct{ Cases []nativeGameplayCPUFixture }
	if err = json.Unmarshal(data, &corpus); err != nil {
		t.Fatal(err)
	}
	reached := map[string]int{}
	for _, c := range corpus.Cases {
		for state, count := range c.FollowerStates {
			reached[state] += count
		}
	}
	for _, state := range []string{"28", "30", "32", "52", "54", "56", "62"} {
		if reached[state] == 0 {
			t.Fatalf("native interaction state%s is not exercised", state)
		}
	}
}
