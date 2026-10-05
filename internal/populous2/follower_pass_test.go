package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestNativeFollowerPassBoundariesAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_pass_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			N                         int
			Initial                   []nativeHeroChange
			InputD, D                 [8]uint32
			Hash                      string
			Prepasses, Bodies, Points int
			Results                   []uint16
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 96 {
		t.Fatal("native follower pass boundary corpus incomplete")
	}
	rules, err := DecodeNativeFollowerPassRules(testBundle(t).Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Cases {
		raw := make([]byte, 0x11280)
		for _, change := range fixture.Initial {
			raw[change.Address] = change.Value
		}
		memory := commandNativeMemory(raw)
		frame := NativeFrameRegisterContext{D: fixture.InputD, AddressBase: 0x200000}
		state := NativeFollowerPassState{TownCacheFlag: 0xabcd, MinimapVariant: 0xabcd}
		prepasses, bodies, points := 0, 0, 0
		results := []uint16{}
		seen := map[NativeRecordReference]int{}
		cb := NativeFollowerPassCallbacks{Memory: memory, Frame: &frame, State: &state,
			Prepass: func(_ NativeRecordReference, frame *NativeFrameRegisterContext, state *NativeFollowerPassState) error {
				if state.TownCacheFlag != 0 || state.MinimapVariant != 0 {
					return fmt.Errorf("native active dispatch did not reset CODE scratch")
				}
				prepasses++
				frame.Word(1, uint16(prepasses))
				return nil
			},
			Body: func(ref NativeRecordReference, target uint16, frame *NativeFrameRegisterContext, _ *NativeFollowerPassState) (NativeFollowerPassFlow, error) {
				bodies++
				slot := int(ref) / 52
				seen[ref]++
				actorState := raw[cleanupRecordAddress(ref)+22]
				if target != rules.Targets[actorState/2] {
					return 0, fmt.Errorf("follower dispatch table target differs")
				}
				frame.D[4] = uint32(slot + int(actorState))
				frame.Word(5, uint16(bodies))
				if fixture.N%4 == 1 && seen[ref] == 1 {
					raw[cleanupRecordAddress(ref)+22] = 4
					return NativeFollowerRedispatch, nil
				}
				if fixture.N%4 == 2 && slot%2 == 1 {
					return NativeFollowerNext, nil
				}
				return NativeFollowerCount, nil
			},
			MapPoint: func(marker uint16, frame *NativeFrameRegisterContext) error {
				points++
				if uint16(frame.D[2]) != marker {
					return fmt.Errorf("map callback descriptor context differs")
				}
				frame.D[6] = uint32(points)
				return nil
			},
			Result: func(identity uint16, frame *NativeFrameRegisterContext) error {
				results = append(results, identity)
				frame.D[3] = 0xfeed0000 | uint32(len(results))
				return nil
			},
		}
		if err := rules.Tick(cb); err != nil {
			t.Fatal(fixture.N, err)
		}
		if frame.D != fixture.D || prepasses != fixture.Prepasses || bodies != fixture.Bodies || points != fixture.Points || !reflect.DeepEqual(results, fixture.Results) {
			t.Fatalf("pass%d boundaries differ: D%x/%x counts%d/%d/%d results%v", fixture.N, frame.D, fixture.D, prepasses, bodies, points, results)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(raw[0xdc2:])); got != fixture.Hash {
			t.Fatalf("pass%d BSS differs: %s / %s", fixture.N, got, fixture.Hash)
		}
	}
}
