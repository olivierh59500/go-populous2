package populous2

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// These references execute the original combat and winner bodies together;
// they do not stop at $1298c or replay reference callback writes in Go.
func TestWorldCombatWinnerFullFrameAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_combat_win_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Cases []struct {
			followerCombatFrameFixture
			Input struct {
				Name, Mode string
				Initial    []nativeHeroPatch
				Registers  [8]uint32
				Seed       uint32
				Landscape  int
			}
			Hash string
		}
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Cases) != 3092 {
		t.Fatal("native combat/winner composition corpus incomplete")
	}
	b := testBundle(t)
	combat, err := DecodeFollowerCombatFrameRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	wins := 0
	for _, f := range catalog.Cases {
		prefix := followerCombatFrameFixture{}
		prefix.Input.Name, prefix.Input.Mode = f.Input.Name, f.Input.Mode
		prefix.Input.Initial, prefix.Input.Registers, prefix.Input.Seed = f.Input.Initial, f.Input.Registers, f.Input.Seed
		m := combatFrameFixtureMemory(prefix)
		w := installNativeFixtureWorld(t, m[:], nil)
		w.setNativeBirthBlockWord(0)
		w.FollowerWin, err = DecodeFollowerWinRules(b.Executable, b.Landscapes[f.Input.Landscape])
		if err != nil {
			t.Fatal(err)
		}
		frame := NativeFrameRegisterContext{D: f.Input.Registers, AddressBase: 0x200000}
		state := NativeFollowerPassState{}
		cb := w.nativeCombatFrameCallbacks(&frame, &state)
		cleanup := cb.CleanupFrame
		calls := 0
		observe := func(pc uint32, ref, winner, loser NativeRecordReference, c *NativeFrameRegisterContext) {
			t.Helper()
			if calls >= len(f.Calls) {
				t.Fatal(f.Input.Name, "unexpected child call")
			}
			want := f.Calls[calls]
			calls++
			if want.PC != pc || want.Reference != uint16(ref) || want.Winner != uint16(winner) || want.Loser != uint16(loser) || want.Registers != c.D {
				t.Fatalf("%s child%x context differs: got%x/%x/%x/%x want%+v", f.Input.Name, pc, ref, winner, loser, c.D, want)
			}
		}
		cb.CleanupFrame = func(ref NativeRecordReference, c *NativeFrameRegisterContext) error {
			observe(0x124a2, ref, 0, 0, c)
			return cleanup(ref, c)
		}
		cb.WinFrame = func(winner, loser, original NativeRecordReference, c *NativeFrameRegisterContext) (NativeRecordReference, error) {
			observe(0x1298c, 0, winner, loser, c)
			wins++
			winCB := w.nativeWinnerFrameCallbacks()
			winCleanup := winCB.Cleanup
			winCB.Cleanup = func(ref NativeRecordReference, c *NativeFrameRegisterContext) error {
				observe(0x124a2, ref, 0, 0, c)
				return winCleanup(ref, c)
			}
			step, err := w.FollowerWin.WinWithFrame(winner, loser, original, c, winCB)
			return step.ReturnedA3, err
		}
		w.nativeCallDepth++
		var step FollowerCombatFrameStep
		if f.Input.Mode == "defender" {
			step, err = combat.Defender(52, cb)
		} else {
			step, err = combat.Aggressor(52, cb)
		}
		w.nativeCallDepth--
		if err != nil {
			t.Fatal(f.Input.Name, err)
		}
		if calls != len(f.Calls) || step.Boundary != f.Boundary || state.MinimapVariant != f.MinimapVariant || frame.D != f.Registers {
			t.Fatalf("%s combat/winner continuation differs: boundary%x/%x D%x/%x calls%d/%d variant%d/%d", f.Input.Name, step.Boundary, f.Boundary, frame.D, f.Registers, calls, len(f.Calls), state.MinimapVariant, f.MinimapVariant)
		}
		all := nativeFixtureWorldImage(w, m[:])
		if got := fmt.Sprintf("%x", sha256.Sum256(all)); got != f.Hash {
			t.Fatalf("%s combat/winner retained BSS differs: %s/%s", f.Input.Name, got, f.Hash)
		}
	}
	if wins != 1120 {
		t.Fatalf("native complete winner call coverage%d", wins)
	}
}
