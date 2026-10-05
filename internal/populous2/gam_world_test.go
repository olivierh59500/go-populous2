package populous2

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"testing"
)

func assertNativeGAMBytes(t *testing.T, actual, expected []byte) {
	t.Helper()
	if bytes.Equal(actual, expected) {
		return
	}
	changes := []string{}
	for index := range min(len(actual), len(expected)) {
		if actual[index] != expected[index] {
			changes = append(changes, fmt.Sprintf("%x:%02x/%02x", NativeGAMStart+index, actual[index], expected[index]))
			if len(changes) == 24 {
				break
			}
		}
	}
	t.Fatalf("native GAM bytes differ: size%d/%d first changes%v", len(actual), len(expected), changes)
}

func TestWorldNativeGAMProvidedReferenceRoundtrip(t *testing.T) {
	data, err := os.ReadFile("../../.local/native-audit/extracted-pop2-b/ARNY 1.GAM")
	if os.IsNotExist(err) {
		t.Skip("supplied original GAM remains private")
	}
	if err != nil {
		t.Fatal(err)
	}
	w, err := ImportNativeGAM(testBundle(t), data)
	if err != nil {
		t.Fatal(err)
	}
	initial, err := NewWorld(testBundle(t), 27, false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(w.NativeControlBytes[:NativeGAMStart-0xdc4], initial.NativeControlBytes[:NativeGAMStart-0xdc4]) || !bytes.Equal(w.NativeCommandBytes[NativeGAMEnd-0xeb18:], initial.NativeCommandBytes[NativeGAMEnd-0xeb18:]) {
		t.Fatal("GAM load overwrote initialized unsaved command/scratch memory")
	}
	if w.Level.Number != 27 || w.Level.Terrain != 1 || w.NativeClock != 112 || w.NativeProfileSide != 1 || w.NativeGameMode != 2 || w.Core.RandomState() != 0xdb9e731b || w.Deity.Name != "DAMOCLES" || w.Deity.Bolts != 13 || w.Deity.FaceParts != [3]uint8{1, 4, 1} {
		t.Fatal("supplied GAM session/profile binding differs")
	}
	if w.Core.Magnets[0].Mana != 2020 || w.Core.Magnets[1].Mana != 116 || w.Level.Players[0].FollowerAttrition() != 3 || w.Level.Players[1].FollowerAttrition() != 7 || w.Rules[0].Raw != 35 || w.Rules[1].Raw != 4 {
		t.Fatal("supplied GAM ledger/template/rule binding differs")
	}
	if binary.BigEndian.Uint16(w.NativeViewBytes[:]) != 8 || binary.BigEndian.Uint16(w.NativeViewBytes[2:]) != 4 {
		t.Fatal("supplied GAM camera did not bind")
	}
	exported, err := w.ExportNativeGAM()
	if err != nil {
		t.Fatal(err)
	}
	assertNativeGAMBytes(t, exported, data)
	if err := w.Occupancy.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestWorldNativeGAMRejectsInvalidSessionAndChains(t *testing.T) {
	var image NativeGAMImage
	_ = image.Write16(0xeb42, 1)
	_ = image.Write16(0xeb44, 8)
	data, _ := image.MarshalBinary()
	if _, err := ImportNativeGAM(nil, data); err == nil {
		t.Fatal("nil GAM asset bundle accepted")
	}
	for _, field := range []struct {
		address int
		value   uint16
	}{{0xeb46, 1000}, {0xeb22, 4}, {0xeb42, 0}, {0xeb44, 1}} {
		bad := image
		_ = bad.Write16(field.address, field.value)
		data, _ := bad.MarshalBinary()
		if _, err := ImportNativeGAM(testBundle(t), data); err == nil {
			t.Fatalf("invalid native GAM field%x accepted", field.address)
		}
	}
	bad := image
	_ = bad.Write16(0xf46, 52)
	_ = bad.Write16(0x76f4+2, 52)
	data, _ = bad.MarshalBinary()
	if _, err := ImportNativeGAM(testBundle(t), data); err == nil {
		t.Fatal("cyclic native GAM map chain accepted")
	}
}

// Original complete native pool passes include later-slot whirlpool children
// executing in the same pass. Importing repeatedly must retain every raw byte,
// map-chain pressure/head and random state at the existing CPU checkpoints.
func TestWorldNativeGAMWaterContinuationAgainstOriginalCPU(t *testing.T) {
	count := 0
	for _, fixture := range nativeWhirlwindWaterFixtures(t) {
		count++
		t.Run(fixture.Name, func(t *testing.T) {
			w := oceanWorld(t, fixture.Seed)
			w.NativeGameMode = 8
			w.Experience[0][Air], w.Experience[0][Water] = fixture.AirExperience, fixture.WaterExperience
			if !w.Cast(0, Whirlwind, Target{X: fixture.Target[0], Y: fixture.Target[1]}) {
				t.Fatal("native controlled parent creation failed")
			}
			reload := func() {
				data, err := w.ExportNativeGAM()
				if err != nil {
					t.Fatal(err)
				}
				w, err = ImportNativeGAM(testBundle(t), data)
				if err != nil {
					t.Fatal(err)
				}
				roundtrip, err := w.ExportNativeGAM()
				if err != nil {
					t.Fatal(err)
				}
				assertNativeGAMBytes(t, roundtrip, data)
			}
			compare := func(golden nativeWhirlwindWaterSnapshot) {
				if worldEffectPoolHash(w) != golden.PoolSHA256 || worldNativeGridHash(w) != golden.GridSHA256 || w.Core.RandomState() != golden.RNG {
					t.Fatalf("native GAM continuation pass%d differs from original pool/map/RNG", golden.Tick)
				}
			}
			reload()
			compare(fixture.Initial)
			next := 0
			for tick := 1; tick <= fixture.Passes; tick++ {
				w.tickNativeEffects()
				if tick == 5 || tick == 50 || tick == 150 {
					reload()
				}
				if next < len(fixture.Snapshots) && fixture.Snapshots[next].Tick == tick {
					compare(fixture.Snapshots[next])
					next++
				}
			}
		})
	}
	if count != 8 {
		t.Fatal("native GAM water continuation catalog is incomplete")
	}
}

func TestWorldNativeGAMOriginalSetupTemplatesRoundtrip(t *testing.T) {
	b := testBundle(t)
	rules, err := DecodeNativeAIRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, level := range []int{0, 27, 215, 999} {
		t.Run(fmt.Sprint(level), func(t *testing.T) {
			w, err := NewWorld(b, level, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := w.initializeNativeAIControls(); err != nil {
				t.Fatal(err)
			}
			xp, err := w.loadNativeAITemplates(&rules, w.Level)
			if err != nil {
				t.Fatal(err)
			}
			w.Experience = xp
			data, err := w.ExportNativeGAM()
			if err != nil {
				t.Fatal(err)
			}
			loaded, err := ImportNativeGAM(b, data)
			if err != nil {
				t.Fatal(err)
			}
			roundtrip, err := loaded.ExportNativeGAM()
			if err != nil {
				t.Fatal(err)
			}
			assertNativeGAMBytes(t, roundtrip, data)
			if loaded.Experience != xp {
				t.Fatal("GAM discarded actual setup XP bytes")
			}
			for player := range 2 {
				if loaded.Level.Players[player] != w.Level.Players[player] {
					t.Fatal("GAM discarded original setup parameters or power flags")
				}
			}
		})
	}
}

func TestWorldNativeGAMFullWorldSaveContinuation(t *testing.T) {
	b := testBundle(t)
	rules, err := DecodeNativeAIRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, level := range []int{0, 27, 215} {
		t.Run(fmt.Sprint(level), func(t *testing.T) {
			w, err := NewWorld(b, level, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := w.initializeNativeAIControls(); err != nil {
				t.Fatal(err)
			}
			w.Experience, err = w.loadNativeAITemplates(&rules, w.Level)
			if err != nil {
				t.Fatal(err)
			}
			for range 25 {
				w.Tick()
			}
			data, err := w.ExportNativeGAM()
			if err != nil {
				t.Fatal(err)
			}
			loaded, err := ImportNativeGAM(b, data)
			if err != nil {
				t.Fatal(err)
			}
			for tick := 0; tick < 80; tick++ {
				w.Tick()
				loaded.Tick()
				if tick%10 == 0 {
					actual, err := loaded.ExportNativeGAM()
					if err != nil {
						t.Fatal(err)
					}
					expected, err := w.ExportNativeGAM()
					if err != nil {
						t.Fatal(err)
					}
					assertNativeGAMBytes(t, actual, expected)
				}
			}
		})
	}
}
