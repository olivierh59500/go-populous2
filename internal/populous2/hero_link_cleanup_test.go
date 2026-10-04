package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestEntryHeroLinkCleanupAgainstOriginal68000(t *testing.T) {
	data, err := os.ReadFile("testdata/hero_link_cleanup_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Multiplier, Add int
		Cases           []struct {
			Name                                 string
			Source                               NativeRecordReference
			BeforeSHA256, AfterSHA256            string
			Visits                               int
			D0Before, D0After, A1Before, A1After uint32
			Initial                              []struct {
				Reference     NativeRecordReference
				Offset, Width int
				Value         uint32
			}
			Writes []struct {
				BSSAddress, Width int
				Value             uint32
			}
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 12 || catalog.Multiplier != 13 || catalog.Add != 7 {
		t.Fatal("native hero-link fixture catalog is incomplete")
	}
	visits, writes := 0, 0
	for _, fixture := range catalog.Cases {
		t.Run(fixture.Name, func(t *testing.T) {
			var image NativeRecordImage
			for index := range image.Bytes {
				image.Bytes[index] = uint8(index*catalog.Multiplier + catalog.Add)
			}
			for _, field := range fixture.Initial {
				var err error
				switch field.Width {
				case 1:
					_, err = image.Write8(field.Reference, field.Offset, uint8(field.Value))
				case 2:
					_, err = image.Write16(field.Reference, field.Offset, uint16(field.Value))
				case 4:
					_, err = image.Write32(field.Reference, field.Offset, field.Value)
				default:
					t.Fatal("invalid native initialization width")
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			if fmt.Sprintf("%x", sha256.Sum256(image.Bytes[:])) != fixture.BeforeSHA256 {
				t.Fatal("native starting image differs")
			}
			step, err := ClearEntryHeroLinks(&image, fixture.Source)
			if err != nil {
				t.Fatal(err)
			}
			if step.CaptivesVisited != fixture.Visits || len(step.Patches) != len(fixture.Writes) {
				t.Fatalf("native visits/assignments differ: %+v", step)
			}
			for index, assignment := range fixture.Writes {
				patch := step.Patches[index]
				if patch.BSSOffset != assignment.BSSAddress || patch.Length != assignment.Width {
					t.Fatalf("native assignment order%d differs: %+v, native%+v", index, patch, assignment)
				}
			}
			if fmt.Sprintf("%x", sha256.Sum256(image.Bytes[:])) != fixture.AfterSHA256 {
				t.Fatal("complete native image differs after cleanup")
			}
			if fixture.D0After != fixture.D0Before || fixture.A1After != fixture.A1Before {
				t.Fatal("original routine register-preservation evidence differs")
			}
			if fixture.Name == "Helen35c-alias" {
				// The second loop's word42 clears slot17 speed/byte19, while
				// flags/state alias slot16 hero-low/variant-high bytes.
				for index, nativeIndex := range []int{17, 16, 16} {
					patch := step.Patches[3+index]
					if len(patch.Slots) != 1 || patch.Slots[0].Location.Pool != NativeFollowerPool || patch.Slots[0].Location.Index != nativeIndex {
						t.Fatal("Helen cleanup omitted an affected reused follower slot")
					}
				}
			}
			if fixture.Name == "hero-both-reciprocals" && (!step.OutgoingCleared || !step.TargetBackCleared || !step.IncomingCleared) {
				t.Fatal("cleanup reciprocal summary differs")
			}
			if fixture.Name == "nonhero-incoming-only" && (step.OutgoingCleared || step.TargetBackCleared || !step.IncomingCleared) {
				t.Fatal("nonhero outgoing links were incorrectly cleared")
			}
			visits += step.CaptivesVisited
			writes += len(step.Patches)
		})
	}
	if visits != 15 || writes != 53 {
		t.Fatalf("native cleanup coverage differs: visits%d writes%d", visits, writes)
	}
}

func TestEntryHeroLinkCleanupBoundsPreservePriorNativeWrites(t *testing.T) {
	var image NativeRecordImage
	if _, err := ClearEntryHeroLinks(nil, 52); err == nil {
		t.Fatal("missing native image accepted")
	}
	if _, err := ClearEntryHeroLinks(&image, 0x7080); err == nil {
		t.Fatal("out-of-pool source accepted")
	}
	image.Write8(52, 13, 2)
	image.Write16(52, 40, 10)
	image.Write16(52, 34, 104)
	image.Write16(104, 36, 52)
	image.Write16(52, 42, 0x7080)
	step, err := ClearEntryHeroLinks(&image, 52)
	if err == nil || len(step.Patches) != 2 || step.CaptivesVisited != 0 {
		t.Fatal("unsafe captive alias was normalized or silently ignored")
	}
	association, _ := image.Read16(52, 34)
	back, _ := image.Read16(104, 36)
	if association != 0 || back != 0 {
		t.Fatal("bounded failure rolled back earlier original assignments")
	}
}
