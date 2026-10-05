package populous2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type winFrameFixture struct {
	nativeWinFixture
	Input struct {
		nativeWinInput
		D         [8]uint32
		Landscape int
	}
	D          [8]uint32
	ReturnedA3 uint16
}

func TestWorldNativeWinnerFrameAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_win_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []winFrameFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 1044 {
		t.Fatal("Worldwinner full-context corpus incomplete")
	}
	for _, fixture := range fixtures {
		rules, err := DecodeFollowerWinRules(testBundle(t).Executable, testBundle(t).Landscapes[fixture.Input.Landscape])
		if err != nil {
			t.Fatal(err)
		}
		m := winFixtureMemory(fixture.Input.nativeWinInput)
		raw := cleanupFixtureBytes(m)
		raw = append(raw, make([]byte, 0xeb90-len(raw))...)
		w := installNativeFixtureWorld(t, raw, nil)
		w.Landscape = testBundle(t).Landscapes[fixture.Input.Landscape]
		w.setNativeBirthBlockWord(binary.BigEndian.Uint16(raw[0xdc2:]))
		w.Core.GameTurn = 123
		frame := NativeFrameRegisterContext{D: fixture.Input.D, AddressBase: 0x200000}
		original := NativeRecordReference(52)
		if fixture.Input.OriginalLoser {
			original = 104
		}
		w.nativeCallDepth++
		step, err := rules.WinWithFrame(52, 104, original, &frame, w.nativeWinnerFrameCallbacks())
		w.nativeCallDepth--
		if err != nil {
			t.Fatal(fixture.Input.Name, err)
		}
		if frame.D != fixture.D || uint16(step.ReturnedA3) != fixture.ReturnedA3 {
			t.Fatalf("World%s winner continuation differs: D%x/%x A3%x/%x", fixture.Input.Name, frame.D, fixture.D, step.ReturnedA3, fixture.ReturnedA3)
		}
		all := nativeFixtureWorldImage(w, raw)
		binary.BigEndian.PutUint16(all[0xdc2:], w.nativeBirthBlockWord())
		if got := fmt.Sprintf("%x", sha256.Sum256(all[:0xeb18])); got != fixture.Hash {
			t.Fatalf("World%s winner BSS differs: %s/%s", fixture.Input.Name, got, fixture.Hash)
		}
	}
}

func TestNativeWinnerFullFrameAgainstOriginalCPU(t *testing.T) {
	data, err := os.ReadFile("testdata/follower_win_frame_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []winFrameFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 1044 {
		t.Fatal("native winner full-context corpus incomplete")
	}
	b := testBundle(t)
	town, err := DecodeNativeTownEvaluator(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	combat, err := DecodeTownCombatRules(b.Executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		rules, err := DecodeFollowerWinRules(testBundle(t).Executable, testBundle(t).Landscapes[fixture.Input.Landscape])
		if err != nil {
			t.Fatal(err)
		}
		m := winFixtureMemory(fixture.Input.nativeWinInput)
		memory := FollowerCleanupMemory{Read8: m.read8, Read16: m.read16, Read32: m.read32, Write8: m.write8, Write16: m.write16, Write32: m.write32}
		frame := NativeFrameRegisterContext{D: fixture.Input.D, AddressBase: 0x200000}
		clear := func(ref NativeRecordReference, tile uint8) error {
			return town.ClearFarms(ref, tile, winTownCallbacks(m))
		}
		cleanup := func(ref NativeRecordReference, mode uint16) error {
			_, err := CleanupFollower(ref, FollowerCleanupRegisters{D0: uint32(mode)}, FollowerCleanupCallbacks{Memory: memory, Unlink: m.unlink, Insert: m.insert, ClearFarms: clear})
			return err
		}
		townCB := TownCombatCallbacks{Read: func(ref NativeRecordReference) (FollowerEntryActor, error) { return winEntryRead(m, ref) }, Write: func(ref NativeRecordReference, a FollowerEntryActor) error { return winEntryWrite(m, ref, a) }, ClearFarms: clear, Cleanup: cleanup, EvaluateTown: func(ref NativeRecordReference) (int, error) {
			stage, err := town.Evaluate(ref, 123, winTownCallbacks(m))
			return int(stage), err
		}}
		cb := NativeFollowerWinFrameCallbacks{Memory: memory, ClearFarms: clear,
			Cleanup: func(ref NativeRecordReference, c *NativeFrameRegisterContext) error {
				_, err := CleanupFollowerWithFrame(ref, c, FollowerCleanupCallbacks{Memory: memory, Unlink: m.unlink, Insert: m.insert, ClearFarms: clear})
				return err
			},
			ReformTown: func(ref, original NativeRecordReference) error {
				_, err := combat.Reform(ref, original, 123, townCB)
				return err
			},
			DestroyTown: func(ref NativeRecordReference) error { _, err := combat.Destroy(ref, townCB); return err },
			PoolBlocked: func() bool { return m.word(0xdc2) != 0 }, Insert: m.insert,
		}
		original := NativeRecordReference(52)
		if fixture.Input.OriginalLoser {
			original = 104
		}
		step, err := rules.WinWithFrame(52, 104, original, &frame, cb)
		if err != nil {
			t.Fatal(fixture.Input.Name, err)
		}
		if frame.D != fixture.D || uint16(step.ReturnedA3) != fixture.ReturnedA3 {
			t.Fatalf("%s winner continuation differs: D%x/%x A3%x/%x", fixture.Input.Name, frame.D, fixture.D, step.ReturnedA3, fixture.ReturnedA3)
		}
		image := cleanupFixtureBytes(m)
		if got := fmt.Sprintf("%x", sha256.Sum256(image)); got != fixture.Hash {
			t.Fatalf("%s winner BSS differs: %s/%s", fixture.Input.Name, got, fixture.Hash)
		}
	}
}
