package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestNativeMapPointPixelsAndRegistersAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/map_point_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			Color, Bit, Y, Seed int
			Input, D            [8]uint32
			Hash                string
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 768 {
		t.Fatal("native map-point corpus incomplete")
	}
	for _, fixture := range catalog.Cases {
		bitmap := make([]byte, 32000)
		for i := range bitmap {
			bitmap[i] = byte(i*37 + fixture.Seed*81)
		}
		frame := NativeFrameRegisterContext{D: fixture.Input}
		plan, err := PlanNativeMapPoint(&frame)
		if err != nil {
			t.Fatal(err)
		}
		if err := plan.Paint(bitmap); err != nil {
			t.Fatal(err)
		}
		if frame.D != fixture.D {
			t.Fatalf("color%d bit%d row%d map-point registers differ: %x/%x", fixture.Color, fixture.Bit, fixture.Y, frame.D, fixture.D)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(bitmap)); got != fixture.Hash {
			t.Fatalf("color%d bit%d row%d pixels differ", fixture.Color, fixture.Bit, fixture.Y)
		}
	}
}
