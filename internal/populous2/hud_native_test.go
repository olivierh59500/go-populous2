package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

type hudNativeInput struct {
	Name              string
	Side, Element     uint16
	Mana              uint32
	Population        [2]uint32
	Experience        [6]uint8
	Flags             uint8
	Registers         [8]uint32
	NoRedraw, Pattern bool
}
type hudNativeFixture struct {
	Input     hudNativeInput
	Registers [8]uint32
	Pixels    []NativeHUDPixel
	Hash      string
}

func hudFixtureMemory(f hudNativeInput) *scenarioScriptMemory {
	m := &scenarioScriptMemory{}
	_ = m.write16(0xeb42, f.Side)
	_ = m.write16(0xf3a, f.Element)
	_ = m.write16(0xf42, 123)
	_ = m.write16(0x138, 110)
	_ = m.write16(0x13a, 80)
	for side := 1; side <= 2; side++ {
		god := 0xe76a + side*314
		_ = m.write32(god, f.Mana)
		_ = m.write32(god+4, f.Population[side-1])
		for i, v := range f.Experience {
			m[god+0x52+i] = v
		}
		for i := range 36 {
			m[god+0x70+i] = f.Flags
		}
	}
	return m
}

func TestNativeHUDRegistersAndPixelsAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/hud_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Cases []hudNativeFixture }
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 336 {
		t.Fatal("native HUD corpus incomplete")
	}
	r, err := DecodeNativeHUDRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	redraws, skipped := 0, 0
	for _, f := range catalog.Cases {
		if f.Input.NoRedraw {
			skipped++
		} else {
			redraws++
		}
		t.Run(f.Input.Name, func(t *testing.T) {
			m := hudFixtureMemory(f.Input)
			c := NativeHUDRegisters{D4: f.Input.Registers[4], D5: f.Input.Registers[5], D7: f.Input.Registers[7]}
			plan, err := r.Continue(m.callbacks(), &c, !f.Input.NoRedraw)
			if err != nil {
				t.Fatal(err)
			}
			if c != (NativeHUDRegisters{D4: f.Registers[4], D5: f.Registers[5], D7: f.Registers[7]}) {
				t.Fatalf("original HUD continuation differs: got%+v want%x/%x/%x", c, f.Registers[4], f.Registers[5], f.Registers[7])
			}
			if !reflect.DeepEqual(plan.Pixels, f.Pixels) {
				t.Fatalf("native HUD palette-index pixels differ: got%v want%v", plan.Pixels, f.Pixels)
			}
			if !f.Input.NoRedraw && (!plan.Drawn || len(plan.Sprites) == 0) {
				t.Fatal("native HUD indicator plan missing")
			}
			var previous [32000]uint8
			if f.Input.Pattern {
				for i := range previous {
					previous[i] = uint8(i*7 + 13)
				}
			}
			bitmap, e := plan.PaintSoftware(previous)
			if e != nil {
				t.Fatal(e)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(bitmap[:])); got != f.Hash {
				t.Fatalf("complete native software bitmap differs: got%s want%s", got, f.Hash)
			}
		})
	}
	if redraws != 328 || skipped != 8 {
		t.Fatalf("native HUD scope differs: %d redraws/%d skipped", redraws, skipped)
	}
}

func TestNativeHUDNoRedrawPreservesCallerRegisters(t *testing.T) {
	r, err := DecodeNativeHUDRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	c := NativeHUDRegisters{D4: 0xaabbccdd, D5: 0x11226778, D7: 0x33445566}
	before := c
	plan, err := r.Continue(FollowerCleanupMemory{}, &c, false)
	if err != nil || c != before || plan.Drawn || len(plan.Pixels) != 0 || len(plan.Sprites) != 0 || len(plan.Population) != 0 {
		t.Fatal("skipped HUD changed continuation or read a missing memory backend")
	}
}
