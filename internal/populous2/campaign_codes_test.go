package populous2

import "testing"

// These observations are independently captured from $103c6 for every actual
// world word. The five-world record stride belongs only to CONQUEST indexing.
func TestPublicCampaignCodesMatchAllOriginalCPUWorlds(t *testing.T) {
	fixtures := nativeInGameFixtures(t)
	if len(fixtures.Codes) != CampaignWorlds {
		t.Fatal("native code corpus incomplete")
	}
	first := map[string]int{}
	for _, reference := range fixtures.Codes {
		number := int(reference.World)
		if got := CodeForLevel(number); got != reference.Code {
			t.Fatalf("world%d public code%q native%q", number, got, reference.Code)
		}
		if _, exists := first[reference.Code]; !exists {
			first[reference.Code] = number
		}
	}
	for code, number := range first {
		got, ok := DecodeLevelCode(code)
		if !ok || got != number {
			t.Fatalf("native code%q did not resolve its first world%d", code, number)
		}
	}
	bundle := testBundle(t)
	for _, reference := range fixtures.Codes {
		if bundle.Levels[reference.World].Code != reference.Code {
			t.Fatal("decoded campaign retained the old record-stride code")
		}
	}
}
