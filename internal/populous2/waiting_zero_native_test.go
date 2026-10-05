package populous2

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

// Native hero search can return without changing waiting state10. The next
// pass then reads ordinary animation0 instead of a designated waiting bank.
// Original CPU fixtures prove those image loops and timer/expiry boundaries.
func TestWaitingZeroContinuationAgainstOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/waiting_zero_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Fixtures []entryFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Fixtures) != 32 {
		t.Fatal("native waiting continuation catalog incomplete")
	}
	r, err := DecodeFollowerEntryRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range catalog.Fixtures {
		t.Run(f.Input.Name, func(t *testing.T) {
			m := entryMemory(f.Input)
			step, err := r.TickWaiting(52, m.callbacks(t, f))
			if err != nil {
				t.Fatal(err)
			}
			if step.RedispatchSearch != (f.Exit == "search") {
				t.Fatal("native waiting expiry differs")
			}
			for _, record := range f.Records {
				a, err := entryAddress(NativeRecordReference(record.Reference), 0, len(record.Raw)/2)
				if err != nil {
					t.Fatal(err)
				}
				if got := hex.EncodeToString(m.bytes[a : a+len(record.Raw)/2]); got != record.Raw {
					t.Fatal("native waiting record differs")
				}
			}
		})
	}
}
