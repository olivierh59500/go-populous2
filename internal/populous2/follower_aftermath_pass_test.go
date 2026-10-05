package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestWorldAftermathCompleteFollowerPassAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_aftermath_pass_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			followerContextFixture
			D     [8]uint32
			Input struct {
				Name, Mode                 string
				Initial                    []nativeHeroPatch
				D                          [8]uint32
				D0, D1, D4, D5             uint32
				DefaultHeader, DefaultTile uint8
				OverrideTerrain            bool
			}
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 112 {
		t.Fatalf("whole aftermath pass corpus differs: %d cases", len(catalog.Cases))
	}
	b := testBundle(t)
	rules, err := DecodeNativeFollowerPassRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	aftermath, err := DecodeNativeFollowerAftermathFrameRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range catalog.Cases {
		input := fixture.followerContextFixture
		input.Input.Name, input.Input.Mode = fixture.Input.Name, fixture.Input.Mode
		input.Input.Initial = fixture.Input.Initial
		input.Input.D0, input.Input.D1, input.Input.D4, input.Input.D5 = fixture.Input.D0, fixture.Input.D1, fixture.Input.D4, fixture.Input.D5
		input.Input.DefaultHeader, input.Input.DefaultTile, input.Input.OverrideTerrain = fixture.Input.DefaultHeader, fixture.Input.DefaultTile, fixture.Input.OverrideTerrain
		m := followerContextMemory(input)
		raw := append([]byte(nil), m.bytes[:]...)
		w := installNativeFixtureWorld(t, raw, nil)
		w.NativeClock = binary.BigEndian.Uint32(raw[0xf40:])
		frame := NativeFrameRegisterContext{D: fixture.Input.D, AddressBase: 0x200000}
		frame.D[0], frame.D[1], frame.D[4], frame.D[5] = fixture.Input.D0, fixture.Input.D1, fixture.Input.D4, fixture.Input.D5
		state := NativeFollowerPassState{}
		w.nativeCallDepth++
		err := w.tickNativeFollowerFrame(&rules, &frame, &state, NativeFollowerFrameBindings{Aftermath: &aftermath})
		w.nativeCallDepth--
		if err != nil {
			t.Fatal(fixture.Input.Name, err)
		}
		if frame.D != fixture.D {
			t.Fatalf("%s whole-pass registers differ: %x / %x", fixture.Input.Name, frame.D, fixture.D)
		}
		all := nativeFixtureWorldImage(w, raw)
		word, _ := w.nativeCleanupMemory().Read16(0xdc2)
		binary.BigEndian.PutUint16(all[0xdc2:], word)
		if got := fmt.Sprintf("%x", sha256.Sum256(all)); got != fixture.Hash {
			t.Fatalf("%s whole-pass BSS differs: %s / %s", fixture.Input.Name, got, fixture.Hash)
		}
	}
}
